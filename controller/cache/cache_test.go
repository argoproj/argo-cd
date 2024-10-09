package cache

import (
	"context"
	"errors"
	"net"
	"net/url"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/cache/mocks"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/mock"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/controller/metrics"
	"github.com/argoproj/argo-cd/v2/controller/sharding"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	dbmocks "github.com/argoproj/argo-cd/v2/util/db/mocks"
	argosettings "github.com/argoproj/argo-cd/v2/util/settings"
)

type netError string

func (n netError) Error() string   { return string(n) }
func (n netError) Timeout() bool   { return false }
func (n netError) Temporary() bool { return false }

func TestHandleModEvent_HasChanges(t *testing.T) {
	clusterCache := &mocks.ClusterCache{}
	clusterCache.On("Invalidate", mock.Anything, mock.Anything).Return(nil).Once()
	clusterCache.On("EnsureSynced").Return(nil).Once()
	db := &dbmocks.ArgoDB{}
	db.On("GetApplicationControllerReplicas").Return(1)
	clustersCache := liveStateCache{
		clusters: map[string]cache.ClusterCache{
			"https://mycluster": clusterCache,
		},
		clusterSharding: sharding.NewClusterSharding(db, 0, 1, common.DefaultShardingAlgorithm),
	}

	clustersCache.handleModEvent(&appv1.Cluster{
		Server: "https://mycluster",
		Config: appv1.ClusterConfig{Username: "foo"},
	}, &appv1.Cluster{
		Server:     "https://mycluster",
		Config:     appv1.ClusterConfig{Username: "bar"},
		Namespaces: []string{"default"},
	})
}

func TestHandleModEvent_ClusterExcluded(t *testing.T) {
	clusterCache := &mocks.ClusterCache{}
	clusterCache.On("Invalidate", mock.Anything, mock.Anything).Return(nil).Once()
	clusterCache.On("EnsureSynced").Return(nil).Once()
	db := &dbmocks.ArgoDB{}
	db.On("GetApplicationControllerReplicas").Return(1)
	clustersCache := liveStateCache{
		db:          nil,
		appInformer: nil,
		onObjectUpdated: func(managedByApp map[string]bool, ref v1.ObjectReference) {
		},
		kubectl:       nil,
		settingsMgr:   &argosettings.SettingsManager{},
		metricsServer: &metrics.MetricsServer{},
		// returns a shard that never process any cluster
		clusterSharding:  sharding.NewClusterSharding(db, 0, 1, common.DefaultShardingAlgorithm),
		resourceTracking: nil,
		clusters:         map[string]cache.ClusterCache{"https://mycluster": clusterCache},
		cacheSettings:    cacheSettings{},
		lock:             sync.RWMutex{},
	}

	clustersCache.handleModEvent(&appv1.Cluster{
		Server: "https://mycluster",
		Config: appv1.ClusterConfig{Username: "foo"},
	}, &appv1.Cluster{
		Server:     "https://mycluster",
		Config:     appv1.ClusterConfig{Username: "bar"},
		Namespaces: []string{"default"},
	})

	assert.Len(t, clustersCache.clusters, 1)
}

func TestHandleModEvent_NoChanges(t *testing.T) {
	clusterCache := &mocks.ClusterCache{}
	clusterCache.On("Invalidate", mock.Anything).Panic("should not invalidate")
	clusterCache.On("EnsureSynced").Return(nil).Panic("should not re-sync")
	db := &dbmocks.ArgoDB{}
	db.On("GetApplicationControllerReplicas").Return(1)
	clustersCache := liveStateCache{
		clusters: map[string]cache.ClusterCache{
			"https://mycluster": clusterCache,
		},
		clusterSharding: sharding.NewClusterSharding(db, 0, 1, common.DefaultShardingAlgorithm),
	}

	clustersCache.handleModEvent(&appv1.Cluster{
		Server: "https://mycluster",
		Config: appv1.ClusterConfig{Username: "bar"},
	}, &appv1.Cluster{
		Server: "https://mycluster",
		Config: appv1.ClusterConfig{Username: "bar"},
	})
}

