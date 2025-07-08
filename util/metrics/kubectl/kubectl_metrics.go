package kubectl

import (
	"context"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/metrics"
)

// The label names are meant to match this: https://github.com/kubernetes/component-base/blob/264c1fd30132a3b36b7588e50ac54eb0ff75f26a/metrics/prometheus/restclient/metrics.go
// Even in cases where the label name doesn't align well with Argo CD's other labels, we use the Kubernetes labels to
// make it easier to copy/paste dashboards/alerts/etc. designed for Kubernetes.
const (
	// LabelCallStatus represents the status of the exec plugin call, indicating whether it was successful or failed.
	// These are the possible values, as of the current client-go version:
	// no_error, plugin_execution_error, plugin_not_found_error, client_internal_error
	LabelCallStatus = "call_status"
	// LabelCode represents either the HTTP status code returned by the request or the exit code of the command run.
	LabelCode = "code"
	// LabelHost represents the hostname of the server to which the request was made.
	LabelHost = "host"
	// LabelMethod represents the HTTP method used for the request (e.g., GET, POST).
	LabelMethod = "method"
	// LabelResult represents an attempt to get a transport from the transport cache.
	// These are the possible values, as of the current client-go version: hit, miss, unreachable
	// `unreachable` indicates that the cache was not usable for a given REST config because, for example, TLS files
	// couldn't be loaded, or a proxy is being used.
	LabelResult = "result"
	// LabelVerb represents the Kubernetes API verb used in the request (e.g., list, get, create).
	LabelVerb = "verb"
)

// All metric names below match https://github.com/kubernetes/component-base/blob/264c1fd30132a3b36b7588e50ac54eb0ff75f26a/metrics/prometheus/restclient/metrics.go
// except rest_client_ is replaced with argocd_kubectl_.
//
// We use similar histogram bucket ranges, but reduce cardinality.
//
// We try to use similar labels, but we adjust to more closely match other Argo CD metrics.
//
// The idea is that if we stay close to the Kubernetes metrics, then people can take more advantage of copy/pasting
// dashboards/alerts/etc. designed for Kubernetes.
var (
	clientCertRotationAgeGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "argocd_kubectl_client_cert_rotation_age_seconds",
		Help: "Age of a certificate that has just been rotated",
	}, []string{})

	requestLatencyHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "argocd_kubectl_request_duration_seconds",
		Help:    "Request latency in seconds",
		Buckets: []float64{0.005, 0.1, 0.5, 2.0, 8.0, 30.0},
	}, []string{LabelHost, LabelVerb})

	resolverLatencyHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "argocd_kubectl_dns_resolution_duration_seconds",
			Help:    "Kubectl resolver latency",
			Buckets: []float64{0.005, 0.1, 0.5, 2.0, 8.0, 30.0},
		},
		[]string{LabelHost},
	)

	requestSizeHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "argocd_kubectl_request_size_bytes",
			Help: "Size of kubectl requests",
			// 64 bytes to 16MB
			Buckets: []float64{64, 512, 4096, 65536, 1048576, 16777216},
		},
		[]string{LabelHost, LabelMethod},
	)

	responseSizeHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "argocd_kubectl_response_size_bytes",
			Help: "Size of kubectl responses",
			// 64 bytes to 16MB
			Buckets: []float64{64, 512, 4096, 65536, 1048576, 16777216},
		},
		[]string{LabelHost, LabelMethod},
	)

	rateLimiterLatencyHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "argocd_kubectl_rate_limiter_duration_seconds",
			Help:    "Kubectl rate limiter latency",
			Buckets: []float64{0.005, 0.1, 0.5, 2.0, 8.0, 30.0},
		},
		[]string{LabelHost, LabelVerb},
	)

	requestResultCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_kubectl_requests_total",
			Help: "Number of kubectl request results",
		},
		[]string{LabelHost, LabelMethod, LabelCode},
	)

	execPluginCallsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_kubectl_exec_plugin_call_total",
			Help: "Number of kubectl exec plugin calls",
		},
		[]string{LabelCode, LabelCallStatus},
	)

	requestRetryCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_kubectl_request_retries_total",
			Help: "Number of kubectl request retries",
		},
		[]string{LabelHost, LabelMethod, LabelCode},
	)

	transportCacheEntriesGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "argocd_kubectl_transport_cache_entries",
			Help: "Number of kubectl transport cache entries",
		},
		[]string{},
	)

	transportCreateCallsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_kubectl_transport_create_calls_total",
			Help: "Number of kubectl transport create calls",
		},
		[]string{LabelResult},
	)
)

// RegisterWithPrometheus registers the kubectl metrics with the given prometheus registry.
func RegisterWithPrometheus(registry prometheus.Registerer) {
	registry.MustRegister(clientCertRotationAgeGauge)
	registry.MustRegister(requestLatencyHistogram)
	registry.MustRegister(resolverLatencyHistogram)
	registry.MustRegister(requestSizeHistogram)
	registry.MustRegister(responseSizeHistogram)
	registry.MustRegister(rateLimiterLatencyHistogram)
	registry.MustRegister(requestResultCounter)
	registry.MustRegister(execPluginCallsCounter)
	registry.MustRegister(requestRetryCounter)
	registry.MustRegister(transportCacheEntriesGauge)
	registry.MustRegister(transportCreateCallsCounter)
}

