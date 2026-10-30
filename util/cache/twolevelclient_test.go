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
