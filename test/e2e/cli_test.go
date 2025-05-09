package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

// createTestPlugin creates a temporary Argo CD CLI plugin script for testing purposes.
// The script is written to a temporary directory with executable permissions.
func createTestPlugin(t *testing.T, name, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "argocd-"+name)

	require.NoError(t, os.WriteFile(pluginPath, []byte(content), 0o755))

	// Ensure the plugin is cleaned up properly
	t.Cleanup(func() {
		_ = os.Remove(pluginPath)
	})

	return pluginPath
}

// TestCliAppCommand verifies the basic Argo CD CLI commands for app synchronization and listing.
func TestCliAppCommand(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		CreateApp().
		And(func() {
			output, err := RunCli("app", "sync", Name(), "--timeout", "90")
			require.NoError(t, err)
			vars := map[string]any{"Name": Name(), "Namespace": DeploymentNamespace()}
			assert.Contains(t, NormalizeOutput(output), Tmpl(t, `Pod {{.Namespace}} pod Synced Progressing pod/pod created`, vars))
			assert.Contains(t, NormalizeOutput(output), Tmpl(t, `Pod {{.Namespace}} hook Succeeded Sync pod/hook created`, vars))
		}).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(_ *Application) {
			output, err := RunCli("app", "list")
			require.NoError(t, err)
			expected := Tmpl(
				t,
				`{{.Name}} https://kubernetes.default.svc {{.Namespace}} default Synced Healthy Manual <none>`,
				map[string]any{"Name": Name(), "Namespace": DeploymentNamespace()})
			assert.Contains(t, NormalizeOutput(output), expected)
		})
}

// TestNormalArgoCDCommandsExecuteOverPluginsWithSameName verifies that normal Argo CD CLI commands
// take precedence over plugins with the same name when both exist in the path.
func TestNormalArgoCDCommandsExecuteOverPluginsWithSameName(t *testing.T) {
	pluginScript := `#!/bin/bash
	echo "I am a plugin, not Argo CD!"
	exit 0`

	pluginPath := createTestPlugin(t, "app", pluginScript)

	origPath := os.Getenv("PATH")
	t.Cleanup(func() {
		t.Setenv("PATH", origPath)
	})
	t.Setenv("PATH", filepath.Dir(pluginPath)+":"+origPath)

	Given(t).
		Path("hook").
		When().
		CreateApp().
		And(func() {
			output, err := RunCli("app", "sync", Name(), "--timeout", "90")
			require.NoError(t, err)

			assert.NotContains(t, NormalizeOutput(output), "I am a plugin, not Argo CD!")

			vars := map[string]any{"Name": Name(), "Namespace": DeploymentNamespace()}
			assert.Contains(t, NormalizeOutput(output), Tmpl(t, `Pod {{.Namespace}} pod Synced Progressing pod/pod created`, vars))
			assert.Contains(t, NormalizeOutput(output), Tmpl(t, `Pod {{.Namespace}} hook Succeeded Sync pod/hook created`, vars))
		}).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(_ *Application) {
			output, err := RunCli("app", "list")
			require.NoError(t, err)

			assert.NotContains(t, NormalizeOutput(output), "I am a plugin, not Argo CD!")

			expected := Tmpl(
				t,
				`{{.Name}} https://kubernetes.default.svc {{.Namespace}} default Synced Healthy Manual <none>`,
				map[string]any{"Name": Name(), "Namespace": DeploymentNamespace()})
			assert.Contains(t, NormalizeOutput(output), expected)
		})
}

// TestCliPluginExecution tests the execution of a valid Argo CD CLI plugin.
func TestCliPluginExecution(t *testing.T) {
	pluginScript := `#!/bin/bash
	echo "Hello from myplugin"
	exit 0`
	pluginPath := createTestPlugin(t, "myplugin", pluginScript)

	origPath := os.Getenv("PATH")
	t.Cleanup(func() {
		t.Setenv("PATH", origPath)
	})
	t.Setenv("PATH", filepath.Dir(pluginPath)+":"+origPath)

	output, err := RunPluginCli("", "myplugin")
	require.NoError(t, err)
	assert.Contains(t, NormalizeOutput(output), "Hello from myplugin")
}

