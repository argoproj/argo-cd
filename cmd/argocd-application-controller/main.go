package main

import (
	"context"
	"fmt"
	"os"
	"time"

	argocd "github.com/argoproj/argo-cd"
	"github.com/argoproj/argo-cd/application"
	"github.com/argoproj/argo-cd/cmd/argocd/commands"
	"github.com/argoproj/argo-cd/controller"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/server/cluster"
	"github.com/argoproj/argo-cd/server/repository"
	"github.com/argoproj/argo-cd/util/git"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	// load the gcp plugin (required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// load the oidc plugin (required to authenticate with OpenID Connect).
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

const (
	// CLIName is the name of the CLI
	cliName = "application-controller"
	// Default time in seconds for application resync period
	defaultAppResyncPeriod = 600
)

func newCommand() *cobra.Command {
	var (
		kubeConfigOverrides clientcmd.ConfigOverrides
		kubeConfigPath      string
		appResyncPeriod     int64
	)
	var command = cobra.Command{
		Use:   cliName,
		Short: "application-controller is a controller to operate on applications CRD",
		RunE: func(c *cobra.Command, args []string) error {
			kubeConfig := commands.GetKubeConfig(kubeConfigPath, kubeConfigOverrides)

			nativeGitClient, err := git.NewNativeGitClient()
			if err != nil {
				return err
			}
			kubeClient := kubernetes.NewForConfigOrDie(kubeConfig)
			appClient := appclientset.NewForConfigOrDie(kubeConfig)

			// TODO (amatyushentsev): Use config map to store controller configuration
			config := controller.ApplicationControllerConfig{
				Namespace:  "default",
				InstanceID: "",
			}
			clusterService := cluster.NewServer(config.Namespace, kubeClient, appClient)
			resyncDuration := time.Duration(appResyncPeriod) * time.Second
			appManager := application.NewAppManager(
				nativeGitClient,
				repository.NewServer(config.Namespace, kubeClient, appClient),
				clusterService,
				application.NewKsonnetAppComparator(clusterService),
				resyncDuration,
			)
			appController := controller.NewApplicationController(kubeClient, appClient, appManager, resyncDuration, &config)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			log.Infof("Application Controller (version: %s) starting", argocd.GetVersion())
			go appController.Run(ctx, 1)
			// Wait forever
			select {}
		},
	}

	command.Flags().StringVar(&kubeConfigPath, "kubeconfig", "", "Path to the config file to use for CLI requests.")
	command.Flags().Int64Var(&appResyncPeriod, "app-resync", defaultAppResyncPeriod, "Time period in seconds for application resync.")
	kubeConfigOverrides = clientcmd.ConfigOverrides{}
	clientcmd.BindOverrideFlags(&kubeConfigOverrides, command.Flags(), clientcmd.RecommendedConfigOverrideFlags(""))

	return &command
}

func main() {
	if err := newCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
