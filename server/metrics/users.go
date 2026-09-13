package metrics

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// ActiveUsersMetricName is the Prometheus metric name for the active user gauge.
	ActiveUsersMetricName = "argocd_server_active_users_24h"

	// DefaultActiveUsersWindow is the default sliding window of user activity.
	DefaultActiveUsersWindow = 24 * time.Hour

	// DefaultActiveUsersCleanupInterval is the default interval between eviction sweeps.
	DefaultActiveUsersCleanupInterval = 5 * time.Minute
)

// activeUsersGaugeOpts are the common opts for the per-tracker active user
// gauge. The gauge itself is created per tracker so each instance owns its
// metric.
var activeUsersGaugeOpts = prometheus.GaugeOpts{
	Name: ActiveUsersMetricName,
	Help: "Number of distinct authenticated principals seen by the Argo CD API server in the last 24 hours.",
}

// ActiveUserTracker keeps track of the last seen time of anonymized
// authenticated principals observed by the Argo CD API server, and exposes
// the number of distinct principals seen within a sliding window as a
// Prometheus gauge.
//
// Principals are anonymized with an HMAC-SHA256 keyed by a random 32-byte
// secret generated at tracker creation, so plaintext user identifiers are
// never stored or exported.
type ActiveUserTracker struct {
	mu              sync.RWMutex
	lastSeen        map[string]time.Time
	key             []byte
	window          time.Duration
	cleanupInterval time.Duration
	gauge           prometheus.Gauge
}

// NewActiveUserTracker returns a new ActiveUserTracker using the given sliding
// window and cleanup interval. window is how long an entry remains "active"
// after its last seen time, and cleanupInterval is how often stale entries are
// evicted. Tests may pass shorter values than the 24h/5m production defaults.
func NewActiveUserTracker(window, cleanupInterval time.Duration) *ActiveUserTracker {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		// A random key is required for the anonymization to be safe; refuse
		// to start rather than fall back to a predictable key.
		panic(fmt.Sprintf("metrics: failed to generate anonymization key: %v", err))
	}
	if window <= 0 {
		window = DefaultActiveUsersWindow
	}
	if cleanupInterval <= 0 {
		cleanupInterval = DefaultActiveUsersCleanupInterval
	}
	return &ActiveUserTracker{
		lastSeen:        make(map[string]time.Time),
		key:             key,
		window:          window,
		cleanupInterval: cleanupInterval,
		gauge:           prometheus.NewGauge(activeUsersGaugeOpts),
	}
}

// Hash returns the anonymized identifier for the given principal: a
// hex-encoded HMAC-SHA256 digest. The same user always maps to the same
// digest within one tracker, while different trackers (and different users)
// produce different digests.
func (t *ActiveUserTracker) Hash(user string) string {
	mac := hmac.New(sha256.New, t.key)
	mac.Write([]byte(user))
	return hex.EncodeToString(mac.Sum(nil))
}

// Record marks the given principal as active now and updates the gauge to the
// current number of tracked entries.
func (t *ActiveUserTracker) Record(user string) {
	t.record(user, time.Now())
}

func (t *ActiveUserTracker) record(user string, now time.Time) {
	id := t.Hash(user)
	t.mu.Lock()
	t.lastSeen[id] = now
	count := len(t.lastSeen)
	t.mu.Unlock()
	t.gauge.Set(float64(count))
}

// Count returns the number of currently tracked entries.
func (t *ActiveUserTracker) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.lastSeen)
}

// Gauge returns the Prometheus gauge exposing the number of distinct active
// principals, so it can be registered with a Prometheus registry.
func (t *ActiveUserTracker) Gauge() prometheus.Gauge {
	return t.gauge
}

// Run evicts entries whose last seen time falls outside the sliding window
// every cleanupInterval until the context is cancelled, updating the gauge
// after each sweep.
func (t *ActiveUserTracker) Run(ctx context.Context) {
	ticker := time.NewTicker(t.cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.evict(time.Now())
		}
	}
}

// evict removes entries whose last seen time is older than the sliding window
// and updates the gauge.
func (t *ActiveUserTracker) evict(now time.Time) {
	cutoff := now.Add(-t.window)
	t.mu.Lock()
	for id, lastSeenAt := range t.lastSeen {
		if lastSeenAt.Before(cutoff) {
			delete(t.lastSeen, id)
		}
	}
	count := len(t.lastSeen)
	t.mu.Unlock()
	t.gauge.Set(float64(count))
}