func TestHandleAddEvent_ClusterExcluded(t *testing.T) {
	db := &dbmocks.ArgoDB{}
	db.On("GetApplicationControllerReplicas").Return(1)
	clustersCache := liveStateCache{
		clusters:        map[string]cache.ClusterCache{},
		clusterSharding: sharding.NewClusterSharding(db, 0, 2, common.DefaultShardingAlgorithm),
	}
	clustersCache.handleAddEvent(&appv1.Cluster{
		Server: "https://mycluster",
		Config: appv1.ClusterConfig{Username: "bar"},
	})

	assert.Empty(t, clustersCache.clusters)
}

func TestHandleDeleteEvent_CacheDeadlock(t *testing.T) {
	testCluster := &appv1.Cluster{
		Server: "https://mycluster",
		Config: appv1.ClusterConfig{Username: "bar"},
	}
	db := &dbmocks.ArgoDB{}
	db.On("GetApplicationControllerReplicas").Return(1)
	fakeClient := fake.NewSimpleClientset()
	settingsMgr := argosettings.NewSettingsManager(context.TODO(), fakeClient, "argocd")
	liveStateCacheLock := sync.RWMutex{}
	gitopsEngineClusterCache := &mocks.ClusterCache{}
	clustersCache := liveStateCache{
		clusters: map[string]cache.ClusterCache{
			testCluster.Server: gitopsEngineClusterCache,
		},
		clusterSharding: sharding.NewClusterSharding(db, 0, 1, common.DefaultShardingAlgorithm),
		settingsMgr:     settingsMgr,
		// Set the lock here so we can reference it later
		// nolint We need to overwrite here to have access to the lock
		lock: liveStateCacheLock,
	}
	channel := make(chan string)
	// Mocked lock held by the gitops-engine cluster cache
	gitopsEngineClusterCacheLock := sync.Mutex{}
	// Ensure completion of both EnsureSynced and Invalidate
	ensureSyncedCompleted := sync.Mutex{}
	invalidateCompleted := sync.Mutex{}
	// Locks to force trigger condition during test
	// Condition order:
	//   EnsuredSynced -> Locks gitops-engine
	//   handleDeleteEvent -> Locks liveStateCache
	//   EnsureSynced via sync, newResource, populateResourceInfoHandler -> attempts to Lock liveStateCache
	//   handleDeleteEvent via cluster.Invalidate -> attempts to Lock gitops-engine
	handleDeleteWasCalled := sync.Mutex{}
	engineHoldsEngineLock := sync.Mutex{}
	ensureSyncedCompleted.Lock()
	invalidateCompleted.Lock()
	handleDeleteWasCalled.Lock()
	engineHoldsEngineLock.Lock()

	gitopsEngineClusterCache.On("EnsureSynced").Run(func(args mock.Arguments) {
		gitopsEngineClusterCacheLock.Lock()
		t.Log("EnsureSynced: Engine has engine lock")
		engineHoldsEngineLock.Unlock()
		defer gitopsEngineClusterCacheLock.Unlock()
		// Wait until handleDeleteEvent holds the liveStateCache lock
		handleDeleteWasCalled.Lock()
		// Try and obtain the liveStateCache lock
		clustersCache.lock.Lock()
		t.Log("EnsureSynced: Engine has LiveStateCache lock")
		clustersCache.lock.Unlock()
		ensureSyncedCompleted.Unlock()
	}).Return(nil).Once()

	gitopsEngineClusterCache.On("Invalidate").Run(func(args mock.Arguments) {
		// Allow EnsureSynced to continue now that we're in the deadlock condition
		handleDeleteWasCalled.Unlock()
		// Wait until gitops engine holds the gitops lock
		// This prevents timing issues if we reach this point before EnsureSynced has obtained the lock
		engineHoldsEngineLock.Lock()
		t.Log("Invalidate: Engine has engine lock")
		engineHoldsEngineLock.Unlock()
		// Lock engine lock
		gitopsEngineClusterCacheLock.Lock()
		t.Log("Invalidate: Invalidate has engine lock")
		gitopsEngineClusterCacheLock.Unlock()
		invalidateCompleted.Unlock()
	}).Return()
	go func() {
		// Start the gitops-engine lock holds
		go func() {
			err := gitopsEngineClusterCache.EnsureSynced()
			if err != nil {
				assert.Fail(t, err.Error())
			}
		}()
		// Run in background
		go clustersCache.handleDeleteEvent(testCluster.Server)
		// Allow execution to continue on clusters cache call to trigger lock
		ensureSyncedCompleted.Lock()
		invalidateCompleted.Lock()
		t.Log("Competing functions were able to obtain locks")
		invalidateCompleted.Unlock()
		ensureSyncedCompleted.Unlock()
		channel <- "PASSED"
	}()
	select {
	case str := <-channel:
		assert.Equal(t, "PASSED", str, str)
	case <-time.After(5 * time.Second):
		assert.Fail(t, "Ended up in deadlock")
	}
}

