package cluster

import (
	"time"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/cluster"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/clusterauth"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/rbac"
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

func createRBACObject(project string, server string) string {
	if project != "" {
		return project + "/" + server
	}
	return server
}

// List returns list of clusters
func (s *Server) List(ctx context.Context, q *cluster.ClusterQuery) (*appv1.ClusterList, error) {
	clusterList, err := s.db.ListClusters(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]appv1.Cluster, 0)
	for _, clust := range clusterList.Items {
		if s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionGet, createRBACObject(clust.Project, clust.Server)) {
			items = append(items, clust)
		}
	}
	err = kube.RunAllAsync(len(items), func(i int) error {
		items[i] = *s.toAPIResponse(&items[i])
		return nil
	})
	if err != nil {
		return nil, err
	}
	clusterList.Items = items
	return clusterList, nil
}

// Create creates a cluster
func (s *Server) Create(ctx context.Context, q *cluster.ClusterCreateRequest) (*appv1.Cluster, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionCreate, createRBACObject(q.Cluster.Project, q.Cluster.Server)); err != nil {
		return nil, err
	}
	c := q.Cluster
	serverVersion, err := s.kubectl.GetServerVersion(c.RESTConfig())
	if err != nil {
		return nil, err
	}

	clust, err := s.db.CreateCluster(ctx, c)
	if err != nil {
		if status.Convert(err).Code() == codes.AlreadyExists {
			// act idempotent if existing spec matches new spec
			existing, getErr := s.db.GetCluster(ctx, c.Server)
			if getErr != nil {
				return nil, status.Errorf(codes.Internal, "unable to check existing cluster details: %v", getErr)
			}

			if existing.Equals(c) {
				clust = existing
			} else if q.Upsert {
				return s.Update(ctx, &cluster.ClusterUpdateRequest{Cluster: c})
			} else {
				return nil, status.Errorf(codes.InvalidArgument, argo.GenerateSpecIsDifferentErrorMessage("cluster", existing, c))
			}
		} else {
			return nil, err
		}
	}

	err = s.cache.SetClusterInfo(c.Server, &appv1.ClusterInfo{
		ServerVersion: serverVersion,
		ConnectionState: appv1.ConnectionState{
			Status:     appv1.ConnectionStatusSuccessful,
			ModifiedAt: &v1.Time{Time: time.Now()},
		},
	})
	if err != nil {
		return nil, err
	}
	return s.toAPIResponse(clust), err
}

// Get returns a cluster from a query
func (s *Server) Get(ctx context.Context, q *cluster.ClusterQuery) (*appv1.Cluster, error) {
	c, err := s.getClusterWith403IfNotExist(ctx, q)
	if err != nil {
		return nil, err
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionGet, createRBACObject(c.Project, q.Server)); err != nil {
		return nil, err
	}

	return s.toAPIResponse(c), nil
}

func (s *Server) getClusterWith403IfNotExist(ctx context.Context, q *cluster.ClusterQuery) (*appv1.Cluster, error) {
	repo, err := s.getCluster(ctx, q)
	if err != nil || repo == nil {
		return nil, status.Error(codes.PermissionDenied, "permission denied")
	}
	return repo, nil
}

func (s *Server) getCluster(ctx context.Context, q *cluster.ClusterQuery) (*appv1.Cluster, error) {
	if q.Id != nil {
		q.Server = ""
		q.Name = ""
		if q.Id.Type == "name" {
			q.Name = q.Id.Value
		} else {
			q.Server = q.Id.Value
		}
	}

	if q.Server != "" {
		c, err := s.db.GetCluster(ctx, q.Server)
		if err != nil {
			return nil, err
		}
		return c, nil
	}

	//we only get the name when we specify Name in ApplicationDestination and next
	//we want to find the server in order to populate ApplicationDestination.Server
	if q.Name != "" {
		clusterList, err := s.db.ListClusters(ctx)
		if err != nil {
			return nil, err
		}
		for _, c := range clusterList.Items {
			if c.Name == q.Name {
				return &c, nil
			}
		}
	}

	return nil, nil
}

var clusterFieldsByPath = map[string]func(updated *appv1.Cluster, existing *appv1.Cluster){
	"name": func(updated *appv1.Cluster, existing *appv1.Cluster) {
		updated.Name = existing.Name
	},
	"namespaces": func(updated *appv1.Cluster, existing *appv1.Cluster) {
		updated.Namespaces = existing.Namespaces
	},
	"config": func(updated *appv1.Cluster, existing *appv1.Cluster) {
		updated.Config = existing.Config
	},
	"shard": func(updated *appv1.Cluster, existing *appv1.Cluster) {
		updated.Shard = existing.Shard
	},
	"clusterResources": func(updated *appv1.Cluster, existing *appv1.Cluster) {
		updated.ClusterResources = existing.ClusterResources
	},
	"labels": func(updated *appv1.Cluster, existing *appv1.Cluster) {
		updated.Labels = existing.Labels
	},
	"annotations": func(updated *appv1.Cluster, existing *appv1.Cluster) {
		updated.Annotations = existing.Annotations
	},
	"project": func(updated *appv1.Cluster, existing *appv1.Cluster) {
		updated.Project = existing.Project
	},
}

