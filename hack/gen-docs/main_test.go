package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// TestGenerateNotificationsDocs verifies generated pages, overview links and navigation contain only supported services.
func TestGenerateNotificationsDocs(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	const navConfig = `nav:
  - Operator Manual:
      - Notifications:
          - Notification Services:
              - operator-manual/notifications/services/teams.md
`
	require.NoError(t, os.WriteFile("mkdocs.yml", []byte(navConfig), 0o600))
	servicesDir := filepath.Join("docs", "operator-manual", "notifications", "services")
	require.NoError(t, os.MkdirAll(servicesDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(servicesDir, "teams.md"), []byte("obsolete page"), 0o600))

	main()

	_, err := os.Stat(filepath.Join(servicesDir, "teams.md"))
	assert.True(t, os.IsNotExist(err))
	workflows, err := os.ReadFile(filepath.Join(servicesDir, "teams-workflows.md"))
	require.NoError(t, err)
	assert.NotEmpty(t, workflows)
	overview, err := os.ReadFile(filepath.Join(servicesDir, "overview.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(overview), "(./teams.md)")
	assert.Contains(t, string(overview), "(./teams-workflows.md)")
	nav, err := os.ReadFile("mkdocs.yml")
	require.NoError(t, err)
	var config struct {
		Nav []map[string][]map[string][]map[string][]string `yaml:"nav"`
	}
	require.NoError(t, yaml.Unmarshal(nav, &config))
	require.Len(t, config.Nav, 1)
	require.Len(t, config.Nav[0]["Operator Manual"], 1)
	services := config.Nav[0]["Operator Manual"][0]["Notifications"]
	require.Len(t, services, 1)
	pages := services[0]["Notification Services"]
	assert.NotContains(t, pages, "operator-manual/notifications/services/teams.md")
	assert.Contains(t, pages, "operator-manual/notifications/services/teams-workflows.md")
	assert.Contains(t, pages, "operator-manual/notifications/services/slack.md")
}

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
				if os.Geteuid() == 0 {
					t.Skip("file permission checks do not apply to root")
				}
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
