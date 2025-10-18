package commands

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupPluginPath sets the PATH to the directory where plugins are stored for testing purpose
func setupPluginPath(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	testdataPath := filepath.Join(wd, "testdata")
	t.Setenv("PATH", testdataPath)
}

// TestNormalCommandWithPlugin ensures that a standard ArgoCD command executes correctly
// even when a plugin with the same name exists in the PATH
func TestNormalCommandWithPlugin(t *testing.T) {
	setupPluginPath(t)

	_ = NewDefaultPluginHandler()
	args := []string{"argocd", "version", "--short", "--client"}
	buf := new(bytes.Buffer)
	cmd := NewVersionCmd(&argocdclient.ClientOptions{}, nil)
	cmd.SetArgs(args[1:])
	cmd.SetOut(buf)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	err := cmd.Execute()
	require.NoError(t, err)
	output := buf.String()
	assert.Equal(t, "argocd: v99.99.99+unknown\n", output)
}

// TestPluginExecution verifies that a plugin found in the PATH executes successfully following the correct naming conventions
func TestPluginExecution(t *testing.T) {
	setupPluginPath(t)

	pluginHandler := NewDefaultPluginHandler()
	cmd := NewCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	tests := []struct {
		name              string
		args              []string
		expectedPluginErr string
	}{
		{
			name:              "'argocd-foo' binary exists in the PATH",
			args:              []string{"argocd", "foo"},
			expectedPluginErr: "",
		},
		{
			name:              "'argocd-demo_plugin' binary exists in the PATH",
			args:              []string{"argocd", "demo_plugin"},
			expectedPluginErr: "",
		},
		{
			name:              "'my-plugin' binary exists in the PATH",
			args:              []string{"argocd", "my-plugin"},
			expectedPluginErr: "unknown command \"my-plugin\" for \"argocd\"",
		},
		{
			name:              "'argocd_my-plugin' binary exists in the PATH",
			args:              []string{"argocd", "my-plugin"},
			expectedPluginErr: "unknown command \"my-plugin\" for \"argocd\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd.SetArgs(tt.args[1:])

			err := cmd.Execute()
			require.Error(t, err)

			// since the command is not a valid argocd command, check for plugin execution
			pluginErr := pluginHandler.HandleCommandExecutionError(err, true, tt.args)
			if tt.expectedPluginErr == "" {
				require.NoError(t, pluginErr)
			} else {
				require.EqualError(t, pluginErr, tt.expectedPluginErr)
			}
		})
	}
}

// TestNormalCommandError checks for an error when executing a normal ArgoCD command with invalid flags
func TestNormalCommandError(t *testing.T) {
	setupPluginPath(t)

	pluginHandler := NewDefaultPluginHandler()
	args := []string{"argocd", "version", "--non-existent-flag"}
	cmd := NewVersionCmd(&argocdclient.ClientOptions{}, nil)
	cmd.SetArgs(args[1:])
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	err := cmd.Execute()
	require.Error(t, err)

	pluginErr := pluginHandler.HandleCommandExecutionError(err, true, args)
	assert.EqualError(t, pluginErr, "unknown flag: --non-existent-flag")
}

// TestUnknownCommandNoPlugin tests the scenario when the command is neither a normal ArgoCD command
// nor exists as a plugin
func TestUnknownCommandNoPlugin(t *testing.T) {
	pluginHandler := NewDefaultPluginHandler()
	cmd := NewCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	args := []string{"argocd", "non-existent"}
	cmd.SetArgs(args[1:])

	err := cmd.Execute()
	require.Error(t, err)

	pluginErr := pluginHandler.HandleCommandExecutionError(err, true, args)
	require.Error(t, pluginErr)
	assert.Equal(t, err, pluginErr)
}

