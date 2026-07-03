package cache

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubetypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
)

// informerEngine is the SharedIndexInformer-based cluster cache implementation
// (ModeInformer, experimental — see issue #19199). It replaces the legacy
// hand-rolled list/watch loop with one client-go informer per (GroupKind,
// namespace) and a TransformFunc that converts objects to cachedResource at
// intake.
//
// It reaches shared state (the resource index, discovery primitives, dispatch)
// through its *store; it never references the clusterCache facade or the legacy
// engine. firstSyncCompleted is the engine's own runtime state and lives here
// rather than on the store because only this engine consults it.
type informerEngine struct {
	c *store

	// informers holds one SharedIndexInformer per (GroupKind, namespace)
	// watched (empty-string key for cluster-scoped watches). It is the
	// engine's private companion to store.apisMeta — entries are added and
	// removed together with the corresponding apiMeta under store.lock, and
	// the index is cleared by onInvalidate when the facade nils apisMeta.
	// All of a GroupKind's informers share apiMeta.watchCancel's context, so
	// a single cancel tears them all down — matching legacy semantics where
	// one context covers every namespace-fanout watch. Lives here rather than
	// on apiMeta so the shared store layer carries no informer-only state.
	informers map[schema.GroupKind]map[string]sharedInformer

	// firstSyncCompleted is set true after the first syncInformers completes.
	// onInformerChange uses it to mirror legacy OnResourceUpdated semantics:
	//   - first global sync (legacy setNode direct path, no dispatch):
	//     suppress OnResourceUpdated for isInInitialList Add events, since the
	//     cache is just being populated and subscribers expect a quiet bulk
	//     load.
	//   - post-startup new-watch initial list (legacy startMissingWatches ->
	//     loadInitialState -> replaceResourceCache -> onNodeUpdated fires
	//     OnResourceUpdated): fire normally so CRD-driven new resource
	//     discovery triggers reconcile.
	// Reset to false by onInvalidate so the post-Invalidate rebuild also gets
	// the quiet-bulk-load semantics. Guarded by store.lock.
	firstSyncCompleted bool
}

// sync performs the informer-mode full (re)sync.
func (e *informerEngine) sync() error {
	c := e.c
	c.log.Info("Start syncing cluster (informer mode)")

	// Cancel any existing informers before rebuilding. watchCancel stops
	// every namespace watch for a GroupKind (shared context).
	for _, meta := range c.apisMeta {
		if meta != nil && meta.watchCancel != nil {
			meta.watchCancel()
		}
	}
	c.apisMeta = map[schema.GroupKind]*apiMeta{}
	e.informers = map[schema.GroupKind]map[string]sharedInformer{}
	c.resources = map[kube.ResourceKey]*Resource{}
	c.namespacedResources = map[schema.GroupKind]bool{}
	c.nsIndex = map[string]map[kube.ResourceKey]*Resource{}
	c.parentUIDToChildren = map[kubetypes.UID]map[kube.ResourceKey]struct{}{}

	version, err := c.kubectl.GetServerVersion(c.config)
	if err != nil {
		return fmt.Errorf("get server version: %w", err)
	}
	c.serverVersion = version

	apiResources, err := c.kubectl.GetAPIResources(c.config, false, NewNoopSettings())
	if err != nil {
		return fmt.Errorf("get api resources: %w", err)
	}
	c.apiResources = apiResources

	openAPISchema, gvkParser, err := c.kubectl.LoadOpenAPISchema(c.config)
	if err != nil {
		return fmt.Errorf("load openapi schema: %w", err)
	}
	if gvkParser != nil {
		c.gvkParser = gvkParser
	}
	c.openAPISchema = openAPISchema

	// Discovery + informer startup is shared with the CRD-driven path (the
	// apisMeta reset above makes its skip-already-watched check a no-op, so
	// every discovered API starts fresh).
	if err := e.startMissingWatches(); err != nil {
		return err
	}

	// Bound the wait by clusterSyncRetryTimeout so a single broken API
	// (forbidden, misbehaving aggregated API server, transient list
	// failure) does not block all reconciliation. Reflectors that haven't
	// synced keep retrying with backoff in the background; if the
	// underlying cause clears (e.g., RBAC granted), HasSynced flips true
	// and c.resources populates via the event handler — no further sync
	// work needed. The outer EnsureSynced loop runs again after
	// clusterSyncRetryTimeout, which tears these informers down and
	// starts a fresh attempt.
	waitCtx, cancelWait := context.WithTimeout(context.Background(), c.clusterSyncRetryTimeout)
	defer cancelWait()

	// Wait in short slices, re-snapshotting the watched informers between
	// them. Two reasons:
	//   - New informers can appear mid-wait when handleCRDEvent fires
	//     (CRD-driven startMissingWatches -> startInformersForAPILocked).
	//     Waiting only on the original snapshot would report success while
	//     the new informers are still doing their initial list.
	//   - Informers torn down mid-wait (stopWatching on NotFound/403) have
	//     a HasSynced that stays false forever — DeltaFIFO only flips it
	//     after an initial list that will now never happen. Waiting on the
	//     stale snapshot until waitCtx expires would fail the whole cluster
	//     sync over a GroupKind that was legitimately purged; re-snapshotting
	//     drops it from the wait set.
	//
	// Release store.lock during each wait — the event handler takes it when
	// informers dispatch their initial list. Holding it across the wait
	// would deadlock.
	var watched []watchedInformer
	for {
		watched = e.snapshotWatched()
		pending := unsyncedInformers(watched)
		if len(pending) == 0 {
			break
		}
		if waitCtx.Err() != nil {
			return e.resolveSyncResult(false, watched)
		}
		sliceCtx, cancelSlice := context.WithTimeout(waitCtx, informerSyncResnapshotInterval)
		c.lock.Unlock()
		// Result deliberately ignored: whether this slice synced everything,
		// timed out, or raced a teardown, the snapshot at the top of the loop
		// is the single source of truth for what is still pending.
		_ = cache.WaitForCacheSync(sliceCtx.Done(), pending...)
		cancelSlice()
		c.lock.Lock()
	}

	return e.resolveSyncResult(true, watched)
}

