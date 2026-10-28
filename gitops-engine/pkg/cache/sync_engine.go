package cache

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// syncEngine is how a cluster cache loads and watches resources. Shared state
// and the read paths live on store; an engine reaches them through its *store
// and never references clusterCache or the other engine.
//
// There are two implementations, chosen once at construction by SetMode:
// legacyEngine (list then watch per GroupKind and namespace) and
// informerEngine (a client-go shared informer per GroupKind and namespace;
// see #19199). Each carries its own discovery and sync preamble rather than
// sharing one behind a mode branch, so either reads on its own.
type syncEngine interface {
	// ensureSynced is EnsureSynced's slow path, called with no lock held once
	// the unlocked check finds the cache unsynced. It takes c.lock before
	// syncStatus.lock (see clusterCacheSync), re-checks synced(), fully
	// (re)loads the cluster and starts every watch, and records syncTime and
	// syncError. The engine decides how long it holds each lock: it may
	// release c.lock during the load (informerEngine does, around its
	// initial-list wait), but must never hold syncStatus.lock while c.lock is
	// released, because GetClusterInfo takes them in that order.
	//
	// Because informerEngine releases c.lock after wiping the index, a reader
	// that skips EnsureSynced can see a partially loaded cache during an
	// informer-mode rebuild (first sync, after Invalidate, periodic resync).
	// Readers that call EnsureSynced first wait for the sync to finish.
	ensureSynced() error
	// startMissingWatches starts a watch for every discovered GroupKind not
	// already watched, so new CRDs sync without a full resync. Caller must hold
	// c.lock.
	startMissingWatches() error
	// onInvalidate cancels the engine's watches and drops its runtime state, so
	// Invalidate needn't know what the engine keeps. Invalidate calls it under
	// c.lock, before applying its opts.
	onInvalidate()
}

// stopWatching isn't on syncEngine: only the engine calls it, and what it tears
// down depends on how the engine watches. legacyEngine stops one (GroupKind,
// namespace) watch; informerEngine currently stops every namespace's informer
// for the GroupKind, since they share one watch context.

// newSyncEngine returns the engine for mode, bound to s. Any mode other than
// ModeInformer gets the legacy engine.
func newSyncEngine(mode Mode, s *store) syncEngine {
	if mode == ModeInformer {
		return &informerEngine{
			c:         s,
			informers: map[schema.GroupKind]map[string]sharedInformer{},
		}
	}
	return &legacyEngine{c: s}
}
