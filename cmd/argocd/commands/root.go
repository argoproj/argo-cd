package commands

import (
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

// NewCommand returns a new instance of an argocd command
func NewCommand() *cobra.Command {
	var (
		clientOpts argocdclient.ClientOptions
		pathOpts   = clientcmd.NewDefaultPathOptions()
	)

	var command = &cobra.Command{
		Use:   cliName,
		Short: "argocd controls a ArgoCD server",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}

	command.AddCommand(cli.NewVersionCmd(cliName))
	command.AddCommand(NewClusterCommand(&clientOpts, pathOpts))
	command.AddCommand(NewApplicationCommand(&clientOpts))
	command.AddCommand(NewRepoCommand(&clientOpts))
	command.AddCommand(NewInstallCommand())

	command.PersistentFlags().StringVar(&clientOpts.ServerAddr, "server", "", "ArgoCD server address")
	command.PersistentFlags().BoolVar(&clientOpts.Insecure, "insecure", true, "Disable transport security for the client connection")

	return command
}
