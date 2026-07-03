// Package cache provides a caching layer for Kubernetes cluster resources with support for
// hierarchical parent-child relationships, including cross-namespace relationships between
// cluster-scoped parents and namespaced children.
//
// The cache maintains:
//   - A complete index of all monitored resources in the cluster
//   - Hierarchical relationships between resources via owner references
//   - Cross-namespace relationships from cluster-scoped resources to namespaced children
//   - Efficient traversal of resource hierarchies for dependency analysis
//
// Key features:
//   - Watches cluster resources and maintains an in-memory cache synchronized with the cluster state
//   - Supports both same-namespace parent-child relationships and cross-namespace relationships
//   - Uses pre-computed indexes for efficient hierarchy traversal without full cluster scans
//   - Provides configurable namespaces and resource filtering
//   - Handles dynamic resource discovery including CRDs
//
// Cross-namespace hierarchy traversal:
// The cache supports discovering namespaced resources that are owned by cluster-scoped resources.
// This is essential for tracking resources like namespaced Deployments owned by cluster-scoped
// custom resources.
//
// The parentUIDToChildren index enables efficient O(1) cross-namespace traversal by mapping
// any resource's UID to its direct children, eliminating the need for O(n) graph building.
package cache

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/sync/semaphore"
	authorizationv1 "k8s.io/api/authorization/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	authType1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/pager"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2/textlogger"
	"k8s.io/kubectl/pkg/util/openapi"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/tracing"
)

const (
	watchResourcesRetryTimeout = 1 * time.Second
	ClusterRetryTimeout        = 10 * time.Second

	// default duration before we invalidate entire cluster cache. Can be set to 0 to never invalidate cache
	defaultClusterResyncTimeout = 24 * time.Hour

	// default duration before restarting individual resource watch
	defaultWatchResyncTimeout = 10 * time.Minute

	// Same page size as in k8s.io/client-go/tools/pager/pager.go
	defaultListPageSize = 500
	// Prefetch only a single page
	defaultListPageBufferSize = 1
	// listSemaphore is used to limit the number of concurrent memory consuming operations on the
	// k8s list queries results.
	// Limit is required to avoid memory spikes during cache initialization.
	// The default limit of 50 is chosen based on experiments.
	defaultListSemaphoreWeight = 50
	// defaultEventProcessingInterval is the default interval for processing events
	defaultEventProcessingInterval = 100 * time.Millisecond
)

// errCacheInvalidatedMidSync signals that an Invalidate fired during the
// WaitForCacheSync window inside syncInformers. EnsureSynced treats it as
// transient and refuses to cache it, so the next call re-syncs immediately
// rather than serving the stale error for clusterSyncRetryTimeout.
var errCacheInvalidatedMidSync = errors.New("cluster cache invalidated during initial informer sync")

const (
	// RespectRbacDisabled default value for respectRbac
	RespectRbacDisabled = iota
	// RespectRbacNormal checks only api response for forbidden/unauthorized errors
	RespectRbacNormal
	// RespectRbacStrict checks both api response for forbidden/unauthorized errors and SelfSubjectAccessReview
	RespectRbacStrict
)

// callState tracks whether action() has been called on a resource during hierarchy iteration.
type callState int

const (
	notCalled  callState = iota // action() has not been called yet
	inProgress                  // action() is currently being processed (in call stack)
	completed                   // action() has been called and processing is complete
)

type apiMeta struct {
	namespaced bool
	// watchCancel stops the watch of all resources for this API. This gets called when the cache is invalidated or when
	// the watched API ceases to exist (e.g. a CRD gets deleted).
	watchCancel context.CancelFunc
}

type eventMeta struct {
	event watch.EventType
	un    *unstructured.Unstructured
}

// ClusterInfo holds cluster cache stats
type ClusterInfo struct {
	// Server holds cluster API server URL
	Server string
	// K8SVersion holds Kubernetes version
	K8SVersion string
	// ResourcesCount holds number of observed Kubernetes resources
	ResourcesCount int
	// APIsCount holds number of observed Kubernetes API count
	APIsCount int
	// LastCacheSyncTime holds time of most recent cache synchronization
	LastCacheSyncTime *time.Time
	// SyncError holds most recent cache synchronization error
	SyncError error
	// APIResources holds list of API resources supported by the cluster
	APIResources []kube.APIResourceInfo
}

// OnEventHandler is a function that handles Kubernetes event
type OnEventHandler func(event watch.EventType, un *unstructured.Unstructured)

// OnProcessEventsHandler handles process events event
type OnProcessEventsHandler func(duration time.Duration, processedEventsNumber int)

// OnPopulateResourceInfoHandler returns additional resource metadata that should be stored in cache
type OnPopulateResourceInfoHandler func(un *unstructured.Unstructured, isRoot bool) (info any, cacheManifest bool)

// OnResourceUpdatedHandler handlers resource update event
type (
	OnResourceUpdatedHandler func(newRes *Resource, oldRes *Resource, namespaceResources map[kube.ResourceKey]*Resource)
	Unsubscribe              func()
)

type ClusterCache interface {
	// EnsureSynced checks cache state and synchronizes it if necessary
	EnsureSynced() error
	// GetServerVersion returns observed cluster version
	GetServerVersion() string
	// GetAPIResources returns information about observed API resources
	GetAPIResources() []kube.APIResourceInfo
	// GetOpenAPISchema returns open API schema of supported API resources
	GetOpenAPISchema() openapi.Resources
	// GetGVKParser returns a parser able to build a TypedValue used in
	// structured merge diffs.
	GetGVKParser() *managedfields.GvkParser
	// Invalidate cache and executes callback that optionally might update cache settings
	Invalidate(opts ...UpdateSettingsFunc)
	// FindResources returns resources that matches given list of predicates from specified namespace or everywhere if specified namespace is empty
	FindResources(namespace string, predicates ...func(r *Resource) bool) map[kube.ResourceKey]*Resource
	// IterateHierarchyV2 iterates resource tree starting from the specified top level resources and executes callback for each resource in the tree.
	// The action callback returns true if iteration should continue and false otherwise.
	IterateHierarchyV2(keys []kube.ResourceKey, action func(resource *Resource, namespaceResources map[kube.ResourceKey]*Resource) bool)
	// IsNamespaced answers if specified group/kind is a namespaced resource API or not
	IsNamespaced(gk schema.GroupKind) (bool, error)
	// GetManagedLiveObjs helps finding matching live K8S resources for a given resources list.
	// The function returns all resources from cache for those `isManaged` function returns true and resources
	// specified in targetObjs list.
	GetManagedLiveObjs(targetObjs []*unstructured.Unstructured, isManaged func(r *Resource) bool) (map[kube.ResourceKey]*unstructured.Unstructured, error)
	// GetClusterInfo returns cluster cache statistics
	GetClusterInfo() ClusterInfo
	// OnResourceUpdated register event handler that is executed every time when resource get's updated in the cache
	OnResourceUpdated(handler OnResourceUpdatedHandler) Unsubscribe
	// OnEvent register event handler that is executed every time when new K8S event received
	OnEvent(handler OnEventHandler) Unsubscribe
	// OnProcessEventsHandler register event handler that is executed every time when events were processed
	OnProcessEventsHandler(handler OnProcessEventsHandler) Unsubscribe
}

type WeightedSemaphore interface {
	Acquire(ctx context.Context, n int64) error
	TryAcquire(n int64) bool
	Release(n int64)
}

type ListRetryFunc func(err error) bool

