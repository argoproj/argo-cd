package commands

import (
	"os"

	"github.com/argoproj/argo-cd/util/cli"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

type globalFlags struct {
	clientConfig clientcmd.ClientConfig
	logLevel     string
}

// NewCommand returns a new instance of an argocd command
func NewCommand() *cobra.Command {
	var (
		globalArgs globalFlags
	)
	pathOptions := clientcmd.NewDefaultPathOptions()
	var command = &cobra.Command{
		Use:   cliName,
		Short: "argocd controls a ArgoCD server",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}

	command.AddCommand(cli.NewVersionCmd(cliName))
	command.AddCommand(NewClusterCommand(pathOptions))
	command.AddCommand(NewApplicationCommand())
	command.AddCommand(NewRepoCommand())
	command.AddCommand(NewInstallCommand(&globalArgs))
	return command
}

func addKubectlFlagsToCmd(cmd *cobra.Command, globalArgs *globalFlags) {
	// The "usual" clientcmd/kubectl flags
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := clientcmd.ConfigOverrides{}
	kflags := clientcmd.RecommendedConfigOverrideFlags("")
	cmd.PersistentFlags().StringVar(&loadingRules.ExplicitPath, "kubeconfig", "", "Path to a kube config. Only required if out-of-cluster")
	clientcmd.BindOverrideFlags(&overrides, cmd.PersistentFlags(), kflags)
	globalArgs.clientConfig = clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)
}