// onInvalidate resets first-sync tracking so the next syncInformers treats its
// initial list as a quiet bulk load, and drops the informer index (the facade
// has just cancelled every apiMeta.watchCancel and is about to nil apisMeta,
// so the informers are all stopping). The facade calls this under store.lock
// during Invalidate.
func (e *informerEngine) onInvalidate() {
	e.firstSyncCompleted = false
	e.informers = map[schema.GroupKind]map[string]sharedInformer{}
}

// startMissingWatches discovers the API surface and starts an informer set
// for every GroupKind not already watched. Both informer start paths route
// here: CRD-driven registration (via handleCRDEvent) and the full sync
// (syncInformers, where the preceding apisMeta reset makes the
// skip-already-watched check a no-op). Caller holds store.lock.
//
// Runs serially — the per-API work is mostly informer setup, not blocking
// operations; parallelism can come later if discovery-heavy clusters show it
// matters. Unlike the legacy path it needs no dynamic client or SSAR
// clientset here: startInformersForAPILocked builds its own client and RBAC
// is enforced by the per-informer watch error handler.
func (e *informerEngine) startMissingWatches() error {
	c := e.c
	apis, err := c.kubectl.GetAPIResources(c.config, true, c.settings.ResourcesFilter)
	if err != nil {
		return fmt.Errorf("failed to get APIResources: %w", err)
	}
	namespacedResources := make(map[schema.GroupKind]bool)
	for i := range apis {
		api := apis[i]
		namespacedResources[api.GroupKind] = api.Meta.Namespaced
		if _, ok := c.apisMeta[api.GroupKind]; !ok {
			if err := e.startInformersForAPILocked(context.Background(), api); err != nil {
				return fmt.Errorf("start informers for %s: %w", api.GroupKind, err)
			}
		}
	}
	c.namespacedResources = namespacedResources
	return nil
}

// stopWatching tears down every informer for a GroupKind and purges all of
// its namespaces' resources. Every per-(GK, namespace) informer shares the
// parent watchCancel — cancelling for one ns terminates them all — so we
// must purge every watched namespace, not just the failing one, or the
// others would keep stale resources forever with no informer feeding updates.
func (e *informerEngine) stopWatching(gk schema.GroupKind, _ string) {
	c := e.c
	c.lock.Lock()
	defer c.lock.Unlock()
	if info, ok := c.apisMeta[gk]; ok {
		info.watchCancel()
		nsToPurge := make([]string, 0, len(e.informers[gk]))
		for watchedNs := range e.informers[gk] {
			nsToPurge = append(nsToPurge, watchedNs)
		}
		delete(c.apisMeta, gk)
		delete(e.informers, gk)
		// Keep namespacedResources consistent with apisMeta: a GroupKind we no
		// longer watch must not keep being advertised (IsNamespaced et al).
		delete(c.namespacedResources, gk)
		for _, n := range nsToPurge {
			c.replaceResourceCache(gk, nil, n)
		}
		c.log.Info(fmt.Sprintf("Stop watching: %s not found", gk))
	}
}

// startInformersForAPI is the public entrypoint for starting the informer
// set for a GroupKind. It acquires store.lock and delegates to the locked
// variant. Used by tests; production call sites (syncInformers,
// startMissingWatches) hold the lock already and call the locked variant
// directly.
func (e *informerEngine) startInformersForAPI(ctx context.Context, api kube.APIResourceInfo) error {
	e.c.lock.Lock()
	defer e.c.lock.Unlock()
	return e.startInformersForAPILocked(ctx, api)
}