// NewClusterCache creates new instance of cluster cache.
//
// The concrete implementation is selected by SetMode; ModeLegacy (the
// default) uses the hand-rolled list/watch loop, and ModeInformer uses
// client-go's SharedIndexInformer (experimental — see issue #19199).
func NewClusterCache(config *rest.Config, opts ...UpdateSettingsFunc) ClusterCache {
	log := textlogger.NewLogger(textlogger.NewConfig())
	s := &store{
		settings:           Settings{ResourceHealthOverride: &noopSettings{}, ResourcesFilter: &noopSettings{}},
		apisMeta:           make(map[schema.GroupKind]*apiMeta),
		listPageSize:       defaultListPageSize,
		listPageBufferSize: defaultListPageBufferSize,
		listSemaphore:      semaphore.NewWeighted(defaultListSemaphoreWeight),
		resources:          make(map[kube.ResourceKey]*Resource),
		nsIndex:            make(map[string]map[kube.ResourceKey]*Resource),
		config:             config,
		kubectl: &kube.KubectlCmd{
			Log:    log,
			Tracer: tracing.NopTracer{},
		},
		syncStatus: clusterCacheSync{
			resyncTimeout: defaultClusterResyncTimeout,
			syncTime:      nil,
		},
		watchResyncTimeout:      defaultWatchResyncTimeout,
		clusterSyncRetryTimeout: ClusterRetryTimeout,
		eventProcessingInterval: defaultEventProcessingInterval,
		resourceUpdatedHandlers: map[uint64]OnResourceUpdatedHandler{},
		eventHandlers:           map[uint64]OnEventHandler{},
		processEventsHandlers:   map[uint64]OnProcessEventsHandler{},
		log:                     log,
		listRetryLimit:          1,
		listRetryUseBackoff:     false,
		listRetryFunc:           ListRetryFuncNever,
		parentUIDToChildren:     make(map[types.UID]map[kube.ResourceKey]struct{}),
	}
	cache := &clusterCache{
		store: s,
		// Default to the legacy engine; a SetMode option below may replace it.
		// Set before applying opts so SetMode (which swaps cache.engine) wins.
		engine: newSyncEngine(ModeLegacy, s),
	}
	// Give the store a way to resolve the active engine at call time (see the
	// currentEngine field doc). Reads cache.engine, so callers hold store.lock.
	s.currentEngine = func() syncEngine { return cache.engine }
	for i := range opts {
		opts[i](cache)
	}
	return cache
}

// store holds the shared, mode-agnostic cluster cache state and operations:
// the resource index, the hierarchy indexes, discovery/config, event handler
// registries, and cluster metadata. It is a leaf: each engine holds a *store,
// but store holds no direct reference to any engine (the one shared op that
// needs one, handleCRDEvent, resolves the ACTIVE engine at call time through
// the currentEngine indirection installed by the facade). This keeps the
// dependency one-directional — engines depend on store, not vice versa — and
// keeps the legacy and informer lifecycles decoupled from each other and from
// the public type.
//
// Field placement follows one rule: store owns configuration and shared
// state; each engine owns its own per-sync runtime state.
//   - Configuration (set through the UpdateSettingsFunc API, which targets the
//     cache, not an engine) lives here even when only one engine consumes it —
//     e.g. batchEventsProcessing / eventProcessingInterval / watchResyncTimeout
//     are read only by legacyEngine, and listPageSize / listRetry* only by
//     store.listResources, but all are cache settings. Moving them onto an
//     engine would force their setters to type-assert the active engine, with
//     an effect that depends on SetMode's position in the opts list.
//   - Per-sync runtime state that is meaningless to share lives on the engine:
//     legacyEngine.eventMetaCh (the batching channel) and
//     informerEngine.firstSyncCompleted.
//
// clusterCache embeds *store and adds only the facade concerns: the active
// engine (the mode selector) and the EnsureSynced single-flight gate.
type store struct {
	syncStatus clusterCacheSync

	// currentEngine returns the facade's ACTIVE engine (clusterCache.engine).
	// Installed by NewClusterCache; must be called under store.lock, since
	// SetMode (via Invalidate) swaps the engine under that lock. It exists so
	// handleCRDEvent always re-enters the engine that is current at call time:
	// dispatching on an engine captured earlier (e.g. by a watch goroutine
	// spawned before an Invalidate(SetMode(...))) would let a stale engine
	// restart its watch machinery alongside the replacement's.
	currentEngine func() syncEngine

	apisMeta              map[schema.GroupKind]*apiMeta
	batchEventsProcessing bool
	serverVersion         string
	apiResources          []kube.APIResourceInfo
	// namespacedResources is a simple map which indicates a groupKind is namespaced
	namespacedResources map[schema.GroupKind]bool

	// maximum time we allow watches to run before relisting the group/kind and restarting the watch
	watchResyncTimeout time.Duration
	// sync retry timeout for cluster when sync error happens
	clusterSyncRetryTimeout time.Duration
	// ticker interval for events processing
	eventProcessingInterval time.Duration

	// size of a page for list operations pager.
	listPageSize int64
	// number of pages to prefetch for list pager.
	listPageBufferSize int32
	listSemaphore      WeightedSemaphore

	// retry options for list operations
	listRetryLimit      int32
	listRetryUseBackoff bool
	listRetryFunc       ListRetryFunc

	// lock is a rw lock which protects the fields of clusterInfo
	lock      sync.RWMutex
	resources map[kube.ResourceKey]*Resource
	nsIndex   map[string]map[kube.ResourceKey]*Resource

	kubectl          kube.Kubectl
	log              logr.Logger
	config           *rest.Config
	namespaces       []string
	clusterResources bool
	settings         Settings

	handlersLock                sync.Mutex
	handlerKey                  uint64
	populateResourceInfoHandler OnPopulateResourceInfoHandler
	resourceUpdatedHandlers     map[uint64]OnResourceUpdatedHandler
	eventHandlers               map[uint64]OnEventHandler
	processEventsHandlers       map[uint64]OnProcessEventsHandler
	openAPISchema               openapi.Resources
	gvkParser                   *managedfields.GvkParser

	respectRBAC int

	// Parent-to-children index for O(1) child lookup during hierarchy traversal
	// Maps any resource's UID to a set of its direct children's ResourceKeys
	// Using a set eliminates O(k) duplicate checking on insertions
	// Used for cross-namespace hierarchy traversal; namespaced traversal still builds a graph
	parentUIDToChildren map[types.UID]map[kube.ResourceKey]struct{}
}

// clusterCache is the public-facing ClusterCache implementation. It embeds
// *store (promoting the shared read/query methods) and owns only the facade
// concerns that don't belong to either engine.
type clusterCache struct {
	*store

	// engine holds the mode-specific lifecycle implementation (legacy
	// list/watch vs. informer) and is the authoritative selector for which
	// implementation is active. Defaults to the legacy engine; SetMode swaps
	// it. Lives on the facade rather than *store so the shared layer stays a
	// leaf; the one store op that needs it (handleCRDEvent) receives it as a
	// parameter. See syncEngine.
	engine syncEngine

	// syncMu serializes EnsureSynced calls to enforce single-flight sync.
	// Under informer mode, sync() releases store.lock around WaitForCacheSync;
	// without this gate a second EnsureSynced caller could acquire store.lock
	// in that window, enter syncInformers, and cancel the first caller's
	// informers mid-flight. Acquired before store.lock; never held by any
	// other code path (notably NOT by Invalidate, which must be able to
	// interrupt an in-progress sync).
	syncMu sync.Mutex
}

type clusterCacheSync struct {
	// When using this struct:
	// 1) 'lock' mutex should be acquired when reading/writing from fields of this struct.
	// 2) The parent 'store.lock' does NOT need to be owned to r/w from fields of this struct (if it is owned, that is fine, but see below)
	// 3) To prevent deadlocks, do not acquire parent 'store.lock' after acquiring this lock; if you need both locks, always acquire the parent lock first
	lock          sync.Mutex
	syncTime      *time.Time
	syncError     error
	resyncTimeout time.Duration
}

