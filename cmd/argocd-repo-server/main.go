package main

import (
	"fmt"
	"net"
	"os"

	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/git"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	// load the gcp plugin (required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// load the oidc plugin (required to authenticate with OpenID Connect).
	"github.com/argoproj/argo-cd"
	"github.com/argoproj/argo-cd/reposerver"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

const (
	// CLIName is the name of the CLI
	cliName = "argocd-manifest-server"
	port    = 8081
)

func newCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
		logLevel     string
	)
	var command = cobra.Command{
		Use:   cliName,
		Short: "Run argocd-repo-server",
		RunE: func(c *cobra.Command, args []string) error {
			level, err := log.ParseLevel(logLevel)
			errors.CheckError(err)
			log.SetLevel(level)

			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)

			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)

			kubeClientset := kubernetes.NewForConfigOrDie(config)

			server := reposerver.NewServer(kubeClientset, namespace)
			nativeGitClient, err := git.NewNativeGitClient()
			errors.CheckError(err)
			grpc := server.CreateGRPC(nativeGitClient)
			listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
			errors.CheckError(err)

			log.Infof("argocd-repo-server %s serving on port %d (namespace: %s)", argocd.GetVersion(), port, namespace)
			err = grpc.Serve(listener)
			errors.CheckError(err)
			return nil
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().StringVar(&logLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	return &command
}

func main() {
	if err := newCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
