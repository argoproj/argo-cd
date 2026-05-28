package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObserveParallelismWaitDuration(t *testing.T) {
	m := NewMetricsServer()

	m.ObserveParallelismWaitDuration(50 * time.Millisecond)
	m.ObserveParallelismWaitDuration(3 * time.Second)
	m.ObserveParallelismWaitDuration(75 * time.Second)

	expected := `
# HELP argocd_repo_parallelism_wait_duration_seconds Time spent waiting for the repo-server manifest generation parallelism semaphore. Observed on every acquire attempt, including those that fail (e.g. context canceled).
# TYPE argocd_repo_parallelism_wait_duration_seconds histogram
argocd_repo_parallelism_wait_duration_seconds_bucket{le="0.1"} 1
argocd_repo_parallelism_wait_duration_seconds_bucket{le="0.25"} 1
argocd_repo_parallelism_wait_duration_seconds_bucket{le="0.5"} 1
argocd_repo_parallelism_wait_duration_seconds_bucket{le="1"} 1
argocd_repo_parallelism_wait_duration_seconds_bucket{le="2"} 1
argocd_repo_parallelism_wait_duration_seconds_bucket{le="4"} 2
argocd_repo_parallelism_wait_duration_seconds_bucket{le="10"} 2
argocd_repo_parallelism_wait_duration_seconds_bucket{le="20"} 2
argocd_repo_parallelism_wait_duration_seconds_bucket{le="60"} 2
argocd_repo_parallelism_wait_duration_seconds_bucket{le="120"} 3
argocd_repo_parallelism_wait_duration_seconds_bucket{le="+Inf"} 3
argocd_repo_parallelism_wait_duration_seconds_sum 78.05
argocd_repo_parallelism_wait_duration_seconds_count 3
`
	err := testutil.GatherAndCompare(m.PrometheusRegistry, strings.NewReader(expected), "argocd_repo_parallelism_wait_duration_seconds")
	require.NoError(t, err)
}

func TestParallelismWaitMetricRegistered(t *testing.T) {
	m := NewMetricsServer()
	count := testutil.CollectAndCount(m.PrometheusRegistry, "argocd_repo_parallelism_wait_duration_seconds")
	assert.Equal(t, 1, count)
}
