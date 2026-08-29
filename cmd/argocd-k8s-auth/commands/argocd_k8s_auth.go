package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/common"
)

type verboseContextKey struct{}

func contextWithVerbose(ctx context.Context, verbose bool) context.Context {
	return context.WithValue(ctx, verboseContextKey{}, verbose)
}

func verboseFromContext(ctx context.Context) bool {
	verbose, _ := ctx.Value(verboseContextKey{}).(bool)
	return verbose
}

func verboseLog(ctx context.Context, format string, args ...any) {
	if verboseFromContext(ctx) {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
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

	command.PersistentFlags().Bool("verbose", false, "Enable verbose logging to stderr for troubleshooting")
	command.AddCommand(newAWSCommand())
	command.AddCommand(newGCPCommand())
	command.AddCommand(newAzureCommand())

	return command
}
