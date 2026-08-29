package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/common"
)

type verboseContextKey struct{}

type verboseState struct {
	enabled bool
	w       io.Writer
}

func contextWithVerbose(ctx context.Context, verbose bool, w io.Writer) context.Context {
	if w == nil {
		w = os.Stderr
	}
	return context.WithValue(ctx, verboseContextKey{}, verboseState{enabled: verbose, w: w})
}

// contextWithVerboseFromCmd reads the persistent --verbose flag and stores it on
// the request context together with the command's error writer (ErrOrStderr).
func contextWithVerboseFromCmd(c *cobra.Command) (context.Context, error) {
	verbose, err := c.Flags().GetBool("verbose")
	if err != nil {
		return nil, err
	}
	ctx := c.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	return contextWithVerbose(ctx, verbose, c.ErrOrStderr()), nil
}

func verboseFromContext(ctx context.Context) bool {
	state, _ := ctx.Value(verboseContextKey{}).(verboseState)
	return state.enabled
}

func verboseWriterFromContext(ctx context.Context) io.Writer {
	state, ok := ctx.Value(verboseContextKey{}).(verboseState)
	if !ok || state.w == nil {
		return os.Stderr
	}
	return state.w
}

func verboseLog(ctx context.Context, format string, args ...any) {
	if verboseFromContext(ctx) {
		fmt.Fprintf(verboseWriterFromContext(ctx), format+"\n", args...)
	}
}

// redactARN returns a shortened form of an IAM role ARN suitable for logs.
// Full account IDs are omitted; only the role path/name is kept.
// Non-ARN values are returned unchanged; empty input stays empty.
func redactARN(arn string) string {
	if arn == "" {
		return ""
	}
	if !strings.HasPrefix(arn, "arn:") {
		return arn
	}
	if i := strings.Index(arn, ":role/"); i >= 0 {
		return arn[i+1:] // role/Name
	}
	if i := strings.LastIndex(arn, "/"); i >= 0 && i+1 < len(arn) {
		return arn[i+1:]
	}
	return "<redacted>"
}

func NewCommand() *cobra.Command {
	command := &cobra.Command{
		Use:               common.CommandK8sAuth,
		Short:             "argocd-k8s-auth a set of commands to generate k8s auth token",
		DisableAutoGenTag: true,
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}

	command.PersistentFlags().Bool("verbose", false, "Log auth flow details to stderr for troubleshooting. May include internal details such as cluster names and role names (ARNs are redacted). Does not write to stdout, which remains JSON token output only.")
	command.AddCommand(newAWSCommand())
	command.AddCommand(newGCPCommand())
	command.AddCommand(newAzureCommand())

	return command
}
