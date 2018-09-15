package reposerver

import (
	"crypto/tls"

	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/server/version"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/git"
	grpc_util "github.com/argoproj/argo-cd/util/grpc"
	tlsutil "github.com/argoproj/argo-cd/util/tls"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

// ArgoCDRepoServer is the repo server implementation
type ArgoCDRepoServer struct {
	log        *log.Entry
	gitFactory git.ClientFactory
	cache      cache.Cache
	opts       []grpc.ServerOption
}

// NewServer returns a new instance of the ArgoCD Repo server
func NewServer(gitFactory git.ClientFactory, cache cache.Cache) (*ArgoCDRepoServer, error) {
	// generate TLS cert
	hosts := []string{
		"localhost",
		"argocd-repo-server",
	}
	cert, err := tlsutil.GenerateX509KeyPair(tlsutil.CertOptions{
		Hosts:        hosts,
		Organization: "Argo CD",
		IsCA:         true,
	})

	if err != nil {
		return nil, err
	}

	opts := []grpc.ServerOption{grpc.Creds(credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{*cert}}))}

	return &ArgoCDRepoServer{
		log:        log.NewEntry(log.New()),
		gitFactory: gitFactory,
		cache:      cache,
		opts:       opts,
	}, nil
}

// CreateGRPC creates new configured grpc server
func (a *ArgoCDRepoServer) CreateGRPC() *grpc.Server {
	server := grpc.NewServer(
		append(a.opts,
			grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
				grpc_logrus.StreamServerInterceptor(a.log),
				grpc_util.PanicLoggerStreamServerInterceptor(a.log),
			)),
			grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
				grpc_logrus.UnaryServerInterceptor(a.log),
				grpc_util.PanicLoggerUnaryServerInterceptor(a.log),
			)))...,
	)
	version.RegisterVersionServiceServer(server, &version.Server{})
	manifestService := repository.NewService(a.gitFactory, a.cache)
	repository.RegisterRepositoryServiceServer(server, manifestService)

	// Register reflection service on gRPC server.
	reflection.Register(server)

	return server
}
