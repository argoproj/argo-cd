package reporter

import (
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	t.Run("Limiter is turned off", func(t *testing.T) {
		rl := NewRateLimiter(&RateLimiterOpts{
			Enabled: false,
		})
		limit, err, _ := rl.Limit("foo")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if limit {
			t.Errorf("Should be no limited")
		}
	})
	t.Run("Limiter is turned on", func(t *testing.T) {
		rl := NewRateLimiter(&RateLimiterOpts{
			Enabled:  true,
			Rate:     time.Second,
			Capacity: 1,
		})
		limit, err, _ := rl.Limit("foo")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if limit {
			t.Errorf("Should be no limited")
		}
	})
	t.Run("Limiter is turned on but with 0 capacity", func(t *testing.T) {
		rl := NewRateLimiter(&RateLimiterOpts{
			Enabled:  true,
			Rate:     time.Second,
			Capacity: 1,
		})
		limit, _, _ := rl.Limit("foo")
		if limit {
			t.Errorf("Expected no limit, got nil")
		}

		limit, _, _ = rl.Limit("foo")
		if !limit {
			t.Errorf("Expected  limit, got nil")
		}
	})
}
