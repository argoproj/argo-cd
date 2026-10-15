package cache

import (
	"context"
	"errors"
	"net"
	"net/url"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/cache"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/cache/mocks"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	"github.com/stretchr/testify/mock"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/controller/metrics"
	"github.com/argoproj/argo-cd/v3/controller/sharding"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	dbmocks "github.com/argoproj/argo-cd/v3/util/db/mocks"
	argosettings "github.com/argoproj/argo-cd/v3/util/settings"
)

type netError string

func (n netError) Error() string   { return string(n) }
func (n netError) Timeout() bool   { return false }
func (n netError) Temporary() bool { return false }

func fixtures(ctx context.Context, data map[string]string, opts ...func(secret *corev1.Secret)) (*fake.Clientset, *argosettings.SettingsManager) {
	cm := &corev1.ConfigMap{
		Name:      common.ArgoCDConfigMapName,
		Namespace: "default",
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "argocd",
		},
		Data: data,
	}
	secret := &corev1.Secret{
		Name:      common.ArgoCDSecretName,
		Namespace: "default",
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "argocd",
		},
		Data: map[string][]byte{},
	}
	for i := range opts {
		opts[i](secret)
	}
	kubeClient := fake.NewClientset(cm, secret)
	settingsManager := argosettings.NewSettingsManager(ctx, kubeClient, "default")

	return kubeClient, settingsManager
}

