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

// supportedKubectlFlags is the explicit allowlist of kubectl-like flags that
// InitCommand copies onto argocd commands. The kubectl flag set is derived
// from client-go (cli.AddKubectlFlagsToSet registers every REST config
// override flag kubectl supports), but the argocd CLI only consumes a subset
// of it. Flags used to be copied deny-by-default with a denylist of
// known-unsupported flags, so every flag added upstream in client-go leaked
// into the argocd CLI as a silently ignored option until it was denylisted
// individually (#25977, #27711, #18198). Copying allow-by-default inverts
// that: a new client-go flag stays out of the argocd CLI until it is
// deliberately supported and added here.
//
// "context" is read back by headless (core) mode via RetrieveContextIfChanged.
// The remaining flags are kept so that existing command lines keep parsing
// exactly as before; removing any of them would turn a previously accepted
// flag into an "unknown flag" error.
var supportedKubectlFlags = []string{
	"cluster",
	"context",
	"insecure-skip-tls-verify",
	"kubeconfig",
	"namespace",
	"password",
	"proxy-url",
	"token",
	"user",
	"username",
}

// InitCommand allows executing commands in a headless mode by internally
// initializing an Argo CD API server and updating client options to use
// the server's listening port.
func InitCommand(cmd *cobra.Command) *cobra.Command {
	flags := pflag.NewFlagSet("tmp", pflag.ContinueOnError)
	cli.AddKubectlFlagsToSet(flags)

	// only copy the kubectl flags that the argocd CLI supports
	flags.VisitAll(func(flag *pflag.Flag) {
		if slices.Contains(supportedKubectlFlags, flag.Name) {
			cmd.Flags().AddFlag(flag)
		}
	})

	return cmd
}
