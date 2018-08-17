package health

import (
	"fmt"

	"golang.org/x/net/context"
)

// Server provides a Health service
type Server struct {
	ns string
}

// NewServer returns a new instance of the Health service
func NewServer(namespace string) HealthServiceServer {
	return &Server{
		ns: namespace,
	}
}

func (s *Server) health(ctx context.Context, healthReq *HealthRequest) (*HealthResponse, error) {
	fmt.Println("HEALTH")
}