// startInformersForAPILocked builds and starts one SharedIndexInformer per
// (GroupKind, namespace) implied by processApi, installs transformForInformer
// as the TransformFunc, and attaches informerEventHandler. The informers
// share a single context derived from ctx; cancelling that context stops
// every namespace watch for the GroupKind together, matching legacy
// semantics.
//
// Idempotent — if apisMeta already has an entry for this GroupKind, the
// call is a no-op. Caller must hold store.lock.
//
// Does not block for initial sync; callers that need that should consult
// each informer's HasSynced via apisMeta.
func (e *informerEngine) startInformersForAPILocked(ctx context.Context, api kube.APIResourceInfo) error {
	c := e.c
	if _, exists := c.apisMeta[api.GroupKind]; exists {
		return nil
	}

	client, err := c.kubectl.NewDynamicClient(c.config)
	if err != nil {
		return fmt.Errorf("create dynamic client for %s: %w", api.GroupKind, err)
	}

	watchCtx, cancel := context.WithCancel(ctx)
	meta := &apiMeta{
		namespaced:  api.Meta.Namespaced,
		watchCancel: cancel,
	}
	informers := map[string]sharedInformer{}

	// Build every informer up front so a partial failure doesn't leave
	// apisMeta half-populated.
	err = c.processApi(client, api, func(resClient dynamic.ResourceInterface, ns string) error {
		informer, registration := e.buildInformer(watchCtx, client, resClient, api, ns)
		informers[ns] = sharedInformer{informer: informer, registration: registration, cancel: cancel}
		return nil
	})
	if err != nil {
		cancel()
		return fmt.Errorf("process api %s: %w", api.GroupKind, err)
	}

	// apisMeta is nil between Invalidate and the next syncInformers — and this
	// path can run during that gap when handleCRDEvent fires from a still-draining
	// pre-Invalidate informer goroutine and routes through startMissingWatches.
	// Lazy-init matches the namespacedResources guard below.
	if c.apisMeta == nil {
		c.apisMeta = map[schema.GroupKind]*apiMeta{}
	}
	c.apisMeta[api.GroupKind] = meta
	e.informers[api.GroupKind] = informers
	if c.namespacedResources == nil {
		c.namespacedResources = map[schema.GroupKind]bool{}
	}
	c.namespacedResources[api.GroupKind] = api.Meta.Namespaced

	for _, si := range informers {
		go si.informer.RunWithContext(watchCtx)
	}
	return nil
}

// sharedInformer couples a SharedIndexInformer with the per-entry cancel
// function. Today every entry for a GroupKind shares the parent
// apiMeta.watchCancel, so this cancel is effectively a duplicate — kept
// as a field so later work can adopt per-(GK, namespace) teardown without
// reshaping the type.
type sharedInformer struct {
	informer cache.SharedIndexInformer
	// registration is the engine event handler's registration on the
	// informer. Its HasSynced only flips true once the initial events have
	// been DELIVERED to the handler — i.e. once onInformerChange has
	// shadowed them into c.resources — which is the condition the sync wait
	// needs (informer.HasSynced covers only the informer's own store).
	registration cache.ResourceEventHandlerRegistration
	cancel       context.CancelFunc
}

// informerSyncResnapshotInterval is how long each WaitForCacheSync slice in
// syncInformers runs before the watched-informer set is re-snapshotted (to
// pick up informers added by CRD events and drop ones torn down by
// stopWatching). Long enough that the re-snapshot lock traffic is noise,
// short enough that a torn-down informer can't stall the sync for long.
const informerSyncResnapshotInterval = time.Second

// snapshotWatched returns the current set of (gk, ns, HasSynced) tuples
// across the engine's informer index. Caller must hold store.lock.
//
// The HasSynced used is the handler REGISTRATION's, not the informer's: the
// registration's flips true only after the initial events have been delivered
// to onInformerChange (populating the c.resources shadow), so a successful
// sync wait guarantees the shared read paths see the initial state.
func (e *informerEngine) snapshotWatched() []watchedInformer {
	var watched []watchedInformer
	for gk, informers := range e.informers {
		for ns, si := range informers {
			watched = append(watched, watchedInformer{gk: gk, ns: ns, hasSynced: si.registration.HasSynced})
		}
	}
	return watched
}

// unsyncedInformers extracts the HasSynced callbacks for informers that
// have NOT yet completed their initial list. Used by syncInformers' loop
// to wait only on informers that still need to sync — and to detect when
// all informers (including any added mid-wait by CRD-driven discovery)
// are done.
func unsyncedInformers(watched []watchedInformer) []cache.InformerSynced {
	pending := make([]cache.InformerSynced, 0, len(watched))
	for _, w := range watched {
		if !w.hasSynced() {
			pending = append(pending, w.hasSynced)
		}
	}
	return pending
}

// watchedInformer captures the (gk, ns, HasSynced) tuple snapshotted by
// syncInformers before releasing store.lock. Kept as a package-level type so
// resolveSyncResult can take a slice of it.
type watchedInformer struct {
	gk        schema.GroupKind
	ns        string
	hasSynced cache.InformerSynced
}

