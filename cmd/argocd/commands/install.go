package commands

import (
	"github.com/argoproj/argo-cd/common"
	"github.com/spf13/cobra"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
)

var (
	// These values may be overridden by the link flags during build
	// (e.g. imageTag will use the official release tag on tagged builds)
	imageNamespace = "argoproj"
	imageTag       = "latest"

	// These are the default image names which `argo install` uses during install
	DefaultControllerImage = imageNamespace + "/argocd-application-controller:" + imageTag
)

// NewInstallCommand returns a new instance of `argocd install` command
func NewInstallCommand(globalArgs *globalFlags) *cobra.Command {
	var (
		installParams common.InstallParameters
	)
	var command = &cobra.Command{
		Use:   "install",
		Short: "Install the argocd components",
		Long:  "Install the argocd components",
		Run: func(c *cobra.Command, args []string) {
			conf := GetKubeConfig(globalArgs.kubeConfigPath, globalArgs.kubeConfigOverrides)
			extensionsClient := apiextensionsclient.NewForConfigOrDie(conf)
			kubeClient := kubernetes.NewForConfigOrDie(conf)
			common.NewInstaller(extensionsClient, kubeClient).Install(installParams)
		},
	}
	command.Flags().BoolVar(&installParams.Upgrade, "upgrade", false, "upgrade controller/ui deployments and configmap if already installed")
	command.Flags().BoolVar(&installParams.DryRun, "dry-run", false, "print the kubernetes manifests to stdout instead of installing")
	command.Flags().StringVar(&installParams.Namespace, "install-namespace", common.DefaultControllerNamespace, "install into a specific Namespace")
	command.Flags().StringVar(&installParams.ControllerName, "controller-name", common.DefaultControllerDeploymentName, "name of controller deployment")
	command.Flags().StringVar(&installParams.ControllerImage, "controller-image", DefaultControllerImage, "use a specified controller image")
	command.Flags().StringVar(&installParams.ServiceAccount, "service-account", "", "use a specified service account for the workflow-controller deployment")

	return command
}
