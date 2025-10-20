package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	t.Run("feature disabled", func(t *testing.T) {
		cache := NewClusterCache(
			&rest.Config{},
			SetKubectl(&kubetest.MockKubectlCmd{}),
			SetNamespaces([]string{"existing-namespace"}),
		)

		// given: cache was previously synced
		now := time.Now()
		cache.syncStatus.lock.Lock()
		cache.syncStatus.syncTime = &now
		cache.syncStatus.lock.Unlock()

		// when: adding a namespace with feature disabled
		err := cache.AddNamespace("new-namespace")
		assert.NoError(t, err)

		// then: should invalidate the cache (observable via syncTime being cleared)
		cache.syncStatus.lock.Lock()
		actual := cache.syncStatus.syncTime
		cache.syncStatus.lock.Unlock()
		assert.Nil(t, actual, "given feature disabled, should invalidate cache when namespace added")

		// then: should add namespace to the list
		assert.Contains(t, cache.namespaces, "new-namespace", "given feature disabled, should add namespace to list")
		assert.Contains(t, cache.namespaces, "existing-namespace", "given feature disabled, should preserve existing namespaces")
	})

	t.Run("feature enabled", func(t *testing.T) {
		cache := NewClusterCache(
			&rest.Config{},
			SetKubectl(&kubetest.MockKubectlCmd{}),
			SetNamespaces([]string{"existing-namespace"}),
			WithIncrementalNamespaceSync(true),
		)

		// given: cache was previously synced
		now := time.Now()
		cache.syncStatus.lock.Lock()
		cache.syncStatus.syncTime = &now
		cache.syncStatus.lock.Unlock()

		// when: adding a namespace with feature enabled
		err := cache.AddNamespace("new-namespace")
		assert.NoError(t, err)

		// then: should NOT invalidate the cache (syncTime preserved)
		cache.syncStatus.lock.Lock()
		actual := cache.syncStatus.syncTime
		cache.syncStatus.lock.Unlock()

		assert.NotNil(t, actual, "given feature enabled, should preserve cache when namespace added")
		assert.Equal(t, now.Unix(), actual.Unix(), "given feature enabled, should not change syncTime")

		// then: should add namespace to the list
		assert.Contains(t, cache.namespaces, "new-namespace", "given feature enabled, should add namespace to list")
		assert.Contains(t, cache.namespaces, "existing-namespace", "given feature enabled, should preserve existing namespaces")
	})

	t.Run("feature enabled syncs resources in new namespace", func(t *testing.T) {
		// given: a pod exists in the new namespace
		pod := &corev1.Pod{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
			ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "new-namespace"},
		}

		_, mockKubectl := setupFakeCluster(pod)
		cache := NewClusterCache(
			&rest.Config{},
			SetKubectl(mockKubectl),
			SetNamespaces([]string{"existing-namespace"}),
			WithIncrementalNamespaceSync(true),
		)

		// given: cache was previously synced (to populate apisMeta)
		err := cache.EnsureSynced()
		assert.NoError(t, err)

		// Store the sync time to verify it's preserved
		cache.syncStatus.lock.Lock()
		syncTimeBefore := cache.syncStatus.syncTime
		cache.syncStatus.lock.Unlock()

		// when: adding a namespace with feature enabled
		err = cache.AddNamespace("new-namespace")
		assert.NoError(t, err)

		// then: should preserve the sync time (not invalidate)
		cache.syncStatus.lock.Lock()
		syncTimeAfter := cache.syncStatus.syncTime
		cache.syncStatus.lock.Unlock()
		assert.Equal(t, syncTimeBefore, syncTimeAfter, "sync time should be preserved")

		//then: should have the pod from new namespace in the cache
		assertPodInCache(t, cache, "new-namespace", "test-pod", "pod from new namespace should be in cache")
	})

	t.Run("feature enabled watches new namespace for changes", func(t *testing.T) {
		// given: initial pod in existing namespace
		existingPod := &corev1.Pod{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
			ObjectMeta: metav1.ObjectMeta{Name: "existing-pod", Namespace: "existing-namespace"},
		}

		client, mockKubectl := setupFakeCluster(existingPod)
		cache := NewClusterCache(
			&rest.Config{},
			SetKubectl(mockKubectl),
			SetNamespaces([]string{"existing-namespace"}),
			WithIncrementalNamespaceSync(true),
		)

		// given: cache was previously synced (starts watches for existing namespace)
		err := cache.EnsureSynced()
		assert.NoError(t, err)

		// when: adding a new namespace
		err = cache.AddNamespace("new-namespace")
		assert.NoError(t, err)

		// when: a new pod is created in the new namespace AFTER AddNamespace
		newPod := &corev1.Pod{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
			ObjectMeta: metav1.ObjectMeta{Name: "new-pod", Namespace: "new-namespace", ResourceVersion: "124"},
		}

		podClient := client.Resource(schema.GroupVersionResource{
			Group: "", Version: "v1", Resource: "pods",
		}).Namespace("new-namespace")

		_, err = podClient.Create(context.Background(), mustToUnstructured(newPod), metav1.CreateOptions{})
		assert.NoError(t, err)

		// then: the watch should pick up the new pod and add it to cache
		time.Sleep(100 * time.Millisecond)

		// then: the new pod should be in the cache (proves watches are active)
		assertPodInCache(t, cache, "new-namespace", "new-pod", "pod created after AddNamespace should be in cache (proves watches are active)")
	})

	t.Run("feature enabled returns error for non-RBAC errors", func(t *testing.T) {
		// given: a fake cluster that returns generic errors (not RBAC)
		client := fake.NewSimpleDynamicClient(scheme.Scheme)

		// given: setup reactor to return generic error for "error-namespace"
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

		// given: cache was previously synced
		err := cache.EnsureSynced()
		assert.NoError(t, err)

		// when: adding a namespace with non-RBAC errors
		err = cache.AddNamespace("error-namespace")

		// then: should return error (only RBAC errors should be ignored)
		assert.Error(t, err, "AddNamespace should return error for non-RBAC errors")
	})
}

func setupFakeCluster(objs ...runtime.Object) (*fake.FakeDynamicClient, *kubetest.MockKubectlCmd) {
	client := fake.NewSimpleDynamicClient(scheme.Scheme, objs...)
	reactor := client.ReactionChain[0]
	client.PrependReactor("list", "*", func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
		handled, ret, err = reactor.React(action)
		if err != nil || !handled {
			return
		}
		ret.(metav1.ListInterface).SetResourceVersion("123")
		return
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
