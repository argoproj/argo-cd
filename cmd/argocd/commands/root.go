package commands

import (
	"github.com/argoproj/argo-cd/util/cmd"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	// load the gcp plugin (required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// load the oidc plugin (required to authenticate with OpenID Connect).
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

type globalFlags struct {
	kubeConfigOverrides clientcmd.ConfigOverrides
	kubeConfigPath      string
	logLevel            string
}

// NewCommand returns a new instance of an argocd command
func NewCommand() *cobra.Command {
	var (
		globalArgs globalFlags
	)
	var command = &cobra.Command{
		Use:   cliName,
		Short: "argocd controls a ArgoCD server",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}

	command.PersistentFlags().StringVar(&globalArgs.kubeConfigPath, "kubeconfig", "", "Path to the config file to use for CLI requests.")
	globalArgs.kubeConfigOverrides = clientcmd.ConfigOverrides{}
	clientcmd.BindOverrideFlags(&globalArgs.kubeConfigOverrides, command.PersistentFlags(), clientcmd.RecommendedConfigOverrideFlags(""))
	command.PersistentFlags().StringVar(&globalArgs.logLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")

	command.AddCommand(cmd.NewVersionCmd(cliName))
	command.AddCommand(NewClusterCommand())
	command.AddCommand(NewApplicationCommand())
	command.AddCommand(NewRepoCommand())
	command.AddCommand(NewInstallCommand(&globalArgs))
	return command
}
