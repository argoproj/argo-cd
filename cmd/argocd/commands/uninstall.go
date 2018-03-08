package commands

import (
	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/install"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

// NewUninstallCommand returns a new instance of `argocd install` command
func NewUninstallCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
		installOpts  install.InstallOptions
	)
	var command = &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall Argo CD",
		Long:  "Uninstall Argo CD",
		Run: func(c *cobra.Command, args []string) {
			conf, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			installer, err := install.NewInstaller(conf, installOpts)
			errors.CheckError(err)
			installer.Uninstall()
		},
	}
	command.Flags().StringVar(&installOpts.Namespace, "install-namespace", install.DefaultInstallNamespace, "uninstall from a specific namespace")
	clientConfig = cli.AddKubectlFlagsToCmd(command)
	return command
}
