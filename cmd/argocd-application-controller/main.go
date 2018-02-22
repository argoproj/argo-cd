package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/argoproj/argo-cd/application"
	"github.com/argoproj/argo-cd/cmd/argocd/commands"
	"github.com/argoproj/argo-cd/controller"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/server/repository"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// CLIName is the name of the CLI
	cliName = "application-controller"
)

func newCommand() *cobra.Command {
	var (
		kubeConfigOverrides clientcmd.ConfigOverrides
		kubeConfigPath      string
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
			namespace := "default"
			appResyncPeriod := time.Minute * 10

			appManager := application.NewAppManager(
				nativeGitClient,
				repository.NewServer(namespace, kubeClient, appClient),
				application.NewKsonnetAppComparator(),
				appResyncPeriod)

			appController := controller.NewApplicationController(kubeClient, appClient, appManager, appResyncPeriod)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go appController.Run(ctx, 1)
			// Wait forever
			select {}
		},
	}

	command.Flags().StringVar(&kubeConfigPath, "kubeconfig", "", "Path to the config file to use for CLI requests.")
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
