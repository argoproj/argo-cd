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
		d, err, _ := rl.Limit("foo")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if d != 0 {
			t.Errorf("Expected 0 duration, got %v", d)
		}
	})
	t.Run("Limiter is turned on", func(t *testing.T) {
		rl := NewRateLimiter(&RateLimiterOpts{
			Enabled:  true,
			Rate:     time.Second,
			Capacity: 1,
		})
		d, err, _ := rl.Limit("foo")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if d != 0 {
			t.Errorf("Expected 0 duration, got %v", d)
		}
	})
	t.Run("Limiter is turned on but with 0 capacity", func(t *testing.T) {
		rl := NewRateLimiter(&RateLimiterOpts{
			Enabled:  true,
			Rate:     time.Second,
			Capacity: 0,
		})
		_, err, _ := rl.Limit("foo")
		if err == nil {
			t.Errorf("Expected error, got nil")
		}
	})
}
