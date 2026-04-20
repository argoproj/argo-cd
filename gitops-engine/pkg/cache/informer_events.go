package cache

import (
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
// Resource-level dispatch (OnResourceUpdated with the namespace siblings
// map) and informer-store-backed hierarchy indexing land in a follow-up
// commit when the base/tail split decides how to expose the informer's
// indexer as the cache's source of truth.
func (c *clusterCache) informerEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			c.onInformerChange(watch.Added, nil, obj)
		},
		UpdateFunc: func(oldObj, newObj any) {
			c.onInformerChange(watch.Modified, oldObj, newObj)
		},
		DeleteFunc: func(obj any) {
			if tomb, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				obj = tomb.Obj
			}
			c.onInformerChange(watch.Deleted, obj, nil)
		},
	}
}

// onInformerChange dispatches a single informer event. It:
//  1. updates the shared cross-GK indexes (nsIndex + parentUIDToChildren),
//  2. fires OnEvent handlers and routes CRD events via dispatchEvent,
//  3. fires OnResourceUpdated handlers with the post-update namespace map.
//
// Under informer mode the informer's own store is the source of truth
// for single-key lookups, so we do not write to c.resources. The cross-GK
// indexes are not derivable from per-GK informer indexers and must be
// maintained here.
func (c *clusterCache) onInformerChange(event watch.EventType, oldObj, newObj any) {
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

	c.lock.Lock()
	defer c.lock.Unlock()

	var existing *Resource
	if oldCr != nil {
		existing = oldCr.Resource
	}

	switch event {
	case watch.Added, watch.Modified:
		// Maintain c.resources as a shadow of the informer's store so every
		// existing read path (GetManagedLiveObjs, IterateHierarchyV2's key
		// lookups, FindResources for the all-namespaces case) works unchanged
		// under informer mode. The ~70 bytes/entry map overhead is trivial
		// compared to the TransformFunc's savings on managedFields.
		c.resources[newCr.Resource.ResourceKey()] = newCr.Resource
		c.updateIndexes(existing, newCr.Resource)
		c.dispatchEvent(event, un)
		c.dispatchResourceUpdated(newCr.Resource, existing, c.nsIndex[newCr.Resource.Ref.Namespace])
	case watch.Deleted:
		// For deletes the informer passes the last-known object as oldObj;
		// we treated it as `primary` above for dispatchEvent purposes.
		delete(c.resources, primary.Resource.ResourceKey())
		ns := c.removeIndexes(primary.Resource)
		c.dispatchEvent(event, un)
		c.dispatchResourceUpdated(nil, primary.Resource, ns)
	}
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