// ListRetryFuncNever never retries on errors
func ListRetryFuncNever(_ error) bool {
	return false
}

// ListRetryFuncAlways always retries on errors
func ListRetryFuncAlways(_ error) bool {
	return true
}

// OnResourceUpdated register event handler that is executed every time when resource get's updated in the cache
func (c *store) OnResourceUpdated(handler OnResourceUpdatedHandler) Unsubscribe {
	c.handlersLock.Lock()
	defer c.handlersLock.Unlock()
	key := c.handlerKey
	c.handlerKey++
	c.resourceUpdatedHandlers[key] = handler
	return func() {
		c.handlersLock.Lock()
		defer c.handlersLock.Unlock()
		delete(c.resourceUpdatedHandlers, key)
	}
}

func (c *store) getResourceUpdatedHandlers() []OnResourceUpdatedHandler {
	c.handlersLock.Lock()
	defer c.handlersLock.Unlock()
	var handlers []OnResourceUpdatedHandler
	for _, h := range c.resourceUpdatedHandlers {
		handlers = append(handlers, h)
	}
	return handlers
}

// OnEvent register event handler that is executed every time when new K8S event received
func (c *store) OnEvent(handler OnEventHandler) Unsubscribe {
	c.handlersLock.Lock()
	defer c.handlersLock.Unlock()
	key := c.handlerKey
	c.handlerKey++
	c.eventHandlers[key] = handler
	return func() {
		c.handlersLock.Lock()
		defer c.handlersLock.Unlock()
		delete(c.eventHandlers, key)
	}
}

func (c *store) getEventHandlers() []OnEventHandler {
	c.handlersLock.Lock()
	defer c.handlersLock.Unlock()
	handlers := make([]OnEventHandler, 0, len(c.eventHandlers))
	for _, h := range c.eventHandlers {
		handlers = append(handlers, h)
	}
	return handlers
}

// OnProcessEventsHandler register event handler that is executed every time when events were processed
func (c *store) OnProcessEventsHandler(handler OnProcessEventsHandler) Unsubscribe {
	c.handlersLock.Lock()
	defer c.handlersLock.Unlock()
	key := c.handlerKey
	c.handlerKey++
	c.processEventsHandlers[key] = handler
	return func() {
		c.handlersLock.Lock()
		defer c.handlersLock.Unlock()
		delete(c.processEventsHandlers, key)
	}
}

func (c *store) getProcessEventsHandlers() []OnProcessEventsHandler {
	c.handlersLock.Lock()
	defer c.handlersLock.Unlock()
	handlers := make([]OnProcessEventsHandler, 0, len(c.processEventsHandlers))
	for _, h := range c.processEventsHandlers {
		handlers = append(handlers, h)
	}
	return handlers
}

// GetServerVersion returns observed cluster version
func (c *store) GetServerVersion() string {
	return c.serverVersion
}

// GetAPIResources returns information about observed API resources
// This method is called frequently during reconciliation to pass API resource info to `helm template`
// NOTE: we do not provide any consistency guarantees about the returned list. The list might be
// updated in place (anytime new CRDs are introduced or removed). If necessary, a separate method
// would need to be introduced to return a copy of the list so it can be iterated consistently.
func (c *store) GetAPIResources() []kube.APIResourceInfo {
	return c.apiResources
}

// GetOpenAPISchema returns open API schema of supported API resources
func (c *store) GetOpenAPISchema() openapi.Resources {
	return c.openAPISchema
}

// GetGVKParser returns a parser able to build a TypedValue used in
// structured merge diffs.
func (c *store) GetGVKParser() *managedfields.GvkParser {
	return c.gvkParser
}

func (c *store) appendAPIResource(info kube.APIResourceInfo) {
	exists := false
	for i := range c.apiResources {
		if c.apiResources[i].GroupKind == info.GroupKind && c.apiResources[i].GroupVersionResource.Version == info.GroupVersionResource.Version {
			exists = true
			break
		}
	}
	if !exists {
		c.apiResources = append(c.apiResources, info)
	}
}

func (c *store) deleteAPIResource(info kube.APIResourceInfo) {
	for i := range c.apiResources {
		if c.apiResources[i].GroupKind == info.GroupKind && c.apiResources[i].GroupVersionResource.Version == info.GroupVersionResource.Version {
			c.apiResources[i] = c.apiResources[len(c.apiResources)-1]
			c.apiResources = c.apiResources[:len(c.apiResources)-1]
			break
		}
	}
}

func (c *store) replaceResourceCache(gk schema.GroupKind, resources []*Resource, ns string) {
	objByKey := make(map[kube.ResourceKey]*Resource)
	for i := range resources {
		objByKey[resources[i].ResourceKey()] = resources[i]
	}

	// update existing nodes
	for i := range resources {
		res := resources[i]
		oldRes := c.resources[res.ResourceKey()]
		if oldRes == nil || oldRes.ResourceVersion != res.ResourceVersion {
			c.onNodeUpdated(oldRes, res)
		}
	}

	for key := range c.resources {
		if key.Kind != gk.Kind || key.Group != gk.Group || ns != "" && key.Namespace != ns {
			continue
		}

		if _, ok := objByKey[key]; !ok {
			c.onNodeRemoved(key)
		}
	}
}

func (c *store) newResource(un *unstructured.Unstructured) *Resource {
	ownerRefs, isInferredParentOf := c.resolveResourceReferences(un)

	cacheManifest := false
	var info any
	if c.populateResourceInfoHandler != nil {
		info, cacheManifest = c.populateResourceInfoHandler(un, len(ownerRefs) == 0)
	}
	var creationTimestamp *metav1.Time
	ct := un.GetCreationTimestamp()
	if !ct.IsZero() {
		creationTimestamp = &ct
	}
	resource := &Resource{
		ResourceVersion:    un.GetResourceVersion(),
		Ref:                kube.GetObjectRef(un),
		OwnerRefs:          ownerRefs,
		Info:               info,
		CreationTimestamp:  creationTimestamp,
		isInferredParentOf: isInferredParentOf,
	}
	if cacheManifest {
		resource.Resource = un
	}

	return resource
}

// setNode is the store's write primitive for resource upserts: it writes to
// c.resources and keeps the shared cross-GK indexes consistent via
// updateIndexes. Both engines route upserts through it — legacy via
// sync/processEvent (and onNodeUpdated), informer via onInformerChange — so
// index maintenance lives in exactly one place. It performs no dispatch;
// callers decide whether and when to fire OnResourceUpdated handlers.
// Caller must hold store.lock.
func (c *store) setNode(n *Resource) {
	key := n.ResourceKey()

	// Keep track of existing resource for index updates
	existing := c.resources[key]

	c.resources[key] = n
	c.updateIndexes(existing, n)
}

