package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCommand_ConcurrentApplicationUpdatesFlag(t *testing.T) {
	cmd := NewCommand()

	flag := cmd.Flags().Lookup("concurrent-application-updates")
	require.NotNil(t, flag, "expected --concurrent-application-updates flag to be registered")
	assert.Equal(t, "int", flag.Value.Type())
	assert.Equal(t, "1", flag.DefValue, "default should be 1")
}

func TestNewCommand_ConcurrentApplicationUpdatesFlagValue(t *testing.T) {
	cmd := NewCommand()

	err := cmd.Flags().Set("concurrent-application-updates", "5")
	require.NoError(t, err)

	val, err := cmd.Flags().GetInt("concurrent-application-updates")
	require.NoError(t, err)
	assert.Equal(t, 5, val)
}

func TestNewCommand_EnableNewSCMProviderFilteringFlag(t *testing.T) {
	cmd := NewCommand()

	flag := cmd.Flags().Lookup("enable-new-scm-provider-filtering")
	require.NotNil(t, flag, "expected --enable-new-scm-provider-filtering flag to be registered")
	assert.Equal(t, "bool", flag.Value.Type())
	// Opt-in: enabling it changes which Applications an existing ApplicationSet
	// generates, so upgrading must not turn it on.
	assert.Equal(t, "false", flag.DefValue, "default should be false")
}
