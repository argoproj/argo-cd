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

	// Snapshot (gk, ns, HasSynced) before releasing c.lock. A concurrent
	// Invalidate sets c.apisMeta = nil mid-wait; re-reading c.apisMeta after
	// re-acquiring the lock would yield an empty pending list and a
	// misleading "did not complete initial list within Xs: []" error.
	type watchedInformer struct {
		gk        schema.GroupKind
		ns        string
		hasSynced cache.InformerSynced
	}
	var watched []watchedInformer
	for gk, meta := range c.apisMeta {
		for ns, si := range meta.informers {
			watched = append(watched, watchedInformer{gk: gk, ns: ns, hasSynced: si.informer.HasSynced})
		}
	}
	hasSyncedFns := make([]cache.InformerSynced, 0, len(watched))
	for _, w := range watched {
		hasSyncedFns = append(hasSyncedFns, w.hasSynced)
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
	//
	// Release c.lock while waiting — the event handler takes it when
	// informers dispatch their initial list. Holding it across the wait
	// would deadlock.
	waitCtx, cancelWait := context.WithTimeout(watchParent, c.clusterSyncRetryTimeout)
	defer cancelWait()
	c.lock.Unlock()
	synced := cache.WaitForCacheSync(waitCtx.Done(), hasSyncedFns...)
	c.lock.Lock()

	if synced {
		c.log.Info("Cluster successfully synced (informer mode)")
		return nil
	}

	// Invalidate is the only caller that nils c.apisMeta, and it can only
	// run during the c.lock.Unlock() window above. Treat it as a distinct
	// transient condition so the cached "did not complete initial list"
	// error doesn't suppress the immediate re-sync that should follow.
	if c.apisMeta == nil {
		return fmt.Errorf("cluster cache invalidated during initial informer sync")
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
		ListWithContextFunc: func(ctx context.Context, o metav1.ListOptions) (runtime.Object, error) {
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
				if clientset, cerr := kubernetes.NewForConfig(c.config); cerr == nil {
					if keep, perr := c.checkPermission(ctx, clientset.AuthorizationV1().SelfSubjectAccessReviews(), api); perr == nil && keep {
						// SSAR says we still have permission — this is a
						// transient blip. Surface the error and let the
						// reflector's backoff handle it.
						cache.DefaultWatchErrorHandler(ctx, r, err)
						return
					}
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
