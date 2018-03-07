package main

import (
	"context"
	"fmt"
	"os"
	"time"

	argocd "github.com/argoproj/argo-cd"
	"github.com/argoproj/argo-cd/controller"
	"github.com/argoproj/argo-cd/errors"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/server/cluster"
	apirepository "github.com/argoproj/argo-cd/server/repository"
	"github.com/argoproj/argo-cd/util/cli"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	// load the gcp plugin (required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// load the oidc plugin (required to authenticate with OpenID Connect).
	"github.com/argoproj/argo-cd/reposerver"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

const (
	// CLIName is the name of the CLI
	cliName = "argocd-application-controller"
	// Default time in seconds for application resync period
	defaultAppResyncPeriod = 600
)

func newCommand() *cobra.Command {
	var (
		clientConfig      clientcmd.ClientConfig
		appResyncPeriod   int64
		repoServerAddress string
	)
	var command = cobra.Command{
		Use:   cliName,
		Short: "application-controller is a controller to operate on applications CRD",
		RunE: func(c *cobra.Command, args []string) error {
			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)

			kubeClient := kubernetes.NewForConfigOrDie(config)
			appClient := appclientset.NewForConfigOrDie(config)

			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)

			// TODO (amatyushentsev): Use config map to store controller configuration
			controllerConfig := controller.ApplicationControllerConfig{
				Namespace:  namespace,
				InstanceID: "",
			}
			resyncDuration := time.Duration(appResyncPeriod) * time.Second
			apiRepoServer := apirepository.NewServer(namespace, kubeClient, appClient)
			clusterService := cluster.NewServer(namespace, kubeClient, appClient)
			appComparator := controller.NewKsonnetAppComparator(clusterService)

			appController := controller.NewApplicationController(
				kubeClient,
				appClient,
				reposerver.NewRepositoryServerClientset(repoServerAddress),
				apiRepoServer,
				appComparator,
				resyncDuration,
				&controllerConfig)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			log.Infof("Application Controller (version: %s) starting (namespace: %s)", argocd.GetVersion(), namespace)
			go appController.Run(ctx, 1)
			// Wait forever
			select {}
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().Int64Var(&appResyncPeriod, "app-resync", defaultAppResyncPeriod, "Time period in seconds for application resync.")
	command.Flags().StringVar(&repoServerAddress, "reposerveraddr", "localhost:8081", "Repo server address.")
	return &command
}

func main() {
	if err := newCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
