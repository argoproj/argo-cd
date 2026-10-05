package commands

import (
	"bytes"
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/cmd/util"
)

// Helper function to run a cobra command and capture its outputs and returned error.
// Mimics the main.go error handling of errors returned by RunE (appends the cli error message to the stderr buffer).
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
	// Make sure the messare from the error reported by Command.RunE() is appended to the errbuf for verification (similar to main.go)
	if err != nil {
		if e, ok := errors.AsType[*util.ExitError](err); ok {
			if util.CLIMessageForError(e) != "" {
				errbuf.WriteString(util.CLIMessageForError(e))
				errbuf.WriteString("\n")
			}

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