// TestPluginNoExecutePermission verifies the behavior when a plugin doesn't have executable permissions
func TestPluginNoExecutePermission(t *testing.T) {
	setupPluginPath(t)

	pluginHandler := NewDefaultPluginHandler()
	cmd := NewCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	args := []string{"argocd", "no-permission"}
	cmd.SetArgs(args[1:])

	err := cmd.Execute()
	require.Error(t, err)

	pluginErr := pluginHandler.HandleCommandExecutionError(err, true, args)
	require.Error(t, pluginErr)
	// The error message may vary depending on the test environment
	// Check that it's either the expected error or a permission-related error
	errorMsg := pluginErr.Error()
	if !strings.Contains(errorMsg, "unknown command") && !strings.Contains(errorMsg, "permission denied") {
		t.Errorf("Expected error to contain 'unknown command' or 'permission denied', got: %s", errorMsg)
	}
}

// TestPluginExecutionError checks for errors that occur during plugin execution
func TestPluginExecutionError(t *testing.T) {
	setupPluginPath(t)

	pluginHandler := NewDefaultPluginHandler()
	cmd := NewCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	args := []string{"argocd", "error"}
	cmd.SetArgs(args[1:])

	err := cmd.Execute()
	require.Error(t, err)

	pluginErr := pluginHandler.HandleCommandExecutionError(err, true, args)
	require.Error(t, pluginErr)
	assert.EqualError(t, pluginErr, "exit status 1")
}

// TestPluginInRelativePathIgnored ensures that plugins in a relative path, even if the path is included in PATH,
// are ignored and not executed.
func TestPluginInRelativePathIgnored(t *testing.T) {
	setupPluginPath(t)

	relativePath := "./relative-plugins"
	err := os.MkdirAll(relativePath, 0o755)
	require.NoError(t, err)
	defer os.RemoveAll(relativePath)

	relativePluginPath := filepath.Join(relativePath, "argocd-ignore-plugin")
	err = os.WriteFile(relativePluginPath, []byte("#!/bin/bash\necho 'This should not execute'\n"), 0o755)
	require.NoError(t, err)

	t.Setenv("PATH", os.Getenv("PATH")+string(os.PathListSeparator)+relativePath)

	pluginHandler := NewDefaultPluginHandler()
	cmd := NewCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	args := []string{"argocd", "ignore-plugin"}
	cmd.SetArgs(args[1:])

	err = cmd.Execute()
	require.Error(t, err)

	pluginErr := pluginHandler.HandleCommandExecutionError(err, true, args)
	require.Error(t, pluginErr)
	assert.EqualError(t, pluginErr, "unknown command \"ignore-plugin\" for \"argocd\"")
}

// TestPluginFlagParsing checks that the flags are parsed correctly by the plugin handler
func TestPluginFlagParsing(t *testing.T) {
	setupPluginPath(t)

	pluginHandler := NewDefaultPluginHandler()

	tests := []struct {
		name           string
		args           []string
		shouldFail     bool
		expectedErrMsg string
	}{
		{
			name:           "Valid flags",
			args:           []string{"argocd", "test-plugin", "--flag1", "value1", "--flag2", "value2"},
			shouldFail:     false,
			expectedErrMsg: "",
		},
		{
			name:           "Unknown flag",
			args:           []string{"argocd", "test-plugin", "--flag3", "invalid"},
			shouldFail:     true,
			expectedErrMsg: "exit status 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand()
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true

			cmd.SetArgs(tt.args[1:])

			err := cmd.Execute()
			require.Error(t, err)

			pluginErr := pluginHandler.HandleCommandExecutionError(err, true, tt.args)

			if tt.shouldFail {
				require.Error(t, pluginErr)
				assert.Equal(t, tt.expectedErrMsg, pluginErr.Error(), "Unexpected error message")
			} else {
				require.NoError(t, pluginErr, "Expected no error for valid flags")
			}
		})
	}
}

