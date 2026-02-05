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
	githubAPIRequestTotalMetricName       = "argocd_github_api_requests_total"
	githubAPIRequestDurationMetricName    = "argocd_github_api_request_duration_seconds"
	githubAPIRateLimitRemainingMetricName = "argocd_github_api_rate_limit_remaining"
	githubAPIRateLimitLimitMetricName     = "argocd_github_api_rate_limit_limit"
	githubAPIRateLimitResetMetricName     = "argocd_github_api_rate_limit_reset_seconds"
	githubAPIRateLimitUsedMetricName      = "argocd_github_api_rate_limit_used"
)

// GitHubMetrics groups all metric vectors for easier injection and registration
type GitHubMetrics struct {
	RequestTotal       *prometheus.CounterVec
	RequestDuration    *prometheus.HistogramVec
	RateLimitRemaining *prometheus.GaugeVec
	RateLimitLimit     *prometheus.GaugeVec
	RateLimitReset     *prometheus.GaugeVec
	RateLimitUsed      *prometheus.GaugeVec
}

// Factory for a new set of GitHub metrics (for tests or custom registries)
func NewGitHubMetrics() *GitHubMetrics {
	return &GitHubMetrics{
		RequestTotal:       NewGitHubAPIRequestTotal(),
		RequestDuration:    NewGitHubAPIRequestDuration(),
		RateLimitRemaining: NewGitHubAPIRateLimitRemaining(),
		RateLimitLimit:     NewGitHubAPIRateLimitLimit(),
		RateLimitReset:     NewGitHubAPIRateLimitReset(),
		RateLimitUsed:      NewGitHubAPIRateLimitUsed(),
	}
}

// Factory functions for each metric vector
func NewGitHubAPIRequestTotal() *prometheus.CounterVec {
	return prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: githubAPIRequestTotalMetricName,
			Help: "Total number of GitHub API requests",
		},
		[]string{"method", "endpoint", "status", "appset_namespace", "appset_name"},
	)
}

func NewGitHubAPIRequestDuration() *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    githubAPIRequestDurationMetricName,
			Help:    "GitHub API request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint", "appset_namespace", "appset_name"},
	)
}

func NewGitHubAPIRateLimitRemaining() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: githubAPIRateLimitRemainingMetricName,
			Help: "The number of requests remaining in the current rate limit window",
		},
		[]string{"endpoint", "appset_namespace", "appset_name", "resource"},
	)
}

func NewGitHubAPIRateLimitLimit() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: githubAPIRateLimitLimitMetricName,
			Help: "The maximum number of requests that you can make per hour",
		},
		[]string{"endpoint", "appset_namespace", "appset_name", "resource"},
	)
}

func NewGitHubAPIRateLimitReset() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: githubAPIRateLimitResetMetricName,
			Help: "The time left till the current rate limit window resets, in seconds",
		},
		[]string{"endpoint", "appset_namespace", "appset_name", "resource"},
	)
}

func NewGitHubAPIRateLimitUsed() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: githubAPIRateLimitUsedMetricName,
			Help: "The number of requests used in the current rate limit window",
		},
		[]string{"endpoint", "appset_namespace", "appset_name", "resource"},
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
		"applicationset": map[string]string{"name": appsetName, "namespace": appsetNamespace},
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
		resource := resp.Header.Get("X-RateLimit-Resource")

		// Record rate limit metrics if available
		if resetTime := resp.Header.Get("X-RateLimit-Reset"); resetTime != "" {
			if resetUnix, err := strconv.ParseInt(resetTime, 10, 64); err == nil {
				// Calculate seconds until reset (reset timestamp - current time)
				secondsUntilReset := resetUnix - time.Now().Unix()
				t.metrics.RateLimitReset.WithLabelValues(endpoint, appsetNamespace, appsetName, resource).Set(float64(secondsUntilReset))
				resetHumanReadableTime = time.Unix(resetUnix, 0).Local().Format("2006-01-02 15:04:05 MST")
			}
		}
		if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
			if remainingInt, err = strconv.Atoi(remaining); err == nil {
				t.metrics.RateLimitRemaining.WithLabelValues(endpoint, appsetNamespace, appsetName, resource).Set(float64(remainingInt))
			}
		}
		if limit := resp.Header.Get("X-RateLimit-Limit"); limit != "" {
			if limitInt, err = strconv.Atoi(limit); err == nil {
				t.metrics.RateLimitLimit.WithLabelValues(endpoint, appsetNamespace, appsetName, resource).Set(float64(limitInt))
			}
		}
		if used := resp.Header.Get("X-RateLimit-Used"); used != "" {
			if usedInt, err = strconv.Atoi(used); err == nil {
				t.metrics.RateLimitUsed.WithLabelValues(endpoint, appsetNamespace, appsetName, resource).Set(float64(usedInt))
			}
		}

		log.WithFields(log.Fields{
			"endpoint":       endpoint,
			"reset":          resetHumanReadableTime,
			"remaining":      remainingInt,
			"limit":          limitInt,
			"used":           usedInt,
			"resource":       resource,
			"applicationset": map[string]string{"name": appsetName, "namespace": appsetNamespace},
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
