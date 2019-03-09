package main

import (
	"fmt"
	"net"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	argocd "github.com/argoproj/argo-cd"
	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/ksonnet"
	"github.com/argoproj/argo-cd/util/repos"
	"github.com/argoproj/argo-cd/util/stats"
	"github.com/argoproj/argo-cd/util/tls"
)

const (
	// CLIName is the name of the CLI
	cliName = "argocd-repo-server"
	port    = 8081
)

func newCommand() *cobra.Command {
	var (
		logLevel               string
		parallelismLimit       int64
		cacheSrc               func() (*cache.Cache, error)
		tlsConfigCustomizerSrc func() (tls.ConfigCustomizer, error)
	)
	var command = cobra.Command{
		Use:   cliName,
		Short: "Run argocd-repo-server",
		RunE: func(c *cobra.Command, args []string) error {
			cli.SetLogLevel(logLevel)

			tlsConfigCustomizer, err := tlsConfigCustomizerSrc()
			errors.CheckError(err)

			cache, err := cacheSrc()
			errors.CheckError(err)

			server, err := reposerver.NewServer(repos.NewFactory(), cache, tlsConfigCustomizer, parallelismLimit)
			errors.CheckError(err)
			grpc := server.CreateGRPC()
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			errors.CheckError(err)

			ksVers, err := ksonnet.KsonnetVersion()
			errors.CheckError(err)

			log.Infof("argocd-repo-server %s serving on %s", argocd.GetVersion(), listener.Addr())
			log.Infof("ksonnet version: %s", ksVers)
			stats.RegisterStackDumper()
			stats.StartStatsTicker(10 * time.Minute)
			stats.RegisterHeapDumper("memprofile")
			err = grpc.Serve(listener)
			errors.CheckError(err)
			return nil
		},
	}

	command.Flags().StringVar(&logLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	command.Flags().Int64Var(&parallelismLimit, "parallelismlimit", 0, "Limit on number of concurrent manifests generate requests. Any value less the 1 means no limit.")
	tlsConfigCustomizerSrc = tls.AddTLSFlagsToCmd(&command)
	cacheSrc = cache.AddCacheFlagsToCmd(&command)
	return &command
}

func main() {
	if err := newCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
