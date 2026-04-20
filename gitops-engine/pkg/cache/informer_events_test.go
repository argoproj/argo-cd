package cache

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr/funcr"
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

// TestOnInformerChange_ModifiedWithNilNewCrIsNoop guards against a
// nil-pointer deref in the watch.Added/Modified branch when newObj fails
// the *cachedResource type assertion. The primary-only nil check above
// the switch passes because oldCr is valid; without the in-branch guard
// the dereference of newCr.Resource.ResourceKey() crashes the controller.
func TestOnInformerChange_ModifiedWithNilNewCrIsNoop(t *testing.T) {
	c := newTransformTestCache(t)
	captured := recordingResourceHandlers(c)

	// Seed an old entry via a normal Add so oldCr passes the type assertion.
	oldCr, err := c.transformForInformer(samplePod())
	require.NoError(t, err)
	c.informerEventHandler().OnAdd(oldCr, false)
	require.Len(t, *captured, 1)

	key := kube.NewResourceKey("", "Pod", "default", "nginx")
	c.lock.RLock()
	pre := c.resources[key]
	c.lock.RUnlock()
	require.NotNil(t, pre, "sanity: pod was Added")

	// Fire OnUpdate with a newObj that isn't a *cachedResource — newCr will
	// be nil after the type assertion, but oldCr is valid so primary=oldCr
	// and the entry-level guard passes. Pre-fix: nil-deref panic.
	require.NotPanics(t, func() {
		c.informerEventHandler().OnUpdate(oldCr, "not a cachedResource")
	})

	// And nothing should have changed in storage either.
	c.lock.RLock()
	post := c.resources[key]
	c.lock.RUnlock()
	assert.Same(t, pre, post, "non-cachedResource newObj must not mutate c.resources")
	assert.Len(t, *captured, 1, "non-cachedResource newObj must not fire OnResourceUpdated")
}

// TestHandleCRDEvent_LogsInnerCRDGroupKind verifies that the log line
// emitted on a CRD watch event identifies the CRD by the GroupKind of
// the resource it defines (e.g. "stable.example.com/CronTab"), not by
// the apiextensions.k8s.io/CustomResourceDefinition wrapper kind. Pre-fix
// the log read `obj.GroupVersionKind().GroupKind()` which is always the
// wrapper — useless for operators triaging schema-reload churn.
func TestHandleCRDEvent_LogsInnerCRDGroupKind(t *testing.T) {
	var (
		mu      sync.Mutex
		logBuf  strings.Builder
		capture = func(prefix, args string) {
			mu.Lock()
			defer mu.Unlock()
			logBuf.WriteString(prefix)
			logBuf.WriteString(" ")
			logBuf.WriteString(args)
			logBuf.WriteString("\n")
		}
	)
	c := newTransformTestCache(t)
	c.log = funcr.New(capture, funcr.Options{Verbosity: 1})

	crd := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata":   map[string]any{"name": "crontabs.stable.example.com"},
		"spec": map[string]any{
			"group": "stable.example.com",
			"names": map[string]any{"kind": "CronTab", "plural": "crontabs", "singular": "crontab"},
			"scope": "Namespaced",
			"versions": []any{
				map[string]any{"name": "v1", "served": true, "storage": true},
			},
		},
	}}

	// Modified is the path that hits the log line (Added/Modified branch).
	// startMissingWatches and reloadOpenAPISchema will error because the
	// MockKubectlCmd here has no APIResources, but handleCRDEvent logs
	// before that — we don't care about the downstream errors.
	c.handleCRDEvent(watch.Modified, crd)

	mu.Lock()
	logged := logBuf.String()
	mu.Unlock()
	// schema.GroupKind.String() formats as "Kind.Group", e.g. "CronTab.stable.example.com".
	assert.Contains(t, logged, "CronTab.stable.example.com",
		"log line should carry the inner CRD GroupKind, not the apiextensions wrapper. got: %s", logged)
	assert.NotContains(t, logged, "CustomResourceDefinition.apiextensions.k8s.io",
		"log line should NOT identify the wrapper kind. got: %s", logged)
}

// TestHandleCRDEvent_LogsInnerCRDGroupKindOnDecodeFailure falls back to
// the CRD's metadata.name when spec.versions decode fails — better than
// the wrapper kind, since the name identifies which CRD object failed
// (e.g. "crontabs.stable.example.com") even when its inner group/kind
// can't be extracted.
func TestHandleCRDEvent_LogsInnerCRDGroupKindOnDecodeFailure(t *testing.T) {
	var (
		mu      sync.Mutex
		logBuf  strings.Builder
		capture = func(prefix, args string) {
			mu.Lock()
			defer mu.Unlock()
			logBuf.WriteString(prefix)
			logBuf.WriteString(" ")
			logBuf.WriteString(args)
			logBuf.WriteString("\n")
		}
	)
	c := newTransformTestCache(t)
	c.log = funcr.New(capture, funcr.Options{Verbosity: 1})

	// spec.versions is the wrong type — DefaultUnstructuredConverter.FromUnstructured
	// returns an error and crdVersionsToAPIResources yields an empty slice.
	badCRD := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata":   map[string]any{"name": "broken.example.com"},
		"spec": map[string]any{
			"group":    "example.com",
			"versions": "not-a-list",
		},
	}}
	c.handleCRDEvent(watch.Modified, badCRD)

	mu.Lock()
	logged := logBuf.String()
	mu.Unlock()
	assert.Contains(t, logged, "broken.example.com",
		"on decode failure the log should still identify the CRD by metadata.name. got: %s", logged)
}

