package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActiveUserTrackerRecordIncreasesGauge(t *testing.T) {
	t.Parallel()

	tracker := NewActiveUserTracker(time.Hour, time.Minute)
	assert.InDelta(t, 0.0, testutil.ToFloat64(tracker.Gauge()), 0.0)

	tracker.Record("alice")
	assert.InDelta(t, 1.0, testutil.ToFloat64(tracker.Gauge()), 0.0)
	assert.Equal(t, 1, tracker.Count())

	tracker.Record("bob")
	assert.InDelta(t, 2.0, testutil.ToFloat64(tracker.Gauge()), 0.0)
	assert.Equal(t, 2, tracker.Count())
}

func TestActiveUserTrackerDuplicateUsersDoNotInflateCount(t *testing.T) {
	t.Parallel()

	tracker := NewActiveUserTracker(time.Hour, time.Minute)
	tracker.Record("alice")
	tracker.Record("alice")
	tracker.Record("alice")

	assert.InDelta(t, 1.0, testutil.ToFloat64(tracker.Gauge()), 0.0)
	assert.Equal(t, 1, tracker.Count())
}

func TestActiveUserTrackerEvictsEntriesOlderThanWindow(t *testing.T) {
	t.Parallel()

	window := 50 * time.Millisecond
	tracker := NewActiveUserTracker(window, time.Second)

	tracker.Record("stale-user")
	time.Sleep(window + 20*time.Millisecond)
	tracker.Record("fresh-user")

	// Evict at "now": the stale user is beyond the window, the fresh user is not.
	tracker.evict(time.Now())

	assert.InDelta(t, 1.0, testutil.ToFloat64(tracker.Gauge()), 0.0)
	assert.Equal(t, 1, tracker.Count())

	// Evicting far in the future evicts the remaining entry as well.
	tracker.evict(time.Now().Add(time.Hour))
	assert.InDelta(t, 0.0, testutil.ToFloat64(tracker.Gauge()), 0.0)
	assert.Equal(t, 0, tracker.Count())
}

func TestActiveUserTrackerRunEvictsStaleEntries(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	tracker := NewActiveUserTracker(20*time.Millisecond, 10*time.Millisecond)
	tracker.Record("alice")
	require.InDelta(t, 1.0, testutil.ToFloat64(tracker.Gauge()), 0.0)

	go tracker.Run(ctx)

	deadline := time.Now().Add(5 * time.Second)
	for testutil.ToFloat64(tracker.Gauge()) > 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	assert.InDelta(t, 0.0, testutil.ToFloat64(tracker.Gauge()), 0.0)
}

func TestActiveUserTrackerHashIsStableAndUniquePerUser(t *testing.T) {
	t.Parallel()

	tracker := NewActiveUserTracker(time.Hour, time.Minute)

	alice1 := tracker.Hash("alice")
	alice2 := tracker.Hash("alice")
	bob := tracker.Hash("bob")

	// Same user produces the same hash.
	assert.Equal(t, alice1, alice2)
	// Different users produce different hashes.
	assert.NotEqual(t, alice1, bob)
	// The digest is the hex encoding of a SHA-256 HMAC.
	assert.Len(t, alice1, 64)
	assert.NotEmpty(t, tracker.Hash(""))

	// A different tracker uses a different key, so the same user hashes differently.
	other := NewActiveUserTracker(time.Hour, time.Minute)
	assert.NotEqual(t, alice1, other.Hash("alice"))
}

func TestActiveUserTrackerRunStopsOnContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	tracker := NewActiveUserTracker(time.Hour, 10*time.Millisecond)

	done := make(chan struct{})
	go func() {
		defer close(done)
		tracker.Run(ctx)
	}()

	// Give the goroutine time to reach the select before cancelling.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("tracker.Run did not return after context cancellation")
	}
}

func TestActiveUserTrackerZeroArgumentsFallBackToDefaults(t *testing.T) {
	t.Parallel()

	tracker := NewActiveUserTracker(0, 0)
	if tracker.window != DefaultActiveUsersWindow {
		t.Fatalf("window = %v, want %v", tracker.window, DefaultActiveUsersWindow)
	}
	if tracker.cleanupInterval != DefaultActiveUsersCleanupInterval {
		t.Fatalf("cleanupInterval = %v, want %v", tracker.cleanupInterval, DefaultActiveUsersCleanupInterval)
	}

	// The cleanup loop must start with the (valid, positive) default interval
	// rather than an invalid ticker duration, and return once cancelled.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		tracker.Run(ctx)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("tracker.Run did not return after context cancellation")
	}
}

func TestActiveUserTrackerOlderTimestampCannotOverwriteNewerOne(t *testing.T) {
	t.Parallel()

	window := time.Hour
	tracker := NewActiveUserTracker(window, time.Minute)

	newer := time.Now()
	tracker.record("alice", newer)
	// A stale update for the same user must not overwrite the newer timestamp.
	tracker.record("alice", newer.Add(-2*window))

	tracker.mu.RLock()
	stored := tracker.lastSeen[tracker.Hash("alice")]
	tracker.mu.RUnlock()
	if !stored.Equal(newer) {
		t.Fatalf("stored lastSeen = %v, want %v", stored, newer)
	}

	// Evicting inside the window of the newer timestamp must keep the entry;
	// had the older timestamp won, this sweep would already have evicted it.
	tracker.evict(newer.Add(-window + time.Millisecond))
	assert.InDelta(t, 1.0, testutil.ToFloat64(tracker.Gauge()), 0.0)
	assert.Equal(t, 1, tracker.Count())

	// Evicting past the window of the newer timestamp evicts the entry.
	tracker.evict(newer.Add(window + time.Millisecond))
	assert.InDelta(t, 0.0, testutil.ToFloat64(tracker.Gauge()), 0.0)
	assert.Equal(t, 0, tracker.Count())
}

func TestMetricsServerRecordActiveUser(t *testing.T) {
	t.Parallel()

	m := NewMetricsServer("127.0.0.1", 8083)
	defer m.Stop()

	m.RecordActiveUser("carol")
	m.RecordActiveUser("carol")
	m.RecordActiveUser("dave")

	assert.InDelta(t, 2.0, testutil.ToFloat64(m.activeUsersTracker.Gauge()), 0.0)

	// A nil tracker must not panic.
	(&MetricsServer{}).RecordActiveUser("carol")
}
