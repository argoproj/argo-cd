package commands

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
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
	// Make sure the messare from the error reported by Command.RunE() is appended to the errbuf for verification (same as in main.go)
	if err != nil {
		errMsg, _ := NewDefaultPluginHandler().HandleCommandExecutionError(err, true, args)
		if errMsg != "" {
			errbuf.WriteString(errMsg)
			errbuf.WriteString("\n")
		}
	}

	return outbuf.String(), errbuf.String(), err
}
