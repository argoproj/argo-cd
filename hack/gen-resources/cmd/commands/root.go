package commands

import (
	"github.com/spf13/cobra"

	generator "github.com/argoproj/argo-cd/v2/hack/gen-resources/generators"

	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/util/cli"
)

const (
	cliName = "argocd-generator"
)

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	cli.SetLogFormat(cmdutil.LogFormat)
	cli.SetLogLevel(cmdutil.LogLevel)
}

// NewCommand returns a new instance of an argocd command
func NewCommand() *cobra.Command {
	var generateOpts generator.GenerateOpts
	var command = &cobra.Command{
		Use:   cliName,
		Short: "Generator for argocd resources",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
		DisableAutoGenTag: true,
	}
	command.AddCommand(NewProjectCommand(&generateOpts))
	command.AddCommand(NewApplicationCommand(&generateOpts))
	command.AddCommand(NewAllResourcesCommand(&generateOpts))
	command.AddCommand(NewReposCommand(&generateOpts))
	command.PersistentFlags().StringVar(&generateOpts.Namespace, "kube-namespace", "argocd", "Name of the namespace on which Argo agent should be installed [$KUBE_NAMESPACE]")
	return command
}