// resolveSyncResult interprets the outcome of WaitForCacheSync. Caller
// must hold store.lock.
//
// The Invalidate-check runs BEFORE the synced-branch by design. DeltaFIFO's
// HasSynced is sticky (vendor/k8s.io/client-go/tools/cache/delta_fifo.go) —
// once true it never resets, not even on watch-context cancellation. So if
// the informers completed their initial list before a concurrent Invalidate
// cancelled them, WaitForCacheSync returns synced=true even though
// apisMeta has just been cleared and c.resources reset. Returning nil in
// that case would cache "success" for clusterSyncRetryTimeout against an
// empty cache; instead, surface errCacheInvalidatedMidSync so EnsureSynced
// (which detects this sentinel via errors.Is) skips caching the result and
// the next call re-syncs immediately.
func (e *informerEngine) resolveSyncResult(synced bool, watched []watchedInformer) error {
	c := e.c
	if c.apisMeta == nil {
		return errCacheInvalidatedMidSync
	}
	if synced {
		// Mark first-sync done so subsequent CRD-driven new-watch initial
		// lists fire OnResourceUpdated (matching legacy startMissingWatches
		// -> loadInitialState -> replaceResourceCache -> onNodeUpdated path).
		// Before this point, onInformerChange suppresses OnResourceUpdated
		// for isInInitialList events so the bulk load stays quiet.
		e.firstSyncCompleted = true
		c.log.Info("Cluster successfully synced (informer mode)")
		return nil
	}

	// Return an error so callers don't operate against a partially
	// populated cache. EnsureSynced caches the error for
	// clusterSyncRetryTimeout, then retries sync() — which tears
	// these informers down and starts fresh. Stricter than legacy
	// (which silently drops forbidden GVRs from apisMeta and reports
	// success), but it prevents app reconciliation from running
	// against a half-loaded cluster view. Future work: lazy GVK
	// loading so a single broken GVR only fails apps that reference
	// it instead of the whole cluster.
	var pending []string
	for _, w := range watched {
		if !w.hasSynced() {
			pending = append(pending, fmt.Sprintf("%s[%s]", w.gk, namespaceDescription(w.ns)))
		}
	}
	return fmt.Errorf("informers did not complete initial list within %s: %v", c.clusterSyncRetryTimeout, pending)
}

// buildInformer creates a SharedIndexInformer wired with the engine's
// TransformFunc and event handler. Exposed as a method (rather than being
// inlined) so tests can exercise a single informer without spinning up the
// full startInformersForAPI lifecycle.
//
// ctx is the informer's owning watch context — when it is cancelled (via
// Invalidate or stopWatching), the attached event handler bails so that
// post-cancel events from a still-draining reflector goroutine cannot
// mutate fresh state owned by a subsequent syncInformers run.
//
// client is the dynamic client that resClient was derived from; it is
// consulted (via ToListWatcherWithWatchListSemantics below) for whether
// WatchList semantics are supported.
func (e *informerEngine) buildInformer(ctx context.Context, client dynamic.Interface, resClient dynamic.ResourceInterface, api kube.APIResourceInfo, ns string) (cache.SharedIndexInformer, cache.ResourceEventHandlerRegistration) {
	c := e.c
	lw := &cache.ListWatch{
		// Wrap List with store.listSemaphore so initial-list memory pressure
		// stays bounded across many (GK, ns) informers. Legacy listResources
		// does the same — without it, every informer's reflector calls List
		// concurrently and a discovery-heavy cluster (hundreds of GVRs) spikes
		// memory during startup. Reflector pager calls List once per page; the
		// semaphore holds for one page at a time, matching legacy behavior.
		ListWithContextFunc: func(ctx context.Context, o metav1.ListOptions) (runtime.Object, error) {
			if err := c.listSemaphore.Acquire(ctx, 1); err != nil {
				return nil, fmt.Errorf("acquire list semaphore for %s[%s]: %w", api.GroupKind, namespaceDescription(ns), err)
			}
			defer c.listSemaphore.Release(1)
			return resClient.List(ctx, o)
		},
		WatchFuncWithContext: func(ctx context.Context, o metav1.ListOptions) (watch.Interface, error) {
			return resClient.Watch(ctx, o)
		},
	}

	informer := cache.NewSharedIndexInformerWithOptions(
		// Wrap the ListWatch so WatchList (streaming initial events,
		// KEP-3157) capability is decided by the underlying client, the same
		// way dynamicinformer does it: real clients use watch-list when the
		// client-go feature gate and the server support it (the reflector
		// falls back to classic LIST/WATCH otherwise), while test fakes —
		// which never send the end-of-initial-events bookmark — declare
		// non-support and get the classic path. Without this wrapper our
		// closure-based ListWatch hides the client's capability marker and
		// the reflector would stream against fakes and hang.
		//
		// Note the listSemaphore above only bounds the classic paged-List
		// path; under watch-list the initial state streams object-by-object,
		// which doesn't produce the page-sized allocation spikes the
		// semaphore exists to bound.
		cache.ToListWatcherWithWatchListSemantics(lw, client),
		&unstructured.Unstructured{},
		cache.SharedIndexInformerOptions{
			// ResyncPeriod=0 disables the informer's own periodic full-list
			// resync; the reflector's watch bookmarks handle drift. The
			// legacy 10-minute watchResyncTimeout is intentionally dropped
			// here — see issue #19199.
			ResyncPeriod:      0,
			Indexers:          cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
			ObjectDescription: fmt.Sprintf("%s[%s]", api.GroupKind, namespaceDescription(ns)),
		},
	)

	// SetTransform must run before the informer starts; it only errors if
	// the informer is already running, which can't happen on a fresh one.
	if err := informer.SetTransform(e.transformForInformer); err != nil {
		panic(fmt.Errorf("unreachable: SetTransform on fresh informer: %w", err))
	}
	if err := informer.SetWatchErrorHandlerWithContext(e.informerWatchErrorHandler(api, ns)); err != nil {
		panic(fmt.Errorf("unreachable: SetWatchErrorHandler on fresh informer: %w", err))
	}
	// Keep the handler registration: its HasSynced covers DELIVERY of the
	// initial events to our handler (which shadows objects into c.resources
	// via onInformerChange), not just population of the informer's own store
	// like informer.HasSynced does. The sync wait must use it, or
	// EnsureSynced can return success while the shadow maps are still being
	// filled by the shared processor's async dispatch.
	registration, err := informer.AddEventHandler(e.informerEventHandlerForCtx(ctx))
	if err != nil {
		panic(fmt.Errorf("unreachable: AddEventHandler on fresh informer: %w", err))
	}
	return informer, registration
}

