package cache

// syncEngine is how a cluster cache loads and watches resources. Shared state
// and the read paths live on store; an engine reaches them through its *store
// and never references clusterCache.
//
// legacyEngine is the only implementation. EnsureSynced hands its locked path
// to ensureSynced, so an engine that releases c.lock during sync, as #27998
// does around WaitForCacheSync, needn't rework EnsureSynced's locking.
type syncEngine interface {
	// ensureSynced is EnsureSynced's slow path, called with no lock held once
	// the unlocked check finds the cache unsynced. It takes c.lock before
	// syncStatus.lock (see clusterCacheSync), re-checks synced(), fully
	// (re)loads the cluster and starts every watch, and records syncTime and
	// syncError. The engine decides how long it holds each lock.
	ensureSynced() error
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
