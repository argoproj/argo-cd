package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube/kubetest"
)

// newSettingsTestCache calls the public constructor and returns the concrete
// legacy impl so white-box assertions on unexported settings fields work.
// Interface-level tests should use cluster_interface_test.go helpers instead.
func newSettingsTestCache(opts ...UpdateSettingsFunc) *clusterCache {
	return NewClusterCache(&rest.Config{}, opts...).(*clusterCache)
}

func TestSetSettings(t *testing.T) {
	t.Parallel()
	cache := newSettingsTestCache(SetKubectl(&kubetest.MockKubectlCmd{}))
	updatedHealth := &noopSettings{}
	updatedFilter := &noopSettings{}
	cache.Invalidate(SetSettings(Settings{ResourceHealthOverride: updatedHealth, ResourcesFilter: updatedFilter}))

	assert.Equal(t, updatedFilter, cache.settings.ResourcesFilter)
	assert.Equal(t, updatedHealth, cache.settings.ResourceHealthOverride)
}

func TestSetConfig(t *testing.T) {
	t.Parallel()
	cache := newSettingsTestCache(SetKubectl(&kubetest.MockKubectlCmd{}))
	updatedConfig := &rest.Config{Host: "http://newhost"}
	cache.Invalidate(SetConfig(updatedConfig))

	assert.Equal(t, updatedConfig, cache.config)
}

func TestSetNamespaces(t *testing.T) {
	t.Parallel()
	cache := newSettingsTestCache(SetKubectl(&kubetest.MockKubectlCmd{}), SetNamespaces([]string{"default"}))

	updatedNamespaces := []string{"updated"}
	cache.Invalidate(SetNamespaces(updatedNamespaces))

	assert.ElementsMatch(t, updatedNamespaces, cache.namespaces)
}

func TestSetResyncTimeout(t *testing.T) {
	t.Parallel()
	cache := newSettingsTestCache()
	assert.Equal(t, defaultClusterResyncTimeout, cache.syncStatus.resyncTimeout)

	timeout := 1 * time.Hour
	cache.Invalidate(SetResyncTimeout(timeout))

	assert.Equal(t, timeout, cache.syncStatus.resyncTimeout)
}

func TestSetWatchResyncTimeout(t *testing.T) {
	t.Parallel()
	cache := newSettingsTestCache()
	assert.Equal(t, defaultWatchResyncTimeout, cache.watchResyncTimeout)

	timeout := 30 * time.Minute
	cache = newSettingsTestCache(SetWatchResyncTimeout(timeout))
	assert.Equal(t, timeout, cache.watchResyncTimeout)
}

func TestSetBatchEventsProcessing(t *testing.T) {
	t.Parallel()
	cache := newSettingsTestCache()
	assert.False(t, cache.batchEventsProcessing)

	cache.Invalidate(SetBatchEventsProcessing(true))
	assert.True(t, cache.batchEventsProcessing)
}

func TestSetEventsProcessingInterval(t *testing.T) {
	t.Parallel()
	cache := newSettingsTestCache()
	assert.Equal(t, defaultEventProcessingInterval, cache.eventProcessingInterval)

	interval := 1 * time.Second
	cache.Invalidate(SetEventProcessingInterval(interval))
	assert.Equal(t, interval, cache.eventProcessingInterval)
}
// TestSetMode_RefusedAfterStart pins the construction-time-only contract:
// once the cache has started (first EnsureSynced spawned the engine's
// machinery), SetMode must keep the running engine rather than swap in a
// fresh one — a swap would leave the old engine's still-draining goroutines
// re-entering the lifecycle alongside the new engine's machinery.
func TestSetMode_RefusedAfterStart(t *testing.T) {
	t.Parallel()

	// Before start, SetMode swaps freely (construction path).
	cache := newSettingsTestCache(SetMode(ModeInformer))
	_, isInformer := cache.engine.(*informerEngine)
	assert.True(t, isInformer, "construction-time SetMode must install the requested engine")

	// After start, SetMode must be refused.
	cluster := newCluster(t)
	require.NoError(t, cluster.EnsureSynced())
	runningEngine := cluster.engine
	cluster.Invalidate(SetMode(ModeInformer))
	assert.Same(t, runningEngine, cluster.engine, "post-start SetMode must keep the running engine")
}
