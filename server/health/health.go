package health

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/kubernetes"
)

// Server provides a Health service
type Server struct {
	ns            string
	kubeclientset kubernetes.Interface
}

// NewServer returns a new instance of the Health service
func NewServer(namespace string, kubeclientset kubernetes.Interface) *Server {
	return &Server{
		ns:            namespace,
		kubeclientset: kubeclientset,
	}
}

func (s *Server) Health(ctx context.Context, healthReq *HealthRequest) (*HealthResponse, error) {
	_, err := s.kubeclientset.(*kubernetes.Clientset).ServerVersion()
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "Could not get Kubernetes version: %v", err)
	}

	return &HealthResponse{}, nil
}

// AuthFuncOverride overrides the authentication function and let us not require auth to receive auth.
// Without this function here, ArgoCDServer.authenticate would be invoked and credentials checked.
// This allows the health check to be publicly available without any authentication required.
func (s *Server) AuthFuncOverride(ctx context.Context, fullMethodName string) (context.Context, error) {
	return ctx, nil
}