// informerWatchErrorHandler is installed on every informer so list/watch
// errors route to stopWatching when appropriate:
//
//   - NotFound: the resource went away (CRD deleted, API removed in an
//     upgrade). Matches the legacy watchEvents loop that called
//     stopWatching when resClient.Watch returned IsNotFound.
//   - Forbidden / Unauthorized: honor respectRBAC. Under RespectRbacStrict
//     we re-check via SelfSubjectAccessReview so a transient token blip
//     doesn't permanently un-watch the resource; under RespectRbacNormal
//     we stopWatching on the first 403.
//   - Everything else (transient network, 5xx): log via DefaultWatchErrorHandler
//     and let the reflector's built-in backoff retry — no legacy equivalent
//     is missed, since the legacy watch loop also just looped on watch errors.
//
// Called from the reflector's goroutine; stopWatching acquires store.lock
// which is not held here.
func (e *informerEngine) informerWatchErrorHandler(api kube.APIResourceInfo, ns string) cache.WatchErrorHandlerWithContext {
	c := e.c
	return func(ctx context.Context, r *cache.Reflector, err error) {
		switch {
		case apierrors.IsNotFound(err):
			c.log.Info("Stop watching (resource not found)",
				"groupKind", api.GroupKind.String(),
				"namespace", namespaceDescription(ns))
			e.stopWatching(api.GroupKind, ns)
		case apierrors.IsForbidden(err), apierrors.IsUnauthorized(err):
			if c.respectRBAC == RespectRbacDisabled {
				cache.DefaultWatchErrorHandler(ctx, r, err)
				return
			}
			if c.respectRBAC == RespectRbacStrict {
				clientset, cerr := kubernetes.NewForConfig(c.config)
				if cerr != nil {
					// Can't verify with SSAR — treat as transient and let
					// the reflector retry rather than permanently un-watch.
					// Matches legacy startMissingWatches/sync which returned
					// the SSAR client construction error to the caller.
					c.log.Error(cerr, "SSAR client construction failed; keeping watch and retrying",
						"groupKind", api.GroupKind.String(),
						"namespace", namespaceDescription(ns))
					cache.DefaultWatchErrorHandler(ctx, r, err)
					return
				}
				keep, perr := c.checkPermission(ctx, clientset.AuthorizationV1().SelfSubjectAccessReviews(), api)
				if perr != nil {
					// SSAR call itself failed (apiserver blip). Same rationale
					// as the NewForConfig branch above.
					c.log.Error(perr, "SSAR permission check failed; keeping watch and retrying",
						"groupKind", api.GroupKind.String(),
						"namespace", namespaceDescription(ns))
					cache.DefaultWatchErrorHandler(ctx, r, err)
					return
				}
				if keep {
					// SSAR says we still have permission — this is a
					// transient blip. Surface the error and let the
					// reflector's backoff handle it.
					cache.DefaultWatchErrorHandler(ctx, r, err)
					return
				}
			}
			c.log.Info("Stop watching (forbidden/unauthorized)",
				"groupKind", api.GroupKind.String(),
				"namespace", namespaceDescription(ns),
				"error", err.Error())
			e.stopWatching(api.GroupKind, ns)
		default:
			cache.DefaultWatchErrorHandler(ctx, r, err)
		}
	}
}