// TestCliPluginExecutionConditions tests for plugin execution conditions
func TestCliPluginExecutionConditions(t *testing.T) {
	createValidPlugin := func(t *testing.T, name string, executable bool) string {
		t.Helper()

		script := `#!/bin/bash
		echo "Hello from $0"
		exit 0
`

		pluginPath := createTestPlugin(t, name, script)

		if executable {
			require.NoError(t, os.Chmod(pluginPath, 0o755))
		} else {
			require.NoError(t, os.Chmod(pluginPath, 0o644))
		}

		return pluginPath
	}

	createInvalidPlugin := func(t *testing.T, name string) string {
		t.Helper()

		script := `#!/bin/bash
		echo "Hello from $0"
		exit 0
`

		tmpDir := t.TempDir()
		pluginPath := filepath.Join(tmpDir, "argocd_"+name) // this is an invalid plugin name format
		require.NoError(t, os.WriteFile(pluginPath, []byte(script), 0o755))

		return pluginPath
	}

	// 'argocd-valid-plugin' is a valid plugin name
	validPlugin := createValidPlugin(t, "valid-plugin", true)
	// 'argocd_invalid-plugin' is an invalid plugin name
	invalidPlugin := createInvalidPlugin(t, "invalid-plugin")
	// 'argocd-nonexec-plugin' is a valid plugin name but lacks executable permissions
	noExecPlugin := createValidPlugin(t, "noexec-plugin", false)

	origPath := os.Getenv("PATH")
	defer func() {
		t.Setenv("PATH", origPath)
	}()
	t.Setenv("PATH", filepath.Dir(validPlugin)+":"+filepath.Dir(invalidPlugin)+":"+filepath.Dir(noExecPlugin)+":"+origPath)

	output, err := RunPluginCli("", "valid-plugin")
	require.NoError(t, err)
	assert.Contains(t, NormalizeOutput(output), "Hello from")

	_, err = RunPluginCli("", "invalid-plugin")
	require.Error(t, err)

	_, err = RunPluginCli("", "noexec-plugin")
	// expects error since plugin lacks executable permissions
	require.Error(t, err)
}

// TestCliPluginStatusCodes verifies that a plugin returns the correct exit codes based on its execution.
func TestCliPluginStatusCodes(t *testing.T) {
	pluginScript := `#!/bin/bash
	case "$1" in
	    "success") exit 0 ;;
	    "error1") exit 1 ;;
	    "error2") exit 2 ;;
	    *) echo "Unknown argument: $1"; exit 3 ;;
	esac`

	pluginPath := createTestPlugin(t, "error-plugin", pluginScript)

	origPath := os.Getenv("PATH")
	t.Cleanup(func() {
		t.Setenv("PATH", origPath)
	})
	t.Setenv("PATH", filepath.Dir(pluginPath)+":"+origPath)

	output, err := RunPluginCli("", "error-plugin", "success")
	require.NoError(t, err)
	assert.Contains(t, NormalizeOutput(output), "")

	_, err = RunPluginCli("", "error-plugin", "error1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exit status 1")

	_, err = RunPluginCli("", "error-plugin", "error2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exit status 2")

	_, err = RunPluginCli("", "error-plugin", "unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exit status 3")
}

// TestCliPluginStdinHandling verifies that a CLI plugin correctly handles input from stdin.
func TestCliPluginStdinHandling(t *testing.T) {
	pluginScript := `#!/bin/bash
	input=$(cat)
	echo "Received: $input"
	exit 0`

	pluginPath := createTestPlugin(t, "stdin-plugin", pluginScript)

	origPath := os.Getenv("PATH")
	t.Cleanup(func() {
		t.Setenv("PATH", origPath)
	})
	t.Setenv("PATH", filepath.Dir(pluginPath)+":"+origPath)

	testCases := []struct {
		name     string
		stdin    string
		expected string
	}{
		{
			"Single line input",
			"Hello, ArgoCD!",
			"Received: Hello, ArgoCD!",
		},
		{
			"Multiline input",
			"Line1\nLine2\nLine3",
			"Received: Line1\nLine2\nLine3",
		},
		{
			"Empty input",
			"",
			"Received:",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := RunPluginCli(tc.stdin, "stdin-plugin")
			require.NoError(t, err)
			assert.Contains(t, NormalizeOutput(output), tc.expected)
		})
	}
}
