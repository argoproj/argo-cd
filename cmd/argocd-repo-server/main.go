package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/go-redis/redis"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd"
	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/ksonnet"
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
		redisAddress           string
		tlsConfigCustomizerSrc func() (tls.ConfigCustomizer, error)
	)
	var command = cobra.Command{
		Use:   cliName,
		Short: "Run argocd-repo-server",
		RunE: func(c *cobra.Command, args []string) error {
			cli.SetLogLevel(logLevel)

			tlsConfigCustomizer, err := tlsConfigCustomizerSrc()
			errors.CheckError(err)

			server, err := reposerver.NewServer(git.NewFactory(), newCache(redisAddress), tlsConfigCustomizer)
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
	command.Flags().StringVar(&redisAddress, "redis", "", "Redis server hostname and port (e.g. argocd-redis:6379). ")
	tlsConfigCustomizerSrc = tls.AddTLSFlagsToCmd(&command)
	return &command
}

func newCache(redisAddress string) cache.Cache {
	if redisAddress == "" {
		return cache.NewInMemoryCache(repository.DefaultRepoCacheExpiration)
	}
	client := redis.NewClient(&redis.Options{
		Addr:     redisAddress,
		Password: "",
		DB:       0,
	})
	return cache.NewRedisCache(client, repository.DefaultRepoCacheExpiration)
}

func main() {
	if err := newCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
