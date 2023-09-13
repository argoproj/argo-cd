package ratelimiter

import (
	"math"
	"sync"
	"time"

	"github.com/argoproj/argo-cd/v2/util/env"
	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
)

type AppControllerRateLimiterConfig struct {
	BucketSize      int
	BucketQPS       int
	FailureCoolDown time.Duration
	BaseDelay       time.Duration
	MaxDelay        time.Duration
}

func GetAppRateLimiterConfig() *AppControllerRateLimiterConfig {
	return &AppControllerRateLimiterConfig{
		int(env.ParseInt64FromEnv("WORKQUEUE_BUCKET_SIZE", 500, 1, math.MaxInt64)),
		int(env.ParseInt64FromEnv("WORKQUEUE_BUCKET_QPS", 50, 1, math.MaxInt64)),
		env.ParseDurationFromEnv("WORKQUEUE_FAILURE_COOLDOWN", 5*time.Minute, 0, 24*time.Hour),
		env.ParseDurationFromEnv("WORKQUEUE_BASE_DELAY", 2*time.Millisecond, 1*time.Millisecond, 24*time.Hour),
		env.ParseDurationFromEnv("WORKQUEUE_MAX_DELAY", 1000*time.Second, 1*time.Millisecond, 24*time.Hour),
	}
}

// NewCustomAppControllerRateLimiter is a constructor for the rate limiter for a workqueue used by app controller.  It has
// both overall and per-item rate limiting.  The overall is a token bucket and the per-item is exponential(with auto resets)
func NewCustomAppControllerRateLimiter(cfg *AppControllerRateLimiterConfig) workqueue.RateLimiter {
	return workqueue.NewMaxOfRateLimiter(
		NewItemExponentialRateLimiterWithAutoReset(cfg.BaseDelay, cfg.MaxDelay, cfg.FailureCoolDown),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(cfg.BucketQPS), cfg.BucketSize)},
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

	baseDelay time.Duration
	maxDelay  time.Duration
	coolDown  time.Duration
}

var _ workqueue.RateLimiter = &ItemExponentialRateLimiterWithAutoReset{}

func NewItemExponentialRateLimiterWithAutoReset(baseDelay, maxDelay, failureCoolDown time.Duration) workqueue.RateLimiter {
	return &ItemExponentialRateLimiterWithAutoReset{
		failures:  map[interface{}]failureData{},
		baseDelay: baseDelay,
		maxDelay:  maxDelay,
		coolDown:  failureCoolDown,
	}
}

func (r *ItemExponentialRateLimiterWithAutoReset) When(item interface{}) time.Duration {
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
	backoff := float64(r.baseDelay.Nanoseconds()) * math.Pow(2, float64(exp.failures))
	if backoff > math.MaxInt64 {
		return r.maxDelay
	}

	calculated := time.Duration(backoff)
	if calculated > r.maxDelay {
		return r.maxDelay
	}

	return calculated
}

func (r *ItemExponentialRateLimiterWithAutoReset) NumRequeues(item interface{}) int {
	r.failuresLock.Lock()
	defer r.failuresLock.Unlock()

	return r.failures[item].failures
}

func (r *ItemExponentialRateLimiterWithAutoReset) Forget(item interface{}) {
	r.failuresLock.Lock()
	defer r.failuresLock.Unlock()

	delete(r.failures, item)
}
