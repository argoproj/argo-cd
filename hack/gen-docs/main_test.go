package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRemoveLegacyTeamsDocs verifies that generation preserves supported services while removing retired Teams documentation.
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

// TestRemoveLegacyTeamsDocsErrors verifies that filesystem failures stop generation.
func TestRemoveLegacyTeamsDocsErrors(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"remove", "read", "write"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			file := filepath.Join(dir, "overview.md")
			switch name {
			case "remove":
				file = filepath.Join(dir, "teams.md")
			case "write":
				require.NoError(t, os.WriteFile(file, []byte("* [Teams](./teams.md)\n"), 0o444))
				t.Cleanup(func() { require.NoError(t, os.Chmod(file, 0o600)) })
			}
			files, err := removeLegacyTeamsDocs([]string{file})
			require.Error(t, err)
			assert.Nil(t, files)
			var pathErr *os.PathError
			require.ErrorAs(t, err, &pathErr)
			assert.Equal(t, file, pathErr.Path)
		})
	}
}
