package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/util/workqueue"
)

const (
	WorkQueueSubsystem         = "workqueue"
	DepthKey                   = "depth"
	AddsKey                    = "adds_total"
	QueueLatencyKey            = "queue_duration_seconds"
	WorkDurationKey            = "work_duration_seconds"
	UnfinishedWorkKey          = "unfinished_work_seconds"
	LongestRunningProcessorKey = "longest_running_processor_seconds"
	RetriesKey                 = "retries_total"
)

var (
	depth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: WorkQueueSubsystem,
		Name:      DepthKey,
		Help:      "Current depth of workqueue",
	}, []string{"name"})

	adds = prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: WorkQueueSubsystem,
		Name:      AddsKey,
		Help:      "Total number of adds handled by workqueue",
	}, []string{"name"})

	latency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Subsystem: WorkQueueSubsystem,
		Name:      QueueLatencyKey,
		Help:      "How long in seconds an item stays in workqueue before being requested",
		Buckets:   []float64{1e-6, 1e-5, 1e-4, 1e-3, 1e-2, 1e-1, 1, 5, 10, 15, 30, 60, 120, 180},
	}, []string{"name"})

	workDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Subsystem: WorkQueueSubsystem,
		Name:      WorkDurationKey,
		Help:      "How long in seconds processing an item from workqueue takes.",
		Buckets:   []float64{1e-6, 1e-5, 1e-4, 1e-3, 1e-2, 1e-1, 1, 5, 10, 15, 30, 60, 120, 180},
	}, []string{"name"})

	unfinished = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: WorkQueueSubsystem,
		Name:      UnfinishedWorkKey,
		Help: "How many seconds of work has been done that " +
			"is in progress and hasn't been observed by work_duration. Large " +
			"values indicate stuck threads. One can deduce the number of stuck " +
			"threads by observing the rate at which this increases.",
	}, []string{"name"})

	longestRunningProcessor = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: WorkQueueSubsystem,
		Name:      LongestRunningProcessorKey,
		Help: "How many seconds has the longest running " +
			"processor for workqueue been running.",
	}, []string{"name"})

	retries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: WorkQueueSubsystem,
		Name:      RetriesKey,
		Help:      "Total number of retries handled by workqueue",
	}, []string{"name"})
)

func init() {
	workqueue.SetProvider(workqueueMetricsProvider{})
}

type workqueueMetricsProvider struct{}

func (workqueueMetricsProvider) NewDepthMetric(name string) workqueue.GaugeMetric {
	return depth.WithLabelValues(name)
}

func (workqueueMetricsProvider) NewAddsMetric(name string) workqueue.CounterMetric {
	return adds.WithLabelValues(name)
}

func (workqueueMetricsProvider) NewLatencyMetric(name string) workqueue.HistogramMetric {
	return latency.WithLabelValues(name)
}

func (workqueueMetricsProvider) NewWorkDurationMetric(name string) workqueue.HistogramMetric {
	return workDuration.WithLabelValues(name)
}

func (workqueueMetricsProvider) NewUnfinishedWorkSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return unfinished.WithLabelValues(name)
}

func (workqueueMetricsProvider) NewLongestRunningProcessorSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return longestRunningProcessor.WithLabelValues(name)
}

func (workqueueMetricsProvider) NewRetriesMetric(name string) workqueue.CounterMetric {
	return retries.WithLabelValues(name)
}
