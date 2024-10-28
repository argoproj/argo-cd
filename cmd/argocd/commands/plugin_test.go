package commands

import (
	"bytes"
	"fmt"
	"os"
	"testing"

)

type testPluginHandler struct {
	pluginsDirectory string
	validPrefixes    []string

	// lookup results
	lookedup  bool
	lookupErr error

	// execution results
	executed           bool
	executedPluginPath string
	withArgs           []string
	withEnv            []string
}

func (t *testPluginHandler) LookForPlugin(filename string) (string, bool) {
	t.lookedup = true

	dir, err := os.Stat(t.pluginsDirectory)
	if err != nil {
		t.lookupErr = err
		return "", false
	}

	if !dir.IsDir() {
		t.lookupErr = fmt.Errorf("expected %q to be a directory", t.pluginsDirectory)
		return "", false
	}

	plugins, err := os.ReadDir(t.pluginsDirectory)
	if err != nil {
		t.lookupErr = err
		return "", false
	}

	filenameWithSuportedPrefix := ""
	for _, prefix := range t.validPrefixes {
		for _, p := range plugins {
			filenameWithSuportedPrefix = fmt.Sprintf("%s-%s", prefix, filename)
			if p.Name() == filenameWithSuportedPrefix {
				t.lookupErr = nil
				return fmt.Sprintf("%s/%s", t.pluginsDirectory, p.Name()), true
			}
		}
	}

	t.lookupErr = fmt.Errorf("unable to find a plugin executable %q", filenameWithSuportedPrefix)
	return "", false
}

func (t *testPluginHandler) ExecutePlugin(executablePath string, cmdArgs, environment []string) error {
	t.executed = true
	t.executedPluginPath = executablePath
	t.withArgs = cmdArgs
	t.withEnv = environment
	return nil
}

// Test_ArgoCD_Normal_Command_Successful_Execution tests the successful execution of a normal Argo CD CLI command.
// Even if a plugin with the same name as normal command exists, the normal command will get executed.
func Test_ArgoCD_Normal_Command_Successful_Execution(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := NewVersionCmd(&argocdclient.ClientOptions{}, nil)
	cmd.SetOut(buf)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	pluginsHandler := &testPluginHandler{
		pluginsDirectory:   "testdata",
		validPrefixes:      []string{"argocd"},
		executedPluginPath: "testdata/argocd-version",
	}
	o := ArgoCDCLIOptions{
		PluginHandler: pluginsHandler,
		Arguments:     []string{"argocd", "version", "--short", "--client"},
	}
	cmd.SetArgs(o.Arguments[1:])
	err := cmd.Execute()
	err = HandleCommandExecutionError(err, true, o)
	output := buf.String()
	assert.Equal(t, "argocd: v99.99.99+unknown\n", output)
	require.NoError(t, err)
}

// Test_ArgoCD_Plugin_Successful_Execution tests for the successful execution of a plugin found in the plugin directory
func Test_ArgoCD_Plugin_Successful_Execution(t *testing.T) {
	cmd := NewCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	pluginsHandler := &testPluginHandler{
		pluginsDirectory:   "testdata",
		validPrefixes:      []string{"argocd"},
		executedPluginPath: "testdata/argocd-foo",
	}
	o := ArgoCDCLIOptions{
		PluginHandler: pluginsHandler,
		Arguments:     []string{"argocd", "foo"},
	}
	cmd.SetArgs(o.Arguments[1:])

	err := cmd.Execute()
	require.Error(t, err, "unknown command \"foo\" for \"argocd\"")

	err = HandleCommandExecutionError(err, true, o)
	require.NoError(t, err)
}

// Test_CommandIsNeitherNormalCommandNorExistsAsPlugin checks when a command is neither a normal Argo CD CLI command nor Plugin
func Test_CommandIsNeitherNormalCommandNorExistsAsPlugin(t *testing.T) {
	cmd := NewCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	pluginsHandler := &testPluginHandler{
		pluginsDirectory:   "testdata",
		validPrefixes:      []string{"argocd"},
		executedPluginPath: "",
	}
	o := ArgoCDCLIOptions{
		PluginHandler: pluginsHandler,
		Arguments:     []string{"argocd", "nonexistent"},
	}
	cmd.SetArgs(o.Arguments[1:])

	err := cmd.Execute()
	require.Error(t, err, "unknown command \"nonexistent\" for \"argocd\"")

	pluginError := HandleCommandExecutionError(err, true, o)
	require.Equal(t, pluginError, err)
}
