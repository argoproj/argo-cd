package cluster

import (
	"reflect"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/rbac"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Server provides a Cluster service
type Server struct {
	kubeclientset kubernetes.Interface
	db            db.ArgoDB
	enf           *rbac.Enforcer
}

// NewServer returns a new instance of the Cluster service
func NewServer(kubeclientset kubernetes.Interface, db db.ArgoDB, enf *rbac.Enforcer) *Server {
	return &Server{
		kubeclientset: kubeclientset,
		db:            db,
		enf:           enf,
	}
}

// List returns list of clusters
func (s *Server) List(ctx context.Context, q *ClusterQuery) (*appv1.ClusterList, error) {
	clusterList, err := s.db.ListClusters(ctx)
	if clusterList != nil {
		newItems := make([]appv1.Cluster, 0)
		for _, clust := range clusterList.Items {
			if s.enf.EnforceClaims(ctx.Value("claims"), "clusters", "get", clust.Server) {
				newItems = append(newItems, *redact(&clust))
			}
		}
		clusterList.Items = newItems
	}
	return clusterList, err
}

// Create creates a cluster
func (s *Server) Create(ctx context.Context, q *ClusterCreateRequest) (*appv1.Cluster, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "clusters", "create", q.Cluster.Server) {
		return nil, grpc.ErrPermissionDenied
	}
	c := q.Cluster
	err := kube.TestConfig(q.Cluster.RESTConfig())
	if err != nil {
		return nil, err
	}

	c.ConnectionState = appv1.ConnectionState{Status: appv1.ConnectionStatusSuccessful}
	clust, err := s.db.CreateCluster(ctx, c)
	if status.Convert(err).Code() == codes.AlreadyExists {
		// act idempotent if existing spec matches new spec
		existing, getErr := s.db.GetCluster(ctx, c.Server)
		if getErr != nil {
			return nil, status.Errorf(codes.Internal, "unable to check existing cluster details: %v", getErr)
		}

		// cluster ConnectionState may differ, so make consistent before testing
		existing.ConnectionState = c.ConnectionState
		if reflect.DeepEqual(existing, c) {
			clust, err = existing, nil
		} else if q.Upsert {
			return s.Update(ctx, &ClusterUpdateRequest{Cluster: c})
		} else {
			return nil, status.Errorf(codes.InvalidArgument, "existing cluster spec is different; use upsert flag to force update")
		}
	}
	return redact(clust), err
}

// Create creates a cluster
func (s *Server) CreateFromKubeConfig(ctx context.Context, q *ClusterCreateFromKubeConfigRequest) (*appv1.Cluster, error) {
	var kubeconfig *clientcmdapi.Config
	err := yaml.Unmarshal([]byte(q.Kubeconfig), &kubeconfig)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not unmarshal kubeconfig: %v", err)
	}

	// Temporarily install RBAC resources for managing the cluster
	defer func() {
		err := common.UninstallClusterManagerRBAC(s.kubeclientset)
		if err != nil {
			log.Errorf("Error occurred uninstalling cluster manager: %v", err)
		}
	}()
	bearerToken, err := common.InstallClusterManagerRBAC(s.kubeclientset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not install cluster manager RBAC: %v", err)
	}

	contextInfo := kubeconfig.Clusters[q.Context]
	c := &appv1.Cluster{
		Server: contextInfo.Server,
		Name:   q.Context,
		Config: appv1.ClusterConfig{
			BearerToken: bearerToken,
		},
	}

	return s.Create(ctx, &ClusterCreateRequest{
		Cluster: c,
		Upsert:  q.Upsert,
	})
}

// Get returns a cluster from a query
func (s *Server) Get(ctx context.Context, q *ClusterQuery) (*appv1.Cluster, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "clusters", "get", q.Server) {
		return nil, grpc.ErrPermissionDenied
	}
	clust, err := s.db.GetCluster(ctx, q.Server)
	return redact(clust), err
}

// Update updates a cluster
func (s *Server) Update(ctx context.Context, q *ClusterUpdateRequest) (*appv1.Cluster, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "clusters", "update", q.Cluster.Server) {
		return nil, grpc.ErrPermissionDenied
	}
	err := kube.TestConfig(q.Cluster.RESTConfig())
	if err != nil {
		return nil, err
	}
	clust, err := s.db.UpdateCluster(ctx, q.Cluster)
	return redact(clust), err
}

// Delete deletes a cluster by name
func (s *Server) Delete(ctx context.Context, q *ClusterQuery) (*ClusterResponse, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "clusters", "delete", q.Server) {
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
