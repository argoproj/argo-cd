package commands

import (
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

func Test_ArgoCDPluginHandler(t *testing.T) {
	cmd := NewCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	tests := []struct {
		name               string
		args               []string
		expectedPluginPath string
		expectPluginArgs   []string
		expectLookupError  string
	}{
		{
			name:               "test that normal commands are able to be executed, when no plugin overshadows them",
			args:               []string{"argocd", "cluster", "list"},
			expectedPluginPath: "",
			expectPluginArgs:   []string{},
		},
		{
			name:               "test that a plugin executable is found based on command args",
			args:               []string{"argocd", "foo"},
			expectedPluginPath: "testdata/argocd-foo",
			expectPluginArgs:   []string{},
		},
		{
			name:               "test that the normal command is executed if the plugin name is same as the command",
			args:               []string{"argocd", "cluster", "list"},
			expectedPluginPath: "testdata/argocd-cluster-list",
			expectPluginArgs:   []string{},
		},
		{
			name:               "test that the command is neither a plugin nor a normal argocd command",
			args:               []string{"argocd", "foos"},
			expectedPluginPath: "",
			expectPluginArgs:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pluginsHandler := &testPluginHandler{
				pluginsDirectory:   "testdata",
				validPrefixes:      []string{"argocd"},
				executedPluginPath: tt.expectedPluginPath,
			}
			o := ArgoCDCLIOptions{
				PluginHandler: pluginsHandler,
				Arguments:     tt.args,
			}
			cmd.SetArgs(tt.args[1:])
			err := cmd.Execute()
			HandleCommandExecutionError(err, true, o)
			// TODO

			//if pluginsHandler.lookedup && !pluginsHandler.executed && len(tt.expectLookupError) == 0 {
			//	// we have to fail here, because we have found the plugin, but not executed the plugin, nor the command (this would normally result in an error: unknown command)
			//	t.Fatalf("expected plugin execution, but did not occur")
			//}
		})
	}
}

//if (pluginsHandler.lookupErr != nil && pluginsHandler.lookupErr.Error() != tt.expectLookupError) ||
//(pluginsHandler.lookupErr == nil && len(tt.expectLookupError) > 0) {
//t.Fatalf("unexpected error: expected %q to occur, but got %q", tt.expectLookupError, pluginsHandler.lookupErr)
//}
//

//
//if pluginsHandler.executedPluginPath != tt.expectedPluginPath {
//t.Fatalf("unexpected plugin execution: expected %q, got %q", tt.expectedPluginPath, pluginsHandler.executedPluginPath)
//}
//
//if pluginsHandler.executed && len(tt.expectedPluginPath) == 0 {
//t.Fatalf("unexpected plugin execution: expected no plugin, got %q", pluginsHandler.executedPluginPath)
//}
//
//if !cmp.Equal(pluginsHandler.withArgs, tt.expectPluginArgs, cmpopts.EquateEmpty()) {
//t.Fatalf("unexpected plugin execution args: expected %q, got %q", tt.expectPluginArgs, pluginsHandler.withArgs)
//}
