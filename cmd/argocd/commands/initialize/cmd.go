package initialize

import (
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

func InitCommand(cmd *cobra.Command) *cobra.Command {
	flags := pflag.NewFlagSet("tmp", pflag.ContinueOnError)
	cli.AddKubectlFlagsToSet(flags)

	// kubectl REST flags that are not supported by argocd CLI
	unsupportedFlags := map[string]bool{
		"disable-compression":   true,
		"certificate-authority": true,
		"client-certificate":    true,
		"client-key":            true,
		"as":                    true,
		"as-group":              true,
		"as-uid":                true,
	}

	flags.VisitAll(func(flag *pflag.Flag) {
		// skip Kubernetes server flags since argocd has its own server flag
		if flag.Name == "server" {
			return
		}
		// skip unsupported kubectl REST flags
		if unsupportedFlags[flag.Name] {
			return
		}
		cmd.Flags().AddFlag(flag)
	})

	return cmd
}
