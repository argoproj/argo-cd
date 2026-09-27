package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type collector struct {
	mu      sync.Mutex
	records []Record
	auth    []string
	fail    atomic.Int32 // number of requests to fail before succeeding
}

func (c *collector) handler(w http.ResponseWriter, r *http.Request) {
	if c.fail.Load() > 0 {
		c.fail.Add(-1)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var batch []Record
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	c.mu.Lock()
	c.records = append(c.records, batch...)
	c.auth = append(c.auth, r.Header.Get("Authorization"))
	c.mu.Unlock()
	w.WriteHeader(http.StatusAccepted)
}

func (c *collector) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.records)
}

func TestIngestURL(t *testing.T) {
	assert.Equal(t, "http://argocd-audit-controller:8088/api/v1/records", IngestURL("argocd-audit-controller:8088"))
	assert.Equal(t, "https://audit.example.com/api/v1/records", IngestURL("https://audit.example.com/"))
	assert.Equal(t, "http://localhost:8088/api/v1/records", IngestURL("http://localhost:8088"))
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "abc", Truncate("abc", 3))
	assert.Equal(t, "ab...(truncated)", Truncate("abc", 2))
}

func TestShipper_DeliversInBatches(t *testing.T) {
	c := &collector{}
	srv := httptest.NewServer(http.HandlerFunc(c.handler))
	defer srv.Close()

	s := NewShipper(srv.URL, "s3cret", WithBatchSize(10), WithFlushInterval(20*time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()

	for i := range 25 {
		s.Enqueue(Record{Action: "application.sync", Resource: Resource{Name: string(rune('a' + i))}})
	}
	require.Eventually(t, func() bool { return c.count() == 25 }, 5*time.Second, 10*time.Millisecond)
	cancel()
	<-done

	assert.Equal(t, int64(25), s.Delivered())
	assert.Equal(t, int64(0), s.Dropped())
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, a := range c.auth {
		assert.Equal(t, "Bearer s3cret", a)
	}
	// Order is preserved.
	assert.Equal(t, "a", c.records[0].Resource.Name)
	assert.Equal(t, "y", c.records[24].Resource.Name)
}

func TestShipper_RetriesTransientFailures(t *testing.T) {
	c := &collector{}
	c.fail.Store(2)
	srv := httptest.NewServer(http.HandlerFunc(c.handler))
	defer srv.Close()

	s := NewShipper(srv.URL, "", WithFlushInterval(10*time.Millisecond), WithRetry(5, time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)

	s.Enqueue(Record{Action: "cluster.delete"})
	require.Eventually(t, func() bool { return c.count() == 1 }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, int64(0), s.Dropped())
}

func TestShipper_GivesUpAfterMaxAttempts(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := NewShipper(srv.URL, "", WithFlushInterval(10*time.Millisecond), WithRetry(3, time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)

	s.Enqueue(Record{Action: "cluster.delete"})
	require.Eventually(t, func() bool { return s.Dropped() == 1 }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, int32(3), requests.Load())
	assert.Equal(t, int64(0), s.Delivered())
}

func TestShipper_EnqueueNeverBlocks(t *testing.T) {
	// No Run loop: the queue fills up and further records are dropped (and logged) instead of blocking.
	s := NewShipper("localhost:1", "", WithQueueSize(2))
	done := make(chan struct{})
	go func() {
		for range 5 {
			s.Enqueue(Record{Action: "application.sync"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Enqueue blocked")
	}
	assert.Equal(t, int64(3), s.Dropped())
}

func TestShipper_FlushesOnShutdown(t *testing.T) {
	c := &collector{}
	srv := httptest.NewServer(http.HandlerFunc(c.handler))
	defer srv.Close()

	// A long flush interval: records are only sent because Run drains the queue on shutdown.
	s := NewShipper(srv.URL, "", WithFlushInterval(time.Hour))
	for range 3 {
		s.Enqueue(Record{Action: "application.delete"})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.Run(ctx)
	assert.Equal(t, 3, c.count())
}
