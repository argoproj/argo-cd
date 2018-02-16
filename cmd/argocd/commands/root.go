package commands

import (
	"github.com/argoproj/argo-cd/util/cmd"
	"github.com/spf13/cobra"
)

// NewCommand returns a new instance of an argocd command
func NewCommand() *cobra.Command {
	var (
		logLevel string
	)
	var command = &cobra.Command{
		Use:   cliName,
		Short: "argocd controls a ArgoCD server",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}

	command.Flags().StringVar(&logLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	command.AddCommand(cmd.NewVersionCmd(cliName))
	command.AddCommand(NewClusterCommand())
	return command
}