func TestIsRetryableError(t *testing.T) {
	var (
		tlsHandshakeTimeoutErr net.Error = netError("net/http: TLS handshake timeout")
		ioTimeoutErr           net.Error = netError("i/o timeout")
		connectionTimedout     net.Error = netError("connection timed out")
		connectionReset        net.Error = netError("connection reset by peer")
	)
	t.Run("Nil", func(t *testing.T) {
		assert.False(t, isRetryableError(nil))
	})
	t.Run("ResourceQuotaConflictErr", func(t *testing.T) {
		assert.False(t, isRetryableError(apierr.NewConflict(schema.GroupResource{}, "", nil)))
		assert.True(t, isRetryableError(apierr.NewConflict(schema.GroupResource{Group: "v1", Resource: "resourcequotas"}, "", nil)))
	})
	t.Run("ExceededQuotaErr", func(t *testing.T) {
		assert.False(t, isRetryableError(apierr.NewForbidden(schema.GroupResource{}, "", nil)))
		assert.True(t, isRetryableError(apierr.NewForbidden(schema.GroupResource{Group: "v1", Resource: "pods"}, "", errors.New("exceeded quota"))))
	})
	t.Run("TooManyRequestsDNS", func(t *testing.T) {
		assert.True(t, isRetryableError(apierr.NewTooManyRequests("", 0)))
	})
	t.Run("DNSError", func(t *testing.T) {
		assert.True(t, isRetryableError(&net.DNSError{}))
	})
	t.Run("OpError", func(t *testing.T) {
		assert.True(t, isRetryableError(&net.OpError{}))
	})
	t.Run("UnknownNetworkError", func(t *testing.T) {
		assert.True(t, isRetryableError(net.UnknownNetworkError("")))
	})
	t.Run("ConnectionClosedErr", func(t *testing.T) {
		assert.False(t, isRetryableError(&url.Error{Err: errors.New("")}))
		assert.True(t, isRetryableError(&url.Error{Err: errors.New("Connection closed by foreign host")}))
	})
	t.Run("TLSHandshakeTimeout", func(t *testing.T) {
		assert.True(t, isRetryableError(tlsHandshakeTimeoutErr))
	})
	t.Run("IOHandshakeTimeout", func(t *testing.T) {
		assert.True(t, isRetryableError(ioTimeoutErr))
	})
	t.Run("ConnectionTimeout", func(t *testing.T) {
		assert.True(t, isRetryableError(connectionTimedout))
	})
	t.Run("ConnectionReset", func(t *testing.T) {
		assert.True(t, isRetryableError(connectionReset))
	})
}

func Test_asResourceNode_owner_refs(t *testing.T) {
	resNode := asResourceNode(&cache.Resource{
		ResourceVersion: "",
		Ref: v1.ObjectReference{
			APIVersion: "v1",
		},
		OwnerRefs: []metav1.OwnerReference{
			{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       "cm-1",
			},
			{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       "cm-2",
			},
		},
		CreationTimestamp: nil,
		Info:              nil,
		Resource:          nil,
	})
	expected := appv1.ResourceNode{
		ResourceRef: appv1.ResourceRef{
			Version: "v1",
		},
		ParentRefs: []appv1.ResourceRef{
			{
				Group: "",
				Kind:  "ConfigMap",
				Name:  "cm-1",
			},
			{
				Group: "",
				Kind:  "ConfigMap",
				Name:  "cm-2",
			},
		},
		Info:            nil,
		NetworkingInfo:  nil,
		ResourceVersion: "",
		Images:          nil,
		Health:          nil,
		CreatedAt:       nil,
	}
	assert.Equal(t, expected, resNode)
}

