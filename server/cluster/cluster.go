package cluster

import (
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"golang.org/x/net/context"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Server provides a Cluster service
type Server struct {
	ns            string
	kubeclientset kubernetes.Interface
	appclientset  appclientset.Interface
}

// NewServer returns a new instance of the Cluster service
func NewServer(namespace string, kubeclientset kubernetes.Interface, appclientset appclientset.Interface) *Server {
	return &Server{
		ns:            namespace,
		appclientset:  appclientset,
		kubeclientset: kubeclientset,
	}
}

// ListPods returns application related pods in a cluster
func (s *Server) ListPods(ctx context.Context, q *ClusterQuery) (*apiv1.PodList, error) {
	// TODO: filter by the app label
	return s.kubeclientset.CoreV1().Pods(s.ns).List(metav1.ListOptions{})
}

// List returns list of clusters
func (s *Server) List(ctx context.Context, q *ClusterQuery) (*appv1.ClusterList, error) {
	return s.appclientset.ArgoprojV1alpha1().Clusters(s.ns).List(metav1.ListOptions{})
}

// Create creates a cluster
func (s *Server) Create(ctx context.Context, c *appv1.Cluster) (*appv1.Cluster, error) {
	return s.appclientset.ArgoprojV1alpha1().Clusters(s.ns).Create(c)
}

// Get returns a cluster from a query
func (s *Server) Get(ctx context.Context, q *ClusterQuery) (*appv1.Cluster, error) {
	return s.appclientset.ArgoprojV1alpha1().Clusters(s.ns).Get(q.Name, metav1.GetOptions{})
}

// Update updates a cluster
func (s *Server) Update(ctx context.Context, c *appv1.Cluster) (*appv1.Cluster, error) {
	return s.appclientset.ArgoprojV1alpha1().Clusters(s.ns).Update(c)
}

// Delete deletes a cluster by name
func (s *Server) Delete(ctx context.Context, q *ClusterQuery) (*ClusterResponse, error) {
	err := s.appclientset.ArgoprojV1alpha1().Clusters(s.ns).Delete(q.Name, &metav1.DeleteOptions{})
	return &ClusterResponse{}, err
}
