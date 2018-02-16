package cluster

import (
	"github.com/argoproj/argo-cd/server/core"
	"golang.org/x/net/context"
)

type Server struct{}

// GetClusters returns list of clusters
func (s *Server) GetClusters(ctx context.Context, in *ClusterQuery) (*ClusterListMessage, error) {
	return &ClusterListMessage{}, nil
}

// GetCluster returns list of clusters
func (s *Server) GetCluster(ctx context.Context, in *core.NameMessage) (*ClusterMessage, error) {
	return &ClusterMessage{}, nil
}