func Test_getAppRecursive(t *testing.T) {
	for _, tt := range []struct {
		name     string
		r        *cache.Resource
		ns       map[kube.ResourceKey]*cache.Resource
		wantName string
		wantOK   assert.BoolAssertionFunc
	}{
		{
			name: "ok: cm1->app1",
			r: &cache.Resource{
				Ref: v1.ObjectReference{
					Name: "cm1",
				},
				OwnerRefs: []metav1.OwnerReference{
					{Name: "app1"},
				},
			},
			ns: map[kube.ResourceKey]*cache.Resource{
				kube.NewResourceKey("", "", "", "app1"): {
					Info: &ResourceInfo{
						AppName: "app1",
					},
				},
			},
			wantName: "app1",
			wantOK:   assert.True,
		},
		{
			name: "ok: cm1->cm2->app1",
			r: &cache.Resource{
				Ref: v1.ObjectReference{
					Name: "cm1",
				},
				OwnerRefs: []metav1.OwnerReference{
					{Name: "cm2"},
				},
			},
			ns: map[kube.ResourceKey]*cache.Resource{
				kube.NewResourceKey("", "", "", "cm2"): {
					Ref: v1.ObjectReference{
						Name: "cm2",
					},
					OwnerRefs: []metav1.OwnerReference{
						{Name: "app1"},
					},
				},
				kube.NewResourceKey("", "", "", "app1"): {
					Info: &ResourceInfo{
						AppName: "app1",
					},
				},
			},
			wantName: "app1",
			wantOK:   assert.True,
		},
		{
			name: "cm1->cm2->app1 & cm1->cm3->app1",
			r: &cache.Resource{
				Ref: v1.ObjectReference{
					Name: "cm1",
				},
				OwnerRefs: []metav1.OwnerReference{
					{Name: "cm2"},
					{Name: "cm3"},
				},
			},
			ns: map[kube.ResourceKey]*cache.Resource{
				kube.NewResourceKey("", "", "", "cm2"): {
					Ref: v1.ObjectReference{
						Name: "cm2",
					},
					OwnerRefs: []metav1.OwnerReference{
						{Name: "app1"},
					},
				},
				kube.NewResourceKey("", "", "", "cm3"): {
					Ref: v1.ObjectReference{
						Name: "cm3",
					},
					OwnerRefs: []metav1.OwnerReference{
						{Name: "app1"},
					},
				},
				kube.NewResourceKey("", "", "", "app1"): {
					Info: &ResourceInfo{
						AppName: "app1",
					},
				},
			},
			wantName: "app1",
			wantOK:   assert.True,
		},
		{
			// Nothing cycle.
			// Issue #11699, fixed #12667.
			name: "ok: cm1->cm2 & cm1->cm3->cm2 & cm1->cm3->app1",
			r: &cache.Resource{
				Ref: v1.ObjectReference{
					Name: "cm1",
				},
				OwnerRefs: []metav1.OwnerReference{
					{Name: "cm2"},
					{Name: "cm3"},
				},
			},
			ns: map[kube.ResourceKey]*cache.Resource{
				kube.NewResourceKey("", "", "", "cm2"): {
					Ref: v1.ObjectReference{
						Name: "cm2",
					},
				},
				kube.NewResourceKey("", "", "", "cm3"): {
					Ref: v1.ObjectReference{
						Name: "cm3",
					},
					OwnerRefs: []metav1.OwnerReference{
						{Name: "cm2"},
						{Name: "app1"},
					},
				},
				kube.NewResourceKey("", "", "", "app1"): {
					Info: &ResourceInfo{
						AppName: "app1",
					},
				},
			},
			wantName: "app1",
			wantOK:   assert.True,
		},
		{
			name: "cycle: cm1<->cm2",
			r: &cache.Resource{
				Ref: v1.ObjectReference{
					Name: "cm1",
				},
				OwnerRefs: []metav1.OwnerReference{
					{Name: "cm2"},
				},
			},
			ns: map[kube.ResourceKey]*cache.Resource{
				kube.NewResourceKey("", "", "", "cm1"): {
					Ref: v1.ObjectReference{
						Name: "cm1",
					},
					OwnerRefs: []metav1.OwnerReference{
						{Name: "cm2"},
					},
				},
				kube.NewResourceKey("", "", "", "cm2"): {
					Ref: v1.ObjectReference{
						Name: "cm2",
					},
					OwnerRefs: []metav1.OwnerReference{
						{Name: "cm1"},
					},
				},
			},
			wantName: "",
			wantOK:   assert.False,
		},
		{
			name: "cycle: cm1->cm2->cm3->cm1",
			r: &cache.Resource{
				Ref: v1.ObjectReference{
					Name: "cm1",
				},
				OwnerRefs: []metav1.OwnerReference{
					{Name: "cm2"},
				},
			},
			ns: map[kube.ResourceKey]*cache.Resource{
				kube.NewResourceKey("", "", "", "cm1"): {
					Ref: v1.ObjectReference{
						Name: "cm1",
					},
					OwnerRefs: []metav1.OwnerReference{
						{Name: "cm2"},
					},
				},
				kube.NewResourceKey("", "", "", "cm2"): {
					Ref: v1.ObjectReference{
						Name: "cm2",
					},
					OwnerRefs: []metav1.OwnerReference{
						{Name: "cm3"},
					},
				},
				kube.NewResourceKey("", "", "", "cm3"): {
					Ref: v1.ObjectReference{
						Name: "cm3",
					},
					OwnerRefs: []metav1.OwnerReference{
						{Name: "cm1"},
					},
				},
			},
			wantName: "",
			wantOK:   assert.False,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			visited := map[kube.ResourceKey]bool{}
			got, ok := getAppRecursive(tt.r, tt.ns, visited)
			assert.Equal(t, tt.wantName, got)
			tt.wantOK(t, ok)
		})
	}
}

