package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Fake implementation for testing
func NewFakeAppsetMetrics() *ApplicationsetMetrics {
	reconcileHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "argocd_appset_reconcile",
			Help: "Application reconciliation performance in seconds.",
			// Buckets can be set later on after observing median time
		},
		[]string{"name", "namespace"},
	)

	return &ApplicationsetMetrics{
		reconcileHistogram:                                reconcileHistogram,
		progressiveSyncAppStatusGauge:                     prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "argocd_appset_progressive_sync_app_status"}, progressiveSyncStepLabels),
		progressiveSyncAppSyncCounter:                     prometheus.NewCounterVec(prometheus.CounterOpts{Name: "argocd_appset_progressive_sync_syncs_triggered_total"}, []string{"namespace", "name", "step"}),
		progressiveSyncTriggerSyncAfterDetectionHistogram: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "argocd_appset_progressive_sync_detection_to_trigger_seconds"}, []string{"namespace", "name"}),
	}
}
