package helm

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Pause the sweep just before it takes the publication lock. A refresh can
// then replace the old candidate, deterministically exercising the interval
// between discovering an expired file and deciding whether to remove it.
func TestIndexStoreSweepRechecksRefreshedCandidate(t *testing.T) {
	t.Run("same store", func(t *testing.T) { testSweepRefresh(t, false) })
	t.Run("another store instance", func(t *testing.T) { testSweepRefresh(t, true) })
}

func testSweepRefresh(t *testing.T, anotherStore bool) {
	t.Helper()
	store := NewIndexStore(t.TempDir(), time.Hour, 1<<20)
	publisher := store
	if anotherStore {
		publisher = NewIndexStore(store.dir, time.Hour, 1<<20)
	}
	identity := "refresh-during-sweep"
	require.NoError(t, store.Set(identity, []byte(regressionIndexBody)))
	old := time.Now().Add(-48 * time.Hour)
	file := store.cacheFileName(identity)
	require.NoError(t, os.Chtimes(file, old, old))
	entered, resumeSweep := make(chan struct{}), make(chan struct{})
	var paused atomic.Bool
	acquire := store.lock
	store.lock = func(ctx context.Context, key string, shared, wait bool) (func(), error) {
		if key == file && !wait && paused.CompareAndSwap(false, true) {
			close(entered)
			<-resumeSweep
		}
		return acquire(ctx, key, shared, wait)
	}
	var release sync.Once
	resume := func() { release.Do(func() { close(resumeSweep) }) }
	t.Cleanup(resume)
	done := make(chan struct{})
	go func() {
		store.Sweep()
		close(done)
	}()
	select {
	case <-entered:
	case <-done:
		t.Fatal("sweep removed its candidate without coordinating with the publication lock")
	case <-time.After(3 * time.Second):
		t.Fatal("sweep did not reach the publication lock")
	}
	fresh := []byte(regressionIndexBody + "# refreshed\n")
	require.NoError(t, publisher.Set(identity, fresh))
	resume()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("sweep did not finish after the refresh")
	}
	got, err := store.Get(identity)
	require.NoError(t, err, "the replacement is fresh and must survive eviction")
	require.Equal(t, fresh, got)
}