func TestSkipResourceUpdate(t *testing.T) {
	var (
		hash1_x string = "x"
		hash2_y string = "y"
		hash3_x string = "x"
	)
	info := &ResourceInfo{
		manifestHash: hash1_x,
		Health: &health.HealthStatus{
			Status:  health.HealthStatusHealthy,
			Message: "default",
		},
	}
	t.Run("Nil", func(t *testing.T) {
		assert.False(t, skipResourceUpdate(nil, nil))
	})
	t.Run("From Nil", func(t *testing.T) {
		assert.False(t, skipResourceUpdate(nil, info))
	})
	t.Run("To Nil", func(t *testing.T) {
		assert.False(t, skipResourceUpdate(info, nil))
	})
	t.Run("No hash", func(t *testing.T) {
		assert.False(t, skipResourceUpdate(&ResourceInfo{}, &ResourceInfo{}))
	})
	t.Run("Same hash", func(t *testing.T) {
		assert.True(t, skipResourceUpdate(&ResourceInfo{
			manifestHash: hash1_x,
		}, &ResourceInfo{
			manifestHash: hash1_x,
		}))
	})
	t.Run("Same hash value", func(t *testing.T) {
		assert.True(t, skipResourceUpdate(&ResourceInfo{
			manifestHash: hash1_x,
		}, &ResourceInfo{
			manifestHash: hash3_x,
		}))
	})
	t.Run("Different hash value", func(t *testing.T) {
		assert.False(t, skipResourceUpdate(&ResourceInfo{
			manifestHash: hash1_x,
		}, &ResourceInfo{
			manifestHash: hash2_y,
		}))
	})
	t.Run("Same hash, empty health", func(t *testing.T) {
		assert.True(t, skipResourceUpdate(&ResourceInfo{
			manifestHash: hash1_x,
			Health:       &health.HealthStatus{},
		}, &ResourceInfo{
			manifestHash: hash3_x,
			Health:       &health.HealthStatus{},
		}))
	})
	t.Run("Same hash, old health", func(t *testing.T) {
		assert.False(t, skipResourceUpdate(&ResourceInfo{
			manifestHash: hash1_x,
			Health: &health.HealthStatus{
				Status: health.HealthStatusHealthy,
			},
		}, &ResourceInfo{
			manifestHash: hash3_x,
			Health:       nil,
		}))
	})
	t.Run("Same hash, new health", func(t *testing.T) {
		assert.False(t, skipResourceUpdate(&ResourceInfo{
			manifestHash: hash1_x,
			Health:       &health.HealthStatus{},
		}, &ResourceInfo{
			manifestHash: hash3_x,
			Health: &health.HealthStatus{
				Status: health.HealthStatusHealthy,
			},
		}))
	})
	t.Run("Same hash, same health", func(t *testing.T) {
		assert.True(t, skipResourceUpdate(&ResourceInfo{
			manifestHash: hash1_x,
			Health: &health.HealthStatus{
				Status:  health.HealthStatusHealthy,
				Message: "same",
			},
		}, &ResourceInfo{
			manifestHash: hash3_x,
			Health: &health.HealthStatus{
				Status:  health.HealthStatusHealthy,
				Message: "same",
			},
		}))
	})
	t.Run("Same hash, different health status", func(t *testing.T) {
		assert.False(t, skipResourceUpdate(&ResourceInfo{
			manifestHash: hash1_x,
			Health: &health.HealthStatus{
				Status:  health.HealthStatusHealthy,
				Message: "same",
			},
		}, &ResourceInfo{
			manifestHash: hash3_x,
			Health: &health.HealthStatus{
				Status:  health.HealthStatusDegraded,
				Message: "same",
			},
		}))
	})
	t.Run("Same hash, different health message", func(t *testing.T) {
		assert.True(t, skipResourceUpdate(&ResourceInfo{
			manifestHash: hash1_x,
			Health: &health.HealthStatus{
				Status:  health.HealthStatusHealthy,
				Message: "same",
			},
		}, &ResourceInfo{
			manifestHash: hash3_x,
			Health: &health.HealthStatus{
				Status:  health.HealthStatusHealthy,
				Message: "different",
			},
		}))
	})
}

