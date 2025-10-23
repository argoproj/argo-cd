package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	testcore "k8s.io/client-go/testing"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/kube/kubetest"
)

func TestAddNamespace(t *testing.T) {
	t.Run("feature disabled invalidates cache and adds namespace", func(t *testing.T) {
		// Given: cache previously synced
		cache := NewClusterCache(
			&rest.Config{},
			SetKubectl(&kubetest.MockKubectlCmd{}),
			SetNamespaces([]string{"existing-namespace"}),
		)
		setSyncTime(cache, time.Now())

		// When: adding a namespace
		err := cache.AddNamespace("new-namespace")

		// Then: cache is invalidated, namespace is added
		assert.NoError(t, err)
		assert.Nil(t, getSyncTime(cache), "syncTime should be nil (invalidated)")
		assert.Contains(t, cache.namespaces, "new-namespace", "new namespace added")
		assert.Contains(t, cache.namespaces, "existing-namespace", "existing namespace preserved")
	})

	t.Run("feature enabled preserves cache and adds namespace", func(t *testing.T) {
		// Given: cache previously synced
		cache := NewClusterCache(
			&rest.Config{},
			SetKubectl(&kubetest.MockKubectlCmd{}),
			SetNamespaces([]string{"existing-namespace"}),
			WithIncrementalNamespaceSync(true),
		)
		now := time.Now()
		setSyncTime(cache, time.Now())

		// When: adding a namespace
		err := cache.AddNamespace("new-namespace")

		// Then: cache is preserved, namespace is added
		assert.NoError(t, err)
		assert.NotNil(t, getSyncTime(cache), "syncTime should not be nil")
		assert.Equal(t, now.Unix(), getSyncTime(cache).Unix(), "syncTime should be unchanged")
		assert.Contains(t, cache.namespaces, "new-namespace", "new namespace added")
		assert.Contains(t, cache.namespaces, "existing-namespace", "existing namespace preserved")
	})

	t.Run("feature enabled preserves existing resources when adding namespace", func(t *testing.T) {
		// Given: cache synced with existing pod
		existingPod := createPod("existing-pod", "existing-namespace")
		_, mockKubectl := setupFakeCluster(existingPod)
		cache := NewClusterCache(
			&rest.Config{},
			SetKubectl(mockKubectl),
			SetNamespaces([]string{"existing-namespace"}),
			WithIncrementalNamespaceSync(true),
		)
		require.NoError(t, cache.EnsureSynced())

		// When: adding new namespace
		err := cache.AddNamespace("new-namespace")

		// Then: existing resources are preserved
		assert.NoError(t, err)
		assertPodInCache(t, cache, "existing-namespace", "existing-pod", "existing pod preserved")
	})

	t.Run("feature enabled watches new namespace for changes", func(t *testing.T) {
		// Given: cache synced, new namespace added
		existingPod := createPod("existing-pod", "existing-namespace")
		client, mockKubectl := setupFakeCluster(existingPod)
		cache := NewClusterCache(
			&rest.Config{},
			SetKubectl(mockKubectl),
			SetNamespaces([]string{"existing-namespace"}),
			WithIncrementalNamespaceSync(true),
		)
		require.NoError(t, cache.EnsureSynced())
		require.NoError(t, cache.AddNamespace("new-namespace"))
		time.Sleep(50 * time.Millisecond) // Allow watch goroutines to start

		// When: creating pod in new namespace
		newPod := createPod("new-pod", "new-namespace")
		podClient := client.Resource(schema.GroupVersionResource{
			Group: "", Version: "v1", Resource: "pods",
		}).Namespace("new-namespace")
		_, err := podClient.Create(context.Background(), mustToUnstructured(newPod), metav1.CreateOptions{})
		require.NoError(t, err)
		time.Sleep(50 * time.Millisecond) // Allow watch to process event

		// Then: watch detects the new pod
		assertPodInCache(t, cache, "new-namespace", "new-pod", "new pod detected by watch")
	})

	t.Run("feature enabled returns error for non-RBAC errors", func(t *testing.T) {
		// Given: cluster returns non-RBAC errors
		client := fake.NewSimpleDynamicClient(scheme.Scheme)
		client.PrependReactor("list", "pods", func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
			listAction := action.(testcore.ListAction)
			if listAction.GetNamespace() == "error-namespace" {
				return true, nil, apierrors.NewInternalError(fmt.Errorf("internal server error"))
			}
			return false, nil, nil
		})
		apiResources := []kube.APIResourceInfo{{
			GroupKind:            schema.GroupKind{Group: "", Kind: "Pod"},
			GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Meta:                 metav1.APIResource{Namespaced: true},
		}}
		mockKubectl := &kubetest.MockKubectlCmd{
			DynamicClient: client,
			APIResources:  apiResources,
			Version:       "v1.28.0",
		}
		cache := NewClusterCache(
			&rest.Config{},
			SetKubectl(mockKubectl),
			SetNamespaces([]string{"existing-namespace"}),
			WithIncrementalNamespaceSync(true),
			SetRespectRBAC(RespectRbacNormal),
		)
		err := cache.EnsureSynced()
		require.NoError(t, err)

		// When: adding namespace that returns non-RBAC error
		err = cache.AddNamespace("error-namespace")

		// Then: error is propagated
		assert.Error(t, err, "non-RBAC errors should be returned")
	})

	t.Run("feature enabled watches are canceled on Invalidate", func(t *testing.T) {
		// Given: cache synced with namespace added
		pod := createPod("test-pod", "new-namespace")
		_, mockKubectl := setupFakeCluster(pod)
		cache := NewClusterCache(
			&rest.Config{},
			SetKubectl(mockKubectl),
			SetNamespaces([]string{"existing-namespace"}),
			WithIncrementalNamespaceSync(true),
		)
		require.NoError(t, cache.EnsureSynced())
		require.NoError(t, cache.AddNamespace("new-namespace"))

		// Given: namespace watch context exists
		cache.lock.RLock()
		podMeta, exists := cache.apisMeta[schema.GroupKind{Group: "", Kind: "Pod"}]
		cache.lock.RUnlock()
		require.True(t, exists, "Pod apiMeta must exist")
		require.NotNil(t, podMeta.watchCtx, "watchCtx must exist")

		// When: invalidating cache
		cache.Invalidate()

		// Then: watch context is canceled
		select {
		case <-podMeta.watchCtx.Done():
			// Success - context was canceled
		case <-time.After(100 * time.Millisecond):
			t.Fatal("watch context should be canceled after Invalidate")
		}
	})
}