// TestOnInformerChange_FiresOnProcessEventsHandler verifies that under
// informer mode, OnProcessEventsHandler is invoked per event with a
// real duration and count=1. Without this, argocd_resource_events_processing
// flatlines the moment ModeInformer is enabled — argo-cd's controller
// registers the handler at controller/cache/cache.go:663 to feed the
// histogram, and dashboards built on it would silently break on rollout.
//
// Legacy fires this from processEventsBatch with a batch duration; informer
// mode has no batching, so we report per-event durations with count=1.
func TestOnInformerChange_FiresOnProcessEventsHandler(t *testing.T) {
	c := newTransformTestCache(t)

	type capture struct {
		duration time.Duration
		count    int
	}
	var captured []capture
	c.processEventsHandlers = map[uint64]OnProcessEventsHandler{
		1: func(duration time.Duration, processedEventsNumber int) {
			captured = append(captured, capture{duration: duration, count: processedEventsNumber})
		},
	}

	cr, err := c.transformForInformer(samplePod())
	require.NoError(t, err)

	c.informerEventHandler().OnAdd(cr, false)
	c.informerEventHandler().OnDelete(cr)

	require.Len(t, captured, 2,
		"OnProcessEventsHandler must fire once per informer event under ModeInformer")
	for i, c := range captured {
		assert.Equal(t, 1, c.count, "event %d: informer mode reports per-event count=1", i)
		assert.GreaterOrEqual(t, c.duration, time.Duration(0),
			"event %d: duration must be non-negative", i)
	}
}

// TestOnInformerChange_FiresOnProcessEventsHandlerEvenForInitialList
// guards against future regressions that gate the handler invocation on
// !isInInitialList. The histogram should track every event the cache
// processes, including initial-list ones — that's where the bulk of the
// startup latency lives and operators need visibility into it.
func TestOnInformerChange_FiresOnProcessEventsHandlerEvenForInitialList(t *testing.T) {
	c := newTransformTestCache(t)

	var count int
	c.processEventsHandlers = map[uint64]OnProcessEventsHandler{
		1: func(_ time.Duration, _ int) { count++ },
	}

	cr, err := c.transformForInformer(samplePod())
	require.NoError(t, err)
	c.informerEventHandler().OnAdd(cr, true)

	assert.Equal(t, 1, count,
		"OnProcessEventsHandler must fire for isInInitialList events too — that's the bulk of startup work")
}

// TestOnInformerChange_ModifiedEndpointsSkipsStorageAndDispatch verifies
// the skipAppRequeuing parity gate under informer mode: Modified events
// on Endpoints (and other ignoredRefreshResources kinds) must skip the
// c.resources/nsIndex write AND OnResourceUpdated dispatch — exactly like
// legacy recordEvent's `if event == watch.Modified && skipAppRequeuing(key)
// { return }` at cluster.go:1678. OnEvent still fires (legacy fires it
// before the skip-gate as well).
func TestOnInformerChange_ModifiedEndpointsSkipsStorageAndDispatch(t *testing.T) {
	c := newTransformTestCache(t)
	informerEngineOf(c).firstSyncCompleted = true // make OnResourceUpdated paths active

	eventCaptured := recordingHandlers(c)
	resCaptured := recordingResourceHandlers(c)

	endpoints := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Endpoints",
		"metadata":   map[string]any{"name": "kubernetes", "namespace": "default", "uid": "ep-uid"},
	}}
	oldCr, err := c.transformForInformer(endpoints)
	require.NoError(t, err)
	newCr, err := c.transformForInformer(endpoints)
	require.NoError(t, err)

	// Modified — should NOT update storage, should NOT fire OnResourceUpdated.
	c.informerEventHandler().OnUpdate(oldCr, newCr)

	key := kube.NewResourceKey("", "Endpoints", "default", "kubernetes")
	c.lock.RLock()
	_, present := c.resources[key]
	c.lock.RUnlock()
	assert.False(t, present, "Modified Endpoints must not write to c.resources (skipAppRequeuing parity)")
	assert.Empty(t, *resCaptured, "Modified Endpoints must not fire OnResourceUpdated")

	// OnEvent must still fire — legacy recordEvent fires it before the skip-gate.
	require.Len(t, *eventCaptured, 1)
	assert.Equal(t, watch.Modified, (*eventCaptured)[0].Event)
	assert.Equal(t, "Endpoints", (*eventCaptured)[0].Un.GetKind())

	// Added Endpoints DO update storage (skipAppRequeuing only suppresses Modified).
	c.informerEventHandler().OnAdd(newCr, false)
	c.lock.RLock()
	_, present = c.resources[key]
	c.lock.RUnlock()
	assert.True(t, present, "Added Endpoints should update c.resources — only Modified is suppressed")
	assert.Len(t, *resCaptured, 1, "Added Endpoints fires OnResourceUpdated")
}
