package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	metricsutil "github.com/argoproj/argo-cd/v2/util/metrics"
)

var (
	descAppsetLabels        *prometheus.Desc
	descAppsetDefaultLabels = []string{"namespace", "name"}
	descAppsetInfo          = prometheus.NewDesc(
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

type ApplicationsetMetrics struct {
	reconcileHistogram *prometheus.HistogramVec
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
	metrics.Registry.MustRegister(appsetCollector)

	return ApplicationsetMetrics{
		reconcileHistogram: reconcileHistogram,
	}
}

func (m *ApplicationsetMetrics) ObserveReconcile(appset *argoappv1.ApplicationSet, duration time.Duration) {
	m.reconcileHistogram.WithLabelValues(appset.Namespace, appset.Name).Observe(duration.Seconds())
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
