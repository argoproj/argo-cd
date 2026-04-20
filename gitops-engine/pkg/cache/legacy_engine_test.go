package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
)

// TestLegacyStartMissingWatches_LazyInitsApisMetaAfterInvalidate is the legacy
// twin of TestStartInformersForAPILocked_LazyInitsApisMetaAfterInvalidate:
// apisMeta is nil between Invalidate and the next sync, and handleCRDEvent can
// fire from a still-draining pre-Invalidate watch goroutine and route through
// startMissingWatches in that window. Pre-fix this panicked with "assignment
// to entry in nil map" (recovered into watch-retry noise, CRD event lost).
func TestLegacyStartMissingWatches_LazyInitsApisMetaAfterInvalidate(t *testing.T) {
	cluster := newCluster(t)

	// Reproduce post-Invalidate state: apisMeta is nil while sync has not yet
	// rebuilt it. startMissingWatches' contract is "caller holds store.lock".
	require.NotPanics(t, func() {
		cluster.lock.Lock()
		defer cluster.lock.Unlock()
		cluster.apisMeta = nil
		require.NoError(t, legacyEngineOf(cluster).startMissingWatches())
	})

	cluster.lock.RLock()
	defer cluster.lock.RUnlock()
	require.NotNil(t, cluster.apisMeta, "startMissingWatches should lazy-init apisMeta")
	assert.NotEmpty(t, cluster.apisMeta, "the discovered GroupKinds should now be watched")
}

// TestLegacySync_ResetsNsIndex verifies that a full re-sync drops nsIndex
// entries for resources that disappeared while watches were down. sync()
// rebuilds c.resources from a fresh list but setNode/updateIndexes only ever
// ADD to nsIndex — without the reset, FindResources and IterateHierarchyV2
// (which read nsIndex directly) would keep returning ghost resources forever.
func TestLegacySync_ResetsNsIndex(t *testing.T) {
	cluster := newCluster(t, testPod1())
	require.NoError(t, cluster.EnsureSynced())

	podKey := kube.GetResourceKey(mustToUnstructured(testPod1()))
	ghostKey := kube.NewResourceKey("", "Pod", "default", "ghost")

	// Simulate a resource that was indexed while watches were up but deleted
	// from the cluster while they were down (its Delete event never arrived).
	cluster.lock.Lock()
	require.Contains(t, cluster.nsIndex["default"], podKey, "sanity: synced pod is namespace-indexed")
	cluster.nsIndex["default"][ghostKey] = cluster.resources[podKey]
	cluster.lock.Unlock()

	// Invalidate deliberately preserves nsIndex (stale-but-present reads until
	// the next sync); the re-sync itself must drop the ghost.
	cluster.Invalidate()
	require.NoError(t, cluster.EnsureSynced())

	cluster.lock.RLock()
	defer cluster.lock.RUnlock()
	assert.Contains(t, cluster.nsIndex["default"], podKey, "live resource survives the re-sync")
	assert.NotContains(t, cluster.nsIndex["default"], ghostKey, "ghost entry must not survive the re-sync")
}

// TestLegacyRecordEvent_UnblocksParkedSenderOnInvalidate pins the batched-event
// teardown contract: Invalidate cannot guarantee watch goroutines have stopped
// (context cancellation does not unpark a goroutine blocked in a channel
// send), so invalidateEventMeta retires the channel generation via eventsDone
// instead of closing the channel under the parked sender. Pre-fix this
// panicked with "send on closed channel".
func TestLegacyRecordEvent_UnblocksParkedSenderOnInvalidate(t *testing.T) {
	cluster := newClusterWithOptions(t, []UpdateSettingsFunc{SetBatchEventsProcessing(true)})
	t.Cleanup(func() { cluster.Invalidate() })
	engine := legacyEngineOf(cluster)

	// Install a channel generation with NO consumer, so the sender parks in
	// the send exactly as it would when processEvents is busy elsewhere.
	cluster.lock.Lock()
	engine.eventMetaCh = make(chan eventMeta)
	engine.eventsDone = make(chan struct{})
	cluster.lock.Unlock()

	unblocked := make(chan struct{})
	go func() {
		engine.recordEvent(watch.Added, unstructuredPod("nginx", "default"))
		close(unblocked)
	}()

	// Let the goroutine park in the channel send before invalidating.
	time.Sleep(50 * time.Millisecond)
	cluster.lock.Lock()
	engine.invalidateEventMeta()
	cluster.lock.Unlock()

	select {
	case <-unblocked:
	case <-time.After(2 * time.Second):
		t.Fatal("recordEvent sender still parked after invalidateEventMeta")
	}

	// After retirement there is no consumer generation; recordEvent must
	// return immediately (dropping the event) rather than blocking.
	done := make(chan struct{})
	go func() {
		engine.recordEvent(watch.Added, unstructuredPod("nginx2", "default"))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("recordEvent blocked after the channel generation was retired")
	}
}

// TestLegacyStopWatching_RemovesNamespacedResourcesEntry verifies that tearing
// down a GroupKind's watch also stops advertising it: a GK absent from
// apisMeta must not linger in namespacedResources (IsNamespaced et al).
func TestLegacyStopWatching_RemovesNamespacedResourcesEntry(t *testing.T) {
	cluster := newCluster(t, testPod1())
	require.NoError(t, cluster.EnsureSynced())

	gk := podsGVK.GroupKind()
	cluster.lock.RLock()
	_, watched := cluster.apisMeta[gk]
	_, advertised := cluster.namespacedResources[gk]
	cluster.lock.RUnlock()
	require.True(t, watched, "sanity: pods are watched after sync")
	require.True(t, advertised, "sanity: pods are advertised after sync")

	legacyEngineOf(cluster).stopWatching(gk, "default")

	cluster.lock.RLock()
	defer cluster.lock.RUnlock()
	_, watched = cluster.apisMeta[gk]
	_, advertised = cluster.namespacedResources[gk]
	assert.False(t, watched, "stopWatching removes the GK from apisMeta")
	assert.False(t, advertised, "stopWatching must also remove the GK from namespacedResources")
}

// TestHandleCRDEvent_DispatchesToCurrentEngine guards the engine-swap race:
// a watch goroutine spawned by the legacy engine can outlive
// Invalidate(SetMode(ModeInformer)) and deliver a CRD event afterwards.
// handleCRDEvent must resolve the engine AT CALL TIME (store.currentEngine)
// so startMissingWatches runs on the active informer engine — dispatching on
// the goroutine's own (stale) engine would restart legacy watch machinery
// alongside the informer engine's on the same store.
func TestHandleCRDEvent_DispatchesToCurrentEngine(t *testing.T) {
	cluster := newCluster(t)
	require.NoError(t, cluster.EnsureSynced())

	// Swap engines the way a library caller would.
	cluster.Invalidate(SetMode(ModeInformer))

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

	// The stale legacy watch goroutine's call site is c.handleCRDEvent(...) —
	// exactly this, with no engine threaded through.
	require.NotPanics(t, func() { cluster.handleCRDEvent(watch.Added, crd) })

	cluster.lock.RLock()
	defer cluster.lock.RUnlock()
	_, isInformer := cluster.engine.(*informerEngine)
	require.True(t, isInformer, "sanity: the active engine is the informer engine")
	assert.NotEmpty(t, informerEngineOf(cluster).informers,
		"startMissingWatches must have run on the ACTIVE (informer) engine — its private informer index is populated")
}
