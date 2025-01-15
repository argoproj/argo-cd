package commands

import (
	"bytes"
	"os"
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

// TestPluginSuccessfulExecution verifies that a plugin found in the PATH executes successfully
func TestPluginSuccessfulExecution(t *testing.T) {
	setupPluginPath(t)

	pluginHandler := NewDefaultPluginHandler([]string{"argocd"})
	cmd := NewCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	args := []string{"argocd", "foo"}
	cmd.SetArgs(args[1:])

	err := cmd.Execute()
	require.Error(t, err)

	pluginErr := pluginHandler.HandleCommandExecutionError(err, true, args)
	require.NoError(t, pluginErr)
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
