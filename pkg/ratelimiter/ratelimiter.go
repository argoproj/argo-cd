package ratelimiter

import (
	"math"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
)

type AppControllerRateLimiterConfig struct {
	BucketSize      int64
	BucketQPS       float64
	FailureCoolDown time.Duration
	BaseDelay       time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
}

func GetDefaultAppRateLimiterConfig() *AppControllerRateLimiterConfig {
	return &AppControllerRateLimiterConfig{
		// global queue rate limit config
		500,
		// when WORKQUEUE_BUCKET_QPS is MaxFloat64 global bucket limiting is disabled(default)
		math.MaxFloat64,
		// individual item rate limit config
		// when WORKQUEUE_FAILURE_COOLDOWN is 0 per item rate limiting is disabled(default)
		0,
		time.Millisecond,
		time.Second,
		1.5,
	}
}

// NewCustomAppControllerRateLimiter is a constructor for the rate limiter for a workqueue used by app controller.  It has
// both overall and per-item rate limiting.  The overall is a token bucket and the per-item is exponential(with auto resets)
func NewCustomAppControllerRateLimiter(cfg *AppControllerRateLimiterConfig) workqueue.TypedRateLimiter[string] {
	return workqueue.NewTypedMaxOfRateLimiter[string](
		NewItemExponentialRateLimiterWithAutoReset(cfg.BaseDelay, cfg.MaxDelay, cfg.FailureCoolDown, cfg.BackoffFactor),
		&workqueue.TypedBucketRateLimiter[string]{Limiter: rate.NewLimiter(rate.Limit(cfg.BucketQPS), int(cfg.BucketSize))},
	)
}

type failureData struct {
	failures    int
	lastFailure time.Time
}

// ItemExponentialRateLimiterWithAutoReset does a simple baseDelay*2^<num-failures> limit
// dealing with max failures and expiration/resets are up dependent on the cooldown period
type ItemExponentialRateLimiterWithAutoReset struct {
	failuresLock sync.Mutex
	failures     map[interface{}]failureData

	baseDelay     time.Duration
	maxDelay      time.Duration
	coolDown      time.Duration
	backoffFactor float64
}

var _ workqueue.TypedRateLimiter[string] = &ItemExponentialRateLimiterWithAutoReset{}

func NewItemExponentialRateLimiterWithAutoReset(baseDelay, maxDelay, failureCoolDown time.Duration, backoffFactor float64) workqueue.TypedRateLimiter[string] {
	return &ItemExponentialRateLimiterWithAutoReset{
		failures:      map[interface{}]failureData{},
		baseDelay:     baseDelay,
		maxDelay:      maxDelay,
		coolDown:      failureCoolDown,
		backoffFactor: backoffFactor,
	}
}

func (r *ItemExponentialRateLimiterWithAutoReset) When(item string) time.Duration {
	r.failuresLock.Lock()
	defer r.failuresLock.Unlock()

	if _, ok := r.failures[item]; !ok {
		r.failures[item] = failureData{
			failures:    0,
			lastFailure: time.Now(),
		}
	}

	exp := r.failures[item]

	// if coolDown period is reached reset failures for item
	if time.Since(exp.lastFailure) >= r.coolDown {
		delete(r.failures, item)
		return r.baseDelay
	}

	r.failures[item] = failureData{
		failures:    exp.failures + 1,
		lastFailure: time.Now(),
	}

	// The backoff is capped such that 'calculated' value never overflows.
	backoff := float64(r.baseDelay.Nanoseconds()) * math.Pow(r.backoffFactor, float64(exp.failures))
	if backoff > math.MaxInt64 {
		return r.maxDelay
	}

	calculated := time.Duration(backoff)
	if calculated > r.maxDelay {
		return r.maxDelay
	}

	return calculated
}

func (r *ItemExponentialRateLimiterWithAutoReset) NumRequeues(item string) int {
	r.failuresLock.Lock()
	defer r.failuresLock.Unlock()

	return r.failures[item].failures
}

func (r *ItemExponentialRateLimiterWithAutoReset) Forget(item string) {
	r.failuresLock.Lock()
	defer r.failuresLock.Unlock()

	delete(r.failures, item)
}
