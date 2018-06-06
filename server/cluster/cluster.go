package cluster

import (
	"fmt"

	"golang.org/x/net/context"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/rbac"
)

// Server provides a Cluster service
type Server struct {
	db  db.ArgoDB
	enf *rbac.Enforcer
}

// NewServer returns a new instance of the Cluster service
func NewServer(db db.ArgoDB, enf *rbac.Enforcer) *Server {
	return &Server{
		db:  db,
		enf: enf,
	}
}

// List returns list of clusters
func (s *Server) List(ctx context.Context, q *ClusterQuery) (*appv1.ClusterList, error) {
	clusterList, err := s.db.ListClusters(ctx)
	if clusterList != nil {
		newItems := make([]appv1.Cluster, 0)
		for _, clust := range clusterList.Items {
			if s.enf.EnforceClaims(ctx.Value("claims"), "clusters", "get", fmt.Sprintf("*/%s", clust.Server)) {
				newItems = append(newItems, *redact(&clust))
			}
		}
		clusterList.Items = newItems
	}
	return clusterList, err
}

// Create creates a cluster
func (s *Server) Create(ctx context.Context, q *ClusterCreateRequest) (*appv1.Cluster, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "clusters", "create", fmt.Sprintf("*/%s", q.Cluster.Server)) {
		return nil, grpc.ErrPermissionDenied
	}
	clust, err := s.db.CreateCluster(ctx, q.Cluster)
	return redact(clust), err
}

// Get returns a cluster from a query
func (s *Server) Get(ctx context.Context, q *ClusterQuery) (*appv1.Cluster, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "clusters", "get", fmt.Sprintf("*/%s", q.Server)) {
		return nil, grpc.ErrPermissionDenied
	}
	clust, err := s.db.GetCluster(ctx, q.Server)
	return redact(clust), err
}

// Update updates a cluster
func (s *Server) Update(ctx context.Context, q *ClusterUpdateRequest) (*appv1.Cluster, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "clusters", "update", fmt.Sprintf("*/%s", q.Cluster.Server)) {
		return nil, grpc.ErrPermissionDenied
	}
	clust, err := s.db.UpdateCluster(ctx, q.Cluster)
	return redact(clust), err
}

// Delete deletes a cluster by name
func (s *Server) Delete(ctx context.Context, q *ClusterQuery) (*ClusterResponse, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "clusters", "delete", fmt.Sprintf("*/%s", q.Server)) {
		return nil, grpc.ErrPermissionDenied
	}
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
