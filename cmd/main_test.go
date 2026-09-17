package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/cmd/util"
)

func TestExitErrorHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		cmdError         error
		expectedExitCode int
		expectedOutput   string
	}{
		{
			name:             "no error",
			cmdError:         nil,
			expectedExitCode: 0,
			expectedOutput:   "",
		},
		{
			name:             "generic error",
			cmdError:         errors.New("test error"),
			expectedExitCode: 1,
			expectedOutput:   "Error: test error\n",
		},
		{
			name:             "exit error 1 without message",
			cmdError:         util.NewExitError(1, nil),
			expectedExitCode: 1,
			expectedOutput:   "",
		},
		{
			name:             "exit error 1 with message",
			cmdError:         util.NewExitError(1, errors.New("test error")),
			expectedExitCode: 1,
			expectedOutput:   "Error: test error\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if os.Getenv("BE_CRASHER") == "1" {
				dummyCmd := &cobra.Command{
					RunE: func(_ *cobra.Command, _ []string) error {
						return test.cmdError
					},
				}

				// mock command selection in main
				selectCommand = func(_ string) (*cobra.Command, bool) {
					return dummyCmd, true
				}

				main()     // in case of error calls os.Exit
				os.Exit(0) // when here, no error - exit OK
			}

			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+t.Name()+"$")
			cmd.Env = append(os.Environ(), "BE_CRASHER=1")
			var stdout bytes.Buffer
			cmd.Stdout = &stdout
			err := cmd.Run()

			if test.expectedExitCode == 0 {
				require.NoError(t, err, "expected command to exit successfully")
				assert.Equal(t, test.expectedOutput, stdout.String())
				return // passed
			}

			if e, ok := errors.AsType[*exec.ExitError](err); ok && !e.Success() {
				assert.Equal(t, test.expectedExitCode, e.ExitCode(), "expected exit code to be %d but got %d", test.expectedExitCode, e.ExitCode())
				assert.Equal(t, test.expectedOutput, stdout.String())
				return
			}

			t.Fatalf("process ran with err %v, want exit status %d", err, test.expectedExitCode)
		})
	}
}

func TestExitErrorHandlingWithPlugin(t *testing.T) {
	t.Parallel()

	statusCodePluginCmdErr := errors.New(`unknown command "status-code-plugin" for "argocd"`)
	unknownNoPluginErr := errors.New(`unknown command "non-existent" for "argocd"`)
	pluginPath := getTestPluginsPath(t)

	tests := []struct {
		name             string
		args             []string
		cmdError         error
		expectedExitCode int
		expectedOutput   string
	}{
		{
			name:             "plugin no error",
			args:             []string{"argocd", "status-code-plugin", "--flag1", "value1"},
			cmdError:         statusCodePluginCmdErr,
			expectedExitCode: 0,
			expectedOutput:   "Flag1 detected: value1\n",
		},
		{
			name:             "plugin exit 1",
			args:             []string{"argocd", "status-code-plugin", "--flag3", "value3"},
			cmdError:         statusCodePluginCmdErr,
			expectedExitCode: 1,
			expectedOutput:   "Error: exit status 1\n",
		},
		{
			name:             "plugin exit 127",
			args:             []string{"argocd", "status-code-plugin", "invalid"},
			cmdError:         statusCodePluginCmdErr,
			expectedExitCode: 127,
			expectedOutput:   "Error: exit status 127\n",
		},
		{
			name:             "plugin not found",
			args:             []string{"argocd", "non-existent"},
			cmdError:         unknownNoPluginErr,
			expectedExitCode: 1,
			expectedOutput:   fmt.Sprintf("Error: %v\nRun 'argocd --help' for usage.\n", unknownNoPluginErr),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if os.Getenv("BE_CRASHER_PLUGIN") == "1" {
				dummyCmd := &cobra.Command{
					DisableFlagParsing: true,
					RunE: func(_ *cobra.Command, _ []string) error {
						return test.cmdError // unknown command error triggers the plugin handling
					},
				}

				selectCommand = func(_ string) (*cobra.Command, bool) {
					return dummyCmd, true
				}

				os.Args = test.args

				main()
				os.Exit(0) // when here, no error - exit OK
			}

			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+t.Name()+"$")
			cmd.Env = envWithPath(pluginPath)
			cmd.Env = append(cmd.Env, "BE_CRASHER_PLUGIN=1")
			var stdout bytes.Buffer
			cmd.Stdout = &stdout
			err := cmd.Run()

			if test.expectedExitCode == 0 {
				require.NoError(t, err, "expected command to exit successfully")
				assert.Equal(t, test.expectedOutput, stdout.String())
				return
			}

			if e, ok := errors.AsType[*exec.ExitError](err); ok && !e.Success() {
				assert.Equal(t, test.expectedExitCode, e.ExitCode(), "expected exit code to be %d but got %d", test.expectedExitCode, e.ExitCode())
				assert.Equal(t, test.expectedOutput, stdout.String())
				return
			}

			t.Fatalf("process ran with err %v, want exit status %d", err, test.expectedExitCode)
		})
	}
}

func TestExitErrorHandlingNotIsArgocdCLI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		args             []string
		cmdError         error
		expectedExitCode int
		expectedOutput   string
	}{
		{
			name:             "no error",
			cmdError:         nil,
			expectedExitCode: 0,
			expectedOutput:   "",
		},
		{
			name:             "generic error",
			cmdError:         errors.New("test error"),
			expectedExitCode: 1,
			expectedOutput:   "Error: test error\n",
		},
		{
			name:             "exit error 1 without message",
			cmdError:         util.NewExitError(1, nil),
			expectedExitCode: 1,
			expectedOutput:   "",
		},
		{
			name:             "exit error 1 with message",
			cmdError:         util.NewExitError(1, errors.New("test error")),
			expectedExitCode: 1,
			expectedOutput:   "Error: test error\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if os.Getenv("BE_CRASHER_NOT_IS_ARG_CLI") == "1" {
				dummyCmd := &cobra.Command{
					RunE: func(_ *cobra.Command, _ []string) error {
						return test.cmdError
					},
				}

				selectCommand = func(_ string) (*cobra.Command, bool) {
					return dummyCmd, false
				}

				main()
				os.Exit(0) // when here, no error - exit OK
			}

			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+t.Name()+"$")
			cmd.Env = append(os.Environ(), "BE_CRASHER_NOT_IS_ARG_CLI=1")
			var stdout bytes.Buffer
			cmd.Stdout = &stdout
			err := cmd.Run()

			if test.expectedExitCode == 0 {
				require.NoError(t, err, "expected command to exit successfully")
				assert.Equal(t, test.expectedOutput, stdout.String())
				return
			}

			if e, ok := errors.AsType[*exec.ExitError](err); ok && !e.Success() {
				assert.Equal(t, test.expectedExitCode, e.ExitCode(), "expected exit code to be %d but got %d", test.expectedExitCode, e.ExitCode())
				assert.Equal(t, test.expectedOutput, stdout.String())
				return
			}

			t.Fatalf("process ran with err %v, want exit status %d", err, test.expectedExitCode)
		})
	}
}

func getTestPluginsPath(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	return filepath.Join(wd, "argocd", "commands", "testdata")
}

func envWithPath(path string) []string {
	env := os.Environ()
	pathSet := false
	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			env[i] = "PATH=" + path
			pathSet = true
		}
	}
	if !pathSet {
		env = append(env, "PATH="+path)
	}
	return env
}
