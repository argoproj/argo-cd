package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	argoappv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
	metricsutil "github.com/argoproj/argo-cd/v3/util/metrics"
	"github.com/argoproj/argo-cd/v3/util/metrics/kubectl"
)

var (
	descAppsetLabels          *prometheus.Desc
	descAppsetDefaultLabels   = []string{"namespace", "name"}
	progressiveSyncStepLabels = []string{"namespace", "name", "step", "progressiveStatus"}
	descAppsetInfo            = prometheus.NewDesc(
		"argocd_appset_info",
		"Information about applicationset",
		append(descAppsetDefaultLabels, "resource_update_status"),
		nil,
	)

	descAppsetGeneratedApps = prometheus.NewDesc(
		"argocd_appset_owned_applications",
		"Number of applications owned by the applicationset",
		descAppsetDefaultLabels,
		nil,
	)
)

// Counters
var (
	progressiveSyncAppSyncCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "argocd_appset_progressive_sync_syncs_triggered_total",
		Help: "Counts sync operations triggered by progressive sync",
	}, []string{"namespace", "name", "step"})
)

// Gauge
var (
	progressiveSyncAppStatusGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "argocd_appset_progressive_sync_app_status",
		Help: "Count of apps per status (Waiting/Pending/Progressing/Healthy) per step",
	}, progressiveSyncStepLabels)
)

// Histograms
var (
	progressiveSyncTriggerSyncAfterDetectionHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "argocd_appset_progressive_sync_detection_to_trigger_seconds",
		Help:    "Time from PerformProgressiveSync to SyncDesiredApplications",
		Buckets: []float64{0.05, 0.1, 0.15, 0.5, 1, 5}, // Default  buckets are {.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
	}, []string{"namespace", "name"})
)

type ApplicationsetMetrics struct {
	reconcileHistogram                                *prometheus.HistogramVec
	progressiveSyncAppStatusGauge                     *prometheus.GaugeVec
	progressiveSyncAppSyncCounter                     *prometheus.CounterVec
	progressiveSyncTriggerSyncAfterDetectionHistogram *prometheus.HistogramVec
}

type appsetCollector struct {
	lister applisters.ApplicationSetLister
	// appsClientSet appclientset.Interface
	labels []string
	filter func(appset *argoappv1.ApplicationSet) bool
}

func NewApplicationsetMetrics(appsetLister applisters.ApplicationSetLister, appsetLabels []string, appsetFilter func(appset *argoappv1.ApplicationSet) bool) ApplicationsetMetrics {
	reconcileHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "argocd_appset_reconcile",
			Help: "Application reconciliation performance in seconds.",
			// Buckets can be set later on after observing median time
		},
		descAppsetDefaultLabels,
	)

	appsetCollector := newAppsetCollector(appsetLister, appsetLabels, appsetFilter)

	// Register collectors and metrics
	metrics.Registry.MustRegister(reconcileHistogram)
	metrics.Registry.MustRegister(progressiveSyncAppStatusGauge)
	metrics.Registry.MustRegister(progressiveSyncAppSyncCounter)
	metrics.Registry.MustRegister(progressiveSyncTriggerSyncAfterDetectionHistogram)
	metrics.Registry.MustRegister(appsetCollector)

	kubectl.RegisterWithClientGo()
	kubectl.RegisterWithPrometheus(metrics.Registry)

	return ApplicationsetMetrics{
		reconcileHistogram:                                reconcileHistogram,
		progressiveSyncAppStatusGauge:                     progressiveSyncAppStatusGauge,
		progressiveSyncAppSyncCounter:                     progressiveSyncAppSyncCounter,
		progressiveSyncTriggerSyncAfterDetectionHistogram: progressiveSyncTriggerSyncAfterDetectionHistogram,
	}
}

