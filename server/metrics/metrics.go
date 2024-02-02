package metrics

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/argoproj/argo-cd/v2/util/profile"
)

type MetricsServer struct {
	*http.Server
	redisRequestCounter      *prometheus.CounterVec
	redisRequestHistogram    *prometheus.HistogramVec
	extensionRequestCounter  *prometheus.CounterVec
	extensionRequestDuration *prometheus.HistogramVec
}

var (
	redisRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_redis_request_total",
			Help: "Number of kubernetes requests executed during application reconciliation.",
		},
		[]string{"initiator", "failed"},
	)
	redisRequestHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "argocd_redis_request_duration",
			Help:    "Redis requests duration.",
			Buckets: []float64{0.1, 0.25, .5, 1, 2},
		},
		[]string{"initiator"},
	)
	extensionRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_proxy_extension_request_total",
			Help: "Number of requests sent to configured proxy extensions.",
		},
		[]string{"extension", "status"},
	)
	extensionRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "argocd_proxy_extension_request_duration_seconds",
			Help:    "Request duration in seconds between the Argo CD API server and the extension backend.",
			Buckets: []float64{0.1, 0.25, .5, 1, 2, 5, 10},
		},
		[]string{"extension"},
	)
)

// NewMetricsServer returns a new prometheus server which collects api server metrics
func NewMetricsServer(host string, port int) *MetricsServer {
	mux := http.NewServeMux()
	registry := prometheus.NewRegistry()
	mux.Handle("/metrics", promhttp.HandlerFor(prometheus.Gatherers{
		registry,
		prometheus.DefaultGatherer,
	}, promhttp.HandlerOpts{}))
	profile.RegisterProfiler(mux)

	registry.MustRegister(redisRequestCounter)
	registry.MustRegister(redisRequestHistogram)
	registry.MustRegister(extensionRequestCounter)
	registry.MustRegister(extensionRequestDuration)

	return &MetricsServer{
		Server: &http.Server{
			Addr:    fmt.Sprintf("%s:%d", host, port),
			Handler: mux,
		},
		redisRequestCounter:      redisRequestCounter,
		redisRequestHistogram:    redisRequestHistogram,
		extensionRequestCounter:  extensionRequestCounter,
		extensionRequestDuration: extensionRequestDuration,
	}
}

func (m *MetricsServer) IncRedisRequest(failed bool) {
	m.redisRequestCounter.WithLabelValues("argocd-server", strconv.FormatBool(failed)).Inc()
}

// ObserveRedisRequestDuration observes redis request duration
func (m *MetricsServer) ObserveRedisRequestDuration(duration time.Duration) {
	m.redisRequestHistogram.WithLabelValues("argocd-server").Observe(duration.Seconds())
}

func (m *MetricsServer) IncExtensionRequestCounter(extension string, status int) {
	m.extensionRequestCounter.WithLabelValues(extension, strconv.Itoa(status)).Inc()
}

func (m *MetricsServer) ObserveExtensionRequestDuration(extension string, duration time.Duration) {
	m.extensionRequestDuration.WithLabelValues(extension).Observe(duration.Seconds())
}