func TestRemoveNamespace(t *testing.T) {
	t.Run("feature disabled invalidates cache and removes namespace", func(t *testing.T) {
		// Given: cache previously synced
		cache := NewClusterCache(
			&rest.Config{},
			SetKubectl(&kubetest.MockKubectlCmd{}),
			SetNamespaces([]string{"ns-1", "ns-2"}),
		)
		setSyncTime(cache, time.Now())

		// When: removing a namespace
		err := cache.RemoveNamespace("ns-2")

		// Then: cache is invalidated, namespace is removed
		assert.NoError(t, err)
		assert.Nil(t, getSyncTime(cache), "syncTime should be nil (invalidated)")
		assert.NotContains(t, cache.namespaces, "ns-2", "removed namespace")
		assert.Contains(t, cache.namespaces, "ns-1", "remaining namespace preserved")
	})

	t.Run("feature enabled preserves cache and removes namespace", func(t *testing.T) {
		// Given: cache previously synced
		cache := NewClusterCache(
			&rest.Config{},
			SetKubectl(&kubetest.MockKubectlCmd{}),
			SetNamespaces([]string{"ns-1", "ns-2"}),
			WithIncrementalNamespaceSync(true),
		)
		now := time.Now()
		setSyncTime(cache, now)

		// When: removing a namespace
		err := cache.RemoveNamespace("ns-2")

		// Then: cache is preserved, namespace is removed
		assert.NoError(t, err)
		assert.NotNil(t, getSyncTime(cache), "syncTime should not be nil")
		assert.Equal(t, now.Unix(), getSyncTime(cache).Unix(), "syncTime should be unchanged")
		assert.NotContains(t, cache.namespaces, "ns-2", "removed namespace")
		assert.Contains(t, cache.namespaces, "ns-1", "remaining namespace preserved")
	})

	t.Run("feature enabled removes resources from removed namespace", func(t *testing.T) {
		// Given: cache synced with pods in both namespaces
		pod1 := createPod("pod-1", "ns-1")
		pod2 := createPod("pod-2", "ns-2")
		_, mockKubectl := setupFakeCluster(pod1, pod2)
		cache := NewClusterCache(
			&rest.Config{},
			SetKubectl(mockKubectl),
			SetNamespaces([]string{"ns-1", "ns-2"}),
			WithIncrementalNamespaceSync(true),
		)
		require.NoError(t, cache.EnsureSynced())
		assertPodInCache(t, cache, "ns-1", "pod-1", "pod-1 in cache before removal")
		assertPodInCache(t, cache, "ns-2", "pod-2", "pod-2 in cache before removal")

		// When: removing ns-2
		err := cache.RemoveNamespace("ns-2")

		// Then: resources from removed namespace are deleted
		assert.NoError(t, err)
		cache.lock.RLock()
		_, exists := cache.resources[kube.NewResourceKey("", "Pod", "ns-2", "pod-2")]
		cache.lock.RUnlock()
		assert.False(t, exists, "pod-2 removed from cache")
		assertPodInCache(t, cache, "ns-1", "pod-1", "pod-1 still in cache")
	})

	t.Run("feature enabled cancels watches when namespace removed", func(t *testing.T) {
		// Given: cache synced with ns-2 added incrementally
		pod1 := createPod("pod-1", "ns-1")
		pod2 := createPod("pod-2", "ns-2")
		_, mockKubectl := setupFakeCluster(pod1, pod2)
		cache := NewClusterCache(
			&rest.Config{},
			SetKubectl(mockKubectl),
			SetNamespaces([]string{"ns-1"}),
			WithIncrementalNamespaceSync(true),
		)
		require.NoError(t, cache.EnsureSynced())
		require.NoError(t, cache.AddNamespace("ns-2"))

		// Given: watch context exists
		cache.lock.RLock()
		podMeta, exists := cache.apisMeta[schema.GroupKind{Group: "", Kind: "Pod"}]
		cache.lock.RUnlock()
		require.True(t, exists, "Pod apiMeta must exist")
		_, hasNs2Cancel := podMeta.namespaceCancels["ns-2"]
		require.True(t, hasNs2Cancel, "ns-2 cancel must exist")

		// When: removing ns-2
		err := cache.RemoveNamespace("ns-2")

		// Then: ns-2 watch is canceled and removed
		assert.NoError(t, err)
		_, hasNs2Cancel = podMeta.namespaceCancels["ns-2"]
		assert.False(t, hasNs2Cancel, "ns-2 removed from namespaceCancels")
	})
}

