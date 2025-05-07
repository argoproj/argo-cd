package services

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Doc for the GitHub API rate limit headers:
// https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api?apiVersion=2022-11-28#checking-the-status-of-your-rate-limit

// Metric names as constants
const (
	githubAPIRequestTotalPerAppSetMetricName       = "argocd_github_api_requests_total_per_appset"
	githubAPIRequestDurationPerAppSetMetricName    = "argocd_github_api_request_duration_seconds_per_appset"
	githubAPIRateLimitRemainingPerAppSetMetricName = "argocd_github_api_rate_limit_remaining_per_appset"
	githubAPIRateLimitLimitPerAppSetMetricName     = "argocd_github_api_rate_limit_limit_per_appset"
	githubAPIRateLimitResetPerAppSetMetricName     = "argocd_github_api_rate_limit_reset_per_appset"
	githubAPIRateLimitUsedPerAppSetMetricName      = "argocd_github_api_rate_limit_used_per_appset"
	githubAPIRateLimitResourcePerAppSetMetricName  = "argocd_github_api_rate_limit_resource_per_appset"
)

// GitHubMetrics groups all metric vectors for easier injection and registration
type GitHubMetrics struct {
	RequestTotal       *prometheus.CounterVec
	RequestDuration    *prometheus.HistogramVec
	RateLimitRemaining *prometheus.GaugeVec
	RateLimitLimit     *prometheus.GaugeVec
	RateLimitReset     *prometheus.GaugeVec
	RateLimitUsed      *prometheus.GaugeVec
	RateLimitResource  *prometheus.GaugeVec
}

// Factory for a new set of GitHub metrics (for tests or custom registries)
func NewGitHubMetrics() *GitHubMetrics {
	return &GitHubMetrics{
		RequestTotal:       NewGitHubAPIRequestTotalPerAppSet(),
		RequestDuration:    NewGitHubAPIRequestDurationPerAppSet(),
		RateLimitRemaining: NewGitHubAPIRateLimitRemainingPerAppSet(),
		RateLimitLimit:     NewGitHubAPIRateLimitLimitPerAppSet(),
		RateLimitReset:     NewGitHubAPIRateLimitResetPerAppSet(),
		RateLimitUsed:      NewGitHubAPIRateLimitUsedPerAppSet(),
		RateLimitResource:  NewGitHubAPIRateLimitResourcePerAppSet(),
	}
}

// Factory functions for each metric vector
func NewGitHubAPIRequestTotalPerAppSet() *prometheus.CounterVec {
	return prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: githubAPIRequestTotalPerAppSetMetricName,
			Help: "Total number of GitHub API requests per ApplicationSet",
		},
		[]string{"method", "endpoint", "status", "appset_namespace", "appset_name"},
	)
}

func NewGitHubAPIRequestDurationPerAppSet() *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    githubAPIRequestDurationPerAppSetMetricName,
			Help:    "GitHub API request duration in seconds, per ApplicationSet",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint", "appset_namespace", "appset_name"},
	)
}

func NewGitHubAPIRateLimitRemainingPerAppSet() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: githubAPIRateLimitRemainingPerAppSetMetricName,
			Help: "The number of requests remaining in the current rate limit window, per ApplicationSet",
		},
		[]string{"endpoint", "appset_namespace", "appset_name"},
	)
}

func NewGitHubAPIRateLimitLimitPerAppSet() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: githubAPIRateLimitLimitPerAppSetMetricName,
			Help: "The maximum number of requests that you can make per hour, per ApplicationSet",
		},
		[]string{"endpoint", "appset_namespace", "appset_name"},
	)
}

func NewGitHubAPIRateLimitResetPerAppSet() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: githubAPIRateLimitResetPerAppSetMetricName,
			Help: "The time at which the current rate limit window resets, in UTC epoch seconds, per ApplicationSet",
		},
		[]string{"endpoint", "appset_namespace", "appset_name"},
	)
}

func NewGitHubAPIRateLimitUsedPerAppSet() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: githubAPIRateLimitUsedPerAppSetMetricName,
			Help: "The number of requests used in the current rate limit window, per ApplicationSet",
		},
		[]string{"endpoint", "appset_namespace", "appset_name"},
	)
}

func NewGitHubAPIRateLimitResourcePerAppSet() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: githubAPIRateLimitResourcePerAppSetMetricName,
			Help: "The rate limit resource that the request counted against, per ApplicationSet",
		},
		[]string{"endpoint", "resource", "appset_namespace", "appset_name"},
	)
}

// Global metrics (registered with the default registry)
var globalGitHubMetrics = NewGitHubMetrics()

