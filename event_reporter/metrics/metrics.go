package metrics

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/argoproj/argo-cd/v2/event_reporter/sharding"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/argoproj/argo-cd/v2/util/profile"
)

type MetricsServer struct {
	*http.Server
	shard string

	queueSizeGauge *prometheus.GaugeVec

	enqueuedEventsCounter *prometheus.CounterVec
	droppedEventsCounter  *prometheus.CounterVec

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
	queueSizeGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "codefresh_event_reporter_queue_size",
			Help: "Size of application events queue of a particular shard.",
		},
		[]string{"reporter_shard"},
	)

	enqueuedEventsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "codefresh_event_reporter_enqueued_events_total",
			Help: "Amount of application events not accepted into the queue of a particular shard.",
		},
		[]string{"reporter_shard", "application", "error_in_learning_mode"},
	)

	droppedEventsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "codefresh_event_reporter_dropped_events_total",
			Help: "Amount of dropped application events queue of taken shard.",
		},
		[]string{"reporter_shard", "application", "error_in_learning_mode"},
	)

	erroredEventsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "codefresh_event_reporter_errored_events_total",
			Help: "Amount of application events not accepted into the queue of a particular shard.",
		},
		[]string{"reporter_shard", "metric_event_type", "error_type", "application"},
	)

	eventProcessingDurationHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "codefresh_event_reporter_event_processing_duration",
			Help:    "Application event processing duration.",
			Buckets: []float64{0.25, .5, 1, 2, 5, 10, 20},
		},
		[]string{"reporter_shard", "application", "metric_event_type"},
	)

	cachedIgnoredEventsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "codefresh_event_reporter_cached_ignored_events",
			Help: "Total number of ignored events because of cache.",
		},
		[]string{"reporter_shard", "metric_event_type", "application"},
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

	registry.MustRegister(queueSizeGauge)

	registry.MustRegister(enqueuedEventsCounter)
	registry.MustRegister(droppedEventsCounter)
	registry.MustRegister(erroredEventsCounter)

	registry.MustRegister(cachedIgnoredEventsCounter)
	registry.MustRegister(eventProcessingDurationHistogram)

	shard := sharding.GetShardNumber()

	return &MetricsServer{
		Server: &http.Server{
			Addr:    fmt.Sprintf("%s:%d", host, port),
			Handler: mux,
		},
		shard:                            strconv.FormatInt(int64(shard), 10),
		queueSizeGauge:                   queueSizeGauge,
		enqueuedEventsCounter:            enqueuedEventsCounter,
		droppedEventsCounter:             droppedEventsCounter,
		erroredEventsCounter:             erroredEventsCounter,
		cachedIgnoredEventsCounter:       cachedIgnoredEventsCounter,
		eventProcessingDurationHistogram: eventProcessingDurationHistogram,
	}
}

func (m *MetricsServer) SetQueueSizeGauge(size int) {
	m.queueSizeGauge.WithLabelValues(m.shard).Set(float64(size))
}

func (m *MetricsServer) IncEnqueuedEventsCounter(application string, errorInLearningMode bool) {
	m.enqueuedEventsCounter.WithLabelValues(m.shard, application, strconv.FormatBool(errorInLearningMode)).Inc()
}

func (m *MetricsServer) IncDroppedEventsCounter(application string, errorInLearningMode bool) {
	m.droppedEventsCounter.WithLabelValues(m.shard, application, strconv.FormatBool(errorInLearningMode)).Inc()
}

func (m *MetricsServer) IncErroredEventsCounter(metricEventType MetricEventType, errorType MetricEventErrorType, application string) {
	m.erroredEventsCounter.WithLabelValues(m.shard, string(metricEventType), string(errorType), application).Inc()
}

func (m *MetricsServer) IncCachedIgnoredEventsCounter(metricEventType MetricEventType, application string) {
	m.cachedIgnoredEventsCounter.WithLabelValues(m.shard, string(metricEventType), application).Inc()
}

func (m *MetricsServer) ObserveEventProcessingDurationHistogramDuration(application string, metricEventType MetricEventType, duration time.Duration) {
	m.eventProcessingDurationHistogram.WithLabelValues(m.shard, application, string(metricEventType)).Observe(duration.Seconds())
}
