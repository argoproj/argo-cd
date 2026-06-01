package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCommandProcessorsCountDefault(t *testing.T) {
	t.Setenv("ARGOCD_NOTIFICATION_CONTROLLER_PROCESSORS_COUNT", "4")

	cmd := NewCommand()

	processorsCount, err := cmd.Flags().GetInt("processors-count")
	require.NoError(t, err)
	assert.Equal(t, 4, processorsCount)
}

func TestNewCommandProcessorsCountInvalidEnvFallsBackToDefault(t *testing.T) {
	t.Setenv("ARGOCD_NOTIFICATION_CONTROLLER_PROCESSORS_COUNT", "0")

	cmd := NewCommand()

	processorsCount, err := cmd.Flags().GetInt("processors-count")
	require.NoError(t, err)
	assert.Equal(t, 1, processorsCount)
}
