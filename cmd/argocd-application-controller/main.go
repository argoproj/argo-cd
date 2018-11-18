package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	// load the gcp plugin (required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// load the oidc plugin (required to authenticate with OpenID Connect).
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"github.com/argoproj/argo-cd"
	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/controller"
	"github.com/argoproj/argo-cd/errors"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/stats"
	"github.com/argoproj/argo-cd/util/tls"
)

const (
	// CLIName is the name of the CLI
	cliName = "argocd-application-controller"
	// Default time in seconds for application resync period
	defaultAppResyncPeriod = 180
)

func newCommand() *cobra.Command {
	var (
		clientConfig           clientcmd.ClientConfig
		appResyncPeriod        int64
		repoServerAddress      string
		statusProcessors       int
		operationProcessors    int
		logLevel               string
		glogLevel              int
		tlsConfigCustomizerSrc func() (tls.ConfigCustomizer, error)
	)
	var command = cobra.Command{
		Use:   cliName,
		Short: "application-controller is a controller to operate on applications CRD",
		RunE: func(c *cobra.Command, args []string) error {
			cli.SetLogLevel(logLevel)
			cli.SetGLogLevel(glogLevel)

			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)

			kubeClient := kubernetes.NewForConfigOrDie(config)
			appClient := appclientset.NewForConfigOrDie(config)

			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)

			resyncDuration := time.Duration(appResyncPeriod) * time.Second
			repoClientset := reposerver.NewRepositoryServerClientset(repoServerAddress)
			appController := controller.NewApplicationController(
				namespace,
				kubeClient,
				appClient,
				repoClientset,
				resyncDuration)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			log.Infof("Application Controller (version: %s) starting (namespace: %s)", argocd.GetVersion(), namespace)
			stats.RegisterStackDumper()
			stats.StartStatsTicker(10 * time.Minute)
			stats.RegisterHeapDumper("memprofile")

			go appController.Run(ctx, statusProcessors, operationProcessors)
			go func() {
				tlsConfigCustomizer, err := tlsConfigCustomizerSrc()
				errors.CheckError(err)
				server, err := appController.CreateGRPC(tlsConfigCustomizer)
				errors.CheckError(err)

				listener, err := net.Listen("tcp", fmt.Sprintf(":%d", 8083))
				errors.CheckError(err)
				log.Infof("application-controller %s serving on %s", argocd.GetVersion(), listener.Addr())

				err = server.Serve(listener)
				errors.CheckError(err)
			}()
			// Wait forever
			select {}
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().Int64Var(&appResyncPeriod, "app-resync", defaultAppResyncPeriod, "Time period in seconds for application resync.")
	command.Flags().StringVar(&repoServerAddress, "repo-server", common.DefaultRepoServerAddr, "Repo server address.")
	command.Flags().IntVar(&statusProcessors, "status-processors", 1, "Number of application status processors")
	command.Flags().IntVar(&operationProcessors, "operation-processors", 1, "Number of application operation processors")
	command.Flags().StringVar(&logLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	command.Flags().IntVar(&glogLevel, "gloglevel", 0, "Set the glog logging level")
	tlsConfigCustomizerSrc = tls.AddTLSFlagsToCmd(&command)
	return &command
}

func main() {
	if err := newCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
