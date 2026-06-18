package commands

import (
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
