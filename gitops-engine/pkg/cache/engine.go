package cache

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// syncEngine encapsulates the mode-specific cluster cache lifecycle: how
// resources are initially listed, how watches (or informers) are started for
// newly discovered APIs, and how a single GroupKind watch is torn down.
//
// Everything shared — the resource index (store.resources / store.nsIndex /
// store.parentUIDToChildren), the cross-GK index maintenance, the read paths
// (FindResources, IterateHierarchyV2, GetManagedLiveObjs), CRD handling, and
// the discovery primitives (processApi, listResources, checkPermission) —
// lives on *store. Each engine holds a *store and reaches shared state through
// it; neither engine references the outer clusterCache facade, nor the other
// engine.
//
// The two implementations are deliberately self-contained: each carries its
// own discovery/sync preamble and its own per-engine runtime state rather
// than sharing one behind a mode branch. That duplication is the accepted
// cost of keeping the two code paths fully separated and independently
// readable (see issue #19199).
type syncEngine interface {
	// sync performs a full (re)synchronization of the cluster cache: fetch
	// cluster metadata, discover APIs, and start watching every monitored
	// resource. On return the cache is populated and watches are running.
	//
	// Locking: called with store.lock held (EnsureSynced). The legacy engine
	// holds the lock for the entire call; the informer engine RELEASES and
	// re-acquires it around its initial-list wait (see syncInformers), so
	// callers must not assume store state is unmodified across sync(). That
	// window is why clusterCache.syncMu serializes EnsureSynced and why an
	// Invalidate landing mid-sync is surfaced as errCacheInvalidatedMidSync.
	// A new engine implementation may pick either behavior, but a new call
	// site must tolerate the lock being released.
	sync() error
	// startMissingWatches discovers the current API surface and starts a
	// watch for every GroupKind not already being watched. Invoked on the
	// CRD add/change path (handleCRDEvent) so newly registered custom
	// resources begin syncing without a full re-sync. Caller holds store.lock.
	startMissingWatches() error
	// onInvalidate drops the engine's per-invalidate runtime state. Called by
	// clusterCache.Invalidate under store.lock so the facade never has to know
	// which engine-private fields exist (legacy retires its batched-event
	// channel; informer resets its first-sync flag and informer index).
	onInvalidate()
}

// Note: stopWatching (tearing down the watch/informer set for a GroupKind) is
// deliberately NOT part of the interface. Its semantics differ per engine —
// legacy tears down a single (GroupKind, namespace) watch, while the informer
// engine ignores the namespace and purges every namespace for the GroupKind
// (all its informers share one watch context) — and every production caller is
// the owning engine itself, so exposing it polymorphically would only invite
// callers to rely on whichever semantics they happened to test against.

// newSyncEngine returns the cluster cache lifecycle implementation for the
// given mode, bound to the shared store. Defaults to the legacy list/watch
// engine.
func newSyncEngine(mode Mode, s *store) syncEngine {
	if mode == ModeInformer {
		return &informerEngine{
			c:         s,
			informers: map[schema.GroupKind]map[string]sharedInformer{},
		}
	}
	return &legacyEngine{c: s}
}
