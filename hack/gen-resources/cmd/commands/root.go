package commands

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

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

	command.PersistentFlags().IntVar(&generateOpts.Samples, "samples", 0, "Amount of samples")
	command.PersistentFlags().StringVar(&installCmdOptions.Kube.Namespace, "kube-namespace", viper.GetString("kube-namespace"), "Name of the namespace on which Argo agent should be installed [$KUBE_NAMESPACE]")
	command.PersistentFlags().StringVar(&installCmdOptions.Kube.Context, "kube-context-name", viper.GetString("kube-context"), "Name of the kubernetes context on which Argo agent should be installed (default is current-context) [$KUBE_CONTEXT]")

	return command
}
