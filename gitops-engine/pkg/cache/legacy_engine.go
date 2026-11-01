package cache

import (
	"context"
	"fmt"
	"maps"
	"runtime/debug"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/pager"
	watchutil "k8s.io/client-go/tools/watch"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
)

const (
	// Watch backoff parameters for handling server/auth errors (401, 403, 429, 500, 503, 504)
	defaultWatchBackoffFactor = 2.0
	defaultWatchBackoffJitter = 0.1
	defaultWatchBackoffSteps  = 5
	defaultWatchBackoffCap    = 30 * time.Second
	// Duration a watch must run stably before resetting backoff, aligned with client-go's defaultBackoffReset
	defaultWatchHealthyDuration = 2 * time.Minute
)

// legacyEngine loads each (GroupKind, namespace) with a paged list, then keeps
// it current from a retrying watch goroutine. It reaches shared state only
// through its *store.
type legacyEngine struct {
	c *store

	// retryDelayFunc is an optional hook used in unit tests to intercept and verify retry delays and errors without sleeping.
	retryDelayFunc func(ctx context.Context, delay time.Duration, err error) error

	// eventMetaCh carries watch events to processEvents when
	// batchEventsProcessing is set, and is nil otherwise. Created in sync,
	// closed by invalidateEventMeta.
	eventMetaCh chan eventMeta
}

// ensureSynced holds c.lock, then syncStatus.lock, for the whole of sync, so
// store state doesn't change during it.
func (e *legacyEngine) ensureSynced() error {
	c := e.c
	syncStatus := &c.syncStatus

	c.lock.Lock()
	defer c.lock.Unlock()
	syncStatus.lock.Lock()
	defer syncStatus.lock.Unlock()

	// before doing any work, check once again now that we have the lock, to see if it got
	// synced between the first check and now
	if syncStatus.synced(c.clusterSyncRetryTimeout) {
		return syncStatus.syncError
	}
	err := e.sync()
	syncTime := time.Now()
	syncStatus.syncTime = &syncTime
	syncStatus.syncError = err
	return syncStatus.syncError
}

