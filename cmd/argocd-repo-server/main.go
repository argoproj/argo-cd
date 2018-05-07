package main

import (
	"fmt"
	"net"
	"os"

	"github.com/argoproj/argo-cd"
	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/ksonnet"
	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	// CLIName is the name of the CLI
	cliName = "argocd-repo-server"
	port    = 8081
)

func newCommand() *cobra.Command {
	var (
		logLevel string
	)
	var command = cobra.Command{
		Use:   cliName,
		Short: "Run argocd-repo-server",
		RunE: func(c *cobra.Command, args []string) error {
			level, err := log.ParseLevel(logLevel)
			errors.CheckError(err)
			log.SetLevel(level)

			server := reposerver.NewServer(git.NewFactory(), newCache())
			grpc := server.CreateGRPC()
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			errors.CheckError(err)

			ksVers, err := ksonnet.KsonnetVersion()
			errors.CheckError(err)

			log.Infof("argocd-repo-server %s serving on %s", argocd.GetVersion(), listener.Addr())
			log.Infof("ksonnet version: %s", ksVers)
			err = grpc.Serve(listener)
			errors.CheckError(err)
			return nil
		},
	}

	command.Flags().StringVar(&logLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	return &command
}

func newCache() cache.Cache {
	//return cache.NewInMemoryCache(repository.DefaultRepoCacheExpiration)
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
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
