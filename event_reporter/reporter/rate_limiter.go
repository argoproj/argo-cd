package reporter

import (
	"context"
	"github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/memorystore"
	"time"
)

type RateLimiterOpts struct {
	Enabled      bool
	Rate         time.Duration
	Capacity     int
	LearningMode bool
}

type RateLimiter struct {
	opts    *RateLimiterOpts
	limiter limiter.Store
}

func NewRateLimiter(opts *RateLimiterOpts) *RateLimiter {
	store, err := memorystore.New(&memorystore.Config{
		Tokens:   uint64(opts.Capacity),
		Interval: opts.Rate,
	})
	if err != nil {
		return &RateLimiter{opts: opts, limiter: nil}
	}
	return &RateLimiter{opts: opts, limiter: store}
}

func (rl *RateLimiter) Limit(applicationName string) (bool, error, bool) {
	if !rl.opts.Enabled {
		return false, nil, rl.opts.LearningMode
	}

	if rl.limiter == nil {
		// TODO: add warning log
		return false, nil, rl.opts.LearningMode
	}

	_, _, _, ok, err := rl.limiter.Take(context.Background(), applicationName)

	if err != nil {
		return false, err, rl.opts.LearningMode
	}

	return !ok, nil, rl.opts.LearningMode
}
