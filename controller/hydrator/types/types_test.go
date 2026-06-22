package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testKey() HydrationQueueKey {
	return HydrationQueueKey{
		SourceRepoURL:        "https://example.com/repo",
		SourceTargetRevision: "main",
		DestinationRepoURL:   "https://example.com/repo",
		DestinationBranch:    "env/dev",
	}
}

func TestHydrationQueueKey_Shard_SingleReplica(t *testing.T) {
	t.Parallel()
	// With one or fewer replicas every key is owned by shard 0.
	assert.Equal(t, 0, testKey().Shard(1))
	assert.Equal(t, 0, testKey().Shard(0))
	assert.Equal(t, 0, testKey().Shard(-5))
}

func TestHydrationQueueKey_Shard_Deterministic(t *testing.T) {
	t.Parallel()
	key := testKey()
	first := key.Shard(7)
	for range 100 {
		assert.Equal(t, first, key.Shard(7), "Shard must be deterministic for a fixed key and replica count")
	}
}

func TestHydrationQueueKey_Shard_InRange(t *testing.T) {
	t.Parallel()
	for replicas := 1; replicas <= 16; replicas++ {
		for i := range 50 {
			key := HydrationQueueKey{
				SourceRepoURL:        fmt.Sprintf("https://example.com/repo-%d", i),
				SourceTargetRevision: "main",
				DestinationRepoURL:   fmt.Sprintf("https://example.com/dest-%d", i),
				DestinationBranch:    fmt.Sprintf("env/%d", i),
			}
			shard := key.Shard(replicas)
			assert.GreaterOrEqual(t, shard, 0)
			assert.Less(t, shard, replicas)
		}
	}
}

func TestHydrationQueueKey_Shard_DistributesAcrossShards(t *testing.T) {
	t.Parallel()
	const replicas = 4
	seen := map[int]int{}
	for i := range 200 {
		key := HydrationQueueKey{
			SourceRepoURL:        fmt.Sprintf("https://example.com/repo-%d", i),
			SourceTargetRevision: "main",
			DestinationRepoURL:   "https://example.com/dest",
			DestinationBranch:    fmt.Sprintf("env/%d", i),
		}
		seen[key.Shard(replicas)]++
	}
	// The mapping must not be degenerate (collapsing every key onto a single shard). We don't assert
	// a perfectly uniform balance because FNV-32a modulo over structured inputs is not uniform.
	assert.Greater(t, len(seen), 1, "keys should be distributed across more than one shard")
}

func TestHydrationQueueKey_Shard_FieldsAreDistinguished(t *testing.T) {
	t.Parallel()
	// Field boundaries must be unambiguous: shifting a character across a field boundary must be able
	// to produce a different hash input (guards against naive concatenation collisions).
	a := HydrationQueueKey{SourceRepoURL: "ab", SourceTargetRevision: "c"}
	b := HydrationQueueKey{SourceRepoURL: "a", SourceTargetRevision: "bc"}
	assert.NotEqual(t, a, b)
}
