package cache

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/semaphore"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2/textlogger"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
)

func Test_isWatchBackoffError(t *testing.T) {
	gr := schema.GroupResource{Group: "apps", Resource: "deployments"}

	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "401 Unauthorized",
			err:      apierrors.NewUnauthorized("token expired"),
			expected: true,
		},
		{
			name:     "401 Unauthorized wrapped",
			err:      fmt.Errorf("failed to list resources: %w", apierrors.NewUnauthorized("token expired")),
			expected: true,
		},
		{
			name:     "403 Forbidden",
			err:      apierrors.NewForbidden(gr, "my-deploy", errors.New("forbidden")),
			expected: true,
		},
		{
			name:     "429 Too Many Requests",
			err:      apierrors.NewTooManyRequests("rate limit exceeded", 5),
			expected: true,
		},
		{
			name:     "500 Internal Error",
			err:      apierrors.NewInternalError(errors.New("internal server error")),
			expected: true,
		},
		{
			name:     "503 Service Unavailable",
			err:      apierrors.NewServiceUnavailable("service unavailable"),
			expected: true,
		},
		{
			name:     "504 Timeout Error",
			err:      apierrors.NewTimeoutError("gateway timeout", 30),
			expected: true,
		},
		{
			name:     "Server Timeout",
			err:      apierrors.NewServerTimeout(gr, "watch", 10),
			expected: true,
		},
		{
			name: "Unexpected Server Error",
			err: apierrors.NewGenericServerResponse(http.StatusInternalServerError, "watch", gr, "my-deploy", "server error", 0, false),
			expected: true,
		},
		{
			name:     "404 NotFound",
			err:      apierrors.NewNotFound(gr, "my-deploy"),
			expected: false,
		},
		{
			name:     "410 Gone (ResourceVersion too old)",
			err:      apierrors.NewGone("too old resource version"),
			expected: false,
		},
		{
			name:     "Generic error (e.g. watch closed)",
			err:      fmt.Errorf("watch apps/deployments on https://kubernetes.default.svc has closed"),
			expected: false,
		},
		{
			name:     "Resync timeout error",
			err:      fmt.Errorf("resyncing apps/deployments on https://kubernetes.default.svc due to timeout"),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := isWatchBackoffError(tc.err)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_WatchBackoff_StepProgression(t *testing.T) {
	backoff := wait.Backoff{
		Duration: watchResourcesRetryTimeout, // 1s
		Factor:   defaultWatchBackoffFactor,   // 2.0
		Jitter:   0,                           // 0 jitter for deterministic progression testing
		Steps:    defaultWatchBackoffSteps,    // 5 steps
		Cap:      defaultWatchBackoffCap,      // 30s
	}

	expectedDelays := []time.Duration{
		1 * time.Second,  // step 1
		2 * time.Second,  // step 2
		4 * time.Second,  // step 3
		8 * time.Second,  // step 4
		16 * time.Second, // step 5
		30 * time.Second, // capped at 30s
		30 * time.Second, // stays capped at 30s
	}

	for i, expected := range expectedDelays {
		delay := backoff.Step()
		assert.Equal(t, expected, delay, "Mismatch on attempt %d", i+1)
	}
}

func Test_WatchBackoff_HealthyReset(t *testing.T) {
	initialBackoff := wait.Backoff{
		Duration: watchResourcesRetryTimeout,
		Factor:   defaultWatchBackoffFactor,
		Jitter:   0,
		Steps:    defaultWatchBackoffSteps,
		Cap:      defaultWatchBackoffCap,
	}

	currentBackoff := initialBackoff

	// Step 1, 2, 3 after failures
	d1 := currentBackoff.Step()
	assert.Equal(t, 1*time.Second, d1)
	d2 := currentBackoff.Step()
	assert.Equal(t, 2*time.Second, d2)
	d3 := currentBackoff.Step()
	assert.Equal(t, 4*time.Second, d3)

	// Flapping scenario: watch survives for only 30s (< defaultWatchHealthyDuration 2m)
	flappingDuration := 30 * time.Second
	if flappingDuration >= defaultWatchHealthyDuration {
		currentBackoff = initialBackoff
	}
	// Backoff should NOT reset, but continue advancing to step 4 (8s)
	flappingDelay := currentBackoff.Step()
	assert.Equal(t, 8*time.Second, flappingDelay)

	// Healthy scenario: watch runs stably for >= defaultWatchHealthyDuration (2m)
	healthyDuration := 3 * time.Minute
	if healthyDuration >= defaultWatchHealthyDuration {
		currentBackoff = initialBackoff
	}

	// After healthy reset, next failure should start back at 1s
	resetDelay := currentBackoff.Step()
	assert.Equal(t, 1*time.Second, resetDelay)
}

func Test_WatchError_FromObject(t *testing.T) {
	// Status objects sent via watch.Error events from api server
	testCases := []struct {
		name     string
		status   *metav1.Status
		expected bool
	}{
		{
			name: "401 Unauthorized watch error",
			status: &metav1.Status{
				Status: metav1.StatusFailure,
				Code:   http.StatusUnauthorized,
				Reason: metav1.StatusReasonUnauthorized,
			},
			expected: true,
		},
		{
			name: "403 Forbidden watch error",
			status: &metav1.Status{
				Status: metav1.StatusFailure,
				Code:   http.StatusForbidden,
				Reason: metav1.StatusReasonForbidden,
			},
			expected: true,
		},
		{
			name: "429 Too Many Requests watch error",
			status: &metav1.Status{
				Status: metav1.StatusFailure,
				Code:   http.StatusTooManyRequests,
				Reason: metav1.StatusReasonTooManyRequests,
			},
			expected: true,
		},
		{
			name: "500 Internal Server Error watch error",
			status: &metav1.Status{
				Status: metav1.StatusFailure,
				Code:   http.StatusInternalServerError,
				Reason: metav1.StatusReasonInternalError,
			},
			expected: true,
		},
		{
			name: "503 Service Unavailable watch error",
			status: &metav1.Status{
				Status: metav1.StatusFailure,
				Code:   http.StatusServiceUnavailable,
				Reason: metav1.StatusReasonServiceUnavailable,
			},
			expected: true,
		},
		{
			name: "504 Gateway Timeout watch error",
			status: &metav1.Status{
				Status: metav1.StatusFailure,
				Code:   http.StatusGatewayTimeout,
				Reason: metav1.StatusReasonTimeout,
			},
			expected: true,
		},
		{
			name: "410 Gone (ResourceVersion too old) watch error",
			status: &metav1.Status{
				Status: metav1.StatusFailure,
				Code:   http.StatusGone,
				Reason: metav1.StatusReasonExpired,
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := apierrors.FromObject(tc.status)
			assert.Equal(t, tc.expected, isWatchBackoffError(err))
		})
	}
}

type mockWatchResourceInterface struct {
	*mockResourceInterface
	watchFunc func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	listFunc  func(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error)
}

func (m *mockWatchResourceInterface) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, opts)
	}
	return m.mockResourceInterface.List(ctx, opts)
}

