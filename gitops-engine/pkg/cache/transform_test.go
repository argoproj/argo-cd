package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

// newTransformTestCache constructs a clusterCache suitable for unit-testing
// transformForInformer directly. Tests that need a specific handler should
// reassign cache.populateResourceInfoHandler after construction.
func newTransformTestCache(t *testing.T) *clusterCache {
	t.Helper()
	return NewClusterCache(&rest.Config{}, SetMode(ModeInformer)).(*clusterCache)
}

func samplePod() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":            "nginx",
			"namespace":       "default",
			"uid":             "pod-uid",
			"resourceVersion": "42",
		},
		"spec":   map[string]any{"restartPolicy": "Always"},
		"status": map[string]any{"phase": "Running"},
	}}
}

func TestTransformForInformer_ProducesCachedResource(t *testing.T) {
	cache := newTransformTestCache(t)
	out, err := cache.transformForInformer(samplePod())
	require.NoError(t, err)

	cr, ok := out.(*cachedResource)
	require.True(t, ok, "transform must return *cachedResource")

	// TypeMeta + ObjectMeta populated so the informer's default KeyFunc works.
	assert.Equal(t, "v1", cr.APIVersion)
	assert.Equal(t, "Pod", cr.Kind)
	assert.Equal(t, "nginx", cr.GetName())
	assert.Equal(t, "default", cr.GetNamespace())
	assert.Equal(t, "pod-uid", string(cr.GetUID()))
	assert.Equal(t, "42", cr.GetResourceVersion())

	// Domain Resource built via newResource.
	require.NotNil(t, cr.Resource)
	assert.Equal(t, "nginx", cr.Resource.Ref.Name)
	assert.Equal(t, "default", cr.Resource.Ref.Namespace)
	assert.Equal(t, "42", cr.Resource.ResourceVersion)
}

func TestTransformForInformer_DropsManifestByDefault(t *testing.T) {
	cache := newTransformTestCache(t)
	// No populateResourceInfoHandler → cacheManifest defaults to false.
	out, err := cache.transformForInformer(samplePod())
	require.NoError(t, err)

	cr := out.(*cachedResource)
	assert.Nil(t, cr.Resource.Resource, "full manifest should not be retained without a handler opting in")
	assert.Nil(t, cr.Resource.Info)
}

func TestTransformForInformer_KeepsManifestWhenHandlerOptsIn(t *testing.T) {
	cache := newTransformTestCache(t)
	type resourceInfo struct{ Health string }
	cache.populateResourceInfoHandler = func(_ *unstructured.Unstructured, _ bool) (any, bool) {
		return &resourceInfo{Health: "Healthy"}, true
	}

	un := samplePod()
	out, err := cache.transformForInformer(un)
	require.NoError(t, err)

	cr := out.(*cachedResource)
	require.NotNil(t, cr.Resource.Resource, "full manifest must be retained when cacheManifest=true")
	assert.Equal(t, "Running", cr.Resource.Resource.Object["status"].(map[string]any)["phase"])

	info, ok := cr.Resource.Info.(*resourceInfo)
	require.True(t, ok)
	assert.Equal(t, "Healthy", info.Health)
}

func TestTransformForInformer_HandlerIsRootArg(t *testing.T) {
	cache := newTransformTestCache(t)
	var sawIsRoot bool
	cache.populateResourceInfoHandler = func(_ *unstructured.Unstructured, isRoot bool) (any, bool) {
		sawIsRoot = isRoot
		return nil, false
	}

	// No ownerReferences → treated as root.
	_, err := cache.transformForInformer(samplePod())
	require.NoError(t, err)
	assert.True(t, sawIsRoot)

	// With an ownerReference → not root.
	un := samplePod()
	un.SetOwnerReferences([]metav1.OwnerReference{{Name: "rs", Kind: "ReplicaSet", APIVersion: "apps/v1"}})
	_, err = cache.transformForInformer(un)
	require.NoError(t, err)
	assert.False(t, sawIsRoot)
}

func TestTransformForInformer_PassesThroughNonUnstructured(t *testing.T) {
	cache := newTransformTestCache(t)

	status := &metav1.Status{Status: "Failure"}
	out, err := cache.transformForInformer(status)
	require.NoError(t, err)
	assert.Same(t, status, out)
}

func TestTransformForInformer_PassesThroughNilUnstructured(t *testing.T) {
	cache := newTransformTestCache(t)
	var un *unstructured.Unstructured
	out, err := cache.transformForInformer(un)
	require.NoError(t, err)
	assert.Equal(t, un, out)
}

func TestCachedResource_DeepCopyObjectReturnsShellCopy(t *testing.T) {
	cache := newTransformTestCache(t)
	out, err := cache.transformForInformer(samplePod())
	require.NoError(t, err)

	orig := out.(*cachedResource)
	copied := orig.DeepCopyObject().(*cachedResource)

	require.NotSame(t, orig, copied, "shell must be a new allocation")
	assert.Same(t, orig.Resource, copied.Resource, "Resource is shared by design — see transform.go")
	assert.Equal(t, orig.GetName(), copied.GetName())
	assert.Equal(t, orig.APIVersion, copied.APIVersion)
}

func TestCachedResource_DeepCopyObjectNilSafe(t *testing.T) {
	var cr *cachedResource
	assert.Nil(t, cr.DeepCopyObject())
}

func TestCachedResource_SatisfiesRuntimeObject(t *testing.T) {
	// Compile-time sanity — cachedResource must be a runtime.Object so it
	// can live in an informer store.
	var _ runtime.Object = (*cachedResource)(nil)
}