// updateIndexes maintains nsIndex, parentUIDToChildren, and inferred-parent
// ref propagation when a Resource is added or replaced. Shared by the
// legacy and informer impls — both need these cross-GK indexes to serve
// IterateHierarchyV2 and OnResourceUpdated dispatch.
//
// existing is the previous Resource for this key, or nil on first sight.
// The legacy caller reads it from c.resources before the write; the
// informer caller receives it via cache.ResourceEventHandler's UpdateFunc.
// Caller must hold c.lock.
func (c *store) updateIndexes(existing, n *Resource) {
	key := n.ResourceKey()
	ns, ok := c.nsIndex[key.Namespace]
	if !ok {
		ns = make(map[kube.ResourceKey]*Resource)
		c.nsIndex[key.Namespace] = ns
	}
	ns[key] = n

	// Update parent-to-children index for all resources with owner refs
	// This is always done, regardless of sync state, as it's cheap to maintain
	c.updateParentUIDToChildren(key, existing, n)

	// update inferred parent references
	if n.isInferredParentOf != nil || mightHaveInferredOwner(n) {
		for k, v := range ns {
			// update child resource owner references
			if n.isInferredParentOf != nil && mightHaveInferredOwner(v) {
				shouldBeParent := n.isInferredParentOf(k)
				v.setOwnerRef(n.toOwnerRef(), shouldBeParent)
				// Update index inline for inferred ref changes.
				// Note: The removal case (shouldBeParent=false) is currently unreachable for
				// StatefulSet→PVC relationships because Kubernetes makes volumeClaimTemplates
				// immutable. We include it for defensive correctness and future-proofing.
				if n.Ref.UID != "" {
					if shouldBeParent {
						c.addToParentUIDToChildren(n.Ref.UID, k)
					} else {
						c.removeFromParentUIDToChildren(n.Ref.UID, k)
					}
				}
			}
			if mightHaveInferredOwner(n) && v.isInferredParentOf != nil {
				childKey := n.ResourceKey()
				shouldBeParent := v.isInferredParentOf(childKey)
				n.setOwnerRef(v.toOwnerRef(), shouldBeParent)
				// Update index inline for inferred ref changes.
				// Note: The removal case (shouldBeParent=false) is currently unreachable for
				// StatefulSet→PVC relationships because Kubernetes makes volumeClaimTemplates
				// immutable. We include it for defensive correctness and future-proofing.
				if v.Ref.UID != "" {
					if shouldBeParent {
						c.addToParentUIDToChildren(v.Ref.UID, childKey)
					} else {
						c.removeFromParentUIDToChildren(v.Ref.UID, childKey)
					}
				}
			}
		}
	}
}

// addToParentUIDToChildren adds a child to the parent-to-children index
func (c *store) addToParentUIDToChildren(parentUID types.UID, childKey kube.ResourceKey) {
	// Get or create the set for this parent
	childrenSet := c.parentUIDToChildren[parentUID]
	if childrenSet == nil {
		childrenSet = make(map[kube.ResourceKey]struct{})
		c.parentUIDToChildren[parentUID] = childrenSet
	}
	// Add child to set (O(1) operation, automatically handles duplicates)
	childrenSet[childKey] = struct{}{}
}

// removeFromParentUIDToChildren removes a child from the parent-to-children index
func (c *store) removeFromParentUIDToChildren(parentUID types.UID, childKey kube.ResourceKey) {
	childrenSet := c.parentUIDToChildren[parentUID]
	if childrenSet == nil {
		return
	}

	// Remove child from set (O(1) operation)
	delete(childrenSet, childKey)

	// Clean up empty sets to avoid memory leaks
	if len(childrenSet) == 0 {
		delete(c.parentUIDToChildren, parentUID)
	}
}

// updateParentUIDToChildren updates the parent-to-children index when a resource's owner refs change
func (c *store) updateParentUIDToChildren(childKey kube.ResourceKey, oldResource *Resource, newResource *Resource) {
	// Build sets of old and new parent UIDs
	oldParents := make(map[types.UID]struct{})
	if oldResource != nil {
		for _, ref := range oldResource.OwnerRefs {
			if ref.UID != "" {
				oldParents[ref.UID] = struct{}{}
			}
		}
	}

	newParents := make(map[types.UID]struct{})
	for _, ref := range newResource.OwnerRefs {
		if ref.UID != "" {
			newParents[ref.UID] = struct{}{}
		}
	}

	// Remove from parents that are no longer in owner refs
	for oldUID := range oldParents {
		if _, exists := newParents[oldUID]; !exists {
			c.removeFromParentUIDToChildren(oldUID, childKey)
		}
	}

	// Add to parents that are new in owner refs
	for newUID := range newParents {
		if _, exists := oldParents[newUID]; !exists {
			c.addToParentUIDToChildren(newUID, childKey)
		}
	}
}

// Invalidate cache and executes callback that optionally might update cache settings
func (c *clusterCache) Invalidate(opts ...UpdateSettingsFunc) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.syncStatus.lock.Lock()
	c.syncStatus.syncTime = nil
	c.syncStatus.lock.Unlock()

	for i := range c.apisMeta {
		c.apisMeta[i].watchCancel()
	}

	// Let the currently-active engine drop its own per-invalidate runtime state
	// (legacy retires its batched-event channel; informer resets its first-sync
	// flag), keeping engine-private fields out of the facade. This MUST run
	// before applying opts: opts may include SetMode, which swaps c.engine for
	// a fresh instance — teardown then has to target the OLD engine (the one
	// holding the live channel / flag), not its replacement. Note that the
	// watchCancel loop above does NOT guarantee producers (watchEvents
	// goroutines) have stopped — cancellation is asynchronous and cannot unpark
	// a goroutine already blocked in a channel send — which is why the legacy
	// engine retires the channel via its done signal instead of closing it
	// (see invalidateEventMeta).
	c.engine.onInvalidate()

	for i := range opts {
		opts[i](c)
	}

	c.apisMeta = nil
	c.namespacedResources = nil
	// Intentionally do NOT clear c.resources / c.nsIndex / c.parentUIDToChildren
	// here under informer mode — matching legacy semantics where those maps
	// survive Invalidate and serve stale-but-present data until the next
	// sync() rebuilds them. Clearing here caused a GetManagedLiveObjs
	// regression: with c.resources empty, its per-target fallback issued
	// N synchronous API GETs per app reconcile in the Invalidate ->
	// EnsureSynced window, instead of serving from the in-memory snapshot.
	// Safe to leave populated because:
	//   1. syncInformers itself wipes and rebuilds these at sync start,
	//   2. the in-lock ctx.Err() guard in onInformerChange prevents stale
	//      pre-cancel event handlers from writing into them after their
	//      apiMeta.watchCtx is cancelled (see informer_events.go).
	c.log.Info("Invalidated cluster")
}

// clusterCacheSync's lock should be held before calling this method
func (syncStatus *clusterCacheSync) synced(clusterRetryTimeout time.Duration) bool {
	syncTime := syncStatus.syncTime

	if syncTime == nil {
		return false
	}
	if syncStatus.syncError != nil {
		return time.Now().Before(syncTime.Add(clusterRetryTimeout))
	}
	if syncStatus.resyncTimeout == 0 {
		// cluster resync timeout has been disabled
		return true
	}
	return time.Now().Before(syncTime.Add(syncStatus.resyncTimeout))
}

func runSynced(lock sync.Locker, action func() error) error {
	lock.Lock()
	defer lock.Unlock()
	return action()
}

// listResources creates list pager and enforces number of concurrent list requests
// The callback should not wait on any locks that may be held by other callers.
func (c *store) listResources(ctx context.Context, resClient dynamic.ResourceInterface, callback func(*pager.ListPager) error) (string, error) {
	if err := c.listSemaphore.Acquire(ctx, 1); err != nil {
		return "", fmt.Errorf("failed to acquire list semaphore: %w", err)
	}
	defer c.listSemaphore.Release(1)

	var retryCount int64
	resourceVersion := ""
	listPager := pager.New(func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
		var res *unstructured.UnstructuredList
		var listRetry wait.Backoff

		if c.listRetryUseBackoff {
			listRetry = retry.DefaultBackoff
		} else {
			listRetry = retry.DefaultRetry
		}

		listRetry.Steps = int(c.listRetryLimit)
		err := retry.OnError(listRetry, c.listRetryFunc, func() error {
			var ierr error
			res, ierr = resClient.List(ctx, opts)
			if ierr != nil {
				// Log out a retry
				if c.listRetryLimit > 1 && c.listRetryFunc(ierr) {
					retryCount++
					c.log.Info(fmt.Sprintf("Error while listing resources: %v (try %d/%d)", ierr, retryCount, c.listRetryLimit))
				}
				// Ensure res is never nil even when there's an error
				if res == nil {
					res = &unstructured.UnstructuredList{}
				}
				//nolint:wrapcheck // wrap outside the retry
				return ierr
			}
			resourceVersion = res.GetResourceVersion()
			return nil
		})
		if err != nil {
			return res, fmt.Errorf("failed to list resources: %w", err)
		}
		return res, nil
	})
	listPager.PageBufferSize = c.listPageBufferSize
	listPager.PageSize = c.listPageSize

	return resourceVersion, callback(listPager)
}

