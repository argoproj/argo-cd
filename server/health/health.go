package health

import (
	"net/http"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server provides a Health service
type Server struct {
	ns string
}

// NewServer returns a new instance of the Health service
func NewServer(namespace string) *Server {
	return &Server{
		ns: namespace,
	}
}

func (s *Server) Health(ctx context.Context, healthReq *HealthRequest) (*HealthResponse, error) {
	const srvr = "kubernetes.default.svc"
	_, err := http.Get(srvr)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Could not contact Kubernetes cluster: %+v", err)
	}

	return &HealthResponse{}, nil
}
