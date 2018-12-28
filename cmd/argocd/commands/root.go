package commands

import (
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/localconfig"
)

func init() {
	cobra.OnInitialize(initConfig)
}

var logLevel string

func initConfig() {
	cli.SetLogLevel(logLevel)
}

// NewCommand returns a new instance of an argocd command
func NewCommand() *cobra.Command {
	var (
		clientOpts argocdclient.ClientOptions
		pathOpts   = clientcmd.NewDefaultPathOptions()
	)

	var command = &cobra.Command{
		Use:   cliName,
		Short: "argocd controls a Argo CD server",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}

	command.AddCommand(NewVersionCmd(&clientOpts))
	command.AddCommand(NewClusterCommand(&clientOpts, pathOpts))
	command.AddCommand(NewApplicationCommand(&clientOpts))
	command.AddCommand(NewLoginCommand(&clientOpts))
	command.AddCommand(NewReloginCommand(&clientOpts))
	command.AddCommand(NewRepoCommand(&clientOpts))
	command.AddCommand(NewContextCommand(&clientOpts))
	command.AddCommand(NewProjectCommand(&clientOpts))
	command.AddCommand(NewAccountCommand(&clientOpts))
	command.AddCommand(NewRolloutCommand(&clientOpts))

	defaultLocalConfigPath, err := localconfig.DefaultLocalConfigPath()
	errors.CheckError(err)
	command.PersistentFlags().StringVar(&clientOpts.ConfigPath, "config", defaultLocalConfigPath, "Path to Argo CD config")
	command.PersistentFlags().StringVar(&clientOpts.ServerAddr, "server", "", "Argo CD server address")
	command.PersistentFlags().BoolVar(&clientOpts.PlainText, "plaintext", false, "Disable TLS")
	command.PersistentFlags().BoolVar(&clientOpts.Insecure, "insecure", false, "Skip server certificate and domain verification")
	command.PersistentFlags().StringVar(&clientOpts.CertFile, "server-crt", "", "Server certificate file")
	command.PersistentFlags().StringVar(&clientOpts.AuthToken, "auth-token", "", "Authentication token")
	command.PersistentFlags().StringVar(&logLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	return command
}
