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
	assert.Equal(t, 0.0, testutil.ToFloat64(tracker.Gauge()))

	tracker.Record("alice")
	assert.Equal(t, 1.0, testutil.ToFloat64(tracker.Gauge()))
	assert.Equal(t, 1, tracker.Count())

	tracker.Record("bob")
	assert.Equal(t, 2.0, testutil.ToFloat64(tracker.Gauge()))
	assert.Equal(t, 2, tracker.Count())
}

func TestActiveUserTrackerDuplicateUsersDoNotInflateCount(t *testing.T) {
	t.Parallel()

	tracker := NewActiveUserTracker(time.Hour, time.Minute)
	tracker.Record("alice")
	tracker.Record("alice")
	tracker.Record("alice")

	assert.Equal(t, 1.0, testutil.ToFloat64(tracker.Gauge()))
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

	assert.Equal(t, 1.0, testutil.ToFloat64(tracker.Gauge()))
	assert.Equal(t, 1, tracker.Count())

	// Evicting far in the future evicts the remaining entry as well.
	tracker.evict(time.Now().Add(time.Hour))
	assert.Equal(t, 0.0, testutil.ToFloat64(tracker.Gauge()))
	assert.Equal(t, 0, tracker.Count())
}

func TestActiveUserTrackerRunEvictsStaleEntries(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	tracker := NewActiveUserTracker(20*time.Millisecond, 10*time.Millisecond)
	tracker.Record("alice")
	require.Equal(t, 1.0, testutil.ToFloat64(tracker.Gauge()))

	go tracker.Run(ctx)

	deadline := time.Now().Add(5 * time.Second)
	for testutil.ToFloat64(tracker.Gauge()) > 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	assert.Equal(t, 0.0, testutil.ToFloat64(tracker.Gauge()))
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

func TestMetricsServerRecordActiveUser(t *testing.T) {
	t.Parallel()

	m := NewMetricsServer("127.0.0.1", 8083)
	defer m.Stop()

	m.RecordActiveUser("carol")
	m.RecordActiveUser("carol")
	m.RecordActiveUser("dave")

	assert.Equal(t, 2.0, testutil.ToFloat64(m.activeUsersTracker.Gauge()))

	// A nil tracker must not panic.
	(&MetricsServer{}).RecordActiveUser("carol")
}