// sync retrieves the current state of the cluster and stores relevant information in the store fields.
//
// First we get some metadata from the cluster, like the server version, OpenAPI document, and the list of all API
// resources.
//
// Then we get a list of the preferred versions of all API resources which are to be monitored (it's possible to exclude
// resources from monitoring). We loop through those APIs asynchronously and for each API we list all resources. We also
// kick off a goroutine to watch the resources for that API and update the cache constantly.
//
// When this function exits, the cluster cache is up to date, and the appropriate resources are being watched for
// changes.
//
// It's called with c.lock held.
func (e *legacyEngine) sync() (err error) {
	c := e.c
	c.log.Info("Start syncing cluster")

	start := time.Now()
	syncLock := sync.Mutex{}

	var discoveryEnd time.Time
	defer func() {
		end := time.Now()
		if discoveryEnd.IsZero() {
			discoveryEnd = end
		}
		log := c.log.WithValues(
			"total_ms", end.Sub(start).Milliseconds(),
			"discovery_ms", discoveryEnd.Sub(start).Milliseconds(),
			"list_ms", end.Sub(discoveryEnd).Milliseconds(),
		)
		if err != nil {
			log.Error(err, "Failed to sync cluster")
		} else {
			log.Info("Cluster successfully synced")
		}
	}()

	for i := range c.apisMeta {
		c.apisMeta[i].watchCancel()
	}

	if c.batchEventsProcessing {
		e.invalidateEventMeta()
		e.eventMetaCh = make(chan eventMeta)
	}

	syncLock.Lock()
	c.apisMeta = make(map[schema.GroupKind]*apiMeta)
	c.resources = make(map[kube.ResourceKey]*Resource)
	c.nsIndex = make(map[string]map[kube.ResourceKey]*Resource)
	// Filled by the workers below and published once they finish.
	namespacedResources := make(map[schema.GroupKind]bool)
	defer c.namespacedResources.Store(&namespacedResources)
	c.parentUIDToChildren = make(map[types.UID]map[kube.ResourceKey]struct{})
	syncLock.Unlock()
	config := c.config
	version, err := c.kubectl.GetServerVersion(config)
	if err != nil {
		return fmt.Errorf("failed to get server version: %w", err)
	}
	c.serverVersion.Store(&version)
	apiResources, err := c.kubectl.GetAPIResources(config, false, NewNoopSettings())
	if err != nil {
		return fmt.Errorf("failed to get api resources: %w", err)
	}
	c.apiResources.Store(&apiResources)

	openAPISchema, gvkParser, err := c.kubectl.LoadOpenAPISchema(config)
	if err != nil {
		return fmt.Errorf("failed to load open api schema while syncing cluster cache: %w", err)
	}

	if gvkParser != nil {
		c.gvkParser.Store(gvkParser)
	}

	c.openAPISchema.Store(&openAPISchema)

	apis, err := c.kubectl.GetAPIResources(c.config, true, c.settings.ResourcesFilter)
	if err != nil {
		return fmt.Errorf("failed to get api resources: %w", err)
	}
	client, err := c.kubectl.NewDynamicClient(c.config)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	if c.batchEventsProcessing {
		go e.processEvents()
	}

	discoveryEnd = time.Now()
	err = kube.RunAllAsync(len(apis), func(i int) error {
		api := apis[i]

		ctx, cancel := context.WithCancel(context.Background())
		info := &apiMeta{namespaced: api.Meta.Namespaced, watchCancel: cancel}
		syncLock.Lock()
		c.apisMeta[api.GroupKind] = info
		namespacedResources[api.GroupKind] = api.Meta.Namespaced
		syncLock.Unlock()

		return c.processApi(client, api, func(resClient dynamic.ResourceInterface, ns string) error {
			resourceVersion, err := c.listResources(ctx, resClient, func(listPager *pager.ListPager) error {
				// Use the WithAlloc variant: newResource may retain the object (as Resource.Resource, or via the
				// Info returned by the OnPopulateResourceInfoHandler). Plain EachListItem yields &list.Items[i],
				// so retaining one item keeps the page's whole backing array, and every manifest in it, reachable.
				return listPager.EachListItemWithAlloc(ctx, metav1.ListOptions{LabelSelector: api.LabelSelector}, func(obj runtime.Object) error {
					if un, ok := obj.(*unstructured.Unstructured); !ok {
						return fmt.Errorf("object %s/%s has an unexpected type", un.GroupVersionKind().String(), un.GetName())
					} else {
						newRes := c.newResource(un)
						syncLock.Lock()
						c.setNode(newRes)
						syncLock.Unlock()
					}
					return nil
				})
			})
			if err != nil {
				if c.isRestrictedResource(err) {
					keep := false
					if c.respectRBAC == RespectRbacStrict {
						k, permErr := c.checkPermission(ctx, clientset.AuthorizationV1().SelfSubjectAccessReviews(), api)
						if permErr != nil {
							return fmt.Errorf("failed to check permissions for resource %s: %w, original error=%v", api.GroupKind.String(), permErr, err.Error())
						}
						keep = k
					}
					// if we are not allowed to list the resource, remove it from the watch list
					if !keep {
						syncLock.Lock()
						delete(c.apisMeta, api.GroupKind)
						delete(namespacedResources, api.GroupKind)
						syncLock.Unlock()
						return nil
					}
				}
				return fmt.Errorf("failed to load initial state of resource %s: %w", api.GroupKind.String(), err)
			}

			go e.watchEvents(ctx, api, resClient, ns, resourceVersion)

			return nil
		})
	})
	if err != nil {
		return fmt.Errorf("failed to sync cluster %s: %w", c.config.Host, err)
	}
	return nil
}

