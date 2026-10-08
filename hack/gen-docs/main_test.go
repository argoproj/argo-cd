package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveLegacyTeamsDocs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	legacy := filepath.Join(dir, "teams.md")
	workflows := filepath.Join(dir, "teams-workflows.md")
	overview := filepath.Join(dir, "overview.md")
	require.NoError(t, os.WriteFile(legacy, []byte("legacy"), 0o600))
	require.NoError(t, os.WriteFile(workflows, []byte("workflows"), 0o600))
	require.NoError(t, os.WriteFile(overview, []byte("* [Teams](./teams.md) - retired\n* [Teams Workflows](./teams-workflows.md)\n* [Slack](./slack.md)\n"), 0o600))
	files, err := removeLegacyTeamsDocs([]string{legacy, workflows, overview})
	require.NoError(t, err)
	assert.Equal(t, []string{workflows, overview}, files)
	_, err = os.Stat(legacy)
	assert.True(t, os.IsNotExist(err))
	data, err := os.ReadFile(workflows)
	require.NoError(t, err)
	assert.Equal(t, "workflows", string(data))
	data, err = os.ReadFile(overview)
	require.NoError(t, err)
	assert.Equal(t, "* [Teams Workflows](./teams-workflows.md)\n* [Slack](./slack.md)\n", string(data))
}
