package metrics

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/argoproj/argo-cd/v2/util/profile"
)

type MetricsServer struct {
	*http.Server
	shard                            string
	redisRequestCounter              *prometheus.CounterVec
	redisRequestHistogram            *prometheus.HistogramVec
	queueSizeCounter                 *prometheus.CounterVec
	erroredEventsCounter             *prometheus.CounterVec
	cachedIgnoredEventsCounter       *prometheus.CounterVec
	eventProcessingDurationHistogram *prometheus.HistogramVec
}

type MetricEventType string

const (
	MetricAppEventType       MetricEventType = "app"
	MetricParentAppEventType MetricEventType = "parent_app"
	MetricChildAppEventType  MetricEventType = "child_app"
	MetricResourceEventType  MetricEventType = "resource"
)

type MetricEventErrorType string

const (
	MetricEventDeliveryErrorType   MetricEventErrorType = "delivery"
	MetricEventGetPayloadErrorType MetricEventErrorType = "get_payload"
	MetricEventUnknownErrorType    MetricEventErrorType = "unknown"
)

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
	queueSizeCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cf_e_reporter_queue_size",
			Help: "Size of application events queue of taked shard.",
		},
		[]string{"reporter_shard"},
	)
	erroredEventsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cf_e_reporter_errored_events",
			Help: "Total amount of errored events.",
		},
		[]string{"reporter_shard", "metric_event_type"},
	)
	cachedIgnoredEventsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cf_e_reporter_cached_ignored_events",
			Help: "Total number of ignored events because of cache.",
		},
		[]string{"reporter_shard", "metric_event_type"},
	)
	eventProcessingDurationHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cf_e_reporter_event_processing_duration",
			Help:    "Event processing duration.",
			Buckets: []float64{0.1, 0.25, .5, 1, 2, 3},
		},
		[]string{"reporter_shard", "metric_event_type"},
	)
)

// NewMetricsServer returns a new prometheus server which collects api server metrics
func NewMetricsServer(host string, port int) *MetricsServer {
	mux := http.NewServeMux()
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collectors.NewGoCollector())

	mux.Handle("/metrics", promhttp.HandlerFor(prometheus.Gatherers{
		registry,
		prometheus.DefaultGatherer,
	}, promhttp.HandlerOpts{}))
	profile.RegisterProfiler(mux)

	registry.MustRegister(redisRequestCounter)
	registry.MustRegister(redisRequestHistogram)

	registry.MustRegister(queueSizeCounter)
	registry.MustRegister(erroredEventsCounter)
	registry.MustRegister(cachedIgnoredEventsCounter)
	registry.MustRegister(eventProcessingDurationHistogram)

	return &MetricsServer{
		Server: &http.Server{
			Addr:    fmt.Sprintf("%s:%d", host, port),
			Handler: mux,
		},
		shard:                            strconv.FormatInt(1, 10),
		queueSizeCounter:                 queueSizeCounter,
		erroredEventsCounter:             erroredEventsCounter,
		cachedIgnoredEventsCounter:       cachedIgnoredEventsCounter,
		eventProcessingDurationHistogram: eventProcessingDurationHistogram,
	}
}

func (m *MetricsServer) IncRedisRequest(failed bool) {
	m.redisRequestCounter.WithLabelValues("argocd-server", strconv.FormatBool(failed)).Inc()
}

// ObserveRedisRequestDuration observes redis request duration
func (m *MetricsServer) ObserveRedisRequestDuration(duration time.Duration) {
	m.redisRequestHistogram.WithLabelValues("argocd-server").Observe(duration.Seconds())
}

func (m *MetricsServer) IncQueueSizeCounter() {
	m.queueSizeCounter.WithLabelValues(m.shard).Inc()
}

func (m *MetricsServer) IncErroredEventsCounter(metricEventType MetricEventType, errorType MetricEventErrorType) {
	m.erroredEventsCounter.WithLabelValues(m.shard, string(metricEventType), string(errorType)).Inc()
}

func (m *MetricsServer) IncCachedIgnoredEventsCounter(metricEventType MetricEventType) {
	m.cachedIgnoredEventsCounter.WithLabelValues(m.shard, string(metricEventType)).Inc()
}

func (m *MetricsServer) ObserveEventProcessingDurationHistogramDuration(metricEventType MetricEventType, duration time.Duration) {
	m.eventProcessingDurationHistogram.WithLabelValues(m.shard, string(metricEventType)).Observe(duration.Seconds())
}
