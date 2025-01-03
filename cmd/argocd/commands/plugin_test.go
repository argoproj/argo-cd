package commands

import (
	"bytes"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pluginPath refers to the path of the plugin found. For tests, the plugins are in testdata dir
var pluginPath string

// TestNormalArgoCDCommandExecution_WhenPluginWithSameNameIsFound tests that a normal argocd command get executed successfully such as argocd version even if a plugin with the same name (argocd-version) is present in PATH
func TestNormalArgoCDCommandExecution_WhenPluginWithSameNameIsFound(t *testing.T) {
	pluginHandler := NewDefaultPluginHandler([]string{"argocd"})
	pluginHandler.lookPath = func(file string) (string, error) {
		if file == "argocd-version" {
			pluginPath = "testdata/argocd-version"
		}
		return pluginPath, nil
	}
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

// Test_Normal_ArgoCD_Command_Error checks for an error that would be received on failure of execution of a normal argocd command
func Test_Normal_ArgoCD_Command_Error(t *testing.T) {
	pluginHandler := NewDefaultPluginHandler([]string{"argocd"})
	pluginHandler.lookPath = func(file string) (string, error) {
		if file == "argocd-version" {
			pluginPath = "testdata/argocd-version"
		}
		return pluginPath, nil
	}
	args := []string{"argocd", "version", "--non-existent-flag"}
	cmd := NewVersionCmd(&argocdclient.ClientOptions{}, nil)
	cmd.SetArgs(args[1:])
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	err := cmd.Execute()
	require.Error(t, err)

	pluginErr := pluginHandler.HandleCommandExecutionError(err, true, args)
	// although the lookPath will set the pluginPath to testdata/argocd-foo, the plugin path never gets updated because we never execute handlePluginCommand() function
	// Why? argocd version is a normal argocd command, hence we never try to look for a plugin
	assert.Equal(t, "", pluginPath)
	assert.EqualError(t, pluginErr, "unknown flag: --non-existent-flag")
}

// Test_ArgoCD_Plugin_Successful_Execution tests that a plugin found in the PATH gets executed successfully
func Test_ArgoCD_Plugin_Successful_Execution(t *testing.T) {
	pluginHandler := NewDefaultPluginHandler([]string{"argocd"})
	pluginHandler.lookPath = func(file string) (string, error) {
		if file == "argocd-foo" {
			pluginPath = "testdata/argocd-foo"
		}
		return pluginPath, nil
	}
	cmd := NewCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	args := []string{"argocd", "foo"}
	cmd.SetArgs(args[1:])

	err := cmd.Execute()
	require.Error(t, err)

	pluginErr := pluginHandler.HandleCommandExecutionError(err, true, args)
	assert.Equal(t, "testdata/argocd-foo", pluginPath)
	require.NoError(t, pluginErr)
}

// Test_Unknown_ArgoCD_Command_With_No_Plugin_Found tests for the scenario when the command is neither a normal argocd command nor exists as plugin
func Test_Unknown_ArgoCD_Command_With_No_Plugin_Found(t *testing.T) {
	pluginHandler := NewDefaultPluginHandler([]string{"argocd"})
	// lookPath will return an empty plugin path and nil error when no plugin is found
	pluginHandler.lookPath = func(file string) (string, error) {
		return pluginPath, nil
	}
	cmd := NewCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	args := []string{"argocd", "non-existent"}
	cmd.SetArgs(args[1:])

	err := cmd.Execute()
	require.Error(t, err)

	pluginErr := pluginHandler.HandleCommandExecutionError(err, true, args)
	assert.Equal(t, "", pluginPath)
	require.Error(t, pluginErr)
	// when no plugin is found, the error will be equal to the one you'd get when trying to execute a unknown argocd command
	assert.Equal(t, err, pluginErr)
}

// Test_Plugin_Execution_Error_When_Plugin_does_not_have_executable_permissions tests for a plugin that doesn't have executable permissions
func Test_Plugin_Execution_Error_When_Plugin_does_not_have_executable_permissions(t *testing.T) {
	pluginHandler := NewDefaultPluginHandler([]string{"argocd"})
	pluginHandler.lookPath = func(file string) (string, error) {
		if file == "argocd-no-permission" || file == "argocd-no" {
			pluginPath = ""
		}
		return pluginPath, nil
	}
	cmd := NewCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	args := []string{"argocd", "no-permission"}
	cmd.SetArgs(args[1:])

	err := cmd.Execute()
	require.Error(t, err)

	pluginErr := pluginHandler.HandleCommandExecutionError(err, true, args)
	assert.Equal(t, "", pluginPath)
	require.Error(t, pluginErr)
	assert.EqualError(t, pluginErr, "unknown command \"no-permission\" for \"argocd\"")
}

// Test_Plugin_Execution_Error tests for the error that you'd typically get from the execution of a plugin
func Test_Plugin_Execution_Error(t *testing.T) {
	pluginHandler := NewDefaultPluginHandler([]string{"argocd"})
	pluginHandler.lookPath = func(file string) (string, error) {
		if file == "argocd-foo" {
			pluginPath = "testdata/argocd-foo"
			return pluginPath, nil
		}
		return pluginPath, nil
	}
	cmd := NewCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	args := []string{"argocd", "foo"}
	cmd.SetArgs(args[1:])

	err := cmd.Execute()
	require.Error(t, err)

	pluginErr := pluginHandler.HandleCommandExecutionError(err, true, args)
	assert.Equal(t, "testdata/argocd-foo", pluginPath)
	require.Error(t, pluginErr)
	assert.EqualError(t, pluginErr, "exit status 1")
}