// TestPluginStatusCode checks for a correct status code that a plugin binary would generate
func TestPluginStatusCode(t *testing.T) {
	setupPluginPath(t)

	pluginHandler := NewDefaultPluginHandler()

	tests := []struct {
		name       string
		args       []string
		wantStatus int
		throwErr   bool
	}{
		{
			name:       "plugin generates the successful exit code",
			args:       []string{"argocd", "status-code-plugin", "--flag1", "value1"},
			wantStatus: 0,
			throwErr:   false,
		},
		{
			name:       "plugin generates an error status code",
			args:       []string{"argocd", "status-code-plugin", "--flag3", "value3"},
			wantStatus: 1,
			throwErr:   true,
		},
		{
			name:       "plugin generates a status code for an invalid command",
			args:       []string{"argocd", "status-code-plugin", "invalid"},
			wantStatus: 127,
			throwErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand()
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true

			cmd.SetArgs(tt.args[1:])

			err := cmd.Execute()
			require.Error(t, err)

			pluginErr := pluginHandler.HandleCommandExecutionError(err, true, tt.args)
			if !tt.throwErr {
				require.NoError(t, pluginErr)
			} else {
				require.Error(t, pluginErr)
				var exitErr *exec.ExitError
				if errors.As(pluginErr, &exitErr) {
					assert.Equal(t, tt.wantStatus, exitErr.ExitCode(), "unexpected exit code")
				} else {
					t.Fatalf("expected an exit error, got: %v", pluginErr)
				}
			}
		})
	}
}

// TestListAvailablePlugins tests the plugin discovery functionality for tab completion
func TestListAvailablePlugins(t *testing.T) {
	setupPluginPath(t)

	tests := []struct {
		name        string
		validPrefix []string
		expected    []string
	}{
		{
			name:     "Standard argocd prefix finds plugins",
			expected: []string{"demo_plugin", "error", "foo", "status-code-plugin", "test-plugin", "version"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pluginHandler := NewDefaultPluginHandler()
			plugins := pluginHandler.ListAvailablePlugins()

			assert.Equal(t, tt.expected, plugins)
		})
	}
}

// TestListAvailablePluginsEmptyPath tests plugin discovery when PATH is empty
func TestListAvailablePluginsEmptyPath(t *testing.T) {
	// Set empty PATH
	t.Setenv("PATH", "")

	pluginHandler := NewDefaultPluginHandler()
	plugins := pluginHandler.ListAvailablePlugins()

	assert.Empty(t, plugins, "Should return empty list when PATH is empty")
}

// TestListAvailablePluginsNonExecutableFiles tests that non-executable files are ignored
func TestListAvailablePluginsNonExecutableFiles(t *testing.T) {
	setupPluginPath(t)

	pluginHandler := NewDefaultPluginHandler()
	plugins := pluginHandler.ListAvailablePlugins()

	// Should not include 'no-permission' since it's not executable
	assert.NotContains(t, plugins, "no-permission")
}

// TestListAvailablePluginsDeduplication tests that duplicate plugins from different PATH dirs are handled
func TestListAvailablePluginsDeduplication(t *testing.T) {
	// Create two temporary directories with the same plugin
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	// Create the same plugin in both directories
	plugin1 := filepath.Join(dir1, "argocd-duplicate")
	plugin2 := filepath.Join(dir2, "argocd-duplicate")

	err := os.WriteFile(plugin1, []byte("#!/bin/bash\necho 'plugin1'\n"), 0o755)
	require.NoError(t, err)

	err = os.WriteFile(plugin2, []byte("#!/bin/bash\necho 'plugin2'\n"), 0o755)
	require.NoError(t, err)

	// Set PATH to include both directories
	testPath := dir1 + string(os.PathListSeparator) + dir2
	t.Setenv("PATH", testPath)

	pluginHandler := NewDefaultPluginHandler()
	plugins := pluginHandler.ListAvailablePlugins()

	assert.Equal(t, []string{"duplicate"}, plugins)
}
