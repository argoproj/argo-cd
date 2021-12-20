package commands

import (
	"github.com/spf13/cobra"

	generator "github.com/argoproj/argo-cd/v2/performance-test/generators"

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

	command.AddCommand(NewProjectGenerationCommand(&generateOpts))
	command.AddCommand(NewApplicationGenerationCommand(&generateOpts))

	command.PersistentFlags().IntVar(&generateOpts.Samples, "samples", 0, "Amount of samples")

	return command
}
