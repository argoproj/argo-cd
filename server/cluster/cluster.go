package cluster

import (
	"fmt"
	"reflect"
	"time"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-cd/pkg/apiclient/cluster"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	servercache "github.com/argoproj/argo-cd/server/cache"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/util/clusterauth"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/rbac"
)

// Server provides a Cluster service
type Server struct {
	db      db.ArgoDB
	enf     *rbac.Enforcer
	cache   *servercache.Cache
	kubectl kube.Kubectl
}

// NewServer returns a new instance of the Cluster service
func NewServer(db db.ArgoDB, enf *rbac.Enforcer, cache *servercache.Cache, kubectl kube.Kubectl) *Server {
	return &Server{
		db:      db,
		enf:     enf,
		cache:   cache,
		kubectl: kubectl,
	}
}

func (s *Server) getConnectionState(cluster appv1.Cluster, errorMessage string) (appv1.ConnectionState, string) {
	if clusterInfo, err := s.cache.GetClusterInfo(cluster.Server); err == nil {
		return clusterInfo.ConnectionState, clusterInfo.Version
	}
	now := v1.Now()
	clusterInfo := servercache.ClusterInfo{
		ConnectionState: appv1.ConnectionState{
			Status:     appv1.ConnectionStatusSuccessful,
			ModifiedAt: &now,
		},
	}

	config := cluster.RESTConfig()
	config.Timeout = time.Second
	version, err := s.kubectl.GetServerVersion(config)
	if err != nil {
		clusterInfo.Status = appv1.ConnectionStatusFailed
		clusterInfo.Message = fmt.Sprintf("Unable to connect to cluster: %v", err)
	} else {
		clusterInfo.Version = version
	}

	if errorMessage != "" {
		clusterInfo.Status = appv1.ConnectionStatusFailed
		clusterInfo.Message = fmt.Sprintf("%s %s", errorMessage, clusterInfo.Message)
	}

	err = s.cache.SetClusterInfo(cluster.Server, &clusterInfo)
	if err != nil {
		log.Warnf("getClusterInfo cache set error %s: %v", cluster.Server, err)
	}
	return clusterInfo.ConnectionState, clusterInfo.Version
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
	err = kube.RunAllAsync(len(servers), func(i int) error {
		clusters := clustersByServer[servers[i]]
		clust := clusters[0]
		warningMessage := ""
		if len(clusters) > 1 {
			warningMessage = fmt.Sprintf("There are %d credentials configured this cluster.", len(clusters))
		}
		if clust.ConnectionState.Status == "" {
			state, serverVersion := s.getConnectionState(clust, warningMessage)
			clust.ConnectionState = state
			clust.ServerVersion = serverVersion
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

// Get returns a cluster from a query
func (s *Server) Get(ctx context.Context, q *cluster.ClusterQuery) (*appv1.Cluster, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionGet, q.Server); err != nil {
		return nil, err
	}
	c, err := s.db.GetCluster(ctx, q.Server)
	if err != nil {
		return nil, err
	}
	c.ServerVersion, err = s.kubectl.GetServerVersion(c.RESTConfig())
	if err != nil {
		return nil, err
	}
	return redact(c), nil
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

// RotateAuth rotates the bearer token used for a cluster
func (s *Server) RotateAuth(ctx context.Context, q *cluster.ClusterQuery) (*cluster.ClusterResponse, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionUpdate, q.Server); err != nil {
		return nil, err
	}
	logCtx := log.WithField("cluster", q.Server)
	logCtx.Info("Rotating auth")
	clust, err := s.db.GetCluster(ctx, q.Server)
	if err != nil {
		return nil, err
	}
	restCfg := clust.RESTConfig()
	if restCfg.BearerToken == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Cluster '%s' does not use bearer token authentication", q.Server)
	}
	claims, err := clusterauth.ParseServiceAccountToken(restCfg.BearerToken)
	if err != nil {
		return nil, err
	}
	kubeclientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	newSecret, err := clusterauth.GenerateNewClusterManagerSecret(kubeclientset, claims)
	if err != nil {
		return nil, err
	}
	// we are using token auth, make sure we don't store client-cert information
	clust.Config.KeyData = nil
	clust.Config.CertData = nil
	clust.Config.BearerToken = string(newSecret.Data["token"])

	// Test the token we just created before persisting it
	err = kube.TestConfig(clust.RESTConfig())
	if err != nil {
		return nil, err
	}
	_, err = s.db.UpdateCluster(ctx, clust)
	if err != nil {
		return nil, err
	}
	err = clusterauth.RotateServiceAccountSecrets(kubeclientset, claims, newSecret)
	if err != nil {
		return nil, err
	}
	logCtx.Infof("Rotated auth (old: %s, new: %s)", claims.SecretName, newSecret.Name)
	return &cluster.ClusterResponse{}, nil
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
