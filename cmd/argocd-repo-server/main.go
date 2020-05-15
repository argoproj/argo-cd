package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/argoproj/gitops-engine/pkg/utils/errors"
	"github.com/argoproj/pkg/stats"
	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/reposerver"
	reposervercache "github.com/argoproj/argo-cd/reposerver/cache"
	"github.com/argoproj/argo-cd/reposerver/metrics"
	cacheutil "github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/tls"
)

const (
	// CLIName is the name of the CLI
	cliName = "argocd-repo-server"
)

func newCommand() *cobra.Command {
	var (
		logFormat              string
		logLevel               string
		parallelismLimit       int64
		listenPort             int
		metricsPort            int
		cacheSrc               func() (*reposervercache.Cache, error)
		tlsConfigCustomizerSrc func() (tls.ConfigCustomizer, error)
		redisClient            *redis.Client
	)
	var command = cobra.Command{
		Use:   cliName,
		Short: "Run argocd-repo-server",
		RunE: func(c *cobra.Command, args []string) error {
			cli.SetLogFormat(logFormat)
			cli.SetLogLevel(logLevel)

			tlsConfigCustomizer, err := tlsConfigCustomizerSrc()
			errors.CheckError(err)

			cache, err := cacheSrc()
			errors.CheckError(err)

			metricsServer := metrics.NewMetricsServer()
			cacheutil.CollectMetrics(redisClient, metricsServer)
			server, err := reposerver.NewServer(metricsServer, cache, tlsConfigCustomizer, parallelismLimit)
			errors.CheckError(err)

			grpc := server.CreateGRPC()
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", listenPort))
			errors.CheckError(err)

			http.Handle("/metrics", metricsServer.GetHandler())
			go func() { errors.CheckError(http.ListenAndServe(fmt.Sprintf(":%d", metricsPort), nil)) }()

			log.Infof("argocd-repo-server %s serving on %s", common.GetVersion(), listener.Addr())
			stats.RegisterStackDumper()
			stats.StartStatsTicker(10 * time.Minute)
			stats.RegisterHeapDumper("memprofile")
			err = grpc.Serve(listener)
			errors.CheckError(err)
			return nil
		},
	}

	command.Flags().StringVar(&logFormat, "logformat", "text", "Set the logging format. One of: text|json")
	command.Flags().StringVar(&logLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	command.Flags().Int64Var(&parallelismLimit, "parallelismlimit", 0, "Limit on number of concurrent manifests generate requests. Any value less the 1 means no limit.")
	command.Flags().IntVar(&listenPort, "port", common.DefaultPortRepoServer, "Listen on given port for incoming connections")
	command.Flags().IntVar(&metricsPort, "metrics-port", common.DefaultPortRepoServerMetrics, "Start metrics server on given port")
	tlsConfigCustomizerSrc = tls.AddTLSFlagsToCmd(&command)
	cacheSrc = reposervercache.AddCacheFlagsToCmd(&command, func(client *redis.Client) {
		redisClient = client
	})
	return &command
}

func main() {
	if err := newCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
