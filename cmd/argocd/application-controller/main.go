package main

import (
	"context"
	"fmt"
	"github.com/argoproj/argo-cd/application/controller"
	"github.com/argoproj/argo-cd/cmd/argocd/commands"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"os"
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
		Run: func(c *cobra.Command, args []string) {
			kubeConfig := commands.GetKubeConfig(kubeConfigPath, kubeConfigOverrides)

			kubeClient := kubernetes.NewForConfigOrDie(kubeConfig)
			appClient := appclientset.NewForConfigOrDie(kubeConfig)

			appController := controller.NewApplicationController(kubeClient, appClient)

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
