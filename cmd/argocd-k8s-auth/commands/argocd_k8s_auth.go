package commands

import (
	"github.com/spf13/cobra"
)

const (
	cliName = "argocd-k8s-auth"
)

func NewCommand() *cobra.Command {
	command := &cobra.Command{
		Use:               cliName,
		Short:             "argocd-k8s-auth a set of commands to generate k8s auth token",
		DisableAutoGenTag: true,
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}

	command.AddCommand(newAWSCommand())
	command.AddCommand(newGCPCommand())
	command.AddCommand(newAzureCommand())

	return command
}
