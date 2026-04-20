package cache

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
)

// This file provides test-only access to engine internals through the
// clusterCache facade. The lifecycle methods live on *legacyEngine /
// *informerEngine (which the tests can't name conveniently, since the cache
// is constructed through the ClusterCache interface). Rather than thread the
// engine type through every white-box test, we forward here.
//
// The informer forwarders assume the cache was built in ModeInformer — the
// informer test constructors (newInformerTestCache, newTransformTestCache)
// set it; the legacy forwarders assume ModeLegacy (the default).

// informerEngineOf returns the active informer engine, panicking if the cache
// is not in informer mode (a test setup bug).
func informerEngineOf(c *clusterCache) *informerEngine {
	return c.engine.(*informerEngine)
}

// legacyEngineOf returns the active legacy engine, panicking if the cache is
// not in legacy mode (a test setup bug).
func legacyEngineOf(c *clusterCache) *legacyEngine {
	return c.engine.(*legacyEngine)
}

// Lifecycle delegators: production routes through c.engine directly
// (EnsureSynced, handleCRDEvent) or the engines call their own methods, so
// these facade shims exist only for white-box tests that drive a single
// lifecycle step.

func (c *clusterCache) sync() error {
	return c.engine.sync()
}

func (c *clusterCache) startMissingWatches() error {
	return c.engine.startMissingWatches()
}

// stopWatching is not on the syncEngine interface (its namespace semantics
// differ per engine — see the note in engine.go), so dispatch on the
// concrete type here.
func (c *clusterCache) stopWatching(gk schema.GroupKind, ns string) {
	switch e := c.engine.(type) {
	case *legacyEngine:
		e.stopWatching(gk, ns)
	case *informerEngine:
		e.stopWatching(gk, ns)
	}
}

func (c *clusterCache) recordEvent(event watch.EventType, un *unstructured.Unstructured) {
	legacyEngineOf(c).recordEvent(event, un)
}

func (c *clusterCache) transformForInformer(obj any) (any, error) {
	return informerEngineOf(c).transformForInformer(obj)
}

func (c *clusterCache) informerEventHandler() cache.ResourceEventHandler {
	return informerEngineOf(c).informerEventHandler()
}

func (c *clusterCache) informerEventHandlerForCtx(ctx context.Context) cache.ResourceEventHandler {
	return informerEngineOf(c).informerEventHandlerForCtx(ctx)
}

func (c *clusterCache) onInformerChange(ctx context.Context, event watch.EventType, oldObj, newObj any, isInInitialList bool) {
	informerEngineOf(c).onInformerChange(ctx, event, oldObj, newObj, isInInitialList)
}

func (c *clusterCache) buildInformer(ctx context.Context, resClient dynamic.ResourceInterface, api kube.APIResourceInfo, ns string) cache.SharedIndexInformer {
	return informerEngineOf(c).buildInformer(ctx, resClient, api, ns)
}

func (c *clusterCache) startInformersForAPI(ctx context.Context, api kube.APIResourceInfo) error {
	return informerEngineOf(c).startInformersForAPI(ctx, api)
}

func (c *clusterCache) informerWatchErrorHandler(api kube.APIResourceInfo, ns string) cache.WatchErrorHandlerWithContext {
	return informerEngineOf(c).informerWatchErrorHandler(api, ns)
}

func (c *clusterCache) resolveSyncResult(synced bool, watched []watchedInformer) error {
	return informerEngineOf(c).resolveSyncResult(synced, watched)
}
