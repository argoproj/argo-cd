package commitserver

import (
	"google.golang.org/grpc"

	"github.com/argoproj/argo-cd/v2/commitserver/metrics"

	"github.com/argoproj/argo-cd/v2/util/git"

	"github.com/argoproj/argo-cd/v2/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v2/commitserver/commit"
)

// ArgoCDCommitServer is the server that handles commit requests.
type ArgoCDCommitServer struct {
	commitService *commit.Service
}

// NewServer returns a new instance of the commit server.
func NewServer(gitCredsStore git.CredsStore, metricsServer *metrics.Server) *ArgoCDCommitServer {
	return &ArgoCDCommitServer{commitService: commit.NewService(gitCredsStore, metricsServer)}
}

// CreateGRPC creates a new gRPC server.
func (a *ArgoCDCommitServer) CreateGRPC() *grpc.Server {
	server := grpc.NewServer()
	apiclient.RegisterCommitServiceServer(server, a.commitService)
	return server
}
