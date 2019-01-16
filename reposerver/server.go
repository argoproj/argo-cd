package reposerver

import (
	"context"
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
	"github.com/yaronsumel/grpc-throttle"

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

// NewServer returns a new instance of the Argo CD Repo server
func NewServer(gitFactory git.ClientFactory, cache cache.Cache, tlsConfCustomizer tlsutil.ConfigCustomizer, parallelismLimit map[string]int) (*ArgoCDRepoServer, error) {
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

	serverLog := log.NewEntry(log.New())
	streamInterceptors := []grpc.StreamServerInterceptor{grpc_logrus.StreamServerInterceptor(serverLog), grpc_util.PanicLoggerStreamServerInterceptor(serverLog)}
	unaryInterceptors := []grpc.UnaryServerInterceptor{grpc_logrus.UnaryServerInterceptor(serverLog), grpc_util.PanicLoggerUnaryServerInterceptor(serverLog)}
	if len(parallelismLimit) > 0 {
		var semaphores = throttle.SemaphoreMap{}
		for m, l := range parallelismLimit {
			semaphores[m] = make(throttle.Semaphore, l)
		}
		throttler := func(ctx context.Context, fullMethod string) (throttle.Semaphore, bool) {
			if s, ok := semaphores[fullMethod]; ok {
				return s, true
			}
			return nil, false
		}
		streamInterceptors = append(streamInterceptors, throttle.StreamServerInterceptor(throttler))
		unaryInterceptors = append(unaryInterceptors, throttle.UnaryServerInterceptor(throttler))
	}

	return &ArgoCDRepoServer{
		log:        serverLog,
		gitFactory: gitFactory,
		cache:      cache,
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
	version.RegisterVersionServiceServer(server, &version.Server{})
	manifestService := repository.NewService(a.gitFactory, a.cache)
	repository.RegisterRepositoryServiceServer(server, manifestService)

	// Register reflection service on gRPC server.
	reflection.Register(server)

	return server
}
