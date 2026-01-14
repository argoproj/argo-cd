package initialize

import (
	"slices"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/argoproj/argo-cd/v3/util/cli"
)

func RetrieveContextIfChanged(contextFlag *pflag.Flag) string {
	if contextFlag != nil && contextFlag.Changed {
		return contextFlag.Value.String()
	}
	return ""
}

// InitCommand allows executing commands in a headless mode by internally
// initializing an Argo CD API server and updating client options to use
// the server's listening port.
func InitCommand(cmd *cobra.Command) *cobra.Command {
	flags := pflag.NewFlagSet("tmp", pflag.ContinueOnError)
	cli.AddKubectlFlagsToSet(flags)

	// kubectl REST flags that are not supported by argocd CLI
	unsupportedFlags := []string{
		"disable-compression",
		"certificate-authority",
		"client-certificate",
		"client-key",
		"as",
		"as-group",
		"as-uid",
	}

	flags.VisitAll(func(flag *pflag.Flag) {
		// skip Kubernetes server flags since argocd has its own server flag
		if flag.Name == "server" {
			return
		}
		// skip unsupported kubectl REST flags
		if slices.Contains(unsupportedFlags, flag.Name) {
			return
		}
		cmd.Flags().AddFlag(flag)
	})

	return cmd
}
