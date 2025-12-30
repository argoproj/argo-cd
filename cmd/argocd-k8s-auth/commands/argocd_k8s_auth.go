package commands

import (
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/util/cli"
)

const (
	cliName = "argocd-k8s-auth"
)

var (
	logFormat string
	logLevel  string
)

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	cli.SetLogFormat(logFormat)
	cli.SetLogLevel(logLevel)
}

func NewCommand() *cobra.Command {
	command := &cobra.Command{
		Use:               cliName,
		Short:             "argocd-k8s-auth tools used for generating authentication tokens",
		DisableAutoGenTag: true,
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}

	command.PersistentFlags().StringVar(&logFormat, "logformat", "text", "Set the logging format. One of: text|json")
	command.PersistentFlags().StringVar(&logLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")

	command.AddCommand(newAWSCommand())
	command.AddCommand(newGCPCommand())
	command.AddCommand(newAzureCommand())
	command.AddCommand(cli.NewVersionCmd(cliName))

	return command
}