// handleCRDEvent reacts to a CRD add/modify/delete event on the watch
// stream. It updates c.apiResources, triggers startMissingWatches for new
// or changed CRDs, and reloads the OpenAPI schema. The engine is resolved via
// c.currentEngine AT CALL TIME, under store.lock: the emitting goroutine may
// have been spawned by an engine that Invalidate(SetMode(...)) has since
// replaced, and startMissingWatches must run on the active engine, not the
// stale one (pre-engine-split code got this for free by re-reading c.mode).
// Safe to call from any event source — the legacy watch loop invokes it
// directly, and the informer engine calls it from its event handler via
// dispatchEvent.
func (c *store) handleCRDEvent(event watch.EventType, obj *unstructured.Unstructured) {
	resources, err := crdVersionsToAPIResources(obj)
	if err != nil {
		c.log.Error(err, "Failed to extract CRD resources")
		// Fall through: startMissingWatches and the OpenAPI reload still
		// run, matching the original inline behavior where extraction
		// failure left resources empty but didn't short-circuit.
	}

	// Identify the CRD by the GroupKind of the resource it defines, not the
	// apiextensions.k8s.io/CustomResourceDefinition wrapper. Falls back to
	// the CRD's name when extraction failed and resources is empty.
	innerGK := obj.GetName()
	if len(resources) > 0 {
		innerGK = resources[0].GroupKind.String()
	}

	if event == watch.Deleted {
		for i := range resources {
			c.deleteAPIResource(resources[i])
		}
	} else {
		c.log.Info("Updating Kubernetes APIs, watches, and Open API schemas due to CRD event",
			"eventType", event, "groupKind", innerGK)
		if event == watch.Added {
			for i := range resources {
				c.appendAPIResource(resources[i])
			}
		}
		if err := runSynced(&c.lock, func() error {
			return c.currentEngine().startMissingWatches()
		}); err != nil {
			c.log.Error(err, "Failed to start missing watch")
		}
	}

	if err := runSynced(&c.lock, c.reloadOpenAPISchema); err != nil {
		c.log.Error(err, "Failed to reload open api schema")
	}
}

// crdVersionsToAPIResources decodes an unstructured CRD and expands its
// spec.versions into one kube.APIResourceInfo per version.
func crdVersionsToAPIResources(obj *unstructured.Unstructured) ([]kube.APIResourceInfo, error) {
	var crd apiextensionsv1.CustomResourceDefinition
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &crd); err != nil {
		return nil, fmt.Errorf("extract CRD from unstructured: %w", err)
	}
	resources := make([]kube.APIResourceInfo, 0, len(crd.Spec.Versions))
	for _, v := range crd.Spec.Versions {
		resources = append(resources, kube.APIResourceInfo{
			GroupKind: schema.GroupKind{Group: crd.Spec.Group, Kind: crd.Spec.Names.Kind},
			GroupVersionResource: schema.GroupVersionResource{
				Group: crd.Spec.Group, Version: v.Name, Resource: crd.Spec.Names.Plural,
			},
			Meta: metav1.APIResource{
				Group:        crd.Spec.Group,
				SingularName: crd.Spec.Names.Singular,
				Namespaced:   crd.Spec.Scope == apiextensionsv1.NamespaceScoped,
				Name:         crd.Spec.Names.Plural,
				Kind:         crd.Spec.Names.Singular,
				Version:      v.Name,
				ShortNames:   crd.Spec.Names.ShortNames,
			},
		})
	}
	return resources, nil
}

// reloadOpenAPISchema fetches the current OpenAPI schema and GVKParser
// from the API server and stores them on the cache. Caller must hold
// c.lock (use runSynced).
func (c *store) reloadOpenAPISchema() error {
	openAPISchema, gvkParser, err := c.kubectl.LoadOpenAPISchema(c.config)
	if err != nil {
		return fmt.Errorf("failed to load open api schema while handling CRD change: %w", err)
	}
	if gvkParser != nil {
		c.gvkParser = gvkParser
	}
	c.openAPISchema = openAPISchema
	return nil
}

// handleAPIServiceEvent reacts to an APIService add/modify/delete event on the
// watch stream. When an aggregated API's extension apiserver becomes available,
// the kube-apiserver starts serving new group/kinds that were not present
// during the initial discovery (e.g. the extension apiserver was down when
// Argo CD started). Re-run discovery so those resources get watched, mirroring
// how CRD events are handled by handleCRDEvent. Otherwise the new kinds remain
// invisible until the next manual cache invalidation or full resync. Like
// handleCRDEvent, it is safe to call from any event source.
func (c *store) handleAPIServiceEvent(event watch.EventType, obj *unstructured.Unstructured) {
	deleted := event == watch.Deleted
	if !deleted && !isAPIServiceAvailable(obj) {
		return
	}
	// The kube-apiserver's aggregated discovery can lag behind an
	// APIService reporting Available, so the group may not be served yet
	// when we first re-run discovery. Reconcile watches with a bounded
	// retry (in a goroutine to avoid blocking the event source) until the
	// APIService's own group is served or we exhaust our attempts.
	group, _, _ := unstructured.NestedString(obj.Object, "spec", "group")
	c.log.Info("Reconciling Kubernetes APIs, watches, and Open API schemas due to APIService event", "eventType", event, "name", obj.GetName(), "group", group)
	go c.reconcileAPIServiceWatches(group, !deleted)
}

// reconcileAPIServiceWatches re-runs discovery and starts any missing watches in
// response to an APIService event. Aggregated discovery can lag behind an
// APIService reporting Available, so when waitForGroup is true (Added/Modified
// while Available) it retries with exponential backoff (9 attempts, intervals
// growing 500ms -> ~8.5s for a ~25s total budget) until the APIService's group
// is served by the cluster (i.e. a watch for it has been started) or the attempts
// are exhausted. For deletions it reconciles once. startMissingWatches resolves
// the ACTIVE engine at call time (currentEngine) — this goroutine can outlive
// an Invalidate(SetMode(...)) engine swap.
func (c *store) reconcileAPIServiceWatches(group string, waitForGroup bool) {
	err := wait.ExponentialBackoff(wait.Backoff{
		Duration: 500 * time.Millisecond,
		Factor:   1.5,
		Steps:    9,
	}, func() (bool, error) {
		if err := runSynced(&c.lock, func() error {
			return c.currentEngine().startMissingWatches()
		}); err != nil {
			c.log.Error(err, "Failed to start missing watches after APIService event")
		}
		if err := runSynced(&c.lock, c.reloadOpenAPISchema); err != nil {
			c.log.Error(err, "Failed to reload open api schema after APIService event")
		}
		return !waitForGroup || group == "" || c.isGroupWatched(group), nil
	})
	if err != nil {
		c.log.Info("Aggregated API group is not served by discovery yet after retries; it will be picked up on the next resync", "group", group)
	}
}

// isGroupWatched reports whether the cache has started watching any resource in
// the given API group.
func (c *store) isGroupWatched(group string) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	for gk := range c.apisMeta {
		if gk.Group == group {
			return true
		}
	}
	return false
}

