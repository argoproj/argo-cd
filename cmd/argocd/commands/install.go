package commands

import (
	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/install"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

// NewInstallCommand returns a new instance of `argocd install` command
func NewInstallCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
		installOpts  install.InstallOptions
	)
	var command = &cobra.Command{
		Use:   "install",
		Short: "Install Argo CD",
		Long:  "Install Argo CD",
		Run: func(c *cobra.Command, args []string) {
			conf, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, wasSpecified, err := clientConfig.Namespace()
			errors.CheckError(err)
			if wasSpecified {
				installOpts.Namespace = namespace
			}
			installer, err := install.NewInstaller(conf, installOpts)
			errors.CheckError(err)
			installer.Install()
		},
	}
	command.Flags().BoolVar(&installOpts.Upgrade, "upgrade", false, "upgrade controller/ui deployments and configmap if already installed")
	command.Flags().BoolVar(&installOpts.DryRun, "dry-run", false, "print the kubernetes manifests to stdout instead of installing")
	command.Flags().StringVar(&installOpts.SuperuserPassword, "superuser-password", "", "password for super user")
	command.Flags().StringVar(&installOpts.ControllerImage, "controller-image", install.DefaultControllerImage, "use a specified controller image")
	command.Flags().StringVar(&installOpts.ServerImage, "server-image", install.DefaultServerImage, "use a specified api server image")
	command.Flags().StringVar(&installOpts.UIImage, "ui-image", install.DefaultUIImage, "use a specified ui image")
	command.Flags().StringVar(&installOpts.RepoServerImage, "repo-server-image", install.DefaultRepoServerImage, "use a specified repo server image")
	command.Flags().StringVar(&installOpts.ImagePullPolicy, "image-pull-policy", "", "set the image pull policy of the pod specs")
	clientConfig = cli.AddKubectlFlagsToCmd(command)
	command.AddCommand(newSettingsCommand())
	return command
}

// newSettingsCommand returns a new instance of `argocd install settings` command
func newSettingsCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
		installOpts  install.InstallOptions
	)
	var command = &cobra.Command{
		Use:   "settings",
		Short: "Creates or updates ArgoCD settings",
		Long:  "Creates or updates ArgoCD settings",
		Run: func(c *cobra.Command, args []string) {
			conf, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, wasSpecified, err := clientConfig.Namespace()
			errors.CheckError(err)
			if wasSpecified {
				installOpts.Namespace = namespace
			}
			installer, err := install.NewInstaller(conf, installOpts)
			errors.CheckError(err)
			installer.InstallSettings()
		},
	}
	command.Flags().BoolVar(&installOpts.UpdateSuperuser, "update-superuser", false, "force updating the  superuser password")
	command.Flags().StringVar(&installOpts.SuperuserPassword, "superuser-password", "", "password for super user")
	command.Flags().BoolVar(&installOpts.UpdateSignature, "update-signature", false, "force updating the server-side token signing signature")
	clientConfig = cli.AddKubectlFlagsToCmd(command)
	return command
}