// startMissingWatches lists supported cluster resources and starts watching for changes unless watch is already running.
// Caller must hold c.lock.
func (e *legacyEngine) startMissingWatches() error {
	c := e.c
	// apisMeta is nil between Invalidate and the next sync, and a CRD or
	// APIService event from a watch started before Invalidate can still get here.
	// Start nothing: the next sync starts every watch, and if the cache was
	// discarded, watches started now would never be cancelled.
	if c.apisMeta == nil {
		return nil
	}
	apis, err := c.kubectl.GetAPIResources(c.config, true, c.settings.ResourcesFilter)
	if err != nil {
		return fmt.Errorf("failed to get APIResources: %w", err)
	}
	client, err := c.kubectl.NewDynamicClient(c.config)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(c.config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}
	namespacedResources := make(map[schema.GroupKind]bool)
	for i := range apis {
		api := apis[i]
		namespacedResources[api.GroupKind] = api.Meta.Namespaced
		if _, ok := c.apisMeta[api.GroupKind]; !ok {
			ctx, cancel := context.WithCancel(context.Background())
			c.apisMeta[api.GroupKind] = &apiMeta{namespaced: api.Meta.Namespaced, watchCancel: cancel}

			err := c.processApi(client, api, func(resClient dynamic.ResourceInterface, ns string) error {
				resourceVersion, err := e.loadInitialState(ctx, api, resClient, ns, false) // don't lock here: startMissingWatches's caller holds c.lock
				if err != nil && c.isRestrictedResource(err) {
					keep := false
					if c.respectRBAC == RespectRbacStrict {
						k, permErr := c.checkPermission(ctx, clientset.AuthorizationV1().SelfSubjectAccessReviews(), api)
						if permErr != nil {
							return fmt.Errorf("failed to check permissions for resource %s: %w, original error=%v", api.GroupKind.String(), permErr, err.Error())
						}
						keep = k
					}
					// if we are not allowed to list the resource, remove it from the watch list
					if !keep {
						delete(c.apisMeta, api.GroupKind)
						delete(namespacedResources, api.GroupKind)
						return nil
					}
				}
				go e.watchEvents(ctx, api, resClient, ns, resourceVersion)
				return nil
			})
			if err != nil {
				return err
			}
		}
	}
	c.namespacedResources.Store(&namespacedResources)
	return nil
}

// stopWatching stops the watch for gk in namespace ns and purges that
// namespace's resources. It isn't on syncEngine; see the note there.
func (e *legacyEngine) stopWatching(gk schema.GroupKind, ns string) {
	c := e.c
	c.lock.Lock()
	defer c.lock.Unlock()
	if info, ok := c.apisMeta[gk]; ok {
		info.watchCancel()
		delete(c.apisMeta, gk)
		// Keep in step with apisMeta.
		namespacedResources := maps.Clone(c.loadNamespacedResources())
		delete(namespacedResources, gk)
		c.namespacedResources.Store(&namespacedResources)
		c.replaceResourceCache(gk, nil, ns)
		c.log.Info(fmt.Sprintf("Stop watching: %s not found", gk))
	}
}

// onInvalidate cancels every watch and retires the batched-event channel.
// Invalidate calls it under c.lock, before applying its opts.
func (e *legacyEngine) onInvalidate() {
	for i := range e.c.apisMeta {
		e.c.apisMeta[i].watchCancel()
	}
	if e.c.batchEventsProcessing {
		e.invalidateEventMeta()
	}
}

// invalidateEventMeta closes the eventMeta channel if it is open. Caller must
// hold c.lock.
func (e *legacyEngine) invalidateEventMeta() {
	if e.eventMetaCh != nil {
		close(e.eventMetaCh)
		e.eventMetaCh = nil
	}
}