func TestHandleModEvent_HasChanges(_ *testing.T) {
	clusterCache := &mocks.ClusterCache{}
	clusterCache.EXPECT().Invalidate(mock.Anything, mock.Anything).Return().Once()
	clusterCache.EXPECT().EnsureSynced().Return(nil).Once()
	db := &dbmocks.ArgoDB{}
	db.EXPECT().GetApplicationControllerReplicas().Return(1)
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
	t.Parallel()
	clusterCache := &mocks.ClusterCache{}
	clusterCache.EXPECT().Invalidate(mock.Anything, mock.Anything).Return().Once()
	clusterCache.EXPECT().EnsureSynced().Return(nil).Once()
	db := &dbmocks.ArgoDB{}
	db.EXPECT().GetApplicationControllerReplicas().Return(1).Maybe()
	clustersCache := liveStateCache{
		db:          nil,
		appInformer: nil,
		onObjectUpdated: func(_ map[string]bool, _ corev1.ObjectReference) {
		},
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

func TestHandleModEvent_NoChanges(_ *testing.T) {
	clusterCache := &mocks.ClusterCache{}
	clusterCache.EXPECT().Invalidate(mock.Anything).Panic("should not invalidate").Maybe()
	clusterCache.EXPECT().EnsureSynced().Return(nil).Panic("should not re-sync").Maybe()
	db := &dbmocks.ArgoDB{}
	db.EXPECT().GetApplicationControllerReplicas().Return(1).Maybe()
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
	t.Parallel()
	db := &dbmocks.ArgoDB{}
	db.EXPECT().GetApplicationControllerReplicas().Return(1).Maybe()
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
	t.Parallel()
	testCluster := &appv1.Cluster{
		Server: "https://mycluster",
		Config: appv1.ClusterConfig{Username: "bar"},
	}
	db := &dbmocks.ArgoDB{}
	db.EXPECT().GetApplicationControllerReplicas().Return(1)
	fakeClient := fake.NewClientset()
	settingsMgr := argosettings.NewSettingsManager(t.Context(), fakeClient, "argocd")
	gitopsEngineClusterCache := &mocks.ClusterCache{}
	clustersCache := liveStateCache{
		clusters: map[string]cache.ClusterCache{
			testCluster.Server: gitopsEngineClusterCache,
		},
		clusterSharding: sharding.NewClusterSharding(db, 0, 1, common.DefaultShardingAlgorithm),
		settingsMgr:     settingsMgr,
		lock:            sync.RWMutex{},
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

	gitopsEngineClusterCache.EXPECT().EnsureSynced().Run(func() {
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

	gitopsEngineClusterCache.EXPECT().Invalidate().Run(func(_ ...cache.UpdateSettingsFunc) {
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
	}).Return().Maybe()
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
	t.Parallel()
	var (
		tlsHandshakeTimeoutErr net.Error = netError("net/http: TLS handshake timeout")
		ioTimeoutErr           net.Error = netError("i/o timeout")
		connectionTimedout     net.Error = netError("connection timed out")
		connectionReset        net.Error = netError("connection reset by peer")
	)
	t.Run("Nil", func(t *testing.T) {
		t.Parallel()
		assert.False(t, isRetryableError(nil))
	})
	t.Run("ResourceQuotaConflictErr", func(t *testing.T) {
		t.Parallel()
		assert.False(t, isRetryableError(apierrors.NewConflict(schema.GroupResource{}, "", nil)))
		assert.True(t, isRetryableError(apierrors.NewConflict(schema.GroupResource{Group: "v1", Resource: "resourcequotas"}, "", nil)))
	})
	t.Run("ExceededQuotaErr", func(t *testing.T) {
		t.Parallel()
		assert.False(t, isRetryableError(apierrors.NewForbidden(schema.GroupResource{}, "", nil)))
		assert.True(t, isRetryableError(apierrors.NewForbidden(schema.GroupResource{Group: "v1", Resource: "pods"}, "", errors.New("exceeded quota"))))
	})
	t.Run("TooManyRequestsDNS", func(t *testing.T) {
		t.Parallel()
		assert.True(t, isRetryableError(apierrors.NewTooManyRequests("", 0)))
	})
	t.Run("DNSError", func(t *testing.T) {
		t.Parallel()
		assert.True(t, isRetryableError(&net.DNSError{}))
	})
	t.Run("OpError", func(t *testing.T) {
		t.Parallel()
		assert.True(t, isRetryableError(&net.OpError{}))
	})
	t.Run("UnknownNetworkError", func(t *testing.T) {
		t.Parallel()
		assert.True(t, isRetryableError(net.UnknownNetworkError("")))
	})
	t.Run("ConnectionClosedErr", func(t *testing.T) {
		t.Parallel()
		assert.False(t, isRetryableError(&url.Error{Err: errors.New("")}))
		assert.True(t, isRetryableError(&url.Error{Err: errors.New("Connection closed by foreign host")}))
	})
	t.Run("TLSHandshakeTimeout", func(t *testing.T) {
		t.Parallel()
		assert.True(t, isRetryableError(tlsHandshakeTimeoutErr))
	})
	t.Run("IOHandshakeTimeout", func(t *testing.T) {
		t.Parallel()
		assert.True(t, isRetryableError(ioTimeoutErr))
	})
	t.Run("ConnectionTimeout", func(t *testing.T) {
		t.Parallel()
		assert.True(t, isRetryableError(connectionTimedout))
	})
	t.Run("ConnectionReset", func(t *testing.T) {
		t.Parallel()
		assert.True(t, isRetryableError(connectionReset))
	})
}

func Test_asResourceNode_owner_refs(t *testing.T) {
	resNode := asResourceNode(&cache.Resource{
		ResourceVersion: "",
		Ref: corev1.ObjectReference{
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
	}, nil)
	expected := appv1.ResourceNode{
		Version: "v1",
		ParentRefs: []appv1.ResourceRef{
			{
				Group:   "",
				Kind:    "ConfigMap",
				Version: "v1",
				Name:    "cm-1",
			},
			{
				Group:   "",
				Kind:    "ConfigMap",
				Version: "v1",
				Name:    "cm-2",
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
				Ref: corev1.ObjectReference{
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
				Ref: corev1.ObjectReference{
					Name: "cm1",
				},
				OwnerRefs: []metav1.OwnerReference{
					{Name: "cm2"},
				},
			},
			ns: map[kube.ResourceKey]*cache.Resource{
				kube.NewResourceKey("", "", "", "cm2"): {
					Ref: corev1.ObjectReference{
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
				Ref: corev1.ObjectReference{
					Name: "cm1",
				},
				OwnerRefs: []metav1.OwnerReference{
					{Name: "cm2"},
					{Name: "cm3"},
				},
			},
			ns: map[kube.ResourceKey]*cache.Resource{
				kube.NewResourceKey("", "", "", "cm2"): {
					Ref: corev1.ObjectReference{
						Name: "cm2",
					},
					OwnerRefs: []metav1.OwnerReference{
						{Name: "app1"},
					},
				},
				kube.NewResourceKey("", "", "", "cm3"): {
					Ref: corev1.ObjectReference{
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
				Ref: corev1.ObjectReference{
					Name: "cm1",
				},
				OwnerRefs: []metav1.OwnerReference{
					{Name: "cm2"},
					{Name: "cm3"},
				},
			},
			ns: map[kube.ResourceKey]*cache.Resource{
				kube.NewResourceKey("", "", "", "cm2"): {
					Ref: corev1.ObjectReference{
						Name: "cm2",
					},
				},
				kube.NewResourceKey("", "", "", "cm3"): {
					Ref: corev1.ObjectReference{
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
				Ref: corev1.ObjectReference{
					Name: "cm1",
				},
				OwnerRefs: []metav1.OwnerReference{
					{Name: "cm2"},
				},
			},
			ns: map[kube.ResourceKey]*cache.Resource{
				kube.NewResourceKey("", "", "", "cm1"): {
					Ref: corev1.ObjectReference{
						Name: "cm1",
					},
					OwnerRefs: []metav1.OwnerReference{
						{Name: "cm2"},
					},
				},
				kube.NewResourceKey("", "", "", "cm2"): {
					Ref: corev1.ObjectReference{
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
				Ref: corev1.ObjectReference{
					Name: "cm1",
				},
				OwnerRefs: []metav1.OwnerReference{
					{Name: "cm2"},
				},
			},
			ns: map[kube.ResourceKey]*cache.Resource{
				kube.NewResourceKey("", "", "", "cm1"): {
					Ref: corev1.ObjectReference{
						Name: "cm1",
					},
					OwnerRefs: []metav1.OwnerReference{
						{Name: "cm2"},
					},
				},
				kube.NewResourceKey("", "", "", "cm2"): {
					Ref: corev1.ObjectReference{
						Name: "cm2",
					},
					OwnerRefs: []metav1.OwnerReference{
						{Name: "cm3"},
					},
				},
				kube.NewResourceKey("", "", "", "cm3"): {
					Ref: corev1.ObjectReference{
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
	t.Parallel()
	var (
		hash1X = "x"
		hash2Y = "y"
		hash3X = "x"
	)
	info := &ResourceInfo{
		manifestHash: hash1X,
		Health: &health.HealthStatus{
			Status:  health.HealthStatusHealthy,
			Message: "default",
		},
	}
	t.Run("Nil", func(t *testing.T) {
		t.Parallel()
		assert.False(t, skipResourceUpdate(nil, nil))
	})
	t.Run("From Nil", func(t *testing.T) {
		t.Parallel()
		assert.False(t, skipResourceUpdate(nil, info))
	})
	t.Run("To Nil", func(t *testing.T) {
		t.Parallel()
		assert.False(t, skipResourceUpdate(info, nil))
	})
	t.Run("No hash", func(t *testing.T) {
		t.Parallel()
		assert.False(t, skipResourceUpdate(&ResourceInfo{}, &ResourceInfo{}))
	})
	t.Run("Same hash", func(t *testing.T) {
		t.Parallel()
		assert.True(t, skipResourceUpdate(&ResourceInfo{
			manifestHash: hash1X,
		}, &ResourceInfo{
			manifestHash: hash1X,
		}))
	})
	t.Run("Same hash value", func(t *testing.T) {
		t.Parallel()
		assert.True(t, skipResourceUpdate(&ResourceInfo{
			manifestHash: hash1X,
		}, &ResourceInfo{
			manifestHash: hash3X,
		}))
	})
	t.Run("Different hash value", func(t *testing.T) {
		t.Parallel()
		assert.False(t, skipResourceUpdate(&ResourceInfo{
			manifestHash: hash1X,
		}, &ResourceInfo{
			manifestHash: hash2Y,
		}))
	})
	t.Run("Same hash, empty health", func(t *testing.T) {
		t.Parallel()
		assert.True(t, skipResourceUpdate(&ResourceInfo{
			manifestHash: hash1X,
			Health:       &health.HealthStatus{},
		}, &ResourceInfo{
			manifestHash: hash3X,
			Health:       &health.HealthStatus{},
		}))
	})
	t.Run("Same hash, old health", func(t *testing.T) {
		t.Parallel()
		assert.False(t, skipResourceUpdate(&ResourceInfo{
			manifestHash: hash1X,
			Health: &health.HealthStatus{
				Status: health.HealthStatusHealthy,
			},
		}, &ResourceInfo{
			manifestHash: hash3X,
			Health:       nil,
		}))
	})
	t.Run("Same hash, new health", func(t *testing.T) {
		t.Parallel()
		assert.False(t, skipResourceUpdate(&ResourceInfo{
			manifestHash: hash1X,
			Health:       &health.HealthStatus{},
		}, &ResourceInfo{
			manifestHash: hash3X,
			Health: &health.HealthStatus{
				Status: health.HealthStatusHealthy,
			},
		}))
	})
	t.Run("Same hash, same health", func(t *testing.T) {
		t.Parallel()
		assert.True(t, skipResourceUpdate(&ResourceInfo{
			manifestHash: hash1X,
			Health: &health.HealthStatus{
				Status:  health.HealthStatusHealthy,
				Message: "same",
			},
		}, &ResourceInfo{
			manifestHash: hash3X,
			Health: &health.HealthStatus{
				Status:  health.HealthStatusHealthy,
				Message: "same",
			},
		}))
	})
	t.Run("Same hash, different health status", func(t *testing.T) {
		t.Parallel()
		assert.False(t, skipResourceUpdate(&ResourceInfo{
			manifestHash: hash1X,
			Health: &health.HealthStatus{
				Status:  health.HealthStatusHealthy,
				Message: "same",
			},
		}, &ResourceInfo{
			manifestHash: hash3X,
			Health: &health.HealthStatus{
				Status:  health.HealthStatusDegraded,
				Message: "same",
			},
		}))
	})
	t.Run("Same hash, different health message", func(t *testing.T) {
		t.Parallel()
		assert.True(t, skipResourceUpdate(&ResourceInfo{
			manifestHash: hash1X,
			Health: &health.HealthStatus{
				Status:  health.HealthStatusHealthy,
				Message: "same",
			},
		}, &ResourceInfo{
			manifestHash: hash3X,
			Health: &health.HealthStatus{
				Status:  health.HealthStatusHealthy,
				Message: "different",
			},
		}))
	})
}

func TestShouldHashManifest(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			if test.annotations != nil {
				test.un.SetAnnotations(test.annotations)
			}
			got := shouldHashManifest(test.appName, test.gvk, test.un)
			require.Equalf(t, test.want, got, "test=%v", test.name)
		})
	}
}

func Test_GetVersionsInfo_error_redacted(t *testing.T) {
	c := liveStateCache{}
	cluster := &appv1.Cluster{
		Server: "https://localhost:1234",
		Config: appv1.ClusterConfig{
			Username: "admin",
			Password: "password",
		},
	}
	_, _, err := c.GetVersionsInfo(cluster)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "password")
}

func TestLoadCacheSettings(t *testing.T) {
	t.Parallel()
	_, settingsManager := fixtures(t.Context(), map[string]string{
		"application.instanceLabelKey":       "testLabel",
		"application.resourceTrackingMethod": string(appv1.TrackingMethodLabel),
		"installationID":                     "123456789",
	})
	ch := liveStateCache{
		settingsMgr: settingsManager,
	}
	label, err := settingsManager.GetAppInstanceLabelKey()
	require.NoError(t, err)
	trackingMethod, err := settingsManager.GetTrackingMethod()
	require.NoError(t, err)
	res, err := ch.loadCacheSettings()
	require.NoError(t, err)

	assert.Equal(t, label, res.appInstanceLabelKey)
	assert.Equal(t, string(appv1.TrackingMethodLabel), trackingMethod)
	assert.Equal(t, "123456789", res.installationID)

	// By default the values won't be nil
	assert.NotNil(t, res.resourceOverrides)
	assert.NotNil(t, res.clusterSettings)
	assert.True(t, res.ignoreResourceUpdatesEnabled)
}

func Test_ownerRefGV(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    metav1.OwnerReference
		expected schema.GroupVersion
	}{
		{
			name: "valid API Version",
			input: metav1.OwnerReference{
				APIVersion: "apps/v1",
			},
			expected: schema.GroupVersion{
				Group:   "apps",
				Version: "v1",
			},
		},
		{
			name: "custom defined version",
			input: metav1.OwnerReference{
				APIVersion: "custom-version",
			},
			expected: schema.GroupVersion{
				Version: "custom-version",
				Group:   "",
			},
		},
		{
			name: "empty APIVersion",
			input: metav1.OwnerReference{
				APIVersion: "",
			},
			expected: schema.GroupVersion{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res := ownerRefGV(tt.input)
			assert.Equal(t, tt.expected, res)
		})
	}
}

func Test_asResourceNode_cross_namespace_parent(t *testing.T) {
	// Test that a namespaced resource with a cluster-scoped parent
	// correctly sets the parent namespace to empty string

	// Create a Role (namespaced) with an owner reference to a ClusterRole (cluster-scoped)
	roleResource := &cache.Resource{
		Ref: corev1.ObjectReference{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
			Namespace:  "my-namespace",
			Name:       "my-role",
		},
		OwnerRefs: []metav1.OwnerReference{
			{
				APIVersion: "rbac.authorization.k8s.io/v1",
				Kind:       "ClusterRole",
				Name:       "my-cluster-role",
				UID:        "cluster-role-uid",
			},
		},
	}

	// Create namespace resources map (ClusterRole won't be in here since it's cluster-scoped)
	namespaceResources := map[kube.ResourceKey]*cache.Resource{
		// Add some other namespace resources but not the ClusterRole
		{
			Group:     "rbac.authorization.k8s.io",
			Kind:      "Role",
			Namespace: "my-namespace",
			Name:      "other-role",
		}: {
			Ref: corev1.ObjectReference{
				APIVersion: "rbac.authorization.k8s.io/v1",
				Kind:       "Role",
				Namespace:  "my-namespace",
				Name:       "other-role",
			},
		},
	}

	resNode := asResourceNode(roleResource, namespaceResources)

	// The parent reference should have empty namespace since ClusterRole is cluster-scoped
	assert.Len(t, resNode.ParentRefs, 1)
	assert.Equal(t, "ClusterRole", resNode.ParentRefs[0].Kind)
	assert.Equal(t, "my-cluster-role", resNode.ParentRefs[0].Name)
	assert.Empty(t, resNode.ParentRefs[0].Namespace, "ClusterRole parent should have empty namespace")
}

func Test_asResourceNode_same_namespace_parent(t *testing.T) {
	// Test that a namespaced resource with a namespaced parent in the same namespace
	// correctly sets the parent namespace

	// Create a ReplicaSet with an owner reference to a Deployment (both namespaced)
	rsResource := &cache.Resource{
		Ref: corev1.ObjectReference{
			APIVersion: "apps/v1",
			Kind:       "ReplicaSet",
			Namespace:  "my-namespace",
			Name:       "my-rs",
		},
		OwnerRefs: []metav1.OwnerReference{
			{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "my-deployment",
				UID:        "deployment-uid",
			},
		},
	}

	// Create namespace resources map with the Deployment
	deploymentKey := kube.ResourceKey{
		Group:     "apps",
		Kind:      "Deployment",
		Namespace: "my-namespace",
		Name:      "my-deployment",
	}
	namespaceResources := map[kube.ResourceKey]*cache.Resource{
		deploymentKey: {
			Ref: corev1.ObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Namespace:  "my-namespace",
				Name:       "my-deployment",
				UID:        "deployment-uid",
			},
			Resource: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "my-deployment",
						"namespace": "my-namespace",
						"uid":       "deployment-uid",
					},
				},
			},
		},
	}

	resNode := asResourceNode(rsResource, namespaceResources)

	// The parent reference should have the same namespace
	assert.Len(t, resNode.ParentRefs, 1)
	assert.Equal(t, "Deployment", resNode.ParentRefs[0].Kind)
	assert.Equal(t, "my-deployment", resNode.ParentRefs[0].Name)
	assert.Equal(t, "my-namespace", resNode.ParentRefs[0].Namespace, "Deployment parent should have same namespace")
}

func setClusterCacheEnv(t *testing.T, value string) {
	t.Helper()
	for _, name := range []string{
		EnvClusterCacheResyncDuration,
		EnvClusterCacheWatchResyncDuration,
		EnvClusterSyncRetryTimeoutDuration,
		EnvClusterCacheListPageSize,
		EnvClusterCacheListPageBufferSize,
		EnvClusterCacheListSemaphore,
		EnvClusterCacheAttemptLimit,
		EnvClusterCacheRetryUseBackoff,
		EnvClusterCacheBatchEventsProcessing,
		EnvClusterCacheEventsProcessingInterval,
	} {
		t.Setenv(name, value)
	}
}

func defaultClusterCacheConfig() ClusterCacheConfig {
	return ClusterCacheConfig{
		ResyncDuration:           12 * time.Hour,
		WatchResyncDuration:      10 * time.Minute,
		SyncRetryTimeout:         10 * time.Second,
		ListPageSize:             500,
		ListPageBufferSize:       1,
		ListSemaphoreSize:        50,
		AttemptLimit:             1,
		RetryUseBackoff:          false,
		BatchEventsProcessing:    true,
		EventsProcessingInterval: 100 * time.Millisecond,
	}
}

func TestClusterCacheConfigFromEnv(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		setClusterCacheEnv(t, "")
		assert.Equal(t, defaultClusterCacheConfig(), ClusterCacheConfigFromEnv())
	})

	t.Run("overrides", func(t *testing.T) {
		setClusterCacheEnv(t, "")
		t.Setenv(EnvClusterCacheResyncDuration, "6h")
		t.Setenv(EnvClusterCacheWatchResyncDuration, "5m")
		t.Setenv(EnvClusterSyncRetryTimeoutDuration, "30s")
		t.Setenv(EnvClusterCacheListPageSize, "250")
		t.Setenv(EnvClusterCacheListPageBufferSize, "4")
		t.Setenv(EnvClusterCacheListSemaphore, "10")
		t.Setenv(EnvClusterCacheAttemptLimit, "3")
		t.Setenv(EnvClusterCacheRetryUseBackoff, "true")
		t.Setenv(EnvClusterCacheBatchEventsProcessing, "false")
		t.Setenv(EnvClusterCacheEventsProcessingInterval, "1s")

		assert.Equal(t, ClusterCacheConfig{
			ResyncDuration:           6 * time.Hour,
			WatchResyncDuration:      5 * time.Minute,
			SyncRetryTimeout:         30 * time.Second,
			ListPageSize:             250,
			ListPageBufferSize:       4,
			ListSemaphoreSize:        10,
			AttemptLimit:             3,
			RetryUseBackoff:          true,
			BatchEventsProcessing:    false,
			EventsProcessingInterval: time.Second,
		}, ClusterCacheConfigFromEnv())
	})

	t.Run("out of range values fall back to defaults", func(t *testing.T) {
		setClusterCacheEnv(t, "")
		t.Setenv(EnvClusterCacheResyncDuration, "-1s")
		t.Setenv(EnvClusterCacheListPageSize, "-1")
		t.Setenv(EnvClusterCacheListPageBufferSize, "0")
		t.Setenv(EnvClusterCacheAttemptLimit, "0")
		t.Setenv(EnvClusterCacheEventsProcessingInterval, "invalid")

		assert.Equal(t, defaultClusterCacheConfig(), ClusterCacheConfigFromEnv())
	})
}

func TestClusterCacheConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*ClusterCacheConfig)
		wantErr string
	}{
		{name: "defaults", modify: func(*ClusterCacheConfig) {}},
		{name: "zero values within range", modify: func(c *ClusterCacheConfig) {
			c.ResyncDuration = 0
			c.ListPageSize = 0
			c.ListSemaphoreSize = 0
		}},
		{name: "negative resync duration", modify: func(c *ClusterCacheConfig) { c.ResyncDuration = -time.Second }, wantErr: "resync duration must not be negative"},
		{name: "negative watch resync duration", modify: func(c *ClusterCacheConfig) { c.WatchResyncDuration = -time.Second }, wantErr: "watch resync duration must not be negative"},
		{name: "negative sync retry timeout", modify: func(c *ClusterCacheConfig) { c.SyncRetryTimeout = -time.Second }, wantErr: "sync retry timeout must not be negative"},
		{name: "negative events processing interval", modify: func(c *ClusterCacheConfig) { c.EventsProcessingInterval = -time.Second }, wantErr: "events processing interval must not be negative"},
		{name: "negative list page size", modify: func(c *ClusterCacheConfig) { c.ListPageSize = -1 }, wantErr: "list page size must not be negative"},
		{name: "list page buffer size below 1", modify: func(c *ClusterCacheConfig) { c.ListPageBufferSize = 0 }, wantErr: "list page buffer size must be at least 1"},
		{name: "negative list semaphore size", modify: func(c *ClusterCacheConfig) { c.ListSemaphoreSize = -1 }, wantErr: "list semaphore size must not be negative"},
		{name: "attempt limit below 1", modify: func(c *ClusterCacheConfig) { c.AttemptLimit = 0 }, wantErr: "attempt limit must be at least 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultClusterCacheConfig()
			tt.modify(&cfg)
			err := cfg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}

	t.Run("reports every invalid value", func(t *testing.T) {
		cfg := defaultClusterCacheConfig()
		cfg.ListPageBufferSize = 0
		cfg.AttemptLimit = 0
		err := cfg.Validate()
		require.ErrorContains(t, err, "list page buffer size must be at least 1")
		require.ErrorContains(t, err, "attempt limit must be at least 1")
	})
}

func TestNewLiveStateCache_UsesClusterCacheConfig(t *testing.T) {
	cfg := defaultClusterCacheConfig()
	cfg.ListPageSize = 42
	cfg.AttemptLimit = 3

	c := NewLiveStateCache(nil, nil, nil, nil, nil, nil, nil, cfg)

	assert.Equal(t, cfg, c.(*liveStateCache).clusterCacheConfig)
}
