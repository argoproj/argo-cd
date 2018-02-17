package cluster

import (
	clusterv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/server/core"
	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
)

// Server provides a Cluster service
type Server struct{}

// GetClusters returns list of clusters
func (s *Server) GetClusters(ctx context.Context, in *ClusterQuery) (*clusterv1.ClusterList, error) {
	return &clusterv1.ClusterList{}, nil
}

// CreateCluster creates a cluster
func (s *Server) CreateCluster(ctx context.Context, in *clusterv1.Cluster) (*clusterv1.Cluster, error) {
	return &clusterv1.Cluster{}, nil
}

// GetCluster returns list of clusters
func (s *Server) GetCluster(ctx context.Context, in *core.NameMessage) (*clusterv1.Cluster, error) {
	return &clusterv1.Cluster{}, nil
}

func (s *Server) DeleteCluster(context.Context, *core.NameMessage) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}
