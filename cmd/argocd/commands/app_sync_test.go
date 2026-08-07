package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewApplicationSyncCommand_ServerSideGenerateFlag(t *testing.T) {
	cmd := NewApplicationSyncCommand(nil)
	flag := cmd.Flags().Lookup("server-side-generate")
	require.NotNil(t, flag, "flag --server-side-generate should exist")
	assert.Equal(t, "false", flag.DefValue)
}

func TestNewApplicationSyncCommand_LocalIncludeFlag(t *testing.T) {
	cmd := NewApplicationSyncCommand(nil)
	includeFlag := cmd.Flags().Lookup("local-include")
	require.NotNil(t, includeFlag, "flag --local-include should exist")
	assert.Equal(t, "[*.yaml,*.yml,*.json]", includeFlag.DefValue)
}

func TestShouldWarnDeprecatedLocalSync(t *testing.T) {
	assert.True(t, shouldWarnDeprecatedLocalSync("/tmp/manifests", false))
	assert.False(t, shouldWarnDeprecatedLocalSync("/tmp/manifests", true))
	assert.False(t, shouldWarnDeprecatedLocalSync("", false))
}