// transformForInformer is the TransformFunc installed on a
// SharedIndexInformer by the informer engine. It converts incoming
// unstructured objects to cachedResource at intake, so the informer's store
// holds the domain Resource directly and the event handler doesn't have to
// re-derive it per event.
//
// OnPopulateResourceInfoHandler is invoked here rather than in the event
// handler. Argo-cd's handler is pure (reads cluster-cache-independent
// state only) so the earlier invocation is safe; it still decides whether
// the full manifest is retained on Resource.Resource.
//
// Non-unstructured inputs (e.g. *metav1.Status from watch stream errors,
// DeletedFinalStateUnknown tombstones) pass through unchanged so callers
// upstream of this function can handle them.
func (e *informerEngine) transformForInformer(obj any) (any, error) {
	un, ok := obj.(*unstructured.Unstructured)
	if !ok || un == nil {
		return obj, nil
	}
	res := e.c.newResource(un)
	// handleCRDEvent -> crdVersionsToAPIResources needs spec.versions to
	// drive startMissingWatches / deleteAPIResource. Always retain the
	// full CRD manifest regardless of the populate handler's cacheManifest
	// return so direct gitops-engine users (and a future argo-cd handler
	// change) keep correct dynamic API updates.
	if res.Resource == nil && kube.IsCRD(un) {
		res.Resource = un
	}
	return &cachedResource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: un.GetAPIVersion(),
			Kind:       un.GetKind(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            un.GetName(),
			Namespace:       un.GetNamespace(),
			UID:             un.GetUID(),
			ResourceVersion: un.GetResourceVersion(),
		},
		Resource: res,
	}, nil
}

// dispatchEvent fires OnEvent handlers and, for CRD events, the CRD handler.
// It is the informer engine's event-side-effect shim — the equivalent of the
// legacy watchEvents path that calls recordEvent + handleCRDEvent inline.
//
// Storage mutations (the resource index) are the caller's responsibility —
// this helper does not touch them.
func (e *informerEngine) dispatchEvent(event watch.EventType, un *unstructured.Unstructured) {
	c := e.c
	for _, h := range c.getEventHandlers() {
		h(event, un)
	}
	if kube.IsCRD(un) {
		c.handleCRDEvent(e, event, un)
	} else if kube.IsAPIService(un) {
		c.handleAPIServiceEvent(e, event, un)
	}
}

// informerEventHandler builds the cache.ResourceEventHandler attached to
// every per-(GroupKind, namespace) informer (see issue #19199). It translates
// Add/Update/Delete into onInformerChange, unwrapping tombstones and
// tolerating unexpected object types defensively.
//
// Test-only entrypoint: production callers go through buildInformer ->
// informerEventHandlerForCtx so that events arriving after the informer's
// watchCtx has been cancelled (Invalidate, stopWatching) are dropped
// instead of mutating freshly-rebuilt state.
func (e *informerEngine) informerEventHandler() cache.ResourceEventHandler {
	return e.informerEventHandlerForCtx(context.Background())
}

// informerEventHandlerForCtx is the production variant: it gates every
// dispatch on the informer's owning watch context. Once that context is
// cancelled (Invalidate, stopWatching), in-flight events from the
// reflector's still-draining DeltaFIFO are dropped on the floor.
//
// Without this guard, a stale event handler can fire after Invalidate
// has set apisMeta=nil — and the CRD dispatch path through handleCRDEvent
// -> startMissingWatches -> startInformersForAPILocked would then panic
// with "assignment to entry in nil map" (see startInformersForAPILocked
// in informer.go).
func (e *informerEngine) informerEventHandlerForCtx(ctx context.Context) cache.ResourceEventHandler {
	// DetailedFuncs exposes isInInitialList on Add so we can match legacy
	// semantics: initial-list resources populate c.resources / indexes (via
	// dispatchResourceUpdated) but do NOT fire OnEvent handlers or
	// handleCRDEvent. That avoids reloading the OpenAPI schema once per
	// CRD on startup — which floods logs with "Duplicate GVKs detected"
	// when the cluster has many CRDs.
	return cache.ResourceEventHandlerDetailedFuncs{
		AddFunc: func(obj any, isInInitialList bool) {
			if ctx.Err() != nil {
				return
			}
			e.onInformerChange(ctx, watch.Added, nil, obj, isInInitialList)
		},
		UpdateFunc: func(oldObj, newObj any) {
			if ctx.Err() != nil {
				return
			}
			e.onInformerChange(ctx, watch.Modified, oldObj, newObj, false)
		},
		DeleteFunc: func(obj any) {
			if ctx.Err() != nil {
				return
			}
			if tomb, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				obj = tomb.Obj
			}
			e.onInformerChange(ctx, watch.Deleted, obj, nil, false)
		},
	}
}

