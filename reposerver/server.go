package reposerver

import (
	"crypto/tls"

	versionpkg "github.com/argoproj/argo-cd/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/server/version"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/git"
	grpc_util "github.com/argoproj/argo-cd/util/grpc"
	tlsutil "github.com/argoproj/argo-cd/util/tls"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_logrus "github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

// ArgoCDRepoServer is the repo server implementation
type ArgoCDRepoServer struct {
	log              *log.Entry
	gitFactory       git.ClientFactory
	cache            *cache.Cache
	opts             []grpc.ServerOption
	parallelismLimit int64
}

// NewServer returns a new instance of the Argo CD Repo server
func NewServer(gitFactory git.ClientFactory, cache *cache.Cache, tlsConfCustomizer tlsutil.ConfigCustomizer, parallelismLimit int64) (*ArgoCDRepoServer, error) {
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

	tlsConfig := &tls.Config{Certificates: []tls.Certificate{*cert}}
	tlsConfCustomizer(tlsConfig)

	serverLog := log.NewEntry(log.StandardLogger())
	streamInterceptors := []grpc.StreamServerInterceptor{grpc_logrus.StreamServerInterceptor(serverLog), grpc_util.PanicLoggerStreamServerInterceptor(serverLog)}
	unaryInterceptors := []grpc.UnaryServerInterceptor{grpc_logrus.UnaryServerInterceptor(serverLog), grpc_util.PanicLoggerUnaryServerInterceptor(serverLog)}

	return &ArgoCDRepoServer{
		log:              serverLog,
		gitFactory:       gitFactory,
		cache:            cache,
		parallelismLimit: parallelismLimit,
		opts: []grpc.ServerOption{
			grpc.Creds(credentials.NewTLS(tlsConfig)),
			grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryInterceptors...)),
			grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streamInterceptors...)),
		},
	}, nil
}

// CreateGRPC creates new configured grpc server
func (a *ArgoCDRepoServer) CreateGRPC() *grpc.Server {
	server := grpc.NewServer(a.opts...)
	versionpkg.RegisterVersionServiceServer(server, &version.Server{})
	manifestService := repository.NewService(a.gitFactory, a.cache, a.parallelismLimit)
	apiclient.RegisterRepoServerServiceServer(server, manifestService)

	// Register reflection service on gRPC server.
	reflection.Register(server)

	return server
}
