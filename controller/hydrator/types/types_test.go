package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHydrationQueueKeyShard(t *testing.T) {
	t.Parallel()

	key := HydrationQueueKey{
		SourceRepoURL:        "https://example.com/dry",
		SourceTargetRevision: "main",
		DestinationRepoURL:   "https://example.com/hydrated",
		DestinationBranch:    "env/test",
	}

	assert.Equal(t, 0, key.Shard(0))
	assert.Equal(t, 0, key.Shard(1))

	for replicas := 2; replicas <= 16; replicas++ {
		owner := key.Shard(replicas)
		assert.GreaterOrEqual(t, owner, 0)
		assert.Less(t, owner, replicas)
		for range 10 {
			assert.Equal(t, owner, key.Shard(replicas), "ownership must be deterministic")
		}
	}
}

func TestHydrationQueueKeyShardDistributesKeys(t *testing.T) {
	t.Parallel()

	const replicas = 4
	owners := map[int]bool{}
	for i := range 100 {
		key := HydrationQueueKey{
			SourceRepoURL:        fmt.Sprintf("https://example.com/dry-%d", i),
			SourceTargetRevision: "main",
			DestinationRepoURL:   "https://example.com/hydrated",
			DestinationBranch:    "env/test",
		}
		owners[key.Shard(replicas)] = true
	}

	assert.Len(t, owners, replicas)
}
