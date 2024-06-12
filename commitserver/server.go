package commitserver

import (
	"google.golang.org/grpc"

	"github.com/argoproj/argo-cd/v2/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v2/commitserver/commit"
)

type ArgoCDCommitServer struct {
	commitService *commit.Service
}

func NewServer() *ArgoCDCommitServer {
	return &ArgoCDCommitServer{commitService: commit.NewService()}
}

func (a *ArgoCDCommitServer) CreateGRPC() *grpc.Server {
	server := grpc.NewServer()
	apiclient.RegisterCommitServiceServer(server, a.commitService)
	return server
}