// onInformerChange dispatches a single informer event. It:
//  1. writes c.resources as a shadow of the informer's store so existing
//     read paths (GetManagedLiveObjs, IterateHierarchyV2's key lookups,
//     FindResources for the all-namespaces case) keep working unchanged,
//  2. updates the shared cross-GK indexes (nsIndex + parentUIDToChildren),
//     which are not derivable from per-GK informer indexers,
//  3. fires OnEvent handlers and routes CRD events via dispatchEvent
//     (skipped when isInInitialList=true to match legacy semantics — the
//     legacy initial-state load via replaceResourceCache fires OnNodeUpdated
//     but never recordEvent/handleCRDEvent),
//  4. fires OnResourceUpdated handlers with the post-update namespace map.
//
// ctx is the per-informer watch context. The handler closure pre-checks
// ctx.Err() before calling us, but that check is not atomic with the
// c.lock acquisition below — a goroutine that passed the outer check can
// stall here while Invalidate cancels ctx and resets c.resources. We
// re-check ctx.Err() AFTER c.lock.Lock() so a stale event can't mutate
// freshly-rebuilt state owned by a subsequent syncInformers run.
func (e *informerEngine) onInformerChange(ctx context.Context, event watch.EventType, oldObj, newObj any, isInInitialList bool) {
	c := e.c
	newCr, _ := newObj.(*cachedResource)
	oldCr, _ := oldObj.(*cachedResource)

	var primary *cachedResource
	switch {
	case newCr != nil:
		primary = newCr
	case oldCr != nil:
		primary = oldCr
	}
	if primary == nil || primary.Resource == nil {
		c.log.V(1).Info("Informer event with unexpected object type",
			"event", event,
			"newType", fmt.Sprintf("%T", newObj),
			"oldType", fmt.Sprintf("%T", oldObj))
		return
	}

	un := primary.Resource.Resource
	if un == nil {
		un = unstructuredFromCachedResource(primary)
	}

	// Time the under-lock storage + dispatch work so we can feed the
	// OnProcessEventsHandler observer below. Legacy fires this handler from
	// legacyEngine.processEventsBatch with a batch duration + count;
	// informer mode has no batching (events flow inline through the
	// reflector), so we report per-event duration with count=1. This keeps
	// the argocd_resource_events_processing histogram alive — without it
	// the metric flatlines the moment ModeInformer is enabled.
	processingStart := time.Now()

	// Storage mutations + OnResourceUpdated dispatch run under c.lock so the
	// nsIndex snapshot handed to handlers is consistent with c.resources —
	// matches the legacy setNode/onNodeRemoved -> dispatchResourceUpdated path.
	c.lock.Lock()
	// Re-check ctx under the lock — if the watch context was cancelled
	// (Invalidate, stopWatching) and a subsequent syncInformers reset
	// c.resources/c.nsIndex/c.parentUIDToChildren between the outer ctx.Err()
	// check and now, this is a stale event from a draining DeltaFIFO that must
	// not mutate freshly-rebuilt state.
	if ctx.Err() != nil {
		c.lock.Unlock()
		return
	}
	// Skip storage write + OnResourceUpdated dispatch for Modified events
	// on resources in ignoredRefreshResources (Endpoints today). Legacy
	// recordEvent does the same gate — it suppresses processEvent (the only
	// path that calls OnResourceUpdated under legacy) for high-churn kinds
	// whose updates are app-irrelevant.
	// Without this, every leader-election Endpoint rewrite acquires
	// c.lock (write), walks parentUIDToChildren, and fires every
	// OnResourceUpdated subscriber — pure overhead even though the
	// downstream filter in argo-cd then drops the requeue.
	skipStorageUpdate := event == watch.Modified && skipAppRequeuing(primary.Resource.ResourceKey())
	switch {
	case skipStorageUpdate:
		// no-op for storage; dispatchEvent below still fires OnEvent.
	case event == watch.Added || event == watch.Modified:
		// Maintain c.resources as a shadow of the informer's store so every
		// existing read path (GetManagedLiveObjs, IterateHierarchyV2's key
		// lookups, FindResources for the all-namespaces case) works unchanged
		// under informer mode. The ~70 bytes/entry map overhead is trivial
		// compared to the TransformFunc's savings on managedFields.
		if newCr == nil || newCr.Resource == nil {
			c.lock.Unlock()
			c.log.V(1).Info("Informer Add/Modify with missing new object — skipping",
				"event", event,
				"newType", fmt.Sprintf("%T", newObj),
				"oldType", fmt.Sprintf("%T", oldObj))
			return
		}
		// Take the previous entry as actually indexed (not the informer's
		// oldObj) for both index maintenance and dispatch — matching the
		// legacy processEvent path, and robust to any drift between the
		// shadow map and the informer's store.
		existing := c.resources[newCr.Resource.ResourceKey()]
		c.setNode(newCr.Resource)
		// Suppress OnResourceUpdated for isInInitialList events during the
		// very first sync — legacy first-sync writes to c.resources via
		// setNode without firing OnResourceUpdated. After
		// firstSyncCompleted flips true, isInInitialList events come from
		// CRD-driven new-watch initial lists, which legacy DOES dispatch
		// via replaceResourceCache -> onNodeUpdated. Watch events
		// (isInInitialList=false) always dispatch.
		if !isInInitialList || e.firstSyncCompleted {
			c.dispatchResourceUpdated(newCr.Resource, existing, c.nsIndex[newCr.Resource.Ref.Namespace])
		}
	case event == watch.Deleted:
		// For deletes the informer passes the last-known object as oldObj;
		// we treated it as `primary` above for dispatchEvent purposes.
		delete(c.resources, primary.Resource.ResourceKey())
		ns := c.removeIndexes(primary.Resource)
		c.dispatchResourceUpdated(nil, primary.Resource, ns)
	}
	c.lock.Unlock()

	// Feed argocd_resource_events_processing (and any other consumer of
	// OnProcessEventsHandler) with the per-event duration. Done outside
	// the lock so handler latency doesn't block other events.
	processingDuration := time.Since(processingStart)
	for _, h := range c.getProcessEventsHandlers() {
		h(processingDuration, 1)
	}

	// Skip OnEvent + CRD routing for initial-list events so we don't reload
	// the OpenAPI schema once per existing CRD at startup. Legacy
	// loadInitialState populates state without firing dispatchEvent —
	// matched here. Genuine watch events (post-initial Add, Update, Delete)
	// continue to flow through dispatchEvent normally.
	if isInInitialList {
		return
	}

	// OnEvent handlers and CRD routing run WITHOUT c.lock — handleCRDEvent
	// re-acquires c.lock via runSynced (startMissingWatches /
	// reloadOpenAPISchema). Mirrors the legacy watchEvents path, which
	// invokes recordEvent (event handlers) and handleCRDEvent outside the
	// lock; only processEvent runs under it.
	e.dispatchEvent(event, un)
}

