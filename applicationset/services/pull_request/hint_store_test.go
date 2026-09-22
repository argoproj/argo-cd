package pull_request

import (
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPRHintStoreSetAndTake(t *testing.T) {
	var s PRHintStore

	assert.Nil(t, s.Take("owner", "repo"), "Take on an empty store should return nil")

	s.Set("owner", "repo", []*PullRequest{{Number: 1}})
	got := s.Take("owner", "repo")
	require.Len(t, got, 1)
	assert.Equal(t, int64(1), got[0].Number)

	assert.Nil(t, s.Take("owner", "repo"), "Take should consume the hints")
}

// Set previously used sync.Map.CompareAndSwap with a []*PullRequest old value.
// Slices are uncomparable, so the second Set for a key panicked at runtime.
func TestPRHintStoreSetAccumulates(t *testing.T) {
	var s PRHintStore

	s.Set("owner", "repo", []*PullRequest{{Number: 1}})
	require.NotPanics(t, func() {
		s.Set("owner", "repo", []*PullRequest{{Number: 2}})
	})

	got := s.Take("owner", "repo")
	require.Len(t, got, 2)
	assert.Equal(t, int64(1), got[0].Number)
	assert.Equal(t, int64(2), got[1].Number)
}

func TestPRHintStoreKeysAreScopedPerRepo(t *testing.T) {
	var s PRHintStore

	s.Set("owner", "a", []*PullRequest{{Number: 1}})
	s.Set("owner", "b", []*PullRequest{{Number: 2}})

	a := s.Take("owner", "a")
	require.Len(t, a, 1)
	assert.Equal(t, int64(1), a[0].Number)

	b := s.Take("owner", "b")
	require.Len(t, b, 1)
	assert.Equal(t, int64(2), b[0].Number)
}

// Concurrent Set must not drop hints — run with -race.
func TestPRHintStoreConcurrentSet(t *testing.T) {
	var s PRHintStore

	const writers = 32
	var wg sync.WaitGroup
	for i := range writers {
		wg.Go(func() {
			s.Set("owner", "repo", []*PullRequest{{Number: int64(i)}})
		})
	}
	wg.Wait()

	got := s.Take("owner", "repo")
	assert.Len(t, got, writers, "every concurrent Set should be retained")

	seen := make(map[int64]bool, writers)
	for _, pr := range got {
		seen[pr.Number] = true
	}
	for i := range writers {
		assert.True(t, seen[int64(i)], "missing PR number "+strconv.Itoa(i))
	}
}
