package reposerver

import (
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	"github.com/argoproj/argo-cd/v3/common"
	versionpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	reposervercache "github.com/argoproj/argo-cd/v3/reposerver/cache"
	"github.com/argoproj/argo-cd/v3/reposerver/metrics"
	"github.com/argoproj/argo-cd/v3/reposerver/repository"
	"github.com/argoproj/argo-cd/v3/server/version"
	"github.com/argoproj/argo-cd/v3/util/env"
	"github.com/argoproj/argo-cd/v3/util/git"
	grpc_util "github.com/argoproj/argo-cd/v3/util/grpc"
	tlsutil "github.com/argoproj/argo-cd/v3/util/tls"
)

// ArgoCDRepoServer is the repo server implementation
type ArgoCDRepoServer struct {
	repoService *repository.Service
	opts        []grpc.ServerOption
}

// The hostnames to generate self-signed issues with
var tlsHostList = []string{"localhost", "reposerver"}

// NewServer returns a new instance of the Argo CD Repo server
func NewServer(metricsServer *metrics.MetricsServer, cache *reposervercache.Cache, tlsConfCustomizer tlsutil.ConfigCustomizer, initConstants repository.RepoServerInitConstants, gitCredsStore git.CredsStore) (*ArgoCDRepoServer, error) {
	var tlsConfig *tls.Config

	// Generate or load TLS server certificates to use with this instance of
	// repository server.
	if tlsConfCustomizer != nil {
		var err error
		certPath := env.StringFromEnv(common.EnvAppConfigPath, common.DefaultAppConfigPath) + "/reposerver/tls/tls.crt"
		keyPath := env.StringFromEnv(common.EnvAppConfigPath, common.DefaultAppConfigPath) + "/reposerver/tls/tls.key"
		tlsConfig, err = tlsutil.CreateServerTLSConfig(certPath, keyPath, tlsHostList)
		if err != nil {
			return nil, fmt.Errorf("error creating server TLS config: %w", err)
		}
		tlsConfCustomizer(tlsConfig)
	}

	var serverMetricsOptions []grpc_prometheus.ServerMetricsOption
	if os.Getenv(common.EnvEnableGRPCTimeHistogramEnv) == "true" {
		serverMetricsOptions = append(serverMetricsOptions, grpc_prometheus.WithServerHandlingTimeHistogram())
	}
	serverMetrics := grpc_prometheus.NewServerMetrics(serverMetricsOptions...)
	reg := prometheus.NewRegistry()
	reg.MustRegister(serverMetrics)

	serverLog := log.NewEntry(log.StandardLogger())
	streamInterceptors := []grpc.StreamServerInterceptor{
		otelgrpc.StreamServerInterceptor(), //nolint:staticcheck // TODO: ignore SA1019 for depreciation: see https://github.com/argoproj/argo-cd/issues/18258
		logging.StreamServerInterceptor(grpc_util.InterceptorLogger(serverLog)),
		serverMetrics.StreamServerInterceptor(),
		recovery.StreamServerInterceptor(recovery.WithRecoveryHandler(grpc_util.LoggerRecoveryHandler(serverLog))),
	}
	unaryInterceptors := []grpc.UnaryServerInterceptor{
		otelgrpc.UnaryServerInterceptor(), //nolint:staticcheck // TODO: ignore SA1019 for depreciation: see https://github.com/argoproj/argo-cd/issues/18258
		logging.UnaryServerInterceptor(grpc_util.InterceptorLogger(serverLog)),
		serverMetrics.UnaryServerInterceptor(),
		recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(grpc_util.LoggerRecoveryHandler(serverLog))),
		grpc_util.ErrorSanitizerUnaryServerInterceptor(),
	}

	serverOpts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(unaryInterceptors...),
		grpc.ChainStreamInterceptor(streamInterceptors...),
		grpc.MaxRecvMsgSize(apiclient.MaxGRPCMessageSize),
		grpc.MaxSendMsgSize(apiclient.MaxGRPCMessageSize),
		grpc.KeepaliveEnforcementPolicy(
			keepalive.EnforcementPolicy{
				MinTime: common.GetGRPCKeepAliveEnforcementMinimum(),
			},
		),
	}

	// We do allow for non-TLS servers to be created, in case of mTLS will be
	// implemented by e.g. a sidecar container.
	if tlsConfig != nil {
		serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(tlsConfig)))
	}
	repoService := repository.NewService(metricsServer, cache, initConstants, gitCredsStore, filepath.Join(os.TempDir(), "_argocd-repo"))
	if err := repoService.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize the repo service: %w", err)
	}

	return &ArgoCDRepoServer{
		opts:        serverOpts,
		repoService: repoService,
	}, nil
}

// CreateGRPC creates new configured grpc server
func (a *ArgoCDRepoServer) CreateGRPC() *grpc.Server {
	server := grpc.NewServer(a.opts...)
	versionpkg.RegisterVersionServiceServer(server, version.NewServer(nil, func() (bool, error) {
		return true, nil
	}))
	apiclient.RegisterRepoServerServiceServer(server, a.repoService)

	healthService := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthService)

	// Register reflection service on gRPC server.
	reflection.Register(server)

	return server
}
