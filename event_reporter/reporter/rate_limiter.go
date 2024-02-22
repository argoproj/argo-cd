package reporter

import (
	"context"
	"github.com/mennanov/limiters"
	"time"
)

type RateLimiterOpts struct {
	Enabled      bool
	Rate         time.Duration
	Capacity     int
	LearningMode bool
}

type RateLimiter struct {
	opts     *RateLimiterOpts
	limiters map[string]*limiters.FixedWindow
}

func NewRateLimiter(opts *RateLimiterOpts) *RateLimiter {
	return &RateLimiter{opts: opts, limiters: make(map[string]*limiters.FixedWindow)}
}

func (rl *RateLimiter) Limit(applicationName string) (time.Duration, error, bool) {
	if !rl.opts.Enabled {
		return time.Duration(0), nil, rl.opts.LearningMode
	}

	limiter := rl.limiters[applicationName]
	if limiter == nil {
		limiter = limiters.NewFixedWindow(int64(rl.opts.Capacity), rl.opts.Rate, limiters.NewFixedWindowInMemory(), limiters.NewSystemClock())
		rl.limiters[applicationName] = limiter
	}

	duration, err := limiter.Limit(context.Background())
	return duration, err, rl.opts.LearningMode
}
