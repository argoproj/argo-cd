package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
)

// recordingHandlers swaps in a fresh handler map on a clusterCache and
// returns a slice that captures every OnEvent invocation.
func recordingHandlers(c *clusterCache) *[]eventCapture {
	captured := make([]eventCapture, 0)
	c.eventHandlers = map[uint64]OnEventHandler{
		1: func(t watch.EventType, un *unstructured.Unstructured) {
			captured = append(captured, eventCapture{t, un})
		},
	}
	return &captured
}

type eventCapture struct {
	Event watch.EventType
	Un    *unstructured.Unstructured
}

func TestInformerEventHandler_AddDispatchesAdded(t *testing.T) {
	c := newTransformTestCache(t)
	captured := recordingHandlers(c)

	cr, err := c.transformForInformer(samplePod())
	require.NoError(t, err)

	c.informerEventHandler().OnAdd(cr, false)

	require.Len(t, *captured, 1)
	assert.Equal(t, watch.Added, (*captured)[0].Event)
	assert.Equal(t, "nginx", (*captured)[0].Un.GetName())
}

func TestInformerEventHandler_UpdateDispatchesModified(t *testing.T) {
	c := newTransformTestCache(t)
	captured := recordingHandlers(c)

	oldCr, err := c.transformForInformer(samplePod())
	require.NoError(t, err)
	newPod := samplePod()
	newPod.SetResourceVersion("43")
	newCr, err := c.transformForInformer(newPod)
	require.NoError(t, err)

	c.informerEventHandler().OnUpdate(oldCr, newCr)

	require.Len(t, *captured, 1)
	assert.Equal(t, watch.Modified, (*captured)[0].Event)
	assert.Equal(t, "43", (*captured)[0].Un.GetResourceVersion())
}

func TestInformerEventHandler_DeleteDispatchesDeleted(t *testing.T) {
	c := newTransformTestCache(t)
	captured := recordingHandlers(c)

	cr, err := c.transformForInformer(samplePod())
	require.NoError(t, err)

	c.informerEventHandler().OnDelete(cr)

	require.Len(t, *captured, 1)
	assert.Equal(t, watch.Deleted, (*captured)[0].Event)
	assert.Equal(t, "nginx", (*captured)[0].Un.GetName())
}

func TestInformerEventHandler_DeleteUnwrapsTombstone(t *testing.T) {
	c := newTransformTestCache(t)
	captured := recordingHandlers(c)

	cr, err := c.transformForInformer(samplePod())
	require.NoError(t, err)
	tomb := cache.DeletedFinalStateUnknown{Key: "default/nginx", Obj: cr}

	c.informerEventHandler().OnDelete(tomb)

	require.Len(t, *captured, 1)
	assert.Equal(t, watch.Deleted, (*captured)[0].Event)
	assert.Equal(t, "nginx", (*captured)[0].Un.GetName())
}

func TestInformerEventHandler_IgnoresUnexpectedType(t *testing.T) {
	c := newTransformTestCache(t)
	captured := recordingHandlers(c)

	c.informerEventHandler().OnAdd("not a cachedResource", false)

	assert.Empty(t, *captured)
}

func TestInformerEventHandler_UsesCachedManifestWhenPresent(t *testing.T) {
	c := newTransformTestCache(t)
	c.populateResourceInfoHandler = func(_ *unstructured.Unstructured, _ bool) (any, bool) {
		return nil, true // cacheManifest=true — keep the full manifest
	}
	captured := recordingHandlers(c)

	cr, err := c.transformForInformer(samplePod())
	require.NoError(t, err)
	require.NotNil(t, cr.(*cachedResource).Resource.Resource)

	c.informerEventHandler().OnAdd(cr, false)

	require.Len(t, *captured, 1)
	// When the manifest is cached, the original un (with spec/status) reaches handlers.
	phase, found, err := unstructured.NestedString((*captured)[0].Un.Object, "status", "phase")
	require.NoError(t, err)
	require.True(t, found, "cached manifest should carry status.phase through to handlers")
	assert.Equal(t, "Running", phase)
}

func TestInformerEventHandler_SynthesizesUnstructuredWhenManifestDropped(t *testing.T) {
	c := newTransformTestCache(t)
	captured := recordingHandlers(c) // no cacheManifest → Resource.Resource is nil

	cr, err := c.transformForInformer(samplePod())
	require.NoError(t, err)
	require.Nil(t, cr.(*cachedResource).Resource.Resource)

	c.informerEventHandler().OnAdd(cr, false)

	require.Len(t, *captured, 1)
	un := (*captured)[0].Un
	// Synthesized object carries only GVK + metadata; spec/status are absent.
	assert.Equal(t, "Pod", un.GetKind())
	assert.Equal(t, "v1", un.GetAPIVersion())
	assert.Equal(t, "nginx", un.GetName())
	assert.Equal(t, "default", un.GetNamespace())
	_, found, _ := unstructured.NestedMap(un.Object, "status")
	assert.False(t, found, "synthesized unstructured must not carry spec/status")
}