func TestShouldHashManifest(t *testing.T) {
	tests := []struct {
		name        string
		appName     string
		gvk         schema.GroupVersionKind
		un          *unstructured.Unstructured
		annotations map[string]string
		want        bool
	}{
		{
			name:    "appName not empty gvk matches",
			appName: "MyApp",
			gvk:     schema.GroupVersionKind{Group: application.Group, Kind: application.ApplicationKind},
			un:      &unstructured.Unstructured{},
			want:    true,
		},
		{
			name:    "appName empty",
			appName: "",
			gvk:     schema.GroupVersionKind{Group: application.Group, Kind: application.ApplicationKind},
			un:      &unstructured.Unstructured{},
			want:    true,
		},
		{
			name:    "appName empty group not match",
			appName: "",
			gvk:     schema.GroupVersionKind{Group: "group1", Kind: application.ApplicationKind},
			un:      &unstructured.Unstructured{},
			want:    false,
		},
		{
			name:    "appName empty kind not match",
			appName: "",
			gvk:     schema.GroupVersionKind{Group: application.Group, Kind: "kind1"},
			un:      &unstructured.Unstructured{},
			want:    false,
		},
		{
			name:        "argocd.argoproj.io/ignore-resource-updates=true",
			appName:     "",
			gvk:         schema.GroupVersionKind{Group: application.Group, Kind: "kind1"},
			un:          &unstructured.Unstructured{},
			annotations: map[string]string{"argocd.argoproj.io/ignore-resource-updates": "true"},
			want:        true,
		},
		{
			name:        "argocd.argoproj.io/ignore-resource-updates=invalid",
			appName:     "",
			gvk:         schema.GroupVersionKind{Group: application.Group, Kind: "kind1"},
			un:          &unstructured.Unstructured{},
			annotations: map[string]string{"argocd.argoproj.io/ignore-resource-updates": "invalid"},
			want:        false,
		},
		{
			name:        "argocd.argoproj.io/ignore-resource-updates=false",
			appName:     "",
			gvk:         schema.GroupVersionKind{Group: application.Group, Kind: "kind1"},
			un:          &unstructured.Unstructured{},
			annotations: map[string]string{"argocd.argoproj.io/ignore-resource-updates": "false"},
			want:        false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.annotations != nil {
				test.un.SetAnnotations(test.annotations)
			}
			got := shouldHashManifest(test.appName, test.gvk, test.un)
			if test.want != got {
				t.Fatalf("test=%v want %v got %v", test.name, test.want, got)
			}
		})
	}
}
