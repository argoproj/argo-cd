package services

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	githubAPIRequestTotalPerAppSetMetricName       = "argocd_github_api_requests_total_per_appset"
	githubAPIRequestDurationPerAppSetMetricName    = "argocd_github_api_request_duration_seconds_per_appset"
	githubAPIRateLimitRemainingPerAppSetMetricName = "argocd_github_api_rate_limit_remaining_per_appset"
	githubAPIRateLimitLimitPerAppSetMetricName     = "argocd_github_api_rate_limit_limit_per_appset"
	githubAPIRateLimitResetPerAppSetMetricName     = "argocd_github_api_rate_limit_reset_per_appset"
	githubAPIRateLimitUsedPerAppSetMetricName      = "argocd_github_api_rate_limit_used_per_appset"
	githubAPIRateLimitResourcePerAppSetMetricName  = "argocd_github_api_rate_limit_resource_per_appset"

	githubAPIRequestTotalPerAppSet = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: githubAPIRequestTotalPerAppSetMetricName,
			Help: "Total number of GitHub API requests per ApplicationSet",
		},
		[]string{"method", "endpoint", "status", "appset_namespace", "appset_name"},
	)
	githubAPIRequestDurationPerAppSet = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    githubAPIRequestDurationPerAppSetMetricName,
			Help:    "GitHub API request duration in seconds, per ApplicationSet",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint", "appset_namespace", "appset_name"},
	)

	githubAPIRateLimitRemainingPerAppSet = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: githubAPIRateLimitRemainingPerAppSetMetricName,
			Help: "The number of requests remaining in the current rate limit window, per ApplicationSet",
		},
		[]string{"endpoint", "appset_namespace", "appset_name"},
	)
	githubAPIRateLimitLimitPerAppSet = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: githubAPIRateLimitLimitPerAppSetMetricName,
			Help: "The maximum number of requests that you can make per hour, per ApplicationSet",
		},
		[]string{"endpoint", "appset_namespace", "appset_name"},
	)
	githubAPIRateLimitResetPerAppSet = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: githubAPIRateLimitResetPerAppSetMetricName,
			Help: "The time at which the current rate limit window resets, in UTC epoch seconds, per ApplicationSet",
		},
		[]string{"endpoint", "appset_namespace", "appset_name"},
	)
	githubAPIRateLimitUsedPerAppSet = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: githubAPIRateLimitUsedPerAppSetMetricName,
			Help: "The number of requests used in the current rate limit window, per ApplicationSet",
		},
		[]string{"endpoint", "appset_namespace", "appset_name"},
	)
	githubAPIRateLimitResourcePerAppSet = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: githubAPIRateLimitResourcePerAppSetMetricName,
			Help: "The rate limit resource that the request counted against, per ApplicationSet",
		},
		[]string{"endpoint", "resource", "appset_namespace", "appset_name"},
	)
)

func init() {
	log.Info("Registering GitHub API AppSet metrics")

	metrics.Registry.MustRegister(githubAPIRequestTotalPerAppSet)
	metrics.Registry.MustRegister(githubAPIRequestDurationPerAppSet)
	metrics.Registry.MustRegister(githubAPIRateLimitRemainingPerAppSet)
	metrics.Registry.MustRegister(githubAPIRateLimitLimitPerAppSet)
	metrics.Registry.MustRegister(githubAPIRateLimitResetPerAppSet)
	metrics.Registry.MustRegister(githubAPIRateLimitUsedPerAppSet)
	metrics.Registry.MustRegister(githubAPIRateLimitResourcePerAppSet)
}

type MetricsContext struct {
	AppSetNamespace string
	AppSetName      string
}

// GitHubMetricsTransport is a custom http.RoundTripper that collects GitHub API metrics
type GitHubMetricsTransport struct {
	transport      http.RoundTripper
	metricsContext *MetricsContext
}

// RoundTrip implements http.RoundTripper interface and collects metrics
func (t *GitHubMetricsTransport) RoundTrip(req *http.Request) (*http.Response, error) {

	// Extract endpoint from URL path
	endpoint := req.URL.Path

	startTime := time.Now()
	// Execute the actual request
	resp, err := t.transport.RoundTrip(req)

	// Calculate duration
	duration := time.Since(startTime)

	// We want the GitHub API metrics to be collected anyway, even if they can't be associated with an ApplicationSet
	appsetNamespace := "unknown"
	appsetName := "unknown"
	if t.metricsContext != nil {
		appsetNamespace = t.metricsContext.AppSetNamespace
		appsetName = t.metricsContext.AppSetName
	}

	// Record metrics
	githubAPIRequestDurationPerAppSet.WithLabelValues(req.Method, endpoint, appsetNamespace, appsetName).Observe(float64(duration.Seconds()))

	status := "0"
	if resp != nil {
		status = strconv.Itoa(resp.StatusCode)
	}
	githubAPIRequestTotalPerAppSet.WithLabelValues(req.Method, endpoint, status, appsetNamespace, appsetName).Inc()

	if resp != nil {
		// Record rate limit metrics if available
		if resetTime := resp.Header.Get("X-RateLimit-Reset"); resetTime != "" {
			if resetUnix, err := strconv.ParseInt(resetTime, 10, 64); err == nil {
				githubAPIRateLimitResetPerAppSet.WithLabelValues(endpoint, appsetNamespace, appsetName).Set(float64(resetUnix))
			}
		}
		if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
			if remainingInt, err := strconv.Atoi(remaining); err == nil {
				githubAPIRateLimitRemainingPerAppSet.WithLabelValues(endpoint, appsetNamespace, appsetName).Set(float64(remainingInt))
			}
		}

		if limit := resp.Header.Get("X-RateLimit-Limit"); limit != "" {
			if limitInt, err := strconv.Atoi(limit); err == nil {
				githubAPIRateLimitLimitPerAppSet.WithLabelValues(endpoint, appsetNamespace, appsetName).Set(float64(limitInt))
			}
		}

		if used := resp.Header.Get("X-RateLimit-Used"); used != "" {
			if usedInt, err := strconv.Atoi(used); err == nil {
				githubAPIRateLimitUsedPerAppSet.WithLabelValues(endpoint, appsetNamespace, appsetName).Set(float64(usedInt))

			}
		}
		if resource := resp.Header.Get("X-RateLimit-Resource"); resource != "" {
			githubAPIRateLimitResourcePerAppSet.WithLabelValues(endpoint, resource, appsetNamespace, appsetName).Set(1)

		}
	}

	return resp, err
}

// NewGitHubMetricsClient wraps an http.Client with metrics middleware
func NewGitHubMetricsClient(metricsContext *MetricsContext) *http.Client {
	log.Info("Creating new GitHub metrics client")
	return &http.Client{
		Transport: &GitHubMetricsTransport{
			transport:      http.DefaultTransport,
			metricsContext: metricsContext,
		},
	}
}
