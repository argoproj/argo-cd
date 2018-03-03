package commands

import (
	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/install"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	// These values may be overridden by the link flags during build
	// (e.g. imageTag will use the official release tag on tagged builds)
	imageNamespace = "argoproj"
	imageTag       = "latest"

	// These are the default image names which `argo install` uses during install
	DefaultControllerImage = imageNamespace + "/argocd-application-controller:" + imageTag
	DefaultUiImage         = imageNamespace + "/argocd-ui:" + imageTag
	DefaultServerImage     = imageNamespace + "/argocd-server:" + imageTag
)

// NewInstallCommand returns a new instance of `argocd install` command
func NewInstallCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
		installOpts  install.InstallOptions
	)
	var command = &cobra.Command{
		Use:   "install",
		Short: "Install the argocd components",
		Long:  "Install the argocd components",
		Run: func(c *cobra.Command, args []string) {
			conf, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			installer, err := install.NewInstaller(conf, installOpts)
			errors.CheckError(err)
			installer.Install()
		},
	}
	command.Flags().BoolVar(&installOpts.Upgrade, "upgrade", false, "upgrade controller/ui deployments and configmap if already installed")
	command.Flags().BoolVar(&installOpts.DryRun, "dry-run", false, "print the kubernetes manifests to stdout instead of installing")
	command.Flags().StringVar(&installOpts.Namespace, "install-namespace", common.DefaultArgoCDNamespace, "install into a specific namespace")
	command.Flags().StringVar(&installOpts.ControllerImage, "controller-image", DefaultControllerImage, "use a specified controller image")
	command.Flags().StringVar(&installOpts.ServerImage, "server-image", DefaultServerImage, "use a specified api server image")
	command.Flags().StringVar(&installOpts.UIImage, "ui-image", DefaultUiImage, "use a specified ui image")
	command.Flags().StringVar(&installOpts.ImagePullPolicy, "image-pull-policy", "", "set the image pull policy of the pod specs")
	clientConfig = cli.AddKubectlFlagsToCmd(command)
	return command
}
