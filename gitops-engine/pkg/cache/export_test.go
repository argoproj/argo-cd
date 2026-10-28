package cache

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
)

// Test-only forwarders to engine methods, so white-box tests can drive one
// lifecycle step through the clusterCache they build.

// legacyEngineOf returns c's engine as a *legacyEngine.
func legacyEngineOf(c *clusterCache) *legacyEngine {
	return c.engine.(*legacyEngine)
}

// informerEngineOf returns c's engine as an *informerEngine.
func informerEngineOf(c *clusterCache) *informerEngine {
	return c.engine.(*informerEngine)
}

// sync isn't on syncEngine, so call it on the concrete engine.
func (c *clusterCache) sync() error {
	return legacyEngineOf(c).sync()
}

func (c *clusterCache) startMissingWatches() error {
	return c.engine.startMissingWatches()
}

// stopWatching isn't on syncEngine, so call it on the concrete engine.
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

func (c *clusterCache) buildInformer(ctx context.Context, client dynamic.Interface, resClient dynamic.ResourceInterface, api kube.APIResourceInfo, ns string) (cache.SharedIndexInformer, cache.ResourceEventHandlerRegistration) {
	return informerEngineOf(c).buildInformer(ctx, client, resClient, api, ns)
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
