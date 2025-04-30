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

	rateLimitMetrics = []Metric{
		{
			name:   githubAPIRateLimitRemainingPerAppSetMetricName,
			labels: []string{endpointLabel, appsetNamespaceLabel, appsetNameLabel},
			value:  "42",
		},
		{
			name:   githubAPIRateLimitLimitPerAppSetMetricName,
			labels: []string{endpointLabel, appsetNamespaceLabel, appsetNameLabel},
			value:  "100",
		},
		{
			name:   githubAPIRateLimitUsedPerAppSetMetricName,
			labels: []string{endpointLabel, appsetNamespaceLabel, appsetNameLabel},
			value:  "58",
		},
		{
			name:   githubAPIRateLimitResourcePerAppSetMetricName,
			labels: []string{endpointLabel, "resource=\"core\"", appsetNamespaceLabel, appsetNameLabel},
			value:  "1",
		},
		{
			name:   githubAPIRateLimitResetPerAppSetMetricName,
			labels: []string{endpointLabel, appsetNamespaceLabel, appsetNameLabel},
			value:  "1",
		},
	}
	successRequestMetrics = Metric{
		name:   githubAPIRequestTotalPerAppSetMetricName,
		labels: []string{"method=\"GET\"", endpointLabel, "status=\"201\"", appsetNamespaceLabel, appsetNameLabel},
		value:  "1",
	}
	failureRequestMetrics = Metric{
		name:   githubAPIRequestTotalPerAppSetMetricName,
		labels: []string{"method=\"GET\"", endpointLabel, "status=\"0\"", appsetNamespaceLabel, appsetNameLabel},
		value:  "1",
	}
)

// Helper to register all metrics with a custom registry
func registerGitHubMetrics(reg *prometheus.Registry) {
	reg.MustRegister(githubAPIRequestTotalPerAppSet)
	reg.MustRegister(githubAPIRequestDurationPerAppSet)
	reg.MustRegister(githubAPIRateLimitRemainingPerAppSet)
	reg.MustRegister(githubAPIRateLimitLimitPerAppSet)
	reg.MustRegister(githubAPIRateLimitResetPerAppSet)
	reg.MustRegister(githubAPIRateLimitUsedPerAppSet)
	reg.MustRegister(githubAPIRateLimitResourcePerAppSet)
}

func TestGitHubMetrics_CollectorApproach_Success(t *testing.T) {

	// Setup a fake HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(1*time.Hour).Unix(), 10))
		w.Header().Set("X-RateLimit-Remaining", "42")
		w.Header().Set("X-RateLimit-Limit", "100")
		w.Header().Set("X-RateLimit-Used", "58")
		w.Header().Set("X-RateLimit-Resource", "core")
		w.WriteHeader(201)
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	metricsCtx := &MetricsContext{AppSetNamespace: appsetNamespace, AppSetName: appsetName}
	client := NewGitHubMetricsClient(metricsCtx)

	req, _ := http.NewRequest("GET", ts.URL+URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	// Setup a custom registry and register metrics
	reg := prometheus.NewRegistry()
	registerGitHubMetrics(reg)

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
	reg := prometheus.NewRegistry()
	registerGitHubMetrics(reg)

	client := &http.Client{
		Transport: &GitHubMetricsTransport{
			transport: RoundTripperFunc(func(*http.Request) (*http.Response, error) {
				return nil, http.ErrServerClosed
			}),
			metricsContext: &MetricsContext{AppSetNamespace: appsetNamespace, AppSetName: appsetName},
		},
	}

	req, _ := http.NewRequest("GET", URL, nil)
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
