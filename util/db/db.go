package db

import (
	"context"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/settings"
)

// SecretMaperValidation determine whether the secret should be transformed(i.e. trailing CRLF characters trimmed)
type SecretMaperValidation struct {
	Dest      *string
	Transform func(string) string
}

type ArgoDB interface {
	// ListClusters lists configured clusters
	ListClusters(ctx context.Context) (*appv1.ClusterList, error)
	// CreateCluster creates a cluster
	CreateCluster(ctx context.Context, c *appv1.Cluster) (*appv1.Cluster, error)
	// WatchClusters allow watching for cluster informer
	WatchClusters(ctx context.Context,
		handleAddEvent func(cluster *appv1.Cluster),
		handleModEvent func(oldCluster *appv1.Cluster, newCluster *appv1.Cluster),
		handleDeleteEvent func(clusterServer string)) error
	// Get returns a cluster from a query
	GetCluster(ctx context.Context, server string) (*appv1.Cluster, error)
	// UpdateCluster updates a cluster
	UpdateCluster(ctx context.Context, c *appv1.Cluster) (*appv1.Cluster, error)
	// DeleteCluster deletes a cluster by name
	DeleteCluster(ctx context.Context, server string) error

	// ListRepositories lists repositories
	ListRepositories(ctx context.Context) ([]*appv1.Repository, error)

	// CreateRepository creates a repository
	CreateRepository(ctx context.Context, r *appv1.Repository) (*appv1.Repository, error)
	// GetRepository returns a repository by URL
	GetRepository(ctx context.Context, url string) (*appv1.Repository, error)
	// UpdateRepository updates a repository
	UpdateRepository(ctx context.Context, r *appv1.Repository) (*appv1.Repository, error)
	// DeleteRepository deletes a repository from config
	DeleteRepository(ctx context.Context, name string) error

	// ListRepoCredentials list all repo credential sets URL patterns
	ListRepositoryCredentials(ctx context.Context) ([]string, error)
	// GetRepoCredentials gets repo credentials for given URL
	GetRepositoryCredentials(ctx context.Context, name string) (*appv1.RepoCreds, error)
	// CreateRepoCredentials creates a repository credential set
	CreateRepositoryCredentials(ctx context.Context, r *appv1.RepoCreds) (*appv1.RepoCreds, error)
	// UpdateRepoCredentials updates a repository credential set
	UpdateRepositoryCredentials(ctx context.Context, r *appv1.RepoCreds) (*appv1.RepoCreds, error)
	// DeleteRepoCredentials deletes a repository credential set from config
	DeleteRepositoryCredentials(ctx context.Context, name string) error

	// ListRepoCertificates lists all configured certificates
	ListRepoCertificates(ctx context.Context, selector *CertificateListSelector) (*appv1.RepositoryCertificateList, error)
	// CreateRepoCertificate creates a new certificate entry
	CreateRepoCertificate(ctx context.Context, certificate *appv1.RepositoryCertificateList, upsert bool) (*appv1.RepositoryCertificateList, error)
	// CreateRepoCertificate creates a new certificate entry
	RemoveRepoCertificates(ctx context.Context, selector *CertificateListSelector) (*appv1.RepositoryCertificateList, error)

	// ListHelmRepositories lists repositories
	ListHelmRepositories(ctx context.Context) ([]*appv1.Repository, error)

	// ListConfiguredGPGPublicKeys returns all GPG public key IDs that are configured
	ListConfiguredGPGPublicKeys(ctx context.Context) (map[string]*appv1.GnuPGPublicKey, error)
	// AddGPGPublicKey adds one ore more GPG public keys to the configuration
	AddGPGPublicKey(ctx context.Context, keyData string) (map[string]*appv1.GnuPGPublicKey, []string, error)
	// DeleteGPGPublicKey removes a GPG public key from the configuration
	DeleteGPGPublicKey(ctx context.Context, keyID string) error
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

func (db *db) unmarshalFromSecretsStr(secrets map[*SecretMaperValidation]*v1.SecretKeySelector, cache map[string]*v1.Secret) error {
	for dst, src := range secrets {
		if src != nil {
			secret, err := db.getSecret(src.Name, cache)
			if err != nil {
				return err
			}
			if dst.Transform != nil {
				*dst.Dest = dst.Transform(string(secret.Data[src.Key]))
			} else {
				*dst.Dest = string(secret.Data[src.Key])
			}
		}
	}
	return nil
}

// StripCRLFCharacter strips the trailing CRLF characters
func StripCRLFCharacter(input string) string {
	return strings.TrimSpace(input)
}
