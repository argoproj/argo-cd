package cache

import (
	"context"
	"fmt"

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

// startInformersForAPI is the public entrypoint for starting the informer
// set for a GroupKind. It acquires c.lock and delegates to the locked
// variant. Used by tests; production call sites (syncInformers,
// startMissingWatches branch) hold the lock already and call the locked
// variant directly.
func (c *clusterCache) startInformersForAPI(ctx context.Context, api kube.APIResourceInfo) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.startInformersForAPILocked(ctx, api)
}

// startInformersForAPILocked builds and starts one SharedIndexInformer per
// (GroupKind, namespace) implied by processApi, installs transformForInformer
// as the TransformFunc, and attaches informerEventHandler. The informers
// share a single context derived from ctx; cancelling that context stops
// every namespace watch for the GroupKind together, matching legacy
// semantics.
//
// Idempotent — if apisMeta already has an entry for this GroupKind, the
// call is a no-op. Caller must hold c.lock.
//
// Does not block for initial sync; callers that need that should consult
// each informer's HasSynced via apisMeta.
//
// Used only under ModeInformer; ModeLegacy continues to go through sync()
// and startMissingWatches() which spawn watchEvents goroutines instead.
func (c *clusterCache) startInformersForAPILocked(ctx context.Context, api kube.APIResourceInfo) error {
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
		informers:   map[string]sharedInformer{},
	}

	// Build every informer up front so a partial failure doesn't leave
	// apisMeta half-populated.
	err = c.processApi(client, api, func(resClient dynamic.ResourceInterface, ns string) error {
		informer := c.buildInformer(watchCtx, resClient, api, ns)
		meta.informers[ns] = sharedInformer{informer: informer, cancel: cancel}
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
	if c.namespacedResources == nil {
		c.namespacedResources = map[schema.GroupKind]bool{}
	}
	c.namespacedResources[api.GroupKind] = api.Meta.Namespaced

	for _, e := range meta.informers {
		go e.informer.RunWithContext(watchCtx)
	}
	return nil
}

// syncInformers is the informer-mode equivalent of sync(). It drops existing
// state, runs discovery, and starts an informer per API, then blocks until
// every informer has completed its initial list. EnsureSynced routes here
// when c.mode == ModeInformer; callers must hold c.lock.
func (c *clusterCache) syncInformers() error {
	c.log.Info("Start syncing cluster (informer mode)")

	// Cancel any existing informers before rebuilding. watchCancel stops
	// every namespace watch for a GroupKind (shared context).
	for _, meta := range c.apisMeta {
		if meta != nil && meta.watchCancel != nil {
			meta.watchCancel()
		}
	}
	c.apisMeta = map[schema.GroupKind]*apiMeta{}
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

	apis, err := c.kubectl.GetAPIResources(c.config, true, c.settings.ResourcesFilter)
	if err != nil {
		return fmt.Errorf("get filtered api resources: %w", err)
	}

	// Serial for now — the per-API work is mostly list/watch setup, not
	// blocking operations. Parallelism can come later if discovery-heavy
	// clusters show it matters.
	watchParent := context.Background()
	for i := range apis {
		if err := c.startInformersForAPILocked(watchParent, apis[i]); err != nil {
			return fmt.Errorf("start informers for %s: %w", apis[i].GroupKind, err)
		}
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
	waitCtx, cancelWait := context.WithTimeout(watchParent, c.clusterSyncRetryTimeout)
	defer cancelWait()

	// Loop: re-snapshot watched informers after each WaitForCacheSync. New
	// informers can appear mid-wait when handleCRDEvent fires (CRD-driven
	// startMissingWatches -> startInformersForAPILocked). Without this
	// loop, WaitForCacheSync returns synced=true based on the original
	// snapshot while the new informers' reflectors are still doing their
	// initial list — EnsureSynced would report success against an
	// incomplete cache.
	//
	// Release c.lock during each wait — the event handler takes it when
	// informers dispatch their initial list. Holding it across the wait
	// would deadlock.
	var watched []watchedInformer
	for {
		watched = c.snapshotWatched()
		pending := unsyncedInformers(watched)
		if len(pending) == 0 {
			break
		}
		c.lock.Unlock()
		synced := cache.WaitForCacheSync(waitCtx.Done(), pending...)
		c.lock.Lock()
		if !synced {
			return c.resolveSyncResult(false, watched)
		}
		// A successful WaitForCacheSync may have raced a CRD-driven
		// startInformersForAPILocked that registered new informers. Loop
		// to pick those up; if none appeared, the next snapshot's
		// `pending` list will be empty and we exit cleanly.
	}

	return c.resolveSyncResult(true, watched)
}

// snapshotWatched returns the current set of (gk, ns, HasSynced) tuples
// across c.apisMeta. Caller must hold c.lock.
func (c *clusterCache) snapshotWatched() []watchedInformer {
	var watched []watchedInformer
	for gk, meta := range c.apisMeta {
		for ns, si := range meta.informers {
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
// syncInformers before releasing c.lock. Kept as a package-level type so
// resolveSyncResult can take a slice of it.
type watchedInformer struct {
	gk        schema.GroupKind
	ns        string
	hasSynced cache.InformerSynced
}

// resolveSyncResult interprets the outcome of WaitForCacheSync. Caller
// must hold c.lock.
//
// The Invalidate-check runs BEFORE the synced-branch by design. DeltaFIFO's
// HasSynced is sticky (vendor/k8s.io/client-go/tools/cache/delta_fifo.go) —
// once true it never resets, not even on watch-context cancellation. So if
// the informers completed their initial list before a concurrent Invalidate
// cancelled them, WaitForCacheSync returns synced=true even though
// c.apisMeta has just been cleared and c.resources reset. Returning nil in
// that case would cache "success" for clusterSyncRetryTimeout against an
// empty cache; instead, surface errCacheInvalidatedMidSync so EnsureSynced
// (which detects this sentinel via errors.Is) skips caching the result and
// the next call re-syncs immediately.
func (c *clusterCache) resolveSyncResult(synced bool, watched []watchedInformer) error {
	if c.apisMeta == nil {
		return errCacheInvalidatedMidSync
	}
	if synced {
		// Mark first-sync done so subsequent CRD-driven new-watch initial
		// lists fire OnResourceUpdated (matching legacy startMissingWatches
		// -> loadInitialState -> replaceResourceCache -> onNodeUpdated path).
		// Before this point, onInformerChange suppresses OnResourceUpdated
		// for isInInitialList events so the bulk load stays quiet.
		c.firstSyncCompleted = true
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

// buildInformer creates a SharedIndexInformer wired with the cluster cache's
// TransformFunc and event handler. Exposed as a method (rather than being
// inlined) so tests can exercise a single informer without spinning up the
// full startInformersForAPI lifecycle.
//
// ctx is the informer's owning watch context — when it is cancelled (via
// Invalidate or stopWatching), the attached event handler bails so that
// post-cancel events from a still-draining reflector goroutine cannot
// mutate fresh state owned by a subsequent syncInformers run.
func (c *clusterCache) buildInformer(ctx context.Context, resClient dynamic.ResourceInterface, api kube.APIResourceInfo, ns string) cache.SharedIndexInformer {
	lw := &cache.ListWatch{
		// Wrap List with c.listSemaphore so initial-list memory pressure
		// stays bounded across many (GK, ns) informers. Legacy listResources
		// (cluster.go:795) does the same — without it, every informer's
		// reflector calls List concurrently and a discovery-heavy cluster
		// (hundreds of GVRs) spikes memory during startup. Reflector pager
		// calls List once per page; the semaphore holds for one page at a
		// time, matching legacy behavior.
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
		lw,
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
	if err := informer.SetTransform(c.transformForInformer); err != nil {
		panic(fmt.Errorf("unreachable: SetTransform on fresh informer: %w", err))
	}
	if err := informer.SetWatchErrorHandlerWithContext(c.informerWatchErrorHandler(api, ns)); err != nil {
		panic(fmt.Errorf("unreachable: SetWatchErrorHandler on fresh informer: %w", err))
	}
	_, _ = informer.AddEventHandler(c.informerEventHandlerForCtx(ctx))
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
// Called from the reflector's goroutine; stopWatching acquires c.lock
// which is not held here.
func (c *clusterCache) informerWatchErrorHandler(api kube.APIResourceInfo, ns string) cache.WatchErrorHandlerWithContext {
	return func(ctx context.Context, r *cache.Reflector, err error) {
		switch {
		case apierrors.IsNotFound(err):
			c.log.Info("Stop watching (resource not found)",
				"groupKind", api.GroupKind.String(),
				"namespace", namespaceDescription(ns))
			c.stopWatching(api.GroupKind, ns)
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
			c.stopWatching(api.GroupKind, ns)
		default:
			cache.DefaultWatchErrorHandler(ctx, r, err)
		}
	}
}

func namespaceDescription(ns string) string {
	if ns == "" {
		return "cluster-scope"
	}
	return "ns=" + ns
}