func (m *ApplicationsetMetrics) ObserveReconcile(appset *argoappv1.ApplicationSet, duration time.Duration) {
	m.reconcileHistogram.WithLabelValues(appset.Namespace, appset.Name).Observe(duration.Seconds())
}

func (m *ApplicationsetMetrics) SetProgressiveSyncAppStatus(appset *argoappv1.ApplicationSet) {
	m.progressiveSyncAppStatusGauge.DeletePartialMatch(prometheus.Labels{
		"namespace": appset.Namespace,
		"name":      appset.Name,
	})

	counts := map[string]map[string]int{}
	for _, appStatus := range appset.Status.ApplicationStatus {
		step := appStatus.Step
		status := string(appStatus.Status)
		if counts[step] == nil {
			counts[step] = map[string]int{}
		}
		counts[step][status]++
	}

	for step, statusCounts := range counts {
		for status, count := range statusCounts {
			m.progressiveSyncAppStatusGauge.WithLabelValues(appset.Namespace, appset.Name, step, status).Set(float64(count))
		}
	}
}

func (m *ApplicationsetMetrics) SetProgressiveSyncAppSync(appset *argoappv1.ApplicationSet, step string) {
	m.progressiveSyncAppSyncCounter.WithLabelValues(appset.Namespace, appset.Name, step).Inc()
}

func (m *ApplicationsetMetrics) ObserveTimeToStartSyncAfterDetection(appset *argoappv1.ApplicationSet, duration time.Duration) {
	m.progressiveSyncTriggerSyncAfterDetectionHistogram.WithLabelValues(appset.Namespace, appset.Name).Observe(duration.Seconds())
}

func newAppsetCollector(lister applisters.ApplicationSetLister, labels []string, filter func(appset *argoappv1.ApplicationSet) bool) *appsetCollector {
	descAppsetDefaultLabels = []string{"namespace", "name"}

	if len(labels) > 0 {
		descAppsetLabels = prometheus.NewDesc(
			"argocd_appset_labels",
			"Applicationset labels translated to Prometheus labels",
			append(descAppsetDefaultLabels, metricsutil.NormalizeLabels("label", labels)...),
			nil,
		)
	}

	return &appsetCollector{
		lister: lister,
		labels: labels,
		filter: filter,
	}
}

// Describe implements the prometheus.Collector interface
func (c *appsetCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descAppsetInfo
	ch <- descAppsetGeneratedApps

	if len(c.labels) > 0 {
		ch <- descAppsetLabels
	}
}

// Collect implements the prometheus.Collector interface
func (c *appsetCollector) Collect(ch chan<- prometheus.Metric) {
	appsets, _ := c.lister.List(labels.NewSelector())

	for _, appset := range appsets {
		if c.filter(appset) {
			collectAppset(appset, c.labels, ch)
		}
	}
}

func collectAppset(appset *argoappv1.ApplicationSet, labelsToCollect []string, ch chan<- prometheus.Metric) {
	labelValues := make([]string, 0)
	commonLabelValues := []string{appset.Namespace, appset.Name}

	for _, label := range labelsToCollect {
		labelValues = append(labelValues, appset.GetLabels()[label])
	}

	resourceUpdateStatus := "Unknown"

	for _, condition := range appset.Status.Conditions {
		if condition.Type == argoappv1.ApplicationSetConditionResourcesUpToDate {
			resourceUpdateStatus = condition.Reason
		}
	}

	if len(labelsToCollect) > 0 {
		ch <- prometheus.MustNewConstMetric(descAppsetLabels, prometheus.GaugeValue, 1, append(commonLabelValues, labelValues...)...)
	}

	ch <- prometheus.MustNewConstMetric(descAppsetInfo, prometheus.GaugeValue, 1, appset.Namespace, appset.Name, resourceUpdateStatus)
	ch <- prometheus.MustNewConstMetric(descAppsetGeneratedApps, prometheus.GaugeValue, float64(len(appset.Status.Resources)), appset.Namespace, appset.Name)
}
