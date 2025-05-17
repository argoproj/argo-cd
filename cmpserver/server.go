package cmpserver

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	"github.com/argoproj/argo-cd/v3/cmpserver/apiclient"
	"github.com/argoproj/argo-cd/v3/cmpserver/plugin"
	"github.com/argoproj/argo-cd/v3/common"
	versionpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/v3/server/version"
	"github.com/argoproj/argo-cd/v3/util/errors"
	grpc_util "github.com/argoproj/argo-cd/v3/util/grpc"
)

// ArgoCDCMPServer is the config management plugin server implementation
type ArgoCDCMPServer struct {
	log           *log.Entry
	opts          []grpc.ServerOption
	initConstants plugin.CMPServerInitConstants
	stopCh        chan os.Signal
	doneCh        chan any
	sig           os.Signal
}

// NewServer returns a new instance of the Argo CD config management plugin server
func NewServer(initConstants plugin.CMPServerInitConstants) (*ArgoCDCMPServer, error) {
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

	return &ArgoCDCMPServer{
		log:           serverLog,
		opts:          serverOpts,
		stopCh:        make(chan os.Signal),
		doneCh:        make(chan any),
		initConstants: initConstants,
	}, nil
}

func (a *ArgoCDCMPServer) Run() {
	config := a.initConstants.PluginConfig

	// Listen on the socket address
	_ = os.Remove(config.Address())
	listener, err := net.Listen("unix", config.Address())
	errors.CheckError(err)
	log.Infof("argocd-cmp-server %s serving on %s", common.GetVersion(), listener.Addr())

	signal.Notify(a.stopCh, syscall.SIGINT, syscall.SIGTERM)
	go a.Shutdown(config.Address())

	grpcServer, err := a.CreateGRPC()
	errors.CheckError(err)
	err = grpcServer.Serve(listener)
	errors.CheckError(err)

	if a.sig != nil {
		<-a.doneCh
	}
}

// CreateGRPC creates new configured grpc server
func (a *ArgoCDCMPServer) CreateGRPC() (*grpc.Server, error) {
	server := grpc.NewServer(a.opts...)
	versionpkg.RegisterVersionServiceServer(server, version.NewServer(nil, func() (bool, error) {
		return true, nil
	}))
	pluginService := plugin.NewService(a.initConstants)
	err := pluginService.Init(common.GetCMPWorkDir())
	if err != nil {
		return nil, fmt.Errorf("error initializing plugin service: %w", err)
	}
	apiclient.RegisterConfigManagementPluginServiceServer(server, pluginService)

	healthService := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthService)

	// Register reflection service on gRPC server.
	reflection.Register(server)

	return server, nil
}

func (a *ArgoCDCMPServer) Shutdown(address string) {
	defer signal.Stop(a.stopCh)
	a.sig = <-a.stopCh
	_ = os.Remove(address)
	close(a.doneCh)
}
