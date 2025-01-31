package metrics

import (
	"context"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/metrics"
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
	}, []string{"host", "verb"})

	resolverLatencyHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "argocd_kubectl_dns_resolution_duration_seconds",
			Help:    "Kubectl resolver latency",
			Buckets: []float64{0.005, 0.1, 0.5, 2.0, 8.0, 30.0},
		},
		[]string{"host"},
	)

	requestSizeHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "argocd_kubectl_request_size_bytes",
			Help: "Size of kubectl requests",
			// 64 bytes to 16MB
			Buckets: []float64{64, 512, 4096, 65536, 1048576, 16777216},
		},
		[]string{"host", "method"},
	)

	responseSizeHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "argocd_kubectl_response_size_bytes",
			Help: "Size of kubectl responses",
			// 64 bytes to 16MB
			Buckets: []float64{64, 512, 4096, 65536, 1048576, 16777216},
		},
		[]string{"host", "method"},
	)

	rateLimiterLatencyHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "argocd_kubectl_rate_limiter_duration_seconds",
			Help:    "Kubectl rate limiter latency",
			Buckets: []float64{0.005, 0.1, 0.5, 2.0, 8.0, 30.0},
		},
		[]string{"host", "verb"},
	)

	requestResultCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_kubectl_requests_total",
			Help: "Number of kubectl request results",
		},
		[]string{"host", "method", "code"},
	)

	execPluginCallsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_kubectl_exec_plugin_call_total",
			Help: "Number of kubectl exec plugin calls",
		},
		[]string{"code", "call_status"},
	)

	requestRetryCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_kubectl_request_retries_total",
			Help: "Number of kubectl request retries",
		},
		[]string{"host", "method", "code"},
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
		[]string{"result"},
	)
)

var alreadyRegistered = false

// RegisterWithPrometheus registers the kubectl metrics with the given prometheus registry. Since the metrics are
// global, this function should only be called once for a given Argo CD component.
func RegisterWithPrometheus(registry prometheus.Registerer) {
	if alreadyRegistered {
		panic("kubectl metrics already registered")
	}

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

	alreadyRegistered = true
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

type KubectlMetrics struct {
	clientCertRotationAgeMetric kubectlClientCertRotationAgeMetric
	requestLatencyMetric        kubectlRequestLatencyMetric
	resolverLatencyMetric       kubectlResolverLatencyMetric
	requestSizeMetric           kubectlRequestSizeMetric
	responseSizeMetric          kubectlResponseSizeMetric
	rateLimiterLatencyMetric    kubectlRateLimiterLatencyMetric
	requestResultMetric         kubectlRequestResultMetric
	execPluginCallsMetric       kubectlExecPluginCallsMetric
	requestRetryMetric          kubectlRequestRetryMetric
	transportCacheEntriesMetric kubectlTransportCacheEntriesMetric
	transportCreateCallsMetric  kubectlTransportCreateCallsMetric
}

// NewKubectlMetrics returns a new KubectlMetrics instance with the given initiator, which should be the name of the
// Argo CD component that is producing the metrics.
//
// After initializing the KubectlMetrics instance, you must call RegisterWithClientGo to register the metrics with the
// client-go metrics library.
//
// You must also call RegisterWithPrometheus to register the metrics with the metrics server's prometheus registry.
//
// So these three lines should be enough to set up kubectl metrics in your metrics server:
//
//	kubectlMetricsServer := metricsutil.NewKubectlMetrics("your-component-name")
//	kubectlMetricsServer.RegisterWithClientGo()
//	metricsutil.RegisterWithPrometheus(registry)
//
// Once those functions have been called, everything else should happen automatically. client-go will send observations
// to the handlers in this struct, and your metrics server will collect and expose the metrics.
func NewKubectlMetrics() *KubectlMetrics {
	return &KubectlMetrics{
		clientCertRotationAgeMetric: kubectlClientCertRotationAgeMetric{},
		requestLatencyMetric:        kubectlRequestLatencyMetric{},
		resolverLatencyMetric:       kubectlResolverLatencyMetric{},
		requestSizeMetric:           kubectlRequestSizeMetric{},
		responseSizeMetric:          kubectlResponseSizeMetric{},
		rateLimiterLatencyMetric:    kubectlRateLimiterLatencyMetric{},
		requestResultMetric:         kubectlRequestResultMetric{},
		execPluginCallsMetric:       kubectlExecPluginCallsMetric{},
		requestRetryMetric:          kubectlRequestRetryMetric{},
		transportCacheEntriesMetric: kubectlTransportCacheEntriesMetric{},
		transportCreateCallsMetric:  kubectlTransportCreateCallsMetric{},
	}
}

// RegisterWithClientGo sets the metrics handlers for the go-client library. We do not use the metrics library's `RegisterWithClientGo` method,
// because it is protected by a sync.Once. controller-runtime registers a single handler, which blocks our registration
// of our own handlers. So we must rudely set them all directly.
func (k *KubectlMetrics) RegisterWithClientGo() {
	metrics.ClientCertRotationAge = &k.clientCertRotationAgeMetric
	metrics.RequestLatency = &k.requestLatencyMetric
	metrics.ResolverLatency = &k.resolverLatencyMetric
	metrics.RequestSize = &k.requestSizeMetric
	metrics.ResponseSize = &k.responseSizeMetric
	metrics.RateLimiterLatency = &k.rateLimiterLatencyMetric
	metrics.RequestResult = &k.requestResultMetric
	metrics.ExecPluginCalls = &k.execPluginCallsMetric
	metrics.RequestRetry = &k.requestRetryMetric
	metrics.TransportCacheEntries = &k.transportCacheEntriesMetric
	metrics.TransportCreateCalls = &k.transportCreateCallsMetric
}

/**
Here we define a bunch of structs that implement the client-go metrics interfaces. Each struct has an "initiator" field
that is set to the name of the Argo CD component that is producing the metrics. We set the "initiator" label of each
metric to the value of this field.
*/

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

const findPathRegex = `/v\d\w*?(/[a-zA-Z0-9-]*)(/[a-zA-Z0-9-]*)?(/[a-zA-Z0-9-]*)?(/[a-zA-Z0-9-]*)?`

var (
	processPath = regexp.MustCompile(findPathRegex)
)

func resolveK8sRequestVerb(u url.URL, method string) string {
	if method == "POST" {
		return "Create"
	}
	if method == "DELETE" {
		return "Delete"
	}
	if method == "PATCH" {
		return "Patch"
	}
	if method == "PUT" {
		return "Update"
	}
	if method == "GET" {
		return discernGetRequest(u)
	}
	return "Unknown"
}

// discernGetRequest uses a path from a request to determine if the request is a GET, LIST, or WATCH.
// The function tries to find an API version within the path and then calculates how many remaining
// segments are after the API version. A LIST/WATCH request has segments for the kind with a
// namespace and the specific namespace if the kind is a namespaced resource. Meanwhile a GET
// request has an additional segment for resource name. As a result, a LIST/WATCH has an odd number
// of segments while a GET request has an even number of segments. Watch is determined if the query
// parameter watch=true is present in the request.
func discernGetRequest(u url.URL) string {
	segments := processPath.FindStringSubmatch(u.Path)
	unusedGroup := 0
	for _, str := range segments {
		if str == "" {
			unusedGroup++
		}
	}
	if unusedGroup%2 == 1 {
		if watchQueryParamValues, ok := u.Query()["watch"]; ok {
			if len(watchQueryParamValues) > 0 && watchQueryParamValues[0] == "true" {
				return "Watch"
			}
		}
		return "List"
	}
	return "Get"
}