func (m *mockWatchResourceInterface) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	if m.watchFunc != nil {
		return m.watchFunc(ctx, opts)
	}
	return nil, errors.New("not implemented")
}

func Test_RetryWatchUntilSucceed_Delays(t *testing.T) {
	t.Parallel()

	t.Run("exponential backoff for retryable errors", func(t *testing.T) {
		cache := &clusterCache{
			config:               &rest.Config{Host: "https://test-server"},
			log:                  textlogger.NewLogger(textlogger.NewConfig()),
			watchRetryUseBackoff: true,
		}

		var recordedDelays []time.Duration
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		attempt := 0
		cache.retryDelayFunc = func(ctx context.Context, delay time.Duration, _ error) error {
			recordedDelays = append(recordedDelays, delay)
			attempt++
			if attempt >= 6 {
				cancel()
				return context.Canceled
			}
			return nil
		}

		cache.retryWatchUntilSucceed(ctx, "test-watch", func(_ func()) error {
			return apierrors.NewTooManyRequests("rate limit", 5)
		})

		assert.Len(t, recordedDelays, 6)
		// Step 1: ~1s (with 10% jitter: [0.9s, 1.1s])
		assert.InDelta(t, 1*time.Second, recordedDelays[0], float64(150*time.Millisecond))
		// Step 2: ~2s (with 10% jitter: [1.8s, 2.2s])
		assert.InDelta(t, 2*time.Second, recordedDelays[1], float64(250*time.Millisecond))
		// Step 3: ~4s
		assert.InDelta(t, 4*time.Second, recordedDelays[2], float64(450*time.Millisecond))
		// Step 4: ~8s
		assert.InDelta(t, 8*time.Second, recordedDelays[3], float64(850*time.Millisecond))
		// Step 5: ~16s
		assert.InDelta(t, 16*time.Second, recordedDelays[4], float64(1700*time.Millisecond))
		// Step 6: 30s cap
		assert.InDelta(t, 30*time.Second, recordedDelays[5], float64(3100*time.Millisecond))
	})

	t.Run("flat 1s delay for non-retryable errors", func(t *testing.T) {
		cache := &clusterCache{
			config:               &rest.Config{Host: "https://test-server"},
			log:                  textlogger.NewLogger(textlogger.NewConfig()),
			watchRetryUseBackoff: true,
		}

		var recordedDelays []time.Duration
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		attempt := 0
		cache.retryDelayFunc = func(ctx context.Context, delay time.Duration, _ error) error {
			recordedDelays = append(recordedDelays, delay)
			attempt++
			if attempt >= 3 {
				cancel()
				return context.Canceled
			}
			return nil
		}

		cache.retryWatchUntilSucceed(ctx, "test-watch", func(_ func()) error {
			return apierrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "deployments"}, "my-deploy")
		})

		assert.Len(t, recordedDelays, 3)
		for i, d := range recordedDelays {
			assert.Equal(t, watchResourcesRetryTimeout, d, "Attempt %d should have base delay", i+1)
		}
	})

	t.Run("flat 1s delay when backoff disabled", func(t *testing.T) {
		cache := &clusterCache{
			config:               &rest.Config{Host: "https://test-server"},
			log:                  textlogger.NewLogger(textlogger.NewConfig()),
			watchRetryUseBackoff: false,
		}

		var recordedDelays []time.Duration
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		attempt := 0
		cache.retryDelayFunc = func(ctx context.Context, delay time.Duration, _ error) error {
			recordedDelays = append(recordedDelays, delay)
			attempt++
			if attempt >= 3 {
				cancel()
				return context.Canceled
			}
			return nil
		}

		cache.retryWatchUntilSucceed(ctx, "test-watch", func(_ func()) error {
			return apierrors.NewTooManyRequests("rate limit", 5)
		})

		assert.Len(t, recordedDelays, 3)
		for i, d := range recordedDelays {
			assert.Equal(t, watchResourcesRetryTimeout, d, "Attempt %d should have base delay", i+1)
		}
	})
}

