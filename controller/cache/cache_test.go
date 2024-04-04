package cache

import (
	"context"
	"errors"
	"net"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/cache/mocks"
	"github.com/stretchr/testify/mock"
	"k8s.io/client-go/kubernetes/fake"

	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
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

	clustersCache := liveStateCache{
		clusters: map[string]cache.ClusterCache{
			"https://mycluster": clusterCache,
		},
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

	clustersCache := liveStateCache{
		clusters: map[string]cache.ClusterCache{
			"https://mycluster": clusterCache,
		},
		clusterFilter: func(cluster *appv1.Cluster) bool {
			return false
		},
	}

	clustersCache.handleModEvent(&appv1.Cluster{
		Server: "https://mycluster",
		Config: appv1.ClusterConfig{Username: "foo"},
	}, &appv1.Cluster{
		Server:     "https://mycluster",
		Config:     appv1.ClusterConfig{Username: "bar"},
		Namespaces: []string{"default"},
	})

	assert.Len(t, clustersCache.clusters, 0)
}

func TestHandleModEvent_NoChanges(t *testing.T) {
	clusterCache := &mocks.ClusterCache{}
	clusterCache.On("Invalidate", mock.Anything).Panic("should not invalidate")
	clusterCache.On("EnsureSynced").Return(nil).Panic("should not re-sync")

	clustersCache := liveStateCache{
		clusters: map[string]cache.ClusterCache{
			"https://mycluster": clusterCache,
		},
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
	clustersCache := liveStateCache{
		clusters: map[string]cache.ClusterCache{},
		clusterFilter: func(cluster *appv1.Cluster) bool {
			return false
		},
	}
	clustersCache.handleAddEvent(&appv1.Cluster{
		Server: "https://mycluster",
		Config: appv1.ClusterConfig{Username: "bar"},
	})

	assert.Len(t, clustersCache.clusters, 0)
}

func TestHandleDeleteEvent_CacheDeadlock(t *testing.T) {
	testCluster := &appv1.Cluster{
		Server: "https://mycluster",
		Config: appv1.ClusterConfig{Username: "bar"},
	}
	fakeClient := fake.NewSimpleClientset()
	settingsMgr := argosettings.NewSettingsManager(context.TODO(), fakeClient, "argocd")
	externalLockRef := sync.RWMutex{}
	gitopsEngineClusterCache := &mocks.ClusterCache{}
	clustersCache := liveStateCache{
		clusters: map[string]cache.ClusterCache{
			testCluster.Server: gitopsEngineClusterCache,
		},
		clusterFilter: func(cluster *appv1.Cluster) bool {
			return true
		},
		settingsMgr: settingsMgr,
		// Set the lock here so we can reference it later
		// nolint We need to overwrite here to have access to the lock
		lock: externalLockRef,
	}
	channel := make(chan string)
	// Mocked lock held by the gitops-engine cluster cache
	mockMutex := sync.RWMutex{}
	// Locks to force trigger condition during test
	// Condition order:
	//   EnsuredSynced -> Locks gitops-engine
	//   handleDeleteEvent -> Locks liveStateCache
	//   EnsureSynced via sync, newResource, populateResourceInfoHandler -> attempts to Lock liveStateCache
	//   handleDeleteEvent via cluster.Invalidate -> attempts to Lock gitops-engine
	handleDeleteWasCalled := sync.Mutex{}
	engineHoldsLock := sync.Mutex{}
	handleDeleteWasCalled.Lock()
	engineHoldsLock.Lock()
	gitopsEngineClusterCache.On("EnsureSynced").Run(func(args mock.Arguments) {
		// Held by EnsureSync calling into sync and watchEvents
		mockMutex.Lock()
		defer mockMutex.Unlock()
		// Continue Execution of timer func
		engineHoldsLock.Unlock()
		// Wait for handleDeleteEvent to be called triggering the lock
		// on the liveStateCache
		handleDeleteWasCalled.Lock()
		t.Logf("handleDelete was called, EnsureSynced continuing...")
		handleDeleteWasCalled.Unlock()
		// Try and obtain the lock on the liveStateCache
		alreadyFailed := !externalLockRef.TryLock()
		if alreadyFailed {
			channel <- "DEADLOCKED -- EnsureSynced could not obtain lock on liveStateCache"
			return
		}
		externalLockRef.Lock()
		t.Logf("EnsureSynce was able to lock liveStateCache")
		externalLockRef.Unlock()
	}).Return(nil).Once()
	gitopsEngineClusterCache.On("Invalidate").Run(func(args mock.Arguments) {
		// If deadlock is fixed should be able to acquire lock here
		alreadyFailed := !mockMutex.TryLock()
		if alreadyFailed {
			channel <- "DEADLOCKED -- Invalidate could not obtain lock on gitops-engine"
			return
		}
		mockMutex.Lock()
		t.Logf("Invalidate was able to lock gitops-engine cache")
		mockMutex.Unlock()
	}).Return()
	go func() {
		// Start the gitops-engine lock holds
		go func() {
			err := gitopsEngineClusterCache.EnsureSynced()
			if err != nil {
				assert.Fail(t, err.Error())
			}
		}()
		// Wait for EnsureSynced to grab the lock for gitops-engine
		engineHoldsLock.Lock()
		t.Log("EnsureSynced has obtained lock on gitops-engine")
		engineHoldsLock.Unlock()
		// Run in background
		go clustersCache.handleDeleteEvent(testCluster.Server)
		// Allow execution to continue on clusters cache call to trigger lock
		handleDeleteWasCalled.Unlock()
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
