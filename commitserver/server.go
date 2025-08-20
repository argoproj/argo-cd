package commitserver

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v3/commitserver/commit"
	"github.com/argoproj/argo-cd/v3/commitserver/metrics"
	versionpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/v3/server/version"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// ArgoCDCommitServer is the server that handles commit requests.
type ArgoCDCommitServer struct {
	commitService *commit.Service
}

// NewServer returns a new instance of the commit server.
func NewServer(gitCredsStore git.CredsStore, metricsServer *metrics.Server, settingsMgr *settings.SettingsManager) *ArgoCDCommitServer {
	return &ArgoCDCommitServer{commitService: commit.NewService(gitCredsStore, metricsServer, settingsMgr)}
}

// CreateGRPC creates a new gRPC server.
func (a *ArgoCDCommitServer) CreateGRPC() *grpc.Server {
	server := grpc.NewServer(grpc.MaxRecvMsgSize(apiclient.MaxGRPCMessageSize))
	versionpkg.RegisterVersionServiceServer(server, version.NewServer(nil, func() (bool, error) {
		return true, nil
	}))

	go a.commitService.WatchSettings(context.Background())
	apiclient.RegisterCommitServiceServer(server, a.commitService)

	healthService := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthService)

	return server
}