// resourceCapture pairs new and old Resources captured by an
// OnResourceUpdatedHandler. Used to assert dispatch correctness.
type resourceCapture struct {
	New *Resource
	Old *Resource
	Ns  map[kube.ResourceKey]*Resource
}

func recordingResourceHandlers(c *clusterCache) *[]resourceCapture {
	captured := make([]resourceCapture, 0)
	c.resourceUpdatedHandlers = map[uint64]OnResourceUpdatedHandler{
		1: func(newRes, oldRes *Resource, ns map[kube.ResourceKey]*Resource) {
			captured = append(captured, resourceCapture{New: newRes, Old: oldRes, Ns: ns})
		},
	}
	return &captured
}

func TestInformerEventHandler_AddUpdatesIndexes(t *testing.T) {
	c := newTransformTestCache(t)
	captured := recordingResourceHandlers(c)

	cr, err := c.transformForInformer(samplePod())
	require.NoError(t, err)

	c.informerEventHandler().OnAdd(cr, false)

	key := kube.NewResourceKey("", "Pod", "default", "nginx")

	// nsIndex populated for the resource's namespace.
	ns, ok := c.nsIndex["default"]
	require.True(t, ok)
	assert.Len(t, ns, 1)
	assert.Contains(t, ns, key)

	// c.resources shadow populated — needed so existing read paths
	// (GetManagedLiveObjs, IterateHierarchyV2) work unchanged.
	shadow, ok := c.resources[key]
	require.True(t, ok, "c.resources should be populated as a shadow of the informer store")
	assert.Equal(t, "nginx", shadow.Ref.Name)

	// OnResourceUpdated fired with newRes set, oldRes nil.
	require.Len(t, *captured, 1)
	assert.NotNil(t, (*captured)[0].New)
	assert.Nil(t, (*captured)[0].Old)
	assert.Equal(t, "nginx", (*captured)[0].New.Ref.Name)
}

func TestInformerEventHandler_UpdatePassesOldAndNew(t *testing.T) {
	c := newTransformTestCache(t)
	captured := recordingResourceHandlers(c)

	oldCr, err := c.transformForInformer(samplePod())
	require.NoError(t, err)
	c.informerEventHandler().OnAdd(oldCr, false)

	newPod := samplePod()
	newPod.SetResourceVersion("43")
	newCr, err := c.transformForInformer(newPod)
	require.NoError(t, err)
	c.informerEventHandler().OnUpdate(oldCr, newCr)

	require.Len(t, *captured, 2)
	update := (*captured)[1]
	require.NotNil(t, update.Old)
	require.NotNil(t, update.New)
	assert.Equal(t, "42", update.Old.ResourceVersion)
	assert.Equal(t, "43", update.New.ResourceVersion)
}

func TestInformerEventHandler_DeleteRemovesFromIndexes(t *testing.T) {
	c := newTransformTestCache(t)
	captured := recordingResourceHandlers(c)

	cr, err := c.transformForInformer(samplePod())
	require.NoError(t, err)
	c.informerEventHandler().OnAdd(cr, false)
	require.Len(t, c.nsIndex, 1)
	require.Len(t, c.resources, 1)

	c.informerEventHandler().OnDelete(cr)

	// Namespace entry pruned once empty.
	_, ok := c.nsIndex["default"]
	assert.False(t, ok, "nsIndex entry should be removed when last resource in namespace is deleted")

	// c.resources shadow cleared.
	assert.Empty(t, c.resources, "c.resources should be pruned on delete")

	// OnResourceUpdated fired with newRes=nil, oldRes=the deleted Resource.
	require.Len(t, *captured, 2)
	del := (*captured)[1]
	assert.Nil(t, del.New)
	require.NotNil(t, del.Old)
	assert.Equal(t, "nginx", del.Old.Ref.Name)
}

func TestInformerEventHandler_ParentUIDToChildrenMaintained(t *testing.T) {
	c := newTransformTestCache(t)
	_ = recordingResourceHandlers(c)

	owner := samplePod()
	owner.SetName("parent")
	owner.SetUID("parent-uid")
	parentCr, err := c.transformForInformer(owner)
	require.NoError(t, err)
	c.informerEventHandler().OnAdd(parentCr, false)

	child := samplePod()
	child.SetName("child")
	child.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "v1", Kind: "Pod", Name: "parent", UID: "parent-uid",
	}})
	childCr, err := c.transformForInformer(child)
	require.NoError(t, err)
	c.informerEventHandler().OnAdd(childCr, false)

	// parentUIDToChildren index should show the parent→child link.
	children, ok := c.parentUIDToChildren["parent-uid"]
	require.True(t, ok, "parent-child relationship should be indexed")
	childKey := kube.NewResourceKey("", "Pod", "default", "child")
	assert.Contains(t, children, childKey)

	// Deleting the child removes the edge.
	c.informerEventHandler().OnDelete(childCr)
	children = c.parentUIDToChildren["parent-uid"]
	assert.NotContains(t, children, childKey)
}
