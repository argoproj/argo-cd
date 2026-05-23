package cache

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
)

// dispatchEvent fires OnEvent handlers and, for CRD events, the CRD
// handler. Both the legacy watch loop and the informer-mode event shim
// funnel here so event-level side effects stay in one place.
//
// Storage mutations (c.resources / c.nsIndex / c.parentUIDToChildren for
// the legacy impl; informer store for the informer impl) are the caller's
// responsibility — this helper does not touch them.
func (c *clusterCache) dispatchEvent(event watch.EventType, un *unstructured.Unstructured) {
	for _, h := range c.getEventHandlers() {
		h(event, un)
	}
	if kube.IsCRD(un) {
		c.handleCRDEvent(event, un)
	}
}

// informerEventHandler builds the cache.ResourceEventHandler attached to
// every per-(GroupKind, namespace) informer by the planned informer-based
// cluster cache (see issue #19199). It translates Add/Update/Delete into
// the shared dispatchEvent pipeline, unwrapping tombstones and tolerating
// unexpected object types defensively.
//
// Test-only entrypoint: production callers go through buildInformer ->
// informerEventHandlerForCtx so that events arriving after the informer's
// watchCtx has been cancelled (Invalidate, stopWatching) are dropped
// instead of mutating freshly-rebuilt state.
//
// Resource-level dispatch (OnResourceUpdated with the namespace siblings
// map) and informer-store-backed hierarchy indexing land in a follow-up
// commit when the base/tail split decides how to expose the informer's
// indexer as the cache's source of truth.
func (c *clusterCache) informerEventHandler() cache.ResourceEventHandler {
	return c.informerEventHandlerForCtx(context.Background())
}

// informerEventHandlerForCtx is the production variant: it gates every
// dispatch on the informer's owning watch context. Once that context is
// cancelled (Invalidate, stopWatching), in-flight events from the
// reflector's still-draining DeltaFIFO are dropped on the floor.
//
// Without this guard, a stale event handler can fire after Invalidate
// has set apisMeta=nil — and the CRD dispatch path through handleCRDEvent
// -> startMissingWatches -> startInformersForAPILocked would then panic
// with "assignment to entry in nil map" (informer_lifecycle.go:76).
func (c *clusterCache) informerEventHandlerForCtx(ctx context.Context) cache.ResourceEventHandler {
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
			c.onInformerChange(watch.Added, nil, obj, isInInitialList)
		},
		UpdateFunc: func(oldObj, newObj any) {
			if ctx.Err() != nil {
				return
			}
			c.onInformerChange(watch.Modified, oldObj, newObj, false)
		},
		DeleteFunc: func(obj any) {
			if ctx.Err() != nil {
				return
			}
			if tomb, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				obj = tomb.Obj
			}
			c.onInformerChange(watch.Deleted, obj, nil, false)
		},
	}
}

// onInformerChange dispatches a single informer event. It:
//  1. updates the shared cross-GK indexes (nsIndex + parentUIDToChildren),
//  2. fires OnEvent handlers and routes CRD events via dispatchEvent
//     (skipped when isInInitialList=true to match legacy semantics — the
//     legacy initial-state load via replaceResourceCache fires OnNodeUpdated
//     but never recordEvent/handleCRDEvent),
//  3. fires OnResourceUpdated handlers with the post-update namespace map.
//
// Under informer mode the informer's own store is the source of truth
// for single-key lookups, so we do not write to c.resources. The cross-GK
// indexes are not derivable from per-GK informer indexers and must be
// maintained here.
func (c *clusterCache) onInformerChange(event watch.EventType, oldObj, newObj any, isInInitialList bool) {
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

	var existing *Resource
	if oldCr != nil {
		existing = oldCr.Resource
	}

	// Storage mutations + OnResourceUpdated dispatch run under c.lock so the
	// nsIndex snapshot handed to handlers is consistent with c.resources —
	// matches the legacy setNode/onNodeRemoved -> dispatchResourceUpdated path.
	c.lock.Lock()
	switch event {
	case watch.Added, watch.Modified:
		// Maintain c.resources as a shadow of the informer's store so every
		// existing read path (GetManagedLiveObjs, IterateHierarchyV2's key
		// lookups, FindResources for the all-namespaces case) works unchanged
		// under informer mode. The ~70 bytes/entry map overhead is trivial
		// compared to the TransformFunc's savings on managedFields.
		c.resources[newCr.Resource.ResourceKey()] = newCr.Resource
		c.updateIndexes(existing, newCr.Resource)
		c.dispatchResourceUpdated(newCr.Resource, existing, c.nsIndex[newCr.Resource.Ref.Namespace])
	case watch.Deleted:
		// For deletes the informer passes the last-known object as oldObj;
		// we treated it as `primary` above for dispatchEvent purposes.
		delete(c.resources, primary.Resource.ResourceKey())
		ns := c.removeIndexes(primary.Resource)
		c.dispatchResourceUpdated(nil, primary.Resource, ns)
	}
	c.lock.Unlock()

	// Skip OnEvent + CRD routing for initial-list events so we don't reload
	// the OpenAPI schema once per existing CRD at startup. Legacy
	// loadInitialState populates state without firing dispatchEvent —
	// matched here. Genuine watch events (post-initial Add, Update, Delete)
	// continue to flow through dispatchEvent normally.
	if isInInitialList {
		return
	}

	// OnEvent handlers and CRD routing run WITHOUT c.lock — handleCRDEvent
	// re-acquires c.lock via runSynced (cluster.go startMissingWatches /
	// reloadOpenAPISchema). Mirrors the legacy watchEvents path, which
	// invokes recordEvent (event handlers) and handleCRDEvent outside the
	// lock; only processEvent runs under it.
	c.dispatchEvent(event, un)
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
