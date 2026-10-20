package cache

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

// Test-only forwarders to engine methods, so white-box tests can drive one
// lifecycle step through the clusterCache they build.

// legacyEngineOf returns c's engine as a *legacyEngine.
func legacyEngineOf(c *clusterCache) *legacyEngine {
	return c.engine.(*legacyEngine)
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
	legacyEngineOf(c).stopWatching(gk, ns)
}

func (c *clusterCache) recordEvent(event watch.EventType, un *unstructured.Unstructured) {
	legacyEngineOf(c).recordEvent(event, un)
}
