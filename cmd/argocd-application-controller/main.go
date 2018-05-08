package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	argocd "github.com/argoproj/argo-cd"
	"github.com/argoproj/argo-cd/controller"
	"github.com/argoproj/argo-cd/errors"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver"
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
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

const (
	// CLIName is the name of the CLI
	cliName = "argocd-application-controller"
	// Default time in seconds for application resync period
	defaultAppResyncPeriod = 180
)

func newCommand() *cobra.Command {
	var (
		clientConfig      clientcmd.ClientConfig
		appResyncPeriod   int64
		repoServerAddress string
		workers           int
		logLevel          string
		glogLevel         int
	)
	var command = cobra.Command{
		Use:   cliName,
		Short: "application-controller is a controller to operate on applications CRD",
		RunE: func(c *cobra.Command, args []string) error {
			level, err := log.ParseLevel(logLevel)
			errors.CheckError(err)
			log.SetLevel(level)

			// Set the glog level for the k8s go-client
			flag.CommandLine.Parse([]string{})
			flag.Lookup("logtostderr").Value.Set("true")
			flag.Lookup("v").Value.Set(strconv.Itoa(glogLevel))

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
			apiClusterServer := cluster.NewServer(namespace, kubeClient, appClient)
			clusterService := cluster.NewServer(namespace, kubeClient, appClient)
			appComparator := controller.NewKsonnetAppComparator(clusterService)

			appController := controller.NewApplicationController(
				namespace,
				kubeClient,
				appClient,
				reposerver.NewRepositoryServerClientset(repoServerAddress),
				apiRepoServer,
				apiClusterServer,
				appComparator,
				resyncDuration,
				&controllerConfig)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			log.Infof("Application Controller (version: %s) starting (namespace: %s)", argocd.GetVersion(), namespace)
			go appController.Run(ctx, workers)
			// Wait forever
			select {}
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().Int64Var(&appResyncPeriod, "app-resync", defaultAppResyncPeriod, "Time period in seconds for application resync.")
	command.Flags().StringVar(&repoServerAddress, "repo-server", "localhost:8081", "Repo server address.")
	command.Flags().IntVar(&workers, "workers", 1, "Number of application workers")
	command.Flags().StringVar(&logLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	command.Flags().IntVar(&glogLevel, "gloglevel", 0, "Set the glog logging level")
	return &command
}

func main() {
	if err := newCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
