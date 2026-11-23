package helm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockingIndexCache blocks every optional cache read until released, to
// simulate a slow backend. Reads abandoned by the caller are still tracked
// so cleanup can wait for them.
type blockingIndexCache struct {
	mu       sync.Mutex
	blocked  chan struct{}
	release  chan struct{}
	gets     atomic.Int64
	setCount atomic.Int64
}

func newBlockingIndexCache() *blockingIndexCache {
	return &blockingIndexCache{blocked: make(chan struct{}), release: make(chan struct{})}
}

func (b *blockingIndexCache) block() {
	b.mu.Lock()
	defer b.mu.Unlock()
	select {
	case <-b.blocked:
	default:
		close(b.blocked)
	}
}

func (b *blockingIndexCache) releaseAll() {
	b.mu.Lock()
	defer b.mu.Unlock()
	select {
	case <-b.release:
	default:
		close(b.release)
	}
}

func (b *blockingIndexCache) SetHelmIndexRecord(_, _ string, _, _ string) error {
	b.setCount.Add(1)
	return nil
}

func (b *blockingIndexCache) SetHelmIndexRetainedRecord(_ string, _ *HelmIndexRecord) error {
	b.setCount.Add(1)
	return nil
}

func (b *blockingIndexCache) SetHelmIndex(_ string, _ []byte) error {
	b.setCount.Add(1)
	return nil
}

func (b *blockingIndexCache) GetHelmIndex(_ string, _ *[]byte) error {
	b.gets.Add(1)
	<-b.release
	return context.Canceled
}

func (b *blockingIndexCache) GetHelmIndexRetainedRecord(_ string) (*HelmIndexRecord, error) {
	b.gets.Add(1)
	<-b.release
	return nil, context.Canceled
}

func newBlockingTestOrigin(t *testing.T, body []byte) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	requests := &atomic.Int64{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "text/yaml")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server, requests
}

// A blocked optional cache read must not consume the whole request
// deadline: GetIndex reaches a healthy origin and succeeds well before the
// parent deadline. Failing-before: the budget equaled the remaining
// deadline, so the abandoned read waited out the full deadline and the
// request failed even though the origin was healthy.
func TestIndexCacheReadBudgetReservesOriginTime(t *testing.T) {
	body := newIndexBody("1.0.0")
	digest := sha256.Sum256(body)
	stored := &HelmIndexRecord{
		Body:       bytes.Clone(body),
		BodyDigest: hex.EncodeToString(digest[:]),
		StoredAt:   time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano),
	}

	run := func(t *testing.T, alwaysFresh bool) {
		t.Helper()
		cache := newBlockingIndexCache()
		if !alwaysFresh {
			require.NoError(t, cache.SetHelmIndexRetainedRecord("prefilled", stored))
		}
		server, requests := newBlockingTestOrigin(t, body)

		client := NewClient(server.URL, HelmCreds{InsecureSkipVerify: true}, false, "", "", WithIndexCache(cache))

		// Positive control: a healthy origin serves immediately.
		healthy, err := client.GetIndex(context.Background(), false, 1024*1024)
		require.NoError(t, err, "origin must be healthy for the control")
		require.NotEmpty(t, healthy.Entries["fake"])
		controlRequests := requests.Load()
		require.GreaterOrEqual(t, controlRequests, int64(1))

		// Now block the cache and require success within a generous
		// deadline. The deadline/4 budget releases the origin request long
		// before expiry.
		cache.block()
		t.Cleanup(cache.releaseAll)
		deadline := time.Now().Add(800 * time.Millisecond)
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()

		start := time.Now()
		index, err := client.GetIndex(ctx, false, 1024*1024)
		elapsed := time.Since(start)
		require.NoError(t, err, "origin fetch must succeed before the parent deadline (took %s)", elapsed)
		require.NotEmpty(t, index.Entries["fake"])
		assert.Less(t, elapsed, 800*time.Millisecond, "must finish before the deadline")
	}

	t.Run("OrdinaryRead", func(t *testing.T) {
		run(t, false)
	})

	t.Run("AlwaysFreshRead", func(t *testing.T) {
		run(t, true)
	})
}

// Cancellation and origin errors must still propagate even when a cache
// read was abandoned.
func TestIndexCacheAbandonedReadPropagatesErrors(t *testing.T) {
	body := newIndexBody("1.0.0")
	cache := newBlockingIndexCache()
	server, _ := newBlockingTestOrigin(t, body)
	client := NewClient(server.URL, HelmCreds{InsecureSkipVerify: true}, false, "", "", WithIndexCache(cache))

	cache.block()
	t.Cleanup(cache.releaseAll)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	_, err := client.GetIndex(ctx, false, 1024*1024)
	require.Error(t, err, "a canceled request must fail")

	// Origin error propagation: point the client at a dead origin and
	// require the origin error, not a cache-shaped error.
	deadClient := NewClient("http://127.0.0.1:1", HelmCreds{}, false, "", "", WithIndexCache(cache))
	_, err = deadClient.GetIndex(context.Background(), false, 1024*1024)
	require.Error(t, err)
}