// ResetAll resets all kubectl metrics
func ResetAll() {
	clientCertRotationAgeGauge.Reset()
	requestLatencyHistogram.Reset()
	resolverLatencyHistogram.Reset()
	requestSizeHistogram.Reset()
	responseSizeHistogram.Reset()
	rateLimiterLatencyHistogram.Reset()
	requestResultCounter.Reset()
	execPluginCallsCounter.Reset()
	requestRetryCounter.Reset()
	transportCacheEntriesGauge.Reset()
	transportCreateCallsCounter.Reset()
}

var newKubectlMetricsOnce sync.Once

// RegisterWithClientGo sets the metrics handlers for the go-client library. We do not use the metrics library's `RegisterWithClientGo` method,
// because it is protected by a sync.Once. controller-runtime registers a single handler, which blocks our registration
// of our own handlers. So we must rudely set them all directly.
//
// Since the metrics are global, this function only needs to be called once for a given Argo CD component.
//
// You must also call RegisterWithPrometheus to register the metrics with the metrics server's prometheus registry.
func RegisterWithClientGo() {
	// Do once to avoid races in unit tests that call this function.
	newKubectlMetricsOnce.Do(func() {
		metrics.ClientCertRotationAge = &kubectlClientCertRotationAgeMetric{}
		metrics.RequestLatency = &kubectlRequestLatencyMetric{}
		metrics.ResolverLatency = &kubectlResolverLatencyMetric{}
		metrics.RequestSize = &kubectlRequestSizeMetric{}
		metrics.ResponseSize = &kubectlResponseSizeMetric{}
		metrics.RateLimiterLatency = &kubectlRateLimiterLatencyMetric{}
		metrics.RequestResult = &kubectlRequestResultMetric{}
		metrics.ExecPluginCalls = &kubectlExecPluginCallsMetric{}
		metrics.RequestRetry = &kubectlRequestRetryMetric{}
		metrics.TransportCacheEntries = &kubectlTransportCacheEntriesMetric{}
		metrics.TransportCreateCalls = &kubectlTransportCreateCallsMetric{}
	})
}

type kubectlClientCertRotationAgeMetric struct{}

func (k *kubectlClientCertRotationAgeMetric) Observe(certDuration time.Duration) {
	clientCertRotationAgeGauge.WithLabelValues().Set(certDuration.Seconds())
}

type kubectlRequestLatencyMetric struct{}

func (k *kubectlRequestLatencyMetric) Observe(_ context.Context, verb string, u url.URL, latency time.Duration) {
	k8sVerb := resolveK8sRequestVerb(u, verb)
	requestLatencyHistogram.WithLabelValues(u.Host, k8sVerb).Observe(latency.Seconds())
}

type kubectlResolverLatencyMetric struct{}

func (k *kubectlResolverLatencyMetric) Observe(_ context.Context, host string, latency time.Duration) {
	resolverLatencyHistogram.WithLabelValues(host).Observe(latency.Seconds())
}

type kubectlRequestSizeMetric struct{}

func (k *kubectlRequestSizeMetric) Observe(_ context.Context, verb string, host string, size float64) {
	requestSizeHistogram.WithLabelValues(host, verb).Observe(size)
}

type kubectlResponseSizeMetric struct{}

func (k *kubectlResponseSizeMetric) Observe(_ context.Context, verb string, host string, size float64) {
	responseSizeHistogram.WithLabelValues(host, verb).Observe(size)
}

type kubectlRateLimiterLatencyMetric struct{}

func (k *kubectlRateLimiterLatencyMetric) Observe(_ context.Context, verb string, u url.URL, latency time.Duration) {
	k8sVerb := resolveK8sRequestVerb(u, verb)
	rateLimiterLatencyHistogram.WithLabelValues(u.Host, k8sVerb).Observe(latency.Seconds())
}

type kubectlRequestResultMetric struct{}

func (k *kubectlRequestResultMetric) Increment(_ context.Context, code string, method string, host string) {
	requestResultCounter.WithLabelValues(host, method, code).Inc()
}

type kubectlExecPluginCallsMetric struct{}

func (k *kubectlExecPluginCallsMetric) Increment(exitCode int, callStatus string) {
	execPluginCallsCounter.WithLabelValues(strconv.Itoa(exitCode), callStatus).Inc()
}

type kubectlRequestRetryMetric struct{}

func (k *kubectlRequestRetryMetric) IncrementRetry(_ context.Context, code string, method string, host string) {
	requestRetryCounter.WithLabelValues(host, method, code).Inc()
}

type kubectlTransportCacheEntriesMetric struct{}

func (k *kubectlTransportCacheEntriesMetric) Observe(value int) {
	transportCacheEntriesGauge.WithLabelValues().Set(float64(value))
}

type kubectlTransportCreateCallsMetric struct{}

func (k *kubectlTransportCreateCallsMetric) Increment(result string) {
	transportCreateCallsCounter.WithLabelValues(result).Inc()
}
