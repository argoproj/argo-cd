package commands

import (
	"github.com/argoproj/argo-cd/errors"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/server"
	"github.com/argoproj/argo-cd/util/cli"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// NewCommand returns a new instance of an argocd command
func NewCommand() *cobra.Command {
	var (
		insecure          bool
		logLevel          string
		clientConfig      clientcmd.ClientConfig
		staticAssetsDir   string
		repoServerAddress string
		disableAuth       bool
	)
	var command = &cobra.Command{
		Use:   cliName,
		Short: "Run the argocd API server",
		Long:  "Run the argocd API server",
		Run: func(c *cobra.Command, args []string) {
			level, err := log.ParseLevel(logLevel)
			errors.CheckError(err)
			log.SetLevel(level)

			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)

			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)

			kubeclientset := kubernetes.NewForConfigOrDie(config)
			appclientset := appclientset.NewForConfigOrDie(config)
			repoclientset := reposerver.NewRepositoryServerClientset(repoServerAddress)

			argoCDOpts := server.ArgoCDServerOpts{
				Insecure:        insecure,
				Namespace:       namespace,
				StaticAssetsDir: staticAssetsDir,
				KubeClientset:   kubeclientset,
				AppClientset:    appclientset,
				RepoClientset:   repoclientset,
				DisableAuth:     disableAuth,
			}
			argocd := server.NewServer(argoCDOpts)
			argocd.Run()
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(command)
	command.Flags().BoolVar(&insecure, "insecure", false, "Run server without TLS")
	command.Flags().StringVar(&staticAssetsDir, "staticassets", "", "Static assets directory path")
	command.Flags().StringVar(&logLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	command.Flags().StringVar(&repoServerAddress, "repo-server", "localhost:8081", "Repo server address.")
	command.Flags().BoolVar(&disableAuth, "disable-auth", false, "Disable client authentication")
	command.AddCommand(cli.NewVersionCmd(cliName))
	return command
}
