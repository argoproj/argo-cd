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
		reconcileHistogram: reconcileHistogram,
	}
}