// Update updates a cluster
func (s *Server) Update(ctx context.Context, q *cluster.ClusterUpdateRequest) (*appv1.Cluster, error) {
	c, err := s.getClusterWith403IfNotExist(ctx, &cluster.ClusterQuery{
		Server: q.Cluster.Server,
		Name:   q.Cluster.Name,
		Id:     q.Id,
	})
	if err != nil {
		return nil, err
	}

	// verify that user can do update inside project where cluster is located
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionUpdate, createRBACObject(c.Project, q.Cluster.Server)); err != nil {
		return nil, err
	}

	if len(q.UpdatedFields) == 0 || sets.NewString(q.UpdatedFields...).Has("project") {
		// verify that user can do update inside project where cluster will be located
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionUpdate, createRBACObject(q.Cluster.Project, q.Cluster.Server)); err != nil {
			return nil, err
		}
	}

	if len(q.UpdatedFields) != 0 {
		for _, path := range q.UpdatedFields {
			if updater, ok := clusterFieldsByPath[path]; ok {
				updater(c, q.Cluster)
			}
		}
		q.Cluster = c
	}

	// Test the token we just created before persisting it
	serverVersion, err := s.kubectl.GetServerVersion(q.Cluster.RESTConfig())
	if err != nil {
		return nil, err
	}

	clust, err := s.db.UpdateCluster(ctx, q.Cluster)
	if err != nil {
		return nil, err
	}
	err = s.cache.SetClusterInfo(clust.Server, &appv1.ClusterInfo{
		ServerVersion: serverVersion,
		ConnectionState: appv1.ConnectionState{
			Status:     appv1.ConnectionStatusSuccessful,
			ModifiedAt: &v1.Time{Time: time.Now()},
		},
	})
	if err != nil {
		return nil, err
	}
	return s.toAPIResponse(clust), nil
}

// Delete deletes a cluster by name
func (s *Server) Delete(ctx context.Context, q *cluster.ClusterQuery) (*cluster.ClusterResponse, error) {
	c, err := s.getClusterWith403IfNotExist(ctx, q)
	if err != nil {
		return nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionDelete, createRBACObject(c.Project, c.Server)); err != nil {
		return nil, err
	}
	err = s.db.DeleteCluster(ctx, q.Server)
	return &cluster.ClusterResponse{}, err
}

// RotateAuth rotates the bearer token used for a cluster
func (s *Server) RotateAuth(ctx context.Context, q *cluster.ClusterQuery) (*cluster.ClusterResponse, error) {
	clust, err := s.getClusterWith403IfNotExist(ctx, q)
	if err != nil {
		return nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionUpdate, createRBACObject(clust.Project, q.Server)); err != nil {
		return nil, err
	}
	logCtx := log.WithField("cluster", q.Server)
	logCtx.Info("Rotating auth")
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
	serverVersion, err := s.kubectl.GetServerVersion(clust.RESTConfig())
	if err != nil {
		return nil, err
	}
	_, err = s.db.UpdateCluster(ctx, clust)
	if err != nil {
		return nil, err
	}
	err = s.cache.SetClusterInfo(clust.Server, &appv1.ClusterInfo{
		ServerVersion: serverVersion,
		ConnectionState: appv1.ConnectionState{
			Status:     appv1.ConnectionStatusSuccessful,
			ModifiedAt: &v1.Time{Time: time.Now()},
		},
	})
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

func (s *Server) toAPIResponse(clust *appv1.Cluster) *appv1.Cluster {
	_ = s.cache.GetClusterInfo(clust.Server, &clust.Info)

	clust.Config.Password = ""
	clust.Config.BearerToken = ""
	clust.Config.TLSClientConfig.KeyData = nil
	if clust.Config.ExecProviderConfig != nil {
		// We can't know what the user has put into args or
		// env vars on the exec provider that might be sensitive
		// (e.g. --private-key=XXX, PASSWORD=XXX)
		// Implicitly assumes the command executable name is non-sensitive
		clust.Config.ExecProviderConfig.Env = make(map[string]string)
		clust.Config.ExecProviderConfig.Args = nil
	}
	// populate deprecated fields for backward compatibility
	clust.ServerVersion = clust.Info.ServerVersion
	clust.ConnectionState = clust.Info.ConnectionState
	return clust
}

// InvalidateCache invalidates cluster cache
func (s *Server) InvalidateCache(ctx context.Context, q *cluster.ClusterQuery) (*appv1.Cluster, error) {
	cls, err := s.getClusterWith403IfNotExist(ctx, q)
	if err != nil {
		return nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionUpdate, createRBACObject(cls.Project, q.Server)); err != nil {
		return nil, err
	}
	now := v1.Now()
	cls.RefreshRequestedAt = &now
	cls, err = s.db.UpdateCluster(ctx, cls)
	if err != nil {
		return nil, err
	}
	return s.toAPIResponse(cls), nil
}
