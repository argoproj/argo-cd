package db

import (
	"time"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type ArgoDB interface {
	// ListClusters lists configured clusters
	ListClusters(ctx context.Context) (*appv1.ClusterList, error)
	// CreateCluster creates a cluster
	CreateCluster(ctx context.Context, c *appv1.Cluster) (*appv1.Cluster, error)
	// WatchClusters allow watching for cluster events
	WatchClusters(ctx context.Context, callback func(*ClusterEvent)) error
	// Get returns a cluster from a query
	GetCluster(ctx context.Context, name string) (*appv1.Cluster, error)
	// UpdateCluster updates a cluster
	UpdateCluster(ctx context.Context, c *appv1.Cluster) (*appv1.Cluster, error)
	// DeleteCluster deletes a cluster by name
	DeleteCluster(ctx context.Context, name string) error
	// InstallClusterManagerRBAC installs RBAC resources for a cluster manager
	InstallClusterManagerRBAC(ctx context.Context) (string, error)
	// UninstallClusterManagerRBAC removes RBAC resources for a cluster manager
	UninstallClusterManagerRBAC(ctx context.Context) error

	// ListRepositories lists repositories
	ListRepositories(ctx context.Context) (*appv1.RepositoryList, error)
	// CreateRepository creates a repository
	CreateRepository(ctx context.Context, r *appv1.Repository) (*appv1.Repository, error)
	// GetRepository returns a repository by URL
	GetRepository(ctx context.Context, name string) (*appv1.Repository, error)
	// UpdateRepository updates a repository
	UpdateRepository(ctx context.Context, r *appv1.Repository) (*appv1.Repository, error)
	// DeleteRepository updates a repository
	DeleteRepository(ctx context.Context, name string) error
}

type db struct {
	ns            string
	kubeclientset kubernetes.Interface
}

// NewDB returns a new instance of the argo database
func NewDB(namespace string, kubeclientset kubernetes.Interface) ArgoDB {
	return &db{
		ns:            namespace,
		kubeclientset: kubeclientset,
	}
}

func AnnotationsFromConnectionState(connectionState *appv1.ConnectionState) map[string]string {
	attemptedAtStr := ""
	if connectionState.ModifiedAt != nil {
		attemptedAtStr = connectionState.ModifiedAt.Format(time.RFC3339)
	}
	return map[string]string{
		common.AnnotationConnectionMessage:    connectionState.Message,
		common.AnnotationConnectionStatus:     connectionState.Status,
		common.AnnotationConnectionModifiedAt: attemptedAtStr,
	}
}

func ConnectionStateFromAnnotations(annotations map[string]string) appv1.ConnectionState {
	status := annotations[common.AnnotationConnectionStatus]
	if status == "" {
		status = appv1.ConnectionStatusUnknown
	}
	attemptedAtStr := annotations[common.AnnotationConnectionModifiedAt]
	var attemptedAtMetaTimePtr *metav1.Time
	if attemptedAtStr != "" {
		attemptedAtTime, err := time.Parse(time.RFC3339, attemptedAtStr)
		if err != nil {
			log.Warnf("Unable to parse connection status attemptedAt time")
		} else {
			attemptedAtMetaTime := metav1.NewTime(attemptedAtTime)
			attemptedAtMetaTimePtr = &attemptedAtMetaTime
		}

	}
	return appv1.ConnectionState{
		Status:     status,
		Message:    annotations[common.AnnotationConnectionMessage],
		ModifiedAt: attemptedAtMetaTimePtr,
	}
}
