package initialize

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/argoproj/argo-cd/v2/util/cli"
)

func RetrieveContextIfChanged(contextFlag *pflag.Flag) string {
	if contextFlag != nil && contextFlag.Changed {
		return contextFlag.Value.String()
	}
	return ""
}

// InitCommand allows executing command in a headless mode: on the fly starts Argo CD API server and
// changes provided client options to use started API server port
func InitCommand(cmd *cobra.Command) *cobra.Command {
	flags := pflag.NewFlagSet("tmp", pflag.ContinueOnError)
	cli.AddKubectlFlagsToSet(flags)
	// copy k8s persistent flags into argocd command flags
	flags.VisitAll(func(flag *pflag.Flag) {
		// skip Kubernetes server flags since argocd has it's own server flag
		if flag.Name == "server" {
			return
		}
		cmd.Flags().AddFlag(flag)
	})
	return cmd
}