func Test_WatchEvents_RateLimit_TriggersBackoff(t *testing.T) {
	t.Parallel()

	cache := &clusterCache{
		config:               &rest.Config{Host: "https://test-server"},
		log:                  textlogger.NewLogger(textlogger.NewConfig()),
		watchRetryUseBackoff: true,
		listSemaphore:        semaphore.NewWeighted(1),
		listPageSize:         100,
		listRetryLimit:       1,
	}

	var recordedDelays []time.Duration
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listCallCount := 0
	mockClient := &mockWatchResourceInterface{
		mockResourceInterface: &mockResourceInterface{},
		watchFunc: func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
			return nil, apierrors.NewTooManyRequests("rate limit exceeded", 5)
		},
		listFunc: func(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
			listCallCount++
			list := &unstructured.UnstructuredList{}
			list.SetResourceVersion("1001")
			return list, nil
		},
	}

	attempt := 0
	cache.retryDelayFunc = func(ctx context.Context, delay time.Duration, _ error) error {
		recordedDelays = append(recordedDelays, delay)
		attempt++
		if attempt >= 2 {
			cancel()
			return context.Canceled
		}
		return nil
	}

	api := kube.APIResourceInfo{
		GroupKind: schema.GroupKind{Group: "apps", Kind: "Deployment"},
	}

	// Passing non-empty resourceVersion bypasses loadInitialState and enters RetryWatcher
	cache.watchEvents(ctx, api, mockClient, "default", "1000")

	assert.GreaterOrEqual(t, len(recordedDelays), 2, "WatchEvents should break out of RetryWatcher and record retry delays")
	assert.InDelta(t, 1*time.Second, recordedDelays[0], float64(150*time.Millisecond))
	assert.InDelta(t, 2*time.Second, recordedDelays[1], float64(250*time.Millisecond))
	assert.Equal(t, 0, listCallCount, "Rate-limited retries should preserve resourceVersion and not trigger full list")
}

