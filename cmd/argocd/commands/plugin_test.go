package commands

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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

	_ = NewDefaultPluginHandler([]string{"argocd"})
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

	pluginHandler := NewDefaultPluginHandler([]string{"argocd"})
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

	pluginHandler := NewDefaultPluginHandler([]string{"argocd"})
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
	pluginHandler := NewDefaultPluginHandler([]string{"argocd"})
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

	pluginHandler := NewDefaultPluginHandler([]string{"argocd"})
	cmd := NewCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	args := []string{"argocd", "no-permission"}
	cmd.SetArgs(args[1:])

	err := cmd.Execute()
	require.Error(t, err)

	pluginErr := pluginHandler.HandleCommandExecutionError(err, true, args)
	require.Error(t, pluginErr)
	assert.EqualError(t, pluginErr, "unknown command \"no-permission\" for \"argocd\"")
}

// TestPluginExecutionError checks for errors that occur during plugin execution
func TestPluginExecutionError(t *testing.T) {
	setupPluginPath(t)

	pluginHandler := NewDefaultPluginHandler([]string{"argocd"})
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

	pluginHandler := NewDefaultPluginHandler([]string{"argocd"})
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

	pluginHandler := NewDefaultPluginHandler([]string{"argocd"})

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

	pluginHandler := NewDefaultPluginHandler([]string{"argocd"})

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