func init() {
	log.Debug("Registering GitHub API AppSet metrics")
	metrics.Registry.MustRegister(globalGitHubMetrics.RequestTotal)
	metrics.Registry.MustRegister(globalGitHubMetrics.RequestDuration)
	metrics.Registry.MustRegister(globalGitHubMetrics.RateLimitRemaining)
	metrics.Registry.MustRegister(globalGitHubMetrics.RateLimitLimit)
	metrics.Registry.MustRegister(globalGitHubMetrics.RateLimitReset)
	metrics.Registry.MustRegister(globalGitHubMetrics.RateLimitUsed)
	metrics.Registry.MustRegister(globalGitHubMetrics.RateLimitResource)
}

type MetricsContext struct {
	AppSetNamespace string
	AppSetName      string
}

// GitHubMetricsTransport is a custom http.RoundTripper that collects GitHub API metrics
type GitHubMetricsTransport struct {
	transport      http.RoundTripper
	metricsContext *MetricsContext
	metrics        *GitHubMetrics
}

// RoundTrip implements http.RoundTripper interface and collects metrics along with debug logging
func (t *GitHubMetricsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	endpoint := req.URL.Path
	method := req.Method

	appsetNamespace := "unknown"
	appsetName := "unknown"
	if t.metricsContext != nil {
		appsetNamespace = t.metricsContext.AppSetNamespace
		appsetName = t.metricsContext.AppSetName
	}

	log.WithFields(log.Fields{
		"method":         method,
		"endpoint":       endpoint,
		"applicationset": appsetName,
		"ns":             appsetNamespace,
	}).Debugf("Invoking GitHub API")

	startTime := time.Now()
	resp, err := t.transport.RoundTrip(req)
	duration := time.Since(startTime)

	// Record metrics
	t.metrics.RequestDuration.WithLabelValues(method, endpoint, appsetNamespace, appsetName).Observe(duration.Seconds())

	status := "0"
	if resp != nil {
		status = strconv.Itoa(resp.StatusCode)
	}
	t.metrics.RequestTotal.WithLabelValues(method, endpoint, status, appsetNamespace, appsetName).Inc()

	if resp != nil {
		resetHumanReadableTime := ""
		remainingInt := 0
		limitInt := 0
		usedInt := 0
		resource := ""

		// Record rate limit metrics if available
		if resetTime := resp.Header.Get("X-RateLimit-Reset"); resetTime != "" {
			if resetUnix, err := strconv.ParseInt(resetTime, 10, 64); err == nil {
				t.metrics.RateLimitReset.WithLabelValues(endpoint, appsetNamespace, appsetName).Set(float64(resetUnix))
				resetHumanReadableTime = time.Unix(resetUnix, 0).Local().Format("2006-01-02 15:04:05 MST")

			}
		}
		if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
			if remainingInt, err = strconv.Atoi(remaining); err == nil {
				t.metrics.RateLimitRemaining.WithLabelValues(endpoint, appsetNamespace, appsetName).Set(float64(remainingInt))
			}
		}
		if limit := resp.Header.Get("X-RateLimit-Limit"); limit != "" {
			if limitInt, err = strconv.Atoi(limit); err == nil {
				t.metrics.RateLimitLimit.WithLabelValues(endpoint, appsetNamespace, appsetName).Set(float64(limitInt))
			}
		}
		if used := resp.Header.Get("X-RateLimit-Used"); used != "" {
			if usedInt, err = strconv.Atoi(used); err == nil {
				t.metrics.RateLimitUsed.WithLabelValues(endpoint, appsetNamespace, appsetName).Set(float64(usedInt))
			}
		}
		if resource = resp.Header.Get("X-RateLimit-Resource"); resource != "" {
			t.metrics.RateLimitResource.WithLabelValues(endpoint, resource, appsetNamespace, appsetName).Set(1)
		}

		log.WithFields(log.Fields{
			"endpoint":       endpoint,
			"reset":          resetHumanReadableTime,
			"remaining":      remainingInt,
			"limit":          limitInt,
			"used":           usedInt,
			"resource":       resource,
			"applicationset": appsetName,
			"ns":             appsetNamespace,
		}).Debugf("GitHub API rate limit info")
	}

	return resp, err
}

// Full constructor (for tests and advanced use)
func NewGitHubMetricsTransport(
	transport http.RoundTripper,
	metricsContext *MetricsContext,
	metrics *GitHubMetrics,
) *GitHubMetricsTransport {
	return &GitHubMetricsTransport{
		transport:      transport,
		metricsContext: metricsContext,
		metrics:        metrics,
	}
}

// Default constructor
func NewDefaultGitHubMetricsTransport(transport http.RoundTripper, metricsContext *MetricsContext) *GitHubMetricsTransport {
	return NewGitHubMetricsTransport(
		transport,
		metricsContext,
		globalGitHubMetrics,
	)
}

// NewGitHubMetricsClient wraps an http.Client with metrics middleware
func NewGitHubMetricsClient(metricsContext *MetricsContext) *http.Client {
	log.Debug("Creating new GitHub metrics client")
	return &http.Client{
		Transport: NewDefaultGitHubMetricsTransport(http.DefaultTransport, metricsContext),
	}
}