func namespaceDescription(ns string) string {
	if ns == "" {
		return "cluster-scope"
	}
	return "ns=" + ns
}

// cachedResource is the type stored in the informer's store by the informer
// engine (see issue #19199). It pairs the domain Resource (what the rest of
// the cluster cache wants) with just enough Kubernetes metadata that
// meta.Accessor / MetaNamespaceKeyFunc can key it for the informer's indexer.
//
// The full unstructured object is kept on Resource.Resource only when
// OnPopulateResourceInfoHandler returns cacheManifest=true — the same
// contract as the legacy path through newResource. Everything else drops
// to the floor after we've extracted the metadata needed for hierarchy
// traversal and the user-supplied Info.
type cachedResource struct {
	metav1.TypeMeta
	metav1.ObjectMeta
	// Resource carries the domain payload (Ref, OwnerRefs, Info, etc.).
	// Named rather than embedded to avoid promoted-field collisions with
	// ObjectMeta (both have e.g. a notion of "Name").
	Resource *Resource
}

// DeepCopyObject satisfies runtime.Object. The returned shell holds copies
// of TypeMeta and ObjectMeta but SHARES the Resource pointer with the
// receiver — by design. The same *Resource lives in the informer's store
// and in c.resources (the shadow map), and onInformerChange mutates
// Resource.OwnerRefs under c.lock during inferred parent-ref propagation
// (updateIndexes). Deep-copying Resource here would not stop that mutation
// pattern, only hide it from readers. Callers that need an isolated
// snapshot — including the client-go cache mutation detector — must copy
// OwnerRefs themselves; the detector is therefore incompatible with this
// mode and should not be enabled together with ModeInformer.
func (cr *cachedResource) DeepCopyObject() runtime.Object {
	if cr == nil {
		return nil
	}
	return &cachedResource{
		TypeMeta:   cr.TypeMeta,
		ObjectMeta: *cr.ObjectMeta.DeepCopy(),
		Resource:   cr.Resource,
	}
}

// GetObjectKind is part of runtime.Object. It returns the embedded TypeMeta.
func (cr *cachedResource) GetObjectKind() schema.ObjectKind {
	return &cr.TypeMeta
}

// unstructuredFromCachedResource synthesizes a minimal *Unstructured from
// a cachedResource when the full manifest was not retained. Populates only
// the fields downstream OnEventHandler callbacks depend on — GVK and basic
// metadata. Callers that need spec/status must arrange for the manifest
// to be cached via the OnPopulateResourceInfoHandler's cacheManifest return.
func unstructuredFromCachedResource(cr *cachedResource) *unstructured.Unstructured {
	un := &unstructured.Unstructured{Object: map[string]any{}}
	un.SetAPIVersion(cr.APIVersion)
	un.SetKind(cr.Kind)
	un.SetName(cr.Name)
	un.SetNamespace(cr.Namespace)
	un.SetUID(cr.UID)
	un.SetResourceVersion(cr.ResourceVersion)
	return un
}