func setupFakeCluster(objs ...runtime.Object) (*fake.FakeDynamicClient, *kubetest.MockKubectlCmd) {
	client := fake.NewSimpleDynamicClient(scheme.Scheme, objs...)
	reactor := client.ReactionChain[0]
	client.PrependReactor("list", "*", func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
		handled, ret, err = reactor.React(action)
		if err != nil || !handled {
			return handled, ret, err
		}
		ret.(metav1.ListInterface).SetResourceVersion("123")
		return handled, ret, err
	})

	apiResources := []kube.APIResourceInfo{{
		GroupKind:            schema.GroupKind{Group: "", Kind: "Pod"},
		GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		Meta:                 metav1.APIResource{Namespaced: true},
	}}

	mockKubectl := &kubetest.MockKubectlCmd{
		DynamicClient: client,
		APIResources:  apiResources,
		Version:       "v1.28.0",
	}

	return client, mockKubectl
}

func assertPodInCache(t *testing.T, cache *clusterCache, namespace, name, message string) {
	t.Helper()
	podKey := kube.NewResourceKey("", "Pod", namespace, name)
	cache.lock.RLock()
	cachedPod, exists := cache.resources[podKey]
	cache.lock.RUnlock()

	assert.True(t, exists, message)
	assert.NotNil(t, cachedPod, "cached pod should not be nil")
	if cachedPod != nil {
		assert.Equal(t, name, cachedPod.Ref.Name)
		assert.Equal(t, namespace, cachedPod.Ref.Namespace)
	}
}

func setSyncTime(cache *clusterCache, t time.Time) {
	cache.syncStatus.lock.Lock()
	cache.syncStatus.syncTime = &t
	cache.syncStatus.lock.Unlock()
}

func getSyncTime(cache *clusterCache) *time.Time {
	cache.syncStatus.lock.Lock()
	defer cache.syncStatus.lock.Unlock()
	return cache.syncStatus.syncTime
}

func createPod(name, namespace string) *corev1.Pod {
	return &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, ResourceVersion: "789"},
	}
}
