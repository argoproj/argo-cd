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
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/pager"
	watchutil "k8s.io/client-go/tools/watch"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
)

// legacyEngine loads each (GroupKind, namespace) with a paged list, then keeps
// it current from a retrying watch goroutine. It reaches shared state only
// through its *store.
type legacyEngine struct {
	c *store

	// eventMetaCh carries watch events to processEvents when
	// batchEventsProcessing is set, and is nil otherwise. Created in sync,
	// closed by invalidateEventMeta.
	eventMetaCh chan eventMeta
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
				resourceVersion, err := e.loadInitialState(ctx, api, resClient, ns, false) // don't lock here, we are already in a lock before startMissingWatches is called inside watchEvents
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

// onInvalidate retires the batched-event channel. Invalidate calls it under
// c.lock after applying its opts, so batchEventsProcessing is current.
func (e *legacyEngine) onInvalidate() {
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

func (e *legacyEngine) watchEvents(ctx context.Context, api kube.APIResourceInfo, resClient dynamic.ResourceInterface, ns string, resourceVersion string) {
	c := e.c
	kube.RetryUntilSucceed(ctx, watchResourcesRetryTimeout, fmt.Sprintf("watch %s on %s", api.GroupKind, c.config.Host), c.log, func() (err error) {
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

		w, err := watchutil.NewRetryWatcherWithContext(ctx, resourceVersion, &cache.ListWatch{
			WatchFuncWithContext: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
				options.LabelSelector = api.LabelSelector
				res, err := resClient.Watch(ctx, options)
				if apierrors.IsNotFound(err) {
					e.stopWatching(api.GroupKind, ns)
				}
				//nolint:wrapcheck // wrap outside the retry
				return res, err
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create resource watcher: %w", err)
		}

		defer func() {
			w.Stop()
			resourceVersion = ""
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

			// re-synchronize API state and restart watch if retry watcher failed to continue watching using provided resource version
			case <-w.Done():
				return fmt.Errorf("watch %s on %s has closed", api.GroupKind, c.config.Host)

			case event, ok := <-w.ResultChan():
				if !ok {
					return fmt.Errorf("watch %s on %s has closed", api.GroupKind, c.config.Host)
				}

				obj, ok := event.Object.(*unstructured.Unstructured)
				if !ok {
					return fmt.Errorf("failed to convert to *unstructured.Unstructured: %v", event.Object)
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
