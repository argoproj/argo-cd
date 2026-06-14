package metrics

import (
	"sync"
	"time"
)

// DAUTracker tracks distinct active users over a sliding window.
// It is safe for concurrent use.
type DAUTracker struct {
	mu      sync.RWMutex
	window  time.Duration
	users   map[string]time.Time
	metrics *MetricsServer
	ticker  *time.Ticker
	stopCh  chan struct{}
}

// NewDAUTracker creates a new DAUTracker with the given window duration.
// If metricsServer is non-nil, the gauge will be updated periodically.
func NewDAUTracker(window time.Duration, metricsServer *MetricsServer) *DAUTracker {
	return &DAUTracker{
		window:  window,
		users:   make(map[string]time.Time),
		metrics: metricsServer,
		stopCh:  make(chan struct{}),
	}
}

// Start begins the background cleanup loop. Call Stop to terminate it.
func (t *DAUTracker) Start(interval time.Duration) {
	t.ticker = time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-t.ticker.C:
				t.cleanup()
			case <-t.stopCh:
				return
			}
		}
	}()
}

// Stop terminates the background cleanup loop.
func (t *DAUTracker) Stop() {
	close(t.stopCh)
	if t.ticker != nil {
		t.ticker.Stop()
	}
}

// RecordUser records that a user was active now.
func (t *DAUTracker) RecordUser(userID string) {
	if userID == "" {
		return
	}
	t.mu.Lock()
	t.users[userID] = time.Now()
	t.mu.Unlock()
}

// Count returns the number of distinct users active within the window.
func (t *DAUTracker) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	cutoff := time.Now().Add(-t.window)
	count := 0
	for _, lastSeen := range t.users {
		if lastSeen.After(cutoff) {
			count++
		}
	}
	return count
}

func (t *DAUTracker) cleanup() {
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := time.Now().Add(-t.window)
	for userID, lastSeen := range t.users {
		if !lastSeen.After(cutoff) {
			delete(t.users, userID)
		}
	}
	if t.metrics != nil {
		t.metrics.SetDailyActiveUsers(float64(len(t.users)))
	}
}
