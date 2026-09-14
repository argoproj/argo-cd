package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				os.Exit(0) // when here, no error - exit the subprocess
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
