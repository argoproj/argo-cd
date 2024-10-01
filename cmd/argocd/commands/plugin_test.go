package commands

import (
	"fmt"
	"github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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
	executed       bool
	executedPlugin string
	withArgs       []string
	withEnv        []string
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

func (t testPluginHandler) ExecutePlugin(executablePath string, cmdArgs, environment []string) error {
	t.executed = true
	t.executedPlugin = executablePath
	t.withArgs = cmdArgs
	t.withEnv = environment
	return nil
}

func Test_ArgoCDPluginHandler(t *testing.T) {
	tests := []struct {
		name              string
		args              []string
		expectedPlugin    string
		expectPluginArgs  []string
		expectLookupError string
	}{
		{
			name:             "test that normal commands are able to be executed, when no plugin overshadows them",
			args:             []string{"argocd", "cluster", "list"},
			expectedPlugin:   "",
			expectPluginArgs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pluginsHandler := &testPluginHandler{
				pluginsDirectory: "testdata",
				validPrefixes:    []string{"argocd"},
			}
			root := NewDefaultArgoCDCommandWithArgs(util.ArgoCDCLIOptions{
				PluginHandler: pluginsHandler,
				Arguments:     tt.args,
			})

			if !pluginsHandler.lookedup && !pluginsHandler.executed {
				// args must be set, otherwise Execute will use os.Args (args used for starting the test) and test.args would not be passed
				// to the command which might invoke only "argocd" without any additional args and give false positives
				root.SetArgs(tt.args[1:])
				// Important note! Incorrect command or command failing validation might just call os.Exit(1) which would interrupt execution of the test
				if err := root.Execute(); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}

			if (pluginsHandler.lookupErr != nil && pluginsHandler.lookupErr.Error() != tt.expectLookupError) ||
				(pluginsHandler.lookupErr == nil && len(tt.expectLookupError) > 0) {
				t.Fatalf("unexpected error: expected %q to occur, but got %q", tt.expectLookupError, pluginsHandler.lookupErr)
			}

			if pluginsHandler.lookedup && !pluginsHandler.executed && len(tt.expectLookupError) == 0 {
				// we have to fail here, because we have found the plugin, but not executed the plugin, nor the command (this would normally result in an error: unknown command)
				t.Fatalf("expected plugin execution, but did not occur")
			}

			if pluginsHandler.executedPlugin != tt.expectedPlugin {
				t.Fatalf("unexpected plugin execution: expected %q, got %q", tt.expectedPlugin, pluginsHandler.executedPlugin)
			}

			if pluginsHandler.executed && len(tt.expectedPlugin) == 0 {
				t.Fatalf("unexpected plugin execution: expected no plugin, got %q", pluginsHandler.executedPlugin)
			}

			if !cmp.Equal(pluginsHandler.withArgs, tt.expectPluginArgs, cmpopts.EquateEmpty()) {
				t.Fatalf("unexpected plugin execution args: expected %q, got %q", tt.expectPluginArgs, pluginsHandler.withArgs)
			}
		})
	}
}
