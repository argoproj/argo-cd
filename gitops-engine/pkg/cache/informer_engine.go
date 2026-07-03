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
		informer := e.buildInformer(watchCtx, client, resClient, api, ns)
		informers[ns] = sharedInformer{informer: informer, cancel: cancel}
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
	cancel   context.CancelFunc
}

// informerSyncResnapshotInterval is how long each WaitForCacheSync slice in
// syncInformers runs before the watched-informer set is re-snapshotted (to
// pick up informers added by CRD events and drop ones torn down by
// stopWatching). Long enough that the re-snapshot lock traffic is noise,
// short enough that a torn-down informer can't stall the sync for long.
const informerSyncResnapshotInterval = time.Second

// snapshotWatched returns the current set of (gk, ns, HasSynced) tuples
// across the engine's informer index. Caller must hold store.lock.
func (e *informerEngine) snapshotWatched() []watchedInformer {
	var watched []watchedInformer
	for gk, informers := range e.informers {
		for ns, si := range informers {
			watched = append(watched, watchedInformer{gk: gk, ns: ns, hasSynced: si.informer.HasSynced})
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
func (e *informerEngine) buildInformer(ctx context.Context, client dynamic.Interface, resClient dynamic.ResourceInterface, api kube.APIResourceInfo, ns string) cache.SharedIndexInformer {
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
	_, _ = informer.AddEventHandler(e.informerEventHandlerForCtx(ctx))
	return informer
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

func namespaceDescription(ns string) string {
	if ns == "" {
		return "cluster-scope"
	}
	return "ns=" + ns
}