// isAPIServiceAvailable reports whether the given APIService object has an
// Available condition set to True, indicating its backing (aggregated) apiserver
// is ready to serve its API group.
func isAPIServiceAvailable(obj *unstructured.Unstructured) bool {
	conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}
	for _, condition := range conditions {
		cond, ok := condition.(map[string]any)
		if !ok {
			continue
		}
		if cond["type"] == "Available" {
			return cond["status"] == "True"
		}
	}
	return false
}

// processApi processes all the resources for a given API. First we construct an API client for the given API. Then we
// call the callback. If we're managing the whole cluster, we call the callback with the client and an empty namespace.
// If we're managing specific namespaces, we call the callback for each namespace.
func (c *store) processApi(client dynamic.Interface, api kube.APIResourceInfo, callback func(resClient dynamic.ResourceInterface, ns string) error) error {
	resClient := client.Resource(api.GroupVersionResource)
	switch {
	// if manage whole cluster or resource is cluster level and cluster resources enabled
	case len(c.namespaces) == 0 || (!api.Meta.Namespaced && c.clusterResources):
		return callback(resClient, "")
	// if manage some namespaces and resource is namespaced
	case len(c.namespaces) != 0 && api.Meta.Namespaced:
		for _, ns := range c.namespaces {
			err := callback(resClient.Namespace(ns), ns)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// isRestrictedResource checks if the kube api call is unauthorized or forbidden
func (c *store) isRestrictedResource(err error) bool {
	return c.respectRBAC != RespectRbacDisabled && (apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err))
}

// checkPermission runs a self subject access review to check if the controller has permissions to list the resource
func (c *store) checkPermission(ctx context.Context, reviewInterface authType1.SelfSubjectAccessReviewInterface, api kube.APIResourceInfo) (keep bool, err error) {
	sar := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: "*",
				Verb:      "list", // uses list verb to check for permissions
				Resource:  api.GroupVersionResource.Resource,
			},
		},
	}

	switch {
	// if manage whole cluster or resource is cluster level and cluster resources enabled
	case len(c.namespaces) == 0 || (!api.Meta.Namespaced && c.clusterResources):
		resp, err := reviewInterface.Create(ctx, sar, metav1.CreateOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to create self subject access review: %w", err)
		}
		if resp != nil && resp.Status.Allowed {
			return true, nil
		}
		// unsupported, remove from watch list
		return false, nil
	// if manage some namespaces and resource is namespaced
	case len(c.namespaces) != 0 && api.Meta.Namespaced:
		for _, ns := range c.namespaces {
			sar.Spec.ResourceAttributes.Namespace = ns
			resp, err := reviewInterface.Create(ctx, sar, metav1.CreateOptions{})
			if err != nil {
				return false, fmt.Errorf("failed to create self subject access review: %w", err)
			}
			if resp != nil && resp.Status.Allowed {
				return true, nil
			}
		}
		// unsupported, remove from watch list
		return false, nil
	}
	// checkPermission follows the same logic of determining namespace/cluster resource as the processApi function
	// so if neither of the cases match it means the controller will not watch for it so it is safe to return true.
	return true, nil
}

// EnsureSynced checks cache state and synchronizes it if necessary
func (c *clusterCache) EnsureSynced() error {
	syncStatus := &c.syncStatus

	// first check if cluster is synced *without acquiring the full store lock*
	syncStatus.lock.Lock()
	if syncStatus.synced(c.clusterSyncRetryTimeout) {
		syncError := syncStatus.syncError
		syncStatus.lock.Unlock()
		return syncError
	}
	syncStatus.lock.Unlock() // release the lock, so that we can acquire the parent lock (see struct comment re: lock acquisition ordering)

	// Single-flight: under informer mode sync() releases c.lock during
	// WaitForCacheSync, opening a window where a second EnsureSynced caller
	// would otherwise acquire c.lock and re-enter sync(), cancelling the
	// first caller's informers. syncMu serialises EnsureSynced callers so
	// the second one finds alreadySynced=true after the first finishes and
	// short-circuits to the cached result. Acquired BEFORE c.lock to keep
	// a consistent lock order; never held by any path other than this one.
	c.syncMu.Lock()
	defer c.syncMu.Unlock()

	c.lock.Lock()
	defer c.lock.Unlock()

	// before doing any work, check once again now that we have the lock, to see if it got
	// synced between the first check and now
	syncStatus.lock.Lock()
	alreadySynced := syncStatus.synced(c.clusterSyncRetryTimeout)
	syncErr := syncStatus.syncError
	syncStatus.lock.Unlock()
	if alreadySynced {
		return syncErr
	}

	// IMPORTANT: do not hold syncStatus.lock across sync(). Under informer mode
	// sync() -> syncInformers() releases c.lock while waiting for the initial
	// informer sync; holding syncStatus.lock across that window deadlocks with
	// GetClusterInfo (which takes c.lock.RLock first, then syncStatus.lock).
	err := c.engine.sync()
	// errCacheInvalidatedMidSync is transient by design — Invalidate already
	// reset syncStatus.syncTime to nil, and caching this error here would
	// reintroduce the clusterSyncRetryTimeout suppression that the sentinel
	// is meant to bypass. Return the error to this caller but leave
	// syncStatus untouched so the next EnsureSynced re-syncs.
	if errors.Is(err, errCacheInvalidatedMidSync) {
		return err
	}
	syncStatus.lock.Lock()
	defer syncStatus.lock.Unlock()
	syncTime := time.Now()
	syncStatus.syncTime = &syncTime
	syncStatus.syncError = err
	return syncStatus.syncError
}

func (c *store) FindResources(namespace string, predicates ...func(r *Resource) bool) map[kube.ResourceKey]*Resource {
	c.lock.RLock()
	defer c.lock.RUnlock()
	result := map[kube.ResourceKey]*Resource{}
	resources := map[kube.ResourceKey]*Resource{}
	if namespace != "" {
		if ns, ok := c.nsIndex[namespace]; ok {
			resources = ns
		}
	} else {
		resources = c.resources
	}

	for k := range resources {
		r := resources[k]
		matches := true
		for i := range predicates {
			if !predicates[i](r) {
				matches = false
				break
			}
		}

		if matches {
			result[k] = r
		}
	}
	return result
}

// IterateHierarchyV2 iterates through the hierarchy of resources starting from the given keys.
// It efficiently traverses parent-child relationships, including cross-namespace relationships
// between cluster-scoped parents and namespaced children, using pre-computed indexes.
func (c *store) IterateHierarchyV2(keys []kube.ResourceKey, action func(resource *Resource, namespaceResources map[kube.ResourceKey]*Resource) bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	// Track whether action() has been called on each resource (notCalled/inProgress/completed).
	// This is shared across processNamespaceHierarchy and processCrossNamespaceChildren.
	// Note: This is distinct from 'crossNSTraversed' in processCrossNamespaceChildren, which tracks
	// whether we've traversed a cluster-scoped key's cross-namespace children.
	actionCallState := make(map[kube.ResourceKey]callState)

	// Group keys by namespace for efficient processing
	keysPerNamespace := make(map[string][]kube.ResourceKey)
	for _, key := range keys {
		_, ok := c.resources[key]
		if !ok {
			continue
		}
		keysPerNamespace[key.Namespace] = append(keysPerNamespace[key.Namespace], key)
	}

	// Process namespaced resources with standard hierarchy
	for namespace, namespaceKeys := range keysPerNamespace {
		nsNodes := c.nsIndex[namespace]
		graph := buildGraph(nsNodes)
		c.processNamespaceHierarchy(namespaceKeys, nsNodes, graph, actionCallState, action)
	}

	// Process pre-computed cross-namespace children
	if clusterKeys, ok := keysPerNamespace[""]; ok {
		// Track which cluster-scoped keys have had their cross-namespace children traversed.
		// This is distinct from 'actionCallState' - a resource may have had action() called
		// (i.e., its actionCallState is in the completed state) but not yet had its cross-namespace
		// children traversed. This prevents infinite recursion when resources have circular
		// ownerReferences.
		crossNSTraversed := make(map[kube.ResourceKey]bool)
		c.processCrossNamespaceChildren(clusterKeys, actionCallState, crossNSTraversed, action)
	}
}

