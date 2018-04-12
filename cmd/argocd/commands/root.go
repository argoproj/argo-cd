package commands

import (
	"context"

	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/util/cli"
	util_config "github.com/argoproj/argo-cd/util/config"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/metadata"
	"k8s.io/client-go/tools/clientcmd"
)

// DefaultClientContext assembles a context with proper auth tokens, etc.
func DefaultClientContext(clientOpts *argocdclient.ClientOptions) context.Context {
	ctx := context.Background()

	localConfig, err := util_config.ReadLocalConfig()
	if err != nil {
		log.Fatal(err)
	}

	token, ok := localConfig.Sessions[clientOpts.ServerAddr]
	if ok {
		ctx = metadata.AppendToOutgoingContext(ctx, "tokens", token)
	}
	return ctx
}

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
	command.AddCommand(NewLoginCommand(&clientOpts))
	command.AddCommand(NewRepoCommand(&clientOpts))
	command.AddCommand(NewInstallCommand())
	command.AddCommand(NewUninstallCommand())

	command.PersistentFlags().StringVar(&clientOpts.ServerAddr, "server", "", "ArgoCD server address")
	command.PersistentFlags().BoolVar(&clientOpts.Insecure, "insecure", false, "Disable transport security for the client connection, including host verification")
	command.PersistentFlags().StringVar(&clientOpts.CertFile, "server-crt", "", "Server certificate file")
	return command
}
