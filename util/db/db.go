package db

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/settings"
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

	// ListRepoURLs lists repositories
	ListRepoURLs(ctx context.Context) ([]string, error)
	// CreateRepository creates a repository
	CreateRepository(ctx context.Context, r *appv1.Repository) (*appv1.Repository, error)
	// GetRepository returns a repository by URL
	GetRepository(ctx context.Context, name string) (*appv1.Repository, error)
	// UpdateRepository updates a repository
	UpdateRepository(ctx context.Context, r *appv1.Repository) (*appv1.Repository, error)
	// DeleteRepository updates a repository
	DeleteRepository(ctx context.Context, name string) error

	// ListHelmRepoURLs lists configured helm repositories
	ListHelmRepos(ctx context.Context) ([]*appv1.HelmRepository, error)

	// ListRepoCerticifates lists all configured certificates
	ListRepoCertificates(ctx context.Context, selector *CertificateListSelector) (*appv1.RepositoryCertificateList, error)
	// CreateRepoCertificate creates a new certificate entry
	CreateRepoCertificate(ctx context.Context, certificate *appv1.RepositoryCertificateList, upsert bool) (*appv1.RepositoryCertificateList, error)
	// CreateRepoCertificate creates a new certificate entry
	RemoveRepoCertificates(ctx context.Context, selector *CertificateListSelector) (*appv1.RepositoryCertificateList, error)
}

type db struct {
	ns            string
	kubeclientset kubernetes.Interface
	settingsMgr   *settings.SettingsManager
}

// NewDB returns a new instance of the argo database
func NewDB(namespace string, settingsMgr *settings.SettingsManager, kubeclientset kubernetes.Interface) ArgoDB {
	return &db{
		settingsMgr:   settingsMgr,
		ns:            namespace,
		kubeclientset: kubeclientset,
	}
}

func (db *db) getSecret(name string, cache map[string]*v1.Secret) (*v1.Secret, error) {
	secret, ok := cache[name]
	if !ok {
		secretsLister, err := db.settingsMgr.GetSecretsLister()
		if err != nil {
			return nil, err
		}
		secret, err = secretsLister.Secrets(db.ns).Get(name)
		if err != nil {
			return nil, err
		}
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		cache[name] = secret
	}
	return secret, nil
}

func (db *db) unmarshalFromSecretsStr(secrets map[*string]*v1.SecretKeySelector, cache map[string]*v1.Secret) error {
	for dst, src := range secrets {
		if src != nil {
			secret, err := db.getSecret(src.Name, cache)
			if err != nil {
				return err
			}
			*dst = string(secret.Data[src.Key])
		}
	}
	return nil
}

func (db *db) unmarshalFromSecretsBytes(secrets map[*[]byte]*v1.SecretKeySelector, cache map[string]*v1.Secret) error {
	for dst, src := range secrets {
		if src != nil {
			secret, err := db.getSecret(src.Name, cache)
			if err != nil {
				return err
			}
			*dst = secret.Data[src.Key]
		}
	}
	return nil
}
