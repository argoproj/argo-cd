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