// loadInitialState loads the state of all the resources retrieved by the given resource client.
func (e *legacyEngine) loadInitialState(ctx context.Context, api kube.APIResourceInfo, resClient dynamic.ResourceInterface, ns string, lock bool) (string, error) {
	c := e.c
	var items []*Resource
	resourceVersion, err := c.listResources(ctx, resClient, func(listPager *pager.ListPager) error {
		// Use the WithAlloc variant: newResource may retain the object (as Resource.Resource, or via the
		// Info returned by the OnPopulateResourceInfoHandler). Plain EachListItem yields &list.Items[i],
		// so retaining one item keeps the page's whole backing array, and every manifest in it, reachable.
		return listPager.EachListItemWithAlloc(ctx, metav1.ListOptions{LabelSelector: api.LabelSelector}, func(obj runtime.Object) error {
			if un, ok := obj.(*unstructured.Unstructured); !ok {
				return fmt.Errorf("object %s/%s has an unexpected type", un.GroupVersionKind().String(), un.GetName())
			} else {
				items = append(items, c.newResource(un))
			}
			return nil
		})
	})
	if err != nil {
		return "", fmt.Errorf("failed to load initial state of resource %s: %w", api.GroupKind.String(), err)
	}

	if lock {
		return resourceVersion, runSynced(&c.lock, func() error {
			c.replaceResourceCache(api.GroupKind, items, ns)
			return nil
		})
	}
	c.replaceResourceCache(api.GroupKind, items, ns)
	return resourceVersion, nil
}

// isWatchBackoffError checks whether an error is an authentication, rate limit, or server error
// (such as 401 Unauthorized, 403 Forbidden, 429 Too Many Requests, 500 Internal, 503 Service Unavailable,
// or 504 Server Timeout) that warrants exponential backoff.
func isWatchBackoffError(err error) bool {
	if err == nil {
		return false
	}
	return apierrors.IsUnauthorized(err) ||
		apierrors.IsForbidden(err) ||
		apierrors.IsTooManyRequests(err) ||
		apierrors.IsServiceUnavailable(err) ||
		apierrors.IsInternalError(err) ||
		apierrors.IsTimeout(err) ||
		apierrors.IsServerTimeout(err) ||
		apierrors.IsUnexpectedServerError(err)
}

func (e *legacyEngine) retryWatchUntilSucceed(ctx context.Context, desc string, action func(watchStarted func()) error) {
	c := e.c
	backoff := wait.Backoff{
		Duration: watchResourcesRetryTimeout,
		Factor:   defaultWatchBackoffFactor,
		Jitter:   defaultWatchBackoffJitter,
		Steps:    defaultWatchBackoffSteps,
		Cap:      defaultWatchBackoffCap,
	}
	currentBackoff := backoff

	for {
		select {
		case <-ctx.Done():
			c.log.V(1).Info("Stop retrying " + desc)
			return
		default:
		}

		c.log.V(1).Info("Start " + desc)
		var watchStartTime time.Time
		watchStarted := func() {
			watchStartTime = time.Now()
		}
		err := action(watchStarted)
		if err == nil {
			c.log.V(1).Info("Completed " + desc)
			return
		}

		if !watchStartTime.IsZero() && time.Since(watchStartTime) >= defaultWatchHealthyDuration {
			// If watch was established and healthy for at least defaultWatchHealthyDuration,
			// reset the backoff so future transient failures start with base delay.
			currentBackoff = backoff
		}

		var delay time.Duration
		if c.watchRetryUseBackoff && isWatchBackoffError(err) {
			delay = currentBackoff.Step()
			c.log.V(1).Info(fmt.Sprintf("Failed to %s: %+v, backing off for %v", desc, err, delay))
		} else {
			// For non-backoff errors (e.g. 410 Gone, normal watch timeout/closure, etc.),
			// or when watch backoff is not explicitly enabled, retry quickly with base timeout.
			delay = watchResourcesRetryTimeout
			c.log.V(1).Info(fmt.Sprintf("Failed to %s: %+v, retrying in %v", desc, err, delay))
		}

		// In unit tests, retryDelayFunc intercepts the delay and error to avoid real-time sleep.
		if e.retryDelayFunc != nil {
			if err := e.retryDelayFunc(ctx, delay, err); err != nil {
				c.log.V(1).Info("Stop retrying " + desc)
				return
			}
		} else {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				c.log.V(1).Info("Stop retrying " + desc)
				return
			}
		}
	}
}

