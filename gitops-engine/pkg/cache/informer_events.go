package cache

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
)

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
