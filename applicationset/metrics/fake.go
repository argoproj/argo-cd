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

	refreshCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "argocd_appset_app_refresh_total",
		Help: "Counts application refresh triggered per step for progressive sync",
	}, []string{"namespace", "name", "step"})

	return &ApplicationsetMetrics{
		reconcileHistogram:                        reconcileHistogram,
		progressiveSyncAppRefreshTriggeredCounter: refreshCounter,
	}
}
