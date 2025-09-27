package services

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
)

type Metric struct {
	name   string
	labels []string
	value  string
}

var (
	endpointLabel        = "endpoint=\"/api/test\""
	URL                  = "/api/test"
	appsetNamespaceLabel = "appset_namespace=\"test-ns\""
	appsetNamespace      = "test-ns"
	appsetName           = "test-appset"
	appsetNameLabel      = "appset_name=\"test-appset\""
	resourceLabel        = "resource=\"core\""

	rateLimitMetrics = []Metric{
		{
			name:   githubAPIRateLimitRemainingMetricName,
			labels: []string{endpointLabel, appsetNamespaceLabel, appsetNameLabel, resourceLabel},
			value:  "42",
		},
		{
			name:   githubAPIRateLimitLimitMetricName,
			labels: []string{endpointLabel, appsetNamespaceLabel, appsetNameLabel, resourceLabel},
			value:  "100",
		},
		{
			name:   githubAPIRateLimitUsedMetricName,
			labels: []string{endpointLabel, appsetNamespaceLabel, appsetNameLabel, resourceLabel},
			value:  "58",
		},
		{
			name:   githubAPIRateLimitResetMetricName,
			labels: []string{endpointLabel, appsetNamespaceLabel, appsetNameLabel, resourceLabel},
			value:  "1",
		},
	}
	successRequestMetrics = Metric{
		name:   githubAPIRequestTotalMetricName,
		labels: []string{"method=\"GET\"", endpointLabel, "status=\"201\"", appsetNamespaceLabel, appsetNameLabel},
		value:  "1",
	}
	failureRequestMetrics = Metric{
		name:   githubAPIRequestTotalMetricName,
		labels: []string{"method=\"GET\"", endpointLabel, "status=\"0\"", appsetNamespaceLabel, appsetNameLabel},
		value:  "1",
	}
)

func TestGitHubMetrics_CollectorApproach_Success(t *testing.T) {
	metrics := NewGitHubMetrics()
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		metrics.RequestTotal,
		metrics.RequestDuration,
		metrics.RateLimitRemaining,
		metrics.RateLimitLimit,
		metrics.RateLimitReset,
		metrics.RateLimitUsed,
	)

	// Setup a fake HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Unix()+1, 10))
		w.Header().Set("X-RateLimit-Remaining", "42")
		w.Header().Set("X-RateLimit-Limit", "100")
		w.Header().Set("X-RateLimit-Used", "58")
		w.Header().Set("X-RateLimit-Resource", "core")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	metricsCtx := &MetricsContext{AppSetNamespace: appsetNamespace, AppSetName: appsetName}
	client := &http.Client{
		Transport: NewGitHubMetricsTransport(
			http.DefaultTransport,
			metricsCtx,
			metrics,
		),
	}

	req, _ := http.NewRequest(http.MethodGet, ts.URL+URL, http.NoBody)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	// Expose and scrape metrics
	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err = http.Get(server.URL)
	if err != nil {
		t.Fatalf("failed to scrape metrics: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	metricsOutput := string(body)

	sort.Strings(successRequestMetrics.labels)
	assert.Contains(t, metricsOutput, successRequestMetrics.name+"{"+strings.Join(successRequestMetrics.labels, ",")+"} "+successRequestMetrics.value)

	for _, metric := range rateLimitMetrics {
		sort.Strings(metric.labels)
		assert.Contains(t, metricsOutput, metric.name+"{"+strings.Join(metric.labels, ",")+"} "+metric.value)
	}
}

type RoundTripperFunc func(*http.Request) (*http.Response, error)

func (f RoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGitHubMetrics_CollectorApproach_NoRateLimitMetricsOnNilResponse(t *testing.T) {
	metrics := NewGitHubMetrics()
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		metrics.RequestTotal,
		metrics.RequestDuration,
		metrics.RateLimitRemaining,
		metrics.RateLimitLimit,
		metrics.RateLimitReset,
		metrics.RateLimitUsed,
	)

	client := &http.Client{
		Transport: &GitHubMetricsTransport{
			transport: RoundTripperFunc(func(*http.Request) (*http.Response, error) {
				return nil, http.ErrServerClosed
			}),
			metricsContext: &MetricsContext{AppSetNamespace: appsetNamespace, AppSetName: appsetName},
			metrics:        metrics,
		},
	}

	req, _ := http.NewRequest(http.MethodGet, URL, http.NoBody)
	_, _ = client.Do(req)

	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("failed to scrape metrics: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	metricsOutput := string(body)

	// Verify request metric exists with status "0"
	sort.Strings(failureRequestMetrics.labels)
	assert.Contains(t, metricsOutput, failureRequestMetrics.name+"{"+strings.Join(failureRequestMetrics.labels, ",")+"} "+failureRequestMetrics.value)

	// Verify rate limit metrics don't exist
	for _, metric := range rateLimitMetrics {
		sort.Strings(metric.labels)
		assert.NotContains(t, metricsOutput, metric.name+"{"+strings.Join(metric.labels, ",")+"} "+metric.value)
	}
}

func TestNewGitHubMetricsClient(t *testing.T) {
	// Test cases
	testCases := []struct {
		name       string
		metricsCtx *MetricsContext
	}{
		{
			name: "with metrics context",
			metricsCtx: &MetricsContext{
				AppSetNamespace: appsetNamespace,
				AppSetName:      appsetName,
			},
		},
		{
			name:       "with nil metrics context",
			metricsCtx: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create client
			client := NewGitHubMetricsClient(tc.metricsCtx)

			// Assert client is not nil
			assert.NotNil(t, client)

			// Assert transport is properly configured
			transport, ok := client.Transport.(*GitHubMetricsTransport)
			assert.True(t, ok, "Transport should be GitHubMetricsTransport")

			// Verify transport configuration
			assert.Equal(t, tc.metricsCtx, transport.metricsContext)
			assert.NotNil(t, transport.metrics, "Metrics should not be nil")
			assert.Equal(t, http.DefaultTransport, transport.transport, "Base transport should be http.DefaultTransport")

			// Verify metrics are global metrics
			assert.Equal(t, globalGitHubMetrics, transport.metrics, "Should use global metrics")
		})
	}
}
