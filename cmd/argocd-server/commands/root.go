package commands

import (
	"github.com/argoproj/argo-cd/errors"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/server"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/kube"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

// NewCommand returns a new instance of an argocd command
func NewCommand() *cobra.Command {
	var (
		logLevel        string
		configMap       string
		kubeConfig      string
		staticAssetsDir string
	)
	var command = &cobra.Command{
		Use:   cliName,
		Short: "Run the argocd API server",
		Long:  "Run the argocd API server",
		Run: func(c *cobra.Command, args []string) {
			level, err := log.ParseLevel(logLevel)
			errors.CheckError(err)
			log.SetLevel(level)

			config, err := kube.GetClientConfig(kubeConfig)
			errors.CheckError(err)
			kubeclientset := kubernetes.NewForConfigOrDie(config)
			appclientset := appclientset.NewForConfigOrDie(config)

			argocd := server.NewServer(kubeclientset, appclientset, staticAssetsDir)
			argocd.Run()
		},
	}

	command.Flags().StringVar(&kubeConfig, "kubeconfig", "", "Kubernetes config (used when running outside of cluster)")
	command.Flags().StringVar(&staticAssetsDir, "staticassets", "", "Static assets directory path")
	command.Flags().StringVar(&configMap, "configmap", defaultArgoCDConfigMap, "Name of K8s configmap to retrieve argocd configuration")
	command.Flags().StringVar(&logLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	command.AddCommand(cli.NewVersionCmd(cliName))
	return command
}