// processCrossNamespaceChildren processes namespaced children of cluster-scoped resources
// This enables traversing from cluster-scoped parents to their namespaced children across namespace boundaries.
// It also handles multi-level hierarchies where cluster-scoped resources own other cluster-scoped resources
// that in turn own namespaced resources (e.g., Provider -> ProviderRevision -> Deployment in Crossplane).
// The crossNSTraversed map tracks which keys have already been processed to prevent infinite recursion
// from circular ownerReferences (e.g., a resource that owns itself).
func (c *store) processCrossNamespaceChildren(
	clusterScopedKeys []kube.ResourceKey,
	actionCallState map[kube.ResourceKey]callState,
	crossNSTraversed map[kube.ResourceKey]bool,
	action func(resource *Resource, namespaceResources map[kube.ResourceKey]*Resource) bool,
) {
	for _, clusterKey := range clusterScopedKeys {
		// Skip if already processed (cycle detection)
		if crossNSTraversed[clusterKey] {
			continue
		}
		crossNSTraversed[clusterKey] = true

		// Get cluster-scoped resource to access its UID
		clusterResource := c.resources[clusterKey]
		if clusterResource == nil {
			continue
		}

		// Use parent-to-children index for O(1) lookup of direct children
		childrenSet := c.parentUIDToChildren[clusterResource.Ref.UID]
		for childKey := range childrenSet {
			child := c.resources[childKey]
			if child == nil {
				continue
			}

			alreadyProcessed := actionCallState[childKey] != notCalled

			// If child is cluster-scoped and action() was already called by processNamespaceHierarchy,
			// we still need to recursively check for its cross-namespace children.
			// This handles multi-level hierarchies like: ClusterScoped -> ClusterScoped -> Namespaced
			// (e.g., Crossplane's Provider -> ProviderRevision -> Deployment)
			if alreadyProcessed {
				if childKey.Namespace == "" {
					// Recursively process cross-namespace children of this cluster-scoped child
					// The crossNSTraversed map prevents infinite recursion on circular ownerReferences
					c.processCrossNamespaceChildren([]kube.ResourceKey{childKey}, actionCallState, crossNSTraversed, action)
				}
				continue
			}

			// Get namespace nodes for this child
			nsNodes := c.nsIndex[childKey.Namespace]
			if nsNodes == nil {
				continue
			}

			// Process this child
			if action(child, nsNodes) {
				actionCallState[childKey] = inProgress
				// Recursively process descendants using index-based traversal
				c.iterateChildrenUsingIndex(child, nsNodes, actionCallState, action)

				// If this child is also cluster-scoped, recursively process its cross-namespace children
				if childKey.Namespace == "" {
					c.processCrossNamespaceChildren([]kube.ResourceKey{childKey}, actionCallState, crossNSTraversed, action)
				}

				actionCallState[childKey] = completed
			}
		}
	}
}

// iterateChildrenUsingIndex recursively processes a resource's children using the parentUIDToChildren index
// This replaces graph-based traversal with O(1) index lookups
func (c *store) iterateChildrenUsingIndex(
	parent *Resource,
	nsNodes map[kube.ResourceKey]*Resource,
	actionCallState map[kube.ResourceKey]callState,
	action func(resource *Resource, namespaceResources map[kube.ResourceKey]*Resource) bool,
) {
	// Look up direct children of this parent using the index
	childrenSet := c.parentUIDToChildren[parent.Ref.UID]
	for childKey := range childrenSet {
		if actionCallState[childKey] != notCalled {
			continue // action() already called or in progress
		}

		child := c.resources[childKey]
		if child == nil {
			continue
		}

		// Only process children in the same namespace (for within-namespace traversal)
		// Cross-namespace children are handled by the outer loop in processCrossNamespaceChildren
		if child.Ref.Namespace != parent.Ref.Namespace {
			continue
		}

		if action(child, nsNodes) {
			actionCallState[childKey] = inProgress
			// Recursively process this child's descendants
			c.iterateChildrenUsingIndex(child, nsNodes, actionCallState, action)
			actionCallState[childKey] = completed
		}
	}
}

// processNamespaceHierarchy processes hierarchy for keys within a single namespace
func (c *store) processNamespaceHierarchy(
	namespaceKeys []kube.ResourceKey,
	nsNodes map[kube.ResourceKey]*Resource,
	graph map[kube.ResourceKey]map[types.UID]*Resource,
	actionCallState map[kube.ResourceKey]callState,
	action func(resource *Resource, namespaceResources map[kube.ResourceKey]*Resource) bool,
) {
	for _, key := range namespaceKeys {
		res := c.resources[key]
		if actionCallState[key] == completed || !action(res, nsNodes) {
			continue
		}
		actionCallState[key] = inProgress
		if _, ok := graph[key]; ok {
			for _, child := range graph[key] {
				if actionCallState[child.ResourceKey()] == notCalled && action(child, nsNodes) {
					child.iterateChildrenV2(graph, nsNodes, actionCallState, func(err error, child *Resource, namespaceResources map[kube.ResourceKey]*Resource) bool {
						if err != nil {
							c.log.V(2).Info(err.Error())
							return false
						}
						return action(child, namespaceResources)
					})
				}
			}
		}
		actionCallState[key] = completed
	}
}

func buildGraph(nsNodes map[kube.ResourceKey]*Resource) map[kube.ResourceKey]map[types.UID]*Resource {
	// Prepare to construct a graph
	nodesByUID := make(map[types.UID][]*Resource, len(nsNodes))
	for _, node := range nsNodes {
		nodesByUID[node.Ref.UID] = append(nodesByUID[node.Ref.UID], node)
	}

	// In graph, the key is the parent and the value is a list of children.
	graph := make(map[kube.ResourceKey]map[types.UID]*Resource)

	// Loop through all nodes, calling each one "childNode," because we're only bothering with it if it has a parent.
	for _, childNode := range nsNodes {
		for _, ownerRef := range childNode.OwnerRefs {
			// Resolve empty owner-ref UIDs into a local variable only. childNode is shared
			// cache state and IterateHierarchyV2 may run concurrently under RLock.
			ownerUID := ownerRef.UID
			if ownerUID == "" {
				// First, backfill UID of inferred owner child references.
				group, err := schema.ParseGroupVersion(ownerRef.APIVersion)
				if err != nil {
					// APIVersion is invalid, so we couldn't find the parent.
					continue
				}
				graphKeyNode, ok := nsNodes[kube.ResourceKey{Group: group.Group, Kind: ownerRef.Kind, Namespace: childNode.Ref.Namespace, Name: ownerRef.Name}]
				if !ok {
					// No resource found with the given graph key, so move on.
					continue
				}
				ownerUID = graphKeyNode.Ref.UID
			}

			// Now that we have the UID of the parent, update the graph.
			uidNodes, ok := nodesByUID[ownerUID]
			if ok {
				for _, uidNode := range uidNodes {
					// Cache ResourceKey() to avoid repeated expensive calls
					uidNodeKey := uidNode.ResourceKey()
					// Update the graph for this owner to include the child.
					if _, ok := graph[uidNodeKey]; !ok {
						graph[uidNodeKey] = make(map[types.UID]*Resource)
					}
					r, ok := graph[uidNodeKey][childNode.Ref.UID]
					if !ok {
						graph[uidNodeKey][childNode.Ref.UID] = childNode
					} else if r != nil {
						// The object might have multiple children with the same UID (e.g. replicaset from apps and extensions group).
						// It is ok to pick any object, but we need to make sure we pick the same child after every refresh.
						key1 := r.ResourceKey()
						key2 := childNode.ResourceKey()
						if key1.String() > key2.String() {
							graph[uidNodeKey][childNode.Ref.UID] = childNode
						}
					}
				}
			}
		}
	}
	return graph
}

