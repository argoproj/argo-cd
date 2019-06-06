package cluster

import (
	"fmt"
	"reflect"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apiclient/cluster"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/rbac"
)

// Server provides a Cluster service
type Server struct {
	db    db.ArgoDB
	enf   *rbac.Enforcer
	cache *cache.Cache
}

// NewServer returns a new instance of the Cluster service
func NewServer(db db.ArgoDB, enf *rbac.Enforcer, cache *cache.Cache) *Server {
	return &Server{
		db:    db,
		enf:   enf,
		cache: cache,
	}
}

func (s *Server) getConnectionState(ctx context.Context, cluster appv1.Cluster, errorMessage string) appv1.ConnectionState {
	if connectionState, err := s.cache.GetClusterConnectionState(cluster.Server); err == nil {
		return connectionState
	}
	now := v1.Now()
	connectionState := appv1.ConnectionState{
		Status:     appv1.ConnectionStatusSuccessful,
		ModifiedAt: &now,
	}

	config := cluster.RESTConfig()
	config.Timeout = time.Second
	kubeClientset, err := kubernetes.NewForConfig(config)
	if err == nil {
		_, err = kubeClientset.Discovery().ServerVersion()
	}
	if err != nil {
		connectionState.Status = appv1.ConnectionStatusFailed
		connectionState.Message = fmt.Sprintf("Unable to connect to cluster: %v", err)
	}

	if errorMessage != "" {
		connectionState.Status = appv1.ConnectionStatusFailed
		connectionState.Message = fmt.Sprintf("%s %s", errorMessage, connectionState.Message)
	}

	err = s.cache.SetClusterConnectionState(cluster.Server, &connectionState)
	if err != nil {
		log.Warnf("getConnectionState cache set error %s: %v", cluster.Server, err)
	}
	return connectionState
}

// List returns list of clusters
func (s *Server) List(ctx context.Context, q *cluster.ClusterQuery) (*appv1.ClusterList, error) {
	clusterList, err := s.db.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	clustersByServer := make(map[string][]appv1.Cluster)
	for _, clust := range clusterList.Items {
		if s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionGet, clust.Server) {
			clustersByServer[clust.Server] = append(clustersByServer[clust.Server], clust)
		}
	}
	servers := make([]string, 0)
	for server := range clustersByServer {
		servers = append(servers, server)
	}

	items := make([]appv1.Cluster, len(servers))
	err = util.RunAllAsync(len(servers), func(i int) error {
		clusters := clustersByServer[servers[i]]
		clust := clusters[0]
		warningMessage := ""
		if len(clusters) > 1 {
			warningMessage = fmt.Sprintf("There are %d credentials configured this cluster.", len(clusters))
		}
		if clust.ConnectionState.Status == "" {
			clust.ConnectionState = s.getConnectionState(ctx, clust, warningMessage)
		}
		items[i] = *redact(&clust)
		return nil
	})
	if err != nil {
		return nil, err
	}

	clusterList.Items = items
	return clusterList, err
}

// Create creates a cluster
func (s *Server) Create(ctx context.Context, q *cluster.ClusterCreateRequest) (*appv1.Cluster, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionCreate, q.Cluster.Server); err != nil {
		return nil, err
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
			return s.Update(ctx, &cluster.ClusterUpdateRequest{Cluster: c})
		} else {
			return nil, status.Errorf(codes.InvalidArgument, "existing cluster spec is different; use upsert flag to force update")
		}
	}
	return redact(clust), err
}

// Create creates a cluster
func (s *Server) CreateFromKubeConfig(ctx context.Context, q *cluster.ClusterCreateFromKubeConfigRequest) (*appv1.Cluster, error) {
	kubeconfig, err := clientcmd.Load([]byte(q.Kubeconfig))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not unmarshal kubeconfig: %v", err)
	}

	var clusterServer string
	var clusterInsecure bool
	var systemNamespace string

	if q.InCluster {
		clusterServer = common.KubernetesInternalAPIServerAddr
	} else if cluster, ok := kubeconfig.Clusters[q.Context]; ok {
		clusterServer = cluster.Server
		clusterInsecure = cluster.InsecureSkipTLSVerify
	} else {
		return nil, status.Errorf(codes.Internal, "Context %s does not exist in kubeconfig", q.Context)
	}

	if q.SystemNamespace != "" {
		systemNamespace = q.SystemNamespace
	} else {
		systemNamespace = common.DefaultSystemNamespace
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

	bearerToken, err := common.InstallClusterManagerRBAC(clientset, systemNamespace)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not install cluster manager RBAC: %v", err)
	}

	c.Config.BearerToken = bearerToken

	return s.Create(ctx, &cluster.ClusterCreateRequest{
		Cluster: c,
		Upsert:  q.Upsert,
	})
}

// Get returns a cluster from a query
func (s *Server) Get(ctx context.Context, q *cluster.ClusterQuery) (*appv1.Cluster, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionGet, q.Server); err != nil {
		return nil, err
	}
	clust, err := s.db.GetCluster(ctx, q.Server)
	return redact(clust), err
}

// Update updates a cluster
func (s *Server) Update(ctx context.Context, q *cluster.ClusterUpdateRequest) (*appv1.Cluster, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionUpdate, q.Cluster.Server); err != nil {
		return nil, err
	}
	err := kube.TestConfig(q.Cluster.RESTConfig())
	if err != nil {
		return nil, err
	}
	clust, err := s.db.UpdateCluster(ctx, q.Cluster)
	return redact(clust), err
}

// Delete deletes a cluster by name
func (s *Server) Delete(ctx context.Context, q *cluster.ClusterQuery) (*cluster.ClusterResponse, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionDelete, q.Server); err != nil {
		return nil, err
	}
	err := s.db.DeleteCluster(ctx, q.Server)
	return &cluster.ClusterResponse{}, err
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
