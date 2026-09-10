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

	rolloutDurationHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "argocd_appset_progressive_sync_rollout_duration_seconds",
			Help: "Duration of a full progressive sync rollout across all steps in seconds.",
		},
		[]string{"namespace", "name"},
	)

	stepCompletionDurationHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "argocd_appset_progressive_sync_step_duration_seconds",
			Help: "Duration of a step to complete - all applications within this step are healthy",
		},
		[]string{"namespace", "name", "step"},
	)

	return &ApplicationsetMetrics{
		reconcileHistogram:                             reconcileHistogram,
		progressiveSyncRolloutDurationHistogram:        rolloutDurationHistogram,
		progressiveSyncStepCompletionDurationHistogram: stepCompletionDurationHistogram,
	}
}
