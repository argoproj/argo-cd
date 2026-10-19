package cache

// syncEngine is how a cluster cache loads and watches resources. Shared state
// and the read paths live on store; an engine reaches them through its *store
// and never references clusterCache.
//
// legacyEngine is the only implementation. The interface makes that boundary
// explicit, but doesn't make engines drop-in: EnsureSynced holds c.lock for the
// whole of sync. An engine that releases c.lock during sync, as #27998 does
// around WaitForCacheSync, must also rework EnsureSynced's locking.
type syncEngine interface {
	// sync fully (re)loads the cluster and starts every watch. It's called with
	// c.lock held, and callers assume store state doesn't change during it.
	sync() error
	// startMissingWatches starts a watch for every discovered GroupKind not
	// already watched, so new CRDs sync without a full resync. Caller must hold
	// c.lock.
	startMissingWatches() error
	// onInvalidate drops the engine's runtime state, so Invalidate needn't know
	// what the engine keeps.
	onInvalidate()
}

// stopWatching isn't on syncEngine: only the engine calls it, and what it tears
// down depends on how the engine watches.
