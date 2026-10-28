package initialize

import (
	"slices"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/argoproj/argo-cd/v3/util/cli"
)

const coreKubeConfigAnnotation = "argocd.argoproj.io/core-kubeconfig"

// MarkCoreKubeConfigFlag distinguishes core API access from command-specific Kubernetes configuration.
func MarkCoreKubeConfigFlag(flag *pflag.Flag) {
	if flag.Annotations == nil {
		flag.Annotations = map[string][]string{}
	}
	flag.Annotations[coreKubeConfigAnnotation] = []string{"true"}
}

// IsCoreKubeConfigFlag reports whether a kubeconfig flag selects the core-mode management cluster.
func IsCoreKubeConfigFlag(flag *pflag.Flag) bool {
	return flag != nil && len(flag.Annotations[coreKubeConfigAnnotation]) > 0 && flag.Annotations[coreKubeConfigAnnotation][0] == "true"
}

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
		"tls-server-name",
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
		// Core-mode subcommands must be able to override a saved kubeconfig path.
		if flag.Name == "kubeconfig" {
			MarkCoreKubeConfigFlag(flag)
			cmd.PersistentFlags().AddFlag(flag)
			return
		}
		cmd.Flags().AddFlag(flag)
	})

	return cmd
}
