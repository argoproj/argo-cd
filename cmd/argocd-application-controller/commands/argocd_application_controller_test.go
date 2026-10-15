package commands

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewCommand_HydrationProcessorsFlag pins down the contract for the manifest hydration concurrency
// knob added for https://github.com/argoproj/argo-cd/issues/27926: the flag exists and defaults to a value
// greater than 1 (so the default deployment exercises hydration concurrency and tests are more likely to
// catch races, per the maintainer's guidance on the issue).
func TestNewCommand_HydrationProcessorsFlag(t *testing.T) {
	cmd := NewCommand()

	f := cmd.Flags().Lookup("hydration-processors")
	require.NotNil(t, f, "expected --hydration-processors flag to be registered")
	assert.Equal(t, "5", f.DefValue, "default hydration processors should be greater than 1")
}

// TestNewCommand_MetricsFlagsFromEnv verifies the metrics label/condition flags
// default from their environment variables, so they can be configured through
// argocd-cmd-params-cm like the other controller parameters.
func TestNewCommand_MetricsFlagsFromEnv(t *testing.T) {
	t.Setenv("ARGOCD_APPLICATION_CONTROLLER_METRICS_APPLICATION_LABELS", "team,env")
	t.Setenv("ARGOCD_APPLICATION_CONTROLLER_METRICS_APPLICATION_CONDITIONS", "OrphanedResourceWarning")
	t.Setenv("ARGOCD_APPLICATION_CONTROLLER_METRICS_CLUSTER_LABELS", "environment")

	cmd := NewCommand()

	labels, err := cmd.Flags().GetStringSlice("metrics-application-labels")
	require.NoError(t, err)
	assert.Equal(t, []string{"team", "env"}, labels)

	conditions, err := cmd.Flags().GetStringSlice("metrics-application-conditions")
	require.NoError(t, err)
	assert.Equal(t, []string{"OrphanedResourceWarning"}, conditions)

	clusterLabels, err := cmd.Flags().GetStringSlice("metrics-cluster-labels")
	require.NoError(t, err)
	assert.Equal(t, []string{"environment"}, clusterLabels)
}

func TestNewCommand_MetricsFlagHelpUsesActualMetricNames(t *testing.T) {
	cmd := NewCommand()

	labelsFlag := cmd.Flags().Lookup("metrics-application-labels")
	require.NotNil(t, labelsFlag)
	assert.Contains(t, labelsFlag.Usage, "argocd_app_labels")
	assert.NotContains(t, labelsFlag.Usage, "argocd_application_labels")

	conditionsFlag := cmd.Flags().Lookup("metrics-application-conditions")
	require.NotNil(t, conditionsFlag)
	assert.Contains(t, conditionsFlag.Usage, "argocd_app_condition")
	assert.NotContains(t, conditionsFlag.Usage, "argocd_application_conditions")
}

func TestNewCommand_ClusterCacheFlagsFromEnv(t *testing.T) {
	tests := []struct {
		env      string
		flag     string
		value    string
		expected string
	}{
		{env: "ARGOCD_CLUSTER_CACHE_RESYNC_DURATION", flag: "cluster-cache-resync-duration", value: "6h", expected: "6h0m0s"},
		{env: "ARGOCD_CLUSTER_CACHE_WATCH_RESYNC_DURATION", flag: "cluster-cache-watch-resync-duration", value: "5m", expected: "5m0s"},
		{env: "ARGOCD_CLUSTER_SYNC_RETRY_TIMEOUT_DURATION", flag: "cluster-cache-sync-retry-timeout", value: "30s", expected: "30s"},
		{env: "ARGOCD_CLUSTER_CACHE_LIST_PAGE_SIZE", flag: "cluster-cache-list-page-size", value: "250", expected: "250"},
		{env: "ARGOCD_CLUSTER_CACHE_LIST_PAGE_BUFFER_SIZE", flag: "cluster-cache-list-page-buffer-size", value: "4", expected: "4"},
		{env: "ARGOCD_CLUSTER_CACHE_LIST_SEMAPHORE", flag: "cluster-cache-list-semaphore", value: "10", expected: "10"},
		{env: "ARGOCD_CLUSTER_CACHE_ATTEMPT_LIMIT", flag: "cluster-cache-attempt-limit", value: "3", expected: "3"},
		{env: "ARGOCD_CLUSTER_CACHE_RETRY_USE_BACKOFF", flag: "cluster-cache-retry-use-backoff", value: "true", expected: "true"},
		{env: "ARGOCD_CLUSTER_CACHE_BATCH_EVENTS_PROCESSING", flag: "cluster-cache-batch-events-processing", value: "false", expected: "false"},
		{env: "ARGOCD_CLUSTER_CACHE_EVENTS_PROCESSING_INTERVAL", flag: "cluster-cache-events-processing-interval", value: "1s", expected: "1s"},
	}
	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			t.Setenv(tt.env, tt.value)

			f := NewCommand().Flags().Lookup(tt.flag)
			require.NotNil(t, f, "expected --%s flag to be registered", tt.flag)
			assert.Equal(t, tt.expected, f.DefValue)
		})
	}
}

func TestNewCommand_InvalidClusterCacheFlag(t *testing.T) {
	cmd := NewCommand()
	cmd.SetArgs([]string{"--cluster-cache-attempt-limit=0"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := cmd.Execute()

	require.ErrorContains(t, err, "invalid cluster cache configuration")
	require.ErrorContains(t, err, "attempt limit must be at least 1")
}