// IsNamespaced answers if specified group/kind is a namespaced resource API or not
func (c *store) IsNamespaced(gk schema.GroupKind) (bool, error) {
	if isNamespaced, ok := c.namespacedResources[gk]; ok {
		return isNamespaced, nil
	}
	return false, apierrors.NewNotFound(schema.GroupResource{Group: gk.Group}, "")
}

func (c *store) managesNamespace(namespace string) bool {
	return slices.Contains(c.namespaces, namespace)
}

// GetManagedLiveObjs helps finding matching live K8S resources for a given resources list.
// The function returns all resources from cache for those `isManaged` function returns true and resources
// specified in targetObjs list.
func (c *store) GetManagedLiveObjs(targetObjs []*unstructured.Unstructured, isManaged func(r *Resource) bool) (map[kube.ResourceKey]*unstructured.Unstructured, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	for _, o := range targetObjs {
		if len(c.namespaces) > 0 {
			if o.GetNamespace() == "" && !c.clusterResources {
				return nil, fmt.Errorf("cluster level %s %q can not be managed when in namespaced mode", o.GetKind(), o.GetName())
			} else if o.GetNamespace() != "" && !c.managesNamespace(o.GetNamespace()) {
				return nil, fmt.Errorf("namespace %q for %s %q is not managed", o.GetNamespace(), o.GetKind(), o.GetName())
			}
		}
	}

	managedObjs := make(map[kube.ResourceKey]*unstructured.Unstructured)
	// iterate all objects in live state cache to find ones associated with app
	for key, o := range c.resources {
		if isManaged(o) && o.Resource != nil && len(o.OwnerRefs) == 0 {
			managedObjs[key] = o.Resource
		}
	}
	// but are simply missing our label
	lock := &sync.Mutex{}
	err := kube.RunAllAsync(len(targetObjs), func(i int) error {
		targetObj := targetObjs[i]
		key := kube.GetResourceKey(targetObj)
		lock.Lock()
		managedObj := managedObjs[key]
		lock.Unlock()

		if managedObj == nil {
			if existingObj, exists := c.resources[key]; exists {
				if existingObj.Resource != nil {
					managedObj = existingObj.Resource
				} else {
					var err error
					managedObj, err = c.kubectl.GetResource(context.TODO(), c.config, targetObj.GroupVersionKind(), existingObj.Ref.Name, existingObj.Ref.Namespace)
					if err != nil {
						if apierrors.IsNotFound(err) {
							return nil
						}
						return fmt.Errorf("unexpected error getting managed object: %w", err)
					}
				}
			} else if _, watched := c.apisMeta[key.GroupKind()]; !watched {
				var err error
				managedObj, err = c.kubectl.GetResource(context.TODO(), c.config, targetObj.GroupVersionKind(), targetObj.GetName(), targetObj.GetNamespace())
				if err != nil {
					if apierrors.IsNotFound(err) {
						return nil
					}
					return fmt.Errorf("unexpected error getting managed object: %w", err)
				}
			}
		}

		if managedObj != nil {
			converted, err := c.kubectl.ConvertToVersion(managedObj, targetObj.GroupVersionKind().Group, targetObj.GroupVersionKind().Version)
			if err != nil {
				// fallback to loading resource from kubernetes if conversion fails
				c.log.V(1).Info(fmt.Sprintf("Failed to convert resource: %v", err))
				managedObj, err = c.kubectl.GetResource(context.TODO(), c.config, targetObj.GroupVersionKind(), managedObj.GetName(), managedObj.GetNamespace())
				if err != nil {
					if apierrors.IsNotFound(err) {
						return nil
					}
					return fmt.Errorf("unexpected error getting managed object: %w", err)
				}
			} else {
				managedObj = converted
			}
			lock.Lock()
			managedObjs[key] = managedObj
			lock.Unlock()
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get managed objects: %w", err)
	}

	return managedObjs, nil
}

func (c *store) onNodeUpdated(oldRes *Resource, newRes *Resource) {
	c.setNode(newRes)
	c.dispatchResourceUpdated(newRes, oldRes, c.nsIndex[newRes.Ref.Namespace])
}

// dispatchResourceUpdated fires all OnResourceUpdated handlers. Shared
// helper: both legacy onNodeUpdated/onNodeRemoved and the informer event
// handler route through here so the dispatch site is identical.
func (c *store) dispatchResourceUpdated(newRes, oldRes *Resource, ns map[kube.ResourceKey]*Resource) {
	for _, h := range c.getResourceUpdatedHandlers() {
		h(newRes, oldRes, ns)
	}
}

// removeIndexes maintains nsIndex and parentUIDToChildren when a Resource
// is removed. Shared by legacy and informer impls (the legacy caller also
// deletes from c.resources; see onNodeRemoved). Returns the post-deletion
// namespace map for OnResourceUpdated dispatch. Caller must hold c.lock.
func (c *store) removeIndexes(existing *Resource) map[kube.ResourceKey]*Resource {
	key := existing.ResourceKey()
	ns, ok := c.nsIndex[key.Namespace]
	if ok {
		delete(ns, key)
		if len(ns) == 0 {
			delete(c.nsIndex, key.Namespace)
		}
		// remove ownership references from children with inferred references
		if existing.isInferredParentOf != nil {
			for k, v := range ns {
				if mightHaveInferredOwner(v) && existing.isInferredParentOf(k) {
					v.setOwnerRef(existing.toOwnerRef(), false)
					// Update index inline when removing inferred ref
					if existing.Ref.UID != "" {
						c.removeFromParentUIDToChildren(existing.Ref.UID, k)
					}
				}
			}
		}
	}

	// Clean up parent-to-children index
	for _, ownerRef := range existing.OwnerRefs {
		if ownerRef.UID != "" {
			c.removeFromParentUIDToChildren(ownerRef.UID, key)
		}
	}

	return ns
}

func (c *store) onNodeRemoved(key kube.ResourceKey) {
	existing, ok := c.resources[key]
	if ok {
		delete(c.resources, key)
		ns := c.removeIndexes(existing)
		c.dispatchResourceUpdated(nil, existing, ns)
	}
}

var ignoredRefreshResources = map[string]bool{
	"/" + kube.EndpointsKind: true,
}

// GetClusterInfo returns cluster cache statistics
func (c *store) GetClusterInfo() ClusterInfo {
	c.lock.RLock()
	defer c.lock.RUnlock()
	c.syncStatus.lock.Lock()
	defer c.syncStatus.lock.Unlock()

	return ClusterInfo{
		APIsCount:         len(c.apisMeta),
		K8SVersion:        c.serverVersion,
		ResourcesCount:    len(c.resources),
		Server:            c.config.Host,
		LastCacheSyncTime: c.syncStatus.syncTime,
		SyncError:         c.syncStatus.syncError,
		APIResources:      c.apiResources,
	}
}

// skipAppRequeuing checks if the object is an API type which we want to skip requeuing against.
// We ignore API types which have a high churn rate, and/or whose updates are irrelevant to the app
func skipAppRequeuing(key kube.ResourceKey) bool {
	return ignoredRefreshResources[key.Group+"/"+key.Kind]
}
