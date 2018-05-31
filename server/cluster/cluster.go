package cluster

import (
	"github.com/argoproj/argo-cd/util/db"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"golang.org/x/net/context"
)

// Server provides a Cluster service
type Server struct {
	db db.ArgoDB
}

// NewServer returns a new instance of the Cluster service
func NewServer(db db.ArgoDB) *Server {
	return &Server{
		db: db,
	}
}

// List returns list of clusters
func (s *Server) List(ctx context.Context, q *ClusterQuery) (*appv1.ClusterList, error) {
	clusterList, err := s.db.ListClusters(ctx)
	if clusterList != nil {
		for i, clust := range clusterList.Items {
			clusterList.Items[i] = *redact(&clust)
		}
	}
	return clusterList, err
}

// Create creates a cluster
func (s *Server) Create(ctx context.Context, q *ClusterCreateRequest) (*appv1.Cluster, error) {
	c := &(*c).Cluster
	clust, err := s.db.CreateCluster(ctx, c)
	return redact(clust), err
}

// Get returns a cluster from a query
func (s *Server) Get(ctx context.Context, q *ClusterQuery) (*appv1.Cluster, error) {
	clust, err := s.db.GetCluster(ctx, q.Server)
	return redact(clust), err
}

// Update updates a cluster
func (s *Server) Update(ctx context.Context, q *ClusterUpdateRequest) (*appv1.Cluster, error) {
	c := &(*q).Cluster
	clust, err := s.db.UpdateCluster(ctx, c)
	return redact(clust), err
}

// UpdateREST updates a cluster (special handler intended to be used only by the gRPC gateway)
func (s *Server) UpdateREST(ctx context.Context, r *ClusterRESTUpdateRequest) (*appv1.Cluster, error) {
	return s.Update(ctx, r.Cluster)
}

// Delete deletes a cluster by name
func (s *Server) Delete(ctx context.Context, q *ClusterQuery) (*ClusterResponse, error) {
	err := s.db.DeleteCluster(ctx, q.Server)
	return &ClusterResponse{}, err
}

func redact(clust *appv1.Cluster) *appv1.Cluster {
	if clust == nil {
		return nil
	}
	clust.Config.Password = ""
	clust.Config.BearerToken = ""
	clust.Config.TLSClientConfig.KeyData = nil
	return clust
}