func Test_WatchEvents_NonBackoffError_ResetsResourceVersionAndTriggersList(t *testing.T) {
	t.Parallel()

	cache := &clusterCache{
		config:               &rest.Config{Host: "https://test-server"},
		log:                  textlogger.NewLogger(textlogger.NewConfig()),
		watchRetryUseBackoff: true,
		listSemaphore:        semaphore.NewWeighted(1),
		listPageSize:         100,
		listRetryLimit:       1,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listCallCount := 0
	var capturedErrs []error
	mockClient := &mockWatchResourceInterface{
		mockResourceInterface: &mockResourceInterface{},
		watchFunc: func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
			fw := watch.NewFakeWithChanSize(1, true)
			fw.Error(&metav1.Status{
				Code:   410,
				Reason: metav1.StatusReasonExpired,
			})
			return fw, nil
		},
		listFunc: func(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
			listCallCount++
			list := &unstructured.UnstructuredList{}
			list.SetResourceVersion("1001")
			return list, nil
		},
	}

	attempt := 0
	cache.retryDelayFunc = func(ctx context.Context, delay time.Duration, err error) error {
		capturedErrs = append(capturedErrs, err)
		attempt++
		if attempt >= 2 {
			cancel()
			return context.Canceled
		}
		return nil
	}

	api := kube.APIResourceInfo{
		GroupKind: schema.GroupKind{Group: "apps", Kind: "Deployment"},
	}

	// Passing non-empty resourceVersion bypasses loadInitialState initially
	cache.watchEvents(ctx, api, mockClient, "default", "1000")

	// On 410 Gone, resourceVersion must be cleared, triggering full list on the subsequent retry
	assert.GreaterOrEqual(t, listCallCount, 1, "410 Gone should reset resourceVersion and trigger loadInitialState (list)")
	assert.NotEmpty(t, capturedErrs, "retryDelayFunc should capture the 410 watch error")
	assert.True(t, apierrors.IsResourceExpired(capturedErrs[0]) || apierrors.IsGone(capturedErrs[0]),
		"410 watch.Error should be converted to apierrors.StatusError, got: %v", capturedErrs[0])
	assert.NotContains(t, capturedErrs[0].Error(), "failed to convert to *unstructured.Unstructured")
}

func Test_WatchEvents_WatchError_ConvertedToStatusError(t *testing.T) {
	t.Parallel()

	api := kube.APIResourceInfo{
		GroupKind: schema.GroupKind{Group: "apps", Kind: "Deployment"},
	}

	t.Run("410 Gone watch.Error event converts to apierrors.StatusError", func(t *testing.T) {
		cache := &clusterCache{
			config:               &rest.Config{Host: "https://test-server"},
			log:                  textlogger.NewLogger(textlogger.NewConfig()),
			watchRetryUseBackoff: true,
			listSemaphore:        semaphore.NewWeighted(1),
			listPageSize:         100,
			listRetryLimit:       1,
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockClient := &mockWatchResourceInterface{
			mockResourceInterface: &mockResourceInterface{},
			watchFunc: func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
				fw := watch.NewFakeWithChanSize(1, true)
				fw.Error(&metav1.Status{
					Code:    410,
					Reason:  metav1.StatusReasonExpired,
					Message: "The resourceVersion for the provided watch is too old.",
				})
				return fw, nil
			},
		}

		var capturedErrs []error
		cache.retryDelayFunc = func(ctx context.Context, _ time.Duration, err error) error {
			capturedErrs = append(capturedErrs, err)
			cancel()
			return context.Canceled
		}

		cache.watchEvents(ctx, api, mockClient, "default", "1000")

		assert.NotEmpty(t, capturedErrs)
		assert.True(t, apierrors.IsResourceExpired(capturedErrs[0]) || apierrors.IsGone(capturedErrs[0]),
			"Expected 410 StatusError, got: %v", capturedErrs[0])
		assert.NotContains(t, capturedErrs[0].Error(), "failed to convert to *unstructured.Unstructured")
	})

	t.Run("403 Forbidden watch.Error event converts to apierrors.StatusError when backoff disabled", func(t *testing.T) {
		// When watchRetryUseBackoff is false (default), RetryWatcher handles 401/403 by sending
		// a watch.Error event on ResultChan. The event.Type == watch.Error branch must convert it
		// to an apierrors.StatusError rather than failing type assertion to *unstructured.Unstructured.
		cache := &clusterCache{
			config:               &rest.Config{Host: "https://test-server"},
			log:                  textlogger.NewLogger(textlogger.NewConfig()),
			watchRetryUseBackoff: false,
			listSemaphore:        semaphore.NewWeighted(1),
			listPageSize:         100,
			listRetryLimit:       1,
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		gr := schema.GroupResource{Group: "apps", Resource: "deployments"}
		mockClient := &mockWatchResourceInterface{
			mockResourceInterface: &mockResourceInterface{},
			watchFunc: func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
				return nil, apierrors.NewForbidden(gr, "my-deploy", errors.New("forbidden"))
			},
		}

		var capturedErrs []error
		cache.retryDelayFunc = func(ctx context.Context, _ time.Duration, err error) error {
			capturedErrs = append(capturedErrs, err)
			cancel()
			return context.Canceled
		}

		cache.watchEvents(ctx, api, mockClient, "default", "1000")

		assert.NotEmpty(t, capturedErrs)
		assert.True(t, apierrors.IsForbidden(capturedErrs[0]),
			"Expected 403 Forbidden StatusError, got: %v", capturedErrs[0])
		assert.NotContains(t, capturedErrs[0].Error(), "failed to convert to *unstructured.Unstructured")
	})
}


