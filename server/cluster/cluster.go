package cluster

import (
	"fmt"
	"reflect"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/rbac"
	log "github.com/sirupsen/logrus"
)

// Server provides a Cluster service
type Server struct {
	db    db.ArgoDB
	enf   *rbac.Enforcer
	cache cache.Cache
}

// NewServer returns a new instance of the Cluster service
func NewServer(db db.ArgoDB, enf *rbac.Enforcer, cache cache.Cache) *Server {
	return &Server{
		db:    db,
		enf:   enf,
		cache: cache,
	}
}

const (
	DefaultClusterStatusCacheExpiration = 1 * time.Hour
)

func (s *Server) getConnectionState(ctx context.Context, cluster appv1.Cluster) appv1.ConnectionState {
	cacheKey := fmt.Sprintf("connection-state-%s", cluster.Server)
	var connectionState appv1.ConnectionState
	if err := s.cache.Get(cacheKey, &connectionState); err == nil {
		return connectionState
	}
	now := v1.Now()
	connectionState = appv1.ConnectionState{
		Status:     appv1.ConnectionStatusSuccessful,
		ModifiedAt: &now,
	}

	kubeClientset, err := kubernetes.NewForConfig(cluster.RESTConfig())
	if err == nil {
		_, err = kubeClientset.Discovery().ServerVersion()
	}
	if err != nil {
		connectionState.Status = appv1.ConnectionStatusFailed
		connectionState.Message = fmt.Sprintf("Unable to connect to cluster: %v", err)
	}
	err = s.cache.Set(&cache.Item{
		Object:     &connectionState,
		Key:        cacheKey,
		Expiration: DefaultClusterStatusCacheExpiration,
	})
	if err != nil {
		log.Warnf("getConnectionState cache set error %s: %v", cacheKey, err)
	}
	return connectionState
}

// List returns list of clusters
func (s *Server) List(ctx context.Context, q *ClusterQuery) (*appv1.ClusterList, error) {
	clusterList, err := s.db.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	newItems := make([]appv1.Cluster, 0)
	for _, clust := range clusterList.Items {
		if s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionGet, clust.Server) {
			newItems = append(newItems, *redact(&clust))
		}
	}

	err = util.RunAllAsync(len(newItems), func(i int) error {
		clust := newItems[i]
		if clust.ConnectionState.Status == "" {
			clust.ConnectionState = s.getConnectionState(ctx, clust)
		}
		newItems[i] = clust
		return nil
	})
	if err != nil {
		return nil, err
	}

	clusterList.Items = newItems
	return clusterList, err
}

// Create creates a cluster
func (s *Server) Create(ctx context.Context, q *ClusterCreateRequest) (*appv1.Cluster, error) {
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionCreate, q.Cluster.Server) {
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
	kubeconfig, err := clientcmd.Load([]byte(q.Kubeconfig))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not unmarshal kubeconfig: %v", err)
	}

	var clusterServer string
	var clusterInsecure bool
	if q.InCluster {
		clusterServer = common.KubernetesInternalAPIServerAddr
	} else if cluster, ok := kubeconfig.Clusters[q.Context]; ok {
		clusterServer = cluster.Server
		clusterInsecure = cluster.InsecureSkipTLSVerify
	} else {
		return nil, status.Errorf(codes.Internal, "Context %s does not exist in kubeconfig", q.Context)
	}

	c := &appv1.Cluster{
		Server: clusterServer,
		Name:   q.Context,
		Config: appv1.ClusterConfig{
			TLSClientConfig: appv1.TLSClientConfig{
				Insecure: clusterInsecure,
			},
		},
	}

	// Temporarily install RBAC resources for managing the cluster
	clientset, err := kubernetes.NewForConfig(c.RESTConfig())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not create Kubernetes clientset: %v", err)
	}

	bearerToken, err := common.InstallClusterManagerRBAC(clientset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not install cluster manager RBAC: %v", err)
	}

	c.Config.BearerToken = bearerToken

	return s.Create(ctx, &ClusterCreateRequest{
		Cluster: c,
		Upsert:  q.Upsert,
	})
}

// Get returns a cluster from a query
func (s *Server) Get(ctx context.Context, q *ClusterQuery) (*appv1.Cluster, error) {
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionGet, q.Server) {
		return nil, grpc.ErrPermissionDenied
	}
	clust, err := s.db.GetCluster(ctx, q.Server)
	return redact(clust), err
}

// Update updates a cluster
func (s *Server) Update(ctx context.Context, q *ClusterUpdateRequest) (*appv1.Cluster, error) {
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionUpdate, q.Cluster.Server) {
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
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionDelete, q.Server) {
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
