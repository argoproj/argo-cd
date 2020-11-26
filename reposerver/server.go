package reposerver

import (
	"crypto/tls"
	"os"

	"github.com/argoproj/argo-cd/common"

	versionpkg "github.com/argoproj/argo-cd/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	reposervercache "github.com/argoproj/argo-cd/reposerver/cache"
	"github.com/argoproj/argo-cd/reposerver/metrics"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/server/version"
	grpc_util "github.com/argoproj/argo-cd/util/grpc"
	tlsutil "github.com/argoproj/argo-cd/util/tls"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_logrus "github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

// ArgoCDRepoServer is the repo server implementation
type ArgoCDRepoServer struct {
	log           *log.Entry
	metricsServer *metrics.MetricsServer
	cache         *reposervercache.Cache
	opts          []grpc.ServerOption
	initConstants repository.RepoServerInitConstants
}

// NewServer returns a new instance of the Argo CD Repo server
func NewServer(metricsServer *metrics.MetricsServer, cache *reposervercache.Cache, tlsConfCustomizer tlsutil.ConfigCustomizer, initConstants repository.RepoServerInitConstants) (*ArgoCDRepoServer, error) {
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

	if os.Getenv(common.EnvEnableGRPCTimeHistogramEnv) == "true" {
		grpc_prometheus.EnableHandlingTimeHistogram()
	}

	serverLog := log.NewEntry(log.StandardLogger())
	streamInterceptors := []grpc.StreamServerInterceptor{grpc_logrus.StreamServerInterceptor(serverLog), grpc_prometheus.StreamServerInterceptor, grpc_util.PanicLoggerStreamServerInterceptor(serverLog)}
	unaryInterceptors := []grpc.UnaryServerInterceptor{grpc_logrus.UnaryServerInterceptor(serverLog), grpc_prometheus.UnaryServerInterceptor, grpc_util.PanicLoggerUnaryServerInterceptor(serverLog)}

	return &ArgoCDRepoServer{
		log:           serverLog,
		metricsServer: metricsServer,
		cache:         cache,
		initConstants: initConstants,
		opts: []grpc.ServerOption{
			grpc.Creds(credentials.NewTLS(tlsConfig)),
			grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryInterceptors...)),
			grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streamInterceptors...)),
			grpc.MaxRecvMsgSize(apiclient.MaxGRPCMessageSize),
			grpc.MaxSendMsgSize(apiclient.MaxGRPCMessageSize),
		},
	}, nil
}

// CreateGRPC creates new configured grpc server
func (a *ArgoCDRepoServer) CreateGRPC() *grpc.Server {
	server := grpc.NewServer(a.opts...)
	versionpkg.RegisterVersionServiceServer(server, &version.Server{})
	manifestService := repository.NewService(a.metricsServer, a.cache, a.initConstants)
	apiclient.RegisterRepoServerServiceServer(server, manifestService)

	// Register reflection service on gRPC server.
	reflection.Register(server)

	return server
}