func (e *legacyEngine) watchEvents(ctx context.Context, api kube.APIResourceInfo, resClient dynamic.ResourceInterface, ns string, resourceVersion string) {
	c := e.c
	e.retryWatchUntilSucceed(ctx, fmt.Sprintf("watch %s on %s", api.GroupKind, c.config.Host), func(watchStarted func()) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("recovered from panic: %+v\n%s", r, debug.Stack())
			}
		}()

		// load API initial state if no resource version provided
		if resourceVersion == "" {
			resourceVersion, err = e.loadInitialState(ctx, api, resClient, ns, true)
			if err != nil {
				return err
			}
		}

		// client-go's RetryWatcher retries non-auth errors (such as 429 and 5xx) internally using a fixed
		// 1-second delay instead of terminating. To allow our outer exponential backoff to handle these errors,
		// we pass a cancellable child context (watchCtx) and intercept qualifying errors in WatchFuncWithContext
		// to stop the RetryWatcher and propagate the root-cause error upward.
		watchCtx, watchCancel := context.WithCancel(ctx)
		defer watchCancel()

		var watchErrLock sync.Mutex
		var lastWatchErr error

		w, err := watchutil.NewRetryWatcherWithContext(watchCtx, resourceVersion, &cache.ListWatch{
			WatchFuncWithContext: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
				options.LabelSelector = api.LabelSelector
				res, err := resClient.Watch(ctx, options)
				if apierrors.IsNotFound(err) {
					e.stopWatching(api.GroupKind, ns)
				}
				if err != nil && c.watchRetryUseBackoff && isWatchBackoffError(err) {
					watchErrLock.Lock()
					lastWatchErr = err
					watchErrLock.Unlock()
					// Cancel the RetryWatcher child context to exit its internal retry loop.
					watchCancel()
					return nil, err
				}
				//nolint:wrapcheck // wrap outside the retry
				return res, err
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create resource watcher: %w", err)
		}

		// Mark the watch start time after initial state loading and watch connection are established.
		watchStarted()

		defer func() {
			w.Stop()
			// Trade-off note: client-go's RetryWatcher consumes watch.Bookmark events internally without
			// forwarding them to ResultChan(). For rarely mutated resources, resourceVersion will not advance
			// via bookmarks and may expire in etcd, resulting in 410 Gone upon reconnect.
			// This trade-off is intentional:
			// 1) High-frequency resources (Pods, Deployments, 95%+ of cluster volume) continuously advance RV
			//    via regular events, avoiding devastating full LIST storms during 429/5xx overload.
			// 2) Rarely mutated resources (e.g. StorageClasses, CRDs) have tiny dataset sizes; hitting a 410
			//    carries negligible LIST cost and safely falls back to standard self-healing re-list.
			// When watch backoff is disabled, or for non-backoff exits (e.g. 410 Gone, normal closure, resync),
			// reset resourceVersion to "" to maintain existing self-healing re-list behavior.
			if !(c.watchRetryUseBackoff && isWatchBackoffError(err)) {
				resourceVersion = ""
			}
		}()

		var watchResyncTimeoutCh <-chan time.Time
		if c.watchResyncTimeout > 0 {
			shouldResync := time.NewTimer(c.watchResyncTimeout)
			defer shouldResync.Stop()
			watchResyncTimeoutCh = shouldResync.C
		}

		for {
			select {
			// stop watching when parent context got cancelled
			case <-ctx.Done():
				return nil

			// re-synchronize API state and restart watch periodically
			case <-watchResyncTimeoutCh:
				return fmt.Errorf("resyncing %s on %s due to timeout", api.GroupKind, c.config.Host)

			case event, ok := <-w.ResultChan():
				if !ok {
					watchErrLock.Lock()
					watchErr := lastWatchErr
					watchErrLock.Unlock()
					if watchErr != nil {
						return watchErr
					}
					return fmt.Errorf("watch %s on %s has closed", api.GroupKind, c.config.Host)
				}

				if event.Type == watch.Error {
					return apierrors.FromObject(event.Object)
				}

				obj, ok := event.Object.(*unstructured.Unstructured)
				if !ok {
					return fmt.Errorf("failed to convert to *unstructured.Unstructured: %v", event.Object)
				}

				// Track the latest resourceVersion observed from the event stream.
				if newRV := obj.GetResourceVersion(); newRV != "" {
					resourceVersion = newRV
				}

				e.recordEvent(event.Type, obj)
				if kube.IsCRD(obj) {
					c.handleCRDEvent(e, event.Type, obj)
				} else if kube.IsAPIService(obj) {
					c.handleAPIServiceEvent(e, event.Type, obj)
				}
			}
		}
	})
}

