package commands

import (
	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/server"
	"github.com/argoproj/argo-cd/util/cmd"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// NewCommand returns a new instance of an argocd command
func NewCommand() *cobra.Command {
	var (
		logLevel   string
		configMap  string
		kubeConfig string
	)
	var command = &cobra.Command{
		Use:   cliName,
		Short: "Run the argocd API server",
		Long:  "Run the argocd API server",
		Run: func(c *cobra.Command, args []string) {
			level, err := log.ParseLevel(logLevel)
			errors.CheckError(err)
			log.SetLevel(level)
			argocd := server.NewServer()
			argocd.Run()
		},
	}

	command.Flags().StringVar(&kubeConfig, "kubeconfig", "", "Kubernetes config (used when running outside of cluster)")
	command.Flags().StringVar(&configMap, "configmap", defaultArgoCDConfigMap, "Name of K8s configmap to retrieve argocd configuration")
	command.Flags().StringVar(&logLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	command.AddCommand(cmd.NewVersionCmd(cliName))
	return command
}
