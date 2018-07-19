package reposerver

import (
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/server/version"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/git"
	grpc_util "github.com/argoproj/argo-cd/util/grpc"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// ArgoCDRepoServer is the repo server implementation
type ArgoCDRepoServer struct {
	log        *log.Entry
	gitFactory git.ClientFactory
	cache      cache.Cache
}

// NewServer returns a new instance of the ArgoCD Repo server
func NewServer(gitFactory git.ClientFactory, cache cache.Cache) *ArgoCDRepoServer {
	return &ArgoCDRepoServer{
		log:        log.NewEntry(log.New()),
		gitFactory: gitFactory,
		cache:      cache,
	}
}

// CreateGRPC creates new configured grpc server
func (a *ArgoCDRepoServer) CreateGRPC() *grpc.Server {
	server := grpc.NewServer(
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
			grpc_logrus.StreamServerInterceptor(a.log),
			grpc_util.PayloadStreamServerInterceptor(a.log, false, func(ctx context.Context, fullMethodName string, servingObject interface{}) bool {
				return true
			}),
			grpc_util.PanicLoggerStreamServerInterceptor(a.log),
		)),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			grpc_logrus.UnaryServerInterceptor(a.log),
			grpc_util.PayloadUnaryServerInterceptor(a.log, false, func(ctx context.Context, fullMethodName string, servingObject interface{}) bool {
				return true
			}),
			grpc_util.PanicLoggerUnaryServerInterceptor(a.log),
		)),
	)
	version.RegisterVersionServiceServer(server, &version.Server{})
	manifestService := repository.NewService(a.gitFactory, a.cache)
	repository.RegisterRepositoryServiceServer(server, manifestService)

	// Register reflection service on gRPC server.
	reflection.Register(server)

	return server
}
