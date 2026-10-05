package commands

import (
	"bytes"
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/cmd/util"
)

// runCmd executes cmd with args and returns cobra stdout, stderr plus the RunE error.
// Cobra's automatic error and usage printing is silenced as in the CLI.
// ExitError is returned as-is (main.go uses errors.Fatal and os.Exit) - tests should assert on the error value.
// Other errors are formatted like main.go (plugin handler) and appended to stderr buffer.
func runCmd(t *testing.T, cmd *cobra.Command, args ...string) (stdout string, stderr string, e error) {
	t.Helper()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	cmd.SetArgs(args)

	var outbuf bytes.Buffer
	cmd.SetOut(&outbuf)
	var errbuf bytes.Buffer
	cmd.SetErr(&errbuf)

	err := cmd.ExecuteContext(t.Context())
	if err != nil {
		if _, ok := errors.AsType[*util.ExitError](err); ok {
			// main.go uses errors.Fatal and os.Exit - do not os.Exit here
			// return the error as is, tests should assert on the error value
			return outbuf.String(), errbuf.String(), err
		}

		args := append([]string{"argocd"}, args...) // plugin handler expects the first argument to be binary name
		errMsg, _ := NewDefaultPluginHandler().HandleCommandExecutionError(err, true, args)
		if errMsg != "" {
			errbuf.WriteString(errMsg)
		}
	}

	return outbuf.String(), errbuf.String(), err
}