func (e *legacyEngine) recordEvent(event watch.EventType, un *unstructured.Unstructured) {
	c := e.c
	for _, h := range c.getEventHandlers() {
		h(event, un)
	}
	key := kube.GetResourceKey(un)
	if event == watch.Modified && skipAppRequeuing(key) {
		return
	}

	if c.batchEventsProcessing {
		e.eventMetaCh <- eventMeta{event, un}
	} else {
		c.lock.Lock()
		defer c.lock.Unlock()
		e.processEvent(key, eventMeta{event, un})
	}
}

func (e *legacyEngine) processEvents() {
	c := e.c
	log := c.log.WithValues("functionName", "processItems")
	log.V(1).Info("Start processing events")

	c.lock.Lock()
	ch := e.eventMetaCh
	c.lock.Unlock()

	eventMetas := make([]eventMeta, 0)
	ticker := time.NewTicker(c.eventProcessingInterval)
	defer ticker.Stop()

	for {
		select {
		case evMeta, ok := <-ch:
			if !ok {
				log.V(2).Info("Event processing channel closed, finish processing")
				return
			}
			eventMetas = append(eventMetas, evMeta)
		case <-ticker.C:
			if len(eventMetas) > 0 {
				e.processEventsBatch(eventMetas)
				eventMetas = eventMetas[:0]
			}
		}
	}
}

func (e *legacyEngine) processEventsBatch(eventMetas []eventMeta) {
	c := e.c
	log := c.log.WithValues("functionName", "processEventsBatch")
	start := time.Now()
	c.lock.RLock()
	populateResourceInfoHandler := c.populateResourceInfoHandler
	manifestCompressionEnabled := c.manifestCompressionEnabled
	manifestStorageType := c.manifestStorageType
	manifestCompressionType := c.manifestCompressionType
	c.lock.RUnlock()

	resources := make([]*Resource, len(eventMetas))
	for i, evMeta := range eventMetas {
		if evMeta.event != watch.Deleted {
			resources[i] = c.newResourceWithManifestSettings(evMeta.un, populateResourceInfoHandler, manifestCompressionEnabled, manifestStorageType, manifestCompressionType)
		}
	}

	c.lock.Lock()
	log.V(1).Info("Lock acquired (ms)", "duration", time.Since(start).Milliseconds())
	defer func() {
		c.lock.Unlock()
		duration := time.Since(start)
		// Update the metric with the duration of the events processing
		for _, handler := range c.getProcessEventsHandlers() {
			handler(duration, len(eventMetas))
		}
	}()

	for i, evMeta := range eventMetas {
		key := kube.GetResourceKey(evMeta.un)
		e.processEventWithResource(key, evMeta, resources[i])
	}

	log.V(1).Info("Processed events (ms)", "count", len(eventMetas), "duration", time.Since(start).Milliseconds())
}

func (e *legacyEngine) processEvent(key kube.ResourceKey, evMeta eventMeta) {
	e.processEventWithResource(key, evMeta, nil)
}

func (e *legacyEngine) processEventWithResource(key kube.ResourceKey, evMeta eventMeta, resource *Resource) {
	c := e.c
	existingNode, exists := c.resources[key]
	if evMeta.event == watch.Deleted {
		if exists {
			c.onNodeRemoved(key)
		}
	} else {
		if resource == nil {
			resource = c.newResource(evMeta.un)
		}
		c.onNodeUpdated(existingNode, resource)
	}
}
