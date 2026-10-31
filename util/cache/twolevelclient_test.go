package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Two controllers share one external cache (Redis) and each has its own in-memory layer.
// When another controller overwrites a key, a later Set of the value this controller wrote
// earlier must still reach the external cache.
func TestTwoLevelClient_SetAfterKeyOverwrittenByAnotherClient(t *testing.T) {
	t.Parallel()
	external := NewInMemoryCache(time.Hour) // stands in for Redis
	controllerA := NewTwoLevelClient(external, time.Hour)
	controllerB := NewTwoLevelClient(external, time.Hour)
	key := "app|resources-tree|my-app"

	// App is on cluster B: B writes its tree.
	require.NoError(t, controllerB.Set(&Item{Key: key, Object: &foo{Bar: "healthy"}}))
	// App moves to cluster A: A overwrites the key with its tree.
	require.NoError(t, controllerA.Set(&Item{Key: key, Object: &foo{Bar: "suspended"}}))
	// App moves back to B: B drops its local copy because another controller may have written the key.
	controllerB.InvalidateLocal(key)
	// B computes the same tree it wrote before.
	require.NoError(t, controllerB.Set(&Item{Key: key, Object: &foo{Bar: "healthy"}}))

	got := &foo{}
	require.NoError(t, external.Get(key, got))
	assert.Equal(t, "healthy", got.Bar)
}

// countingCache counts writes to the external cache so tests can tell a skipped Set from a write-through.
type countingCache struct {
	*InMemoryCache
	setCalls int
}

func (c *countingCache) Set(item *Item) error {
	c.setCalls++
	return c.InMemoryCache.Set(item)
}

// A stable app produces an identical value on every reconcile. Once the entry has aged past half of its
// expiration, an identical Set must be written through so that the TTL is refreshed in BOTH tiers —
// otherwise the key expires everywhere at once and every read misses (#26710).
func TestTwoLevelClient_IdenticalSetRefreshesTTLs(t *testing.T) {
	t.Parallel()
	external := NewInMemoryCache(time.Hour) // stands in for Redis
	client := NewTwoLevelClient(external, time.Hour)
	key := "app|managed-resources|my-app"
	expiration := time.Second

	require.NoError(t, client.Set(&Item{Key: key, Object: &foo{Bar: "stable"}, CacheActionOpts: CacheActionOpts{Expiration: expiration}}))

	// Let the entry consume more than half of its TTL, then Set the identical value again.
	time.Sleep(600 * time.Millisecond)
	// Guard against sleep overshoot: the original entry must still be alive for the refresh to be meaningful.
	require.NoError(t, client.GetLocal(key, &foo{}), "test timing invalid: entry already expired before the refreshing Set")
	require.NoError(t, client.Set(&Item{Key: key, Object: &foo{Bar: "stable"}, CacheActionOpts: CacheActionOpts{Expiration: expiration}}))

	// Wait past the ORIGINAL expiry window. Both tiers must still hold the value because the second Set
	// restamped both TTLs.
	time.Sleep(600 * time.Millisecond)
	assert.NoError(t, client.GetLocal(key, &foo{}), "in-memory TTL was not refreshed by the identical Set")
	assert.NoError(t, external.Get(key, &foo{}), "external cache TTL was not refreshed by the identical Set")
}

// While the entry is still fresh (younger than half of its expiration), an identical Set must keep being
// skipped — minimizing external cache requests is the whole point of the two-level client.
func TestTwoLevelClient_IdenticalSetWithinFreshWindowSkipsExternalWrite(t *testing.T) {
	t.Parallel()
	external := &countingCache{InMemoryCache: NewInMemoryCache(time.Hour)}
	client := NewTwoLevelClient(external, time.Hour)
	key := "app|managed-resources|my-app"

	item := &Item{Key: key, Object: &foo{Bar: "stable"}, CacheActionOpts: CacheActionOpts{Expiration: time.Hour}}
	require.NoError(t, client.Set(item))
	require.NoError(t, client.Set(item))
	require.NoError(t, client.Set(item))
	assert.Equal(t, 1, external.setCalls, "identical Sets within the fresh window must not hit the external cache")

	// Items without a per-item expiration keep the previous behavior: identical Sets are always skipped.
	noExpiry := &Item{Key: "no-expiry", Object: &foo{Bar: "stable"}}
	require.NoError(t, client.Set(noExpiry))
	require.NoError(t, client.Set(noExpiry))
	assert.Equal(t, 2, external.setCalls, "identical Sets without per-item expiration must not hit the external cache")
}

// A changed value must always be written through immediately, fresh or not.
func TestTwoLevelClient_ChangedValueWritesThrough(t *testing.T) {
	t.Parallel()
	external := &countingCache{InMemoryCache: NewInMemoryCache(time.Hour)}
	client := NewTwoLevelClient(external, time.Hour)
	key := "app|managed-resources|my-app"

	require.NoError(t, client.Set(&Item{Key: key, Object: &foo{Bar: "v1"}, CacheActionOpts: CacheActionOpts{Expiration: time.Hour}}))
	require.NoError(t, client.Set(&Item{Key: key, Object: &foo{Bar: "v2"}, CacheActionOpts: CacheActionOpts{Expiration: time.Hour}}))
	assert.Equal(t, 2, external.setCalls)

	got := &foo{}
	require.NoError(t, external.Get(key, got))
	assert.Equal(t, "v2", got.Bar)
}
