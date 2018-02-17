package cluster

import (
	clusterv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/server/core"
	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Server provides a Cluster service
type Server struct {
	ns           string
	appclientset appclientset.Interface
}

// NewServer returns a new instance of the Cluster service
func NewServer(appclientset appclientset.Interface) *Server {
	return &Server{
		ns:           "default",
		appclientset: appclientset,
	}
}

// GetClusters returns list of clusters
func (s *Server) GetClusters(ctx context.Context, in *ClusterQuery) (*clusterv1.ClusterList, error) {
	return s.appclientset.ArgoprojV1alpha1().Clusters(s.ns).List(metav1.ListOptions{})
}

// CreateCluster creates a cluster
func (s *Server) CreateCluster(ctx context.Context, c *clusterv1.Cluster) (*clusterv1.Cluster, error) {
	return s.appclientset.ArgoprojV1alpha1().Clusters(s.ns).Create(c)
}

// GetCluster returns list of clusters
func (s *Server) GetCluster(ctx context.Context, name *core.NameMessage) (*clusterv1.Cluster, error) {
	return s.appclientset.ArgoprojV1alpha1().Clusters(s.ns).Get(name.Name, metav1.GetOptions{})
}

// DeleteCluster deletes a cluster by name
func (s *Server) DeleteCluster(ctx context.Context, name *core.NameMessage) (*empty.Empty, error) {
	err := s.appclientset.ArgoprojV1alpha1().Clusters(s.ns).Delete(name.Name, &metav1.DeleteOptions{})
	return &empty.Empty{}, err
}
