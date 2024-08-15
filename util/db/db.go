package db

import (
	"context"
	"math"
	"strings"

	v1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/common"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/settings"
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
	// GetCluster returns a cluster by given server url
	GetCluster(ctx context.Context, server string) (*appv1.Cluster, error)
	// GetClusterServersByName returns a cluster server urls by given cluster name
	GetClusterServersByName(ctx context.Context, name string) ([]string, error)
	// GetProjectClusters return project scoped clusters by given project name
	GetProjectClusters(ctx context.Context, project string) ([]*appv1.Cluster, error)
	// UpdateCluster updates a cluster
	UpdateCluster(ctx context.Context, c *appv1.Cluster) (*appv1.Cluster, error)
	// DeleteCluster deletes a cluster by name
	DeleteCluster(ctx context.Context, server string) error

	// ListRepositories lists repositories
	ListRepositories(ctx context.Context) ([]*appv1.Repository, error)

	// CreateRepository creates a repository
	CreateRepository(ctx context.Context, r *appv1.Repository) (*appv1.Repository, error)
	// GetRepository returns a repository by URL
	GetRepository(ctx context.Context, url, project string) (*appv1.Repository, error)
	// GetProjectRepositories returns project scoped repositories by given project name
	GetProjectRepositories(ctx context.Context, project string) ([]*appv1.Repository, error)
	// RepositoryExists returns whether a repository is configured for the given URL
	RepositoryExists(ctx context.Context, repoURL, project string) (bool, error)
	// UpdateRepository updates a repository
	UpdateRepository(ctx context.Context, r *appv1.Repository) (*appv1.Repository, error)
	// DeleteRepository deletes a repository from config
	DeleteRepository(ctx context.Context, name, project string) error

	// ListRepositoryCredentials list all repo credential sets URL patterns
	ListRepositoryCredentials(ctx context.Context) ([]string, error)
	// GetRepositoryCredentials gets repo credentials for given URL
	GetRepositoryCredentials(ctx context.Context, name string) (*appv1.RepoCreds, error)
	// CreateRepositoryCredentials creates a repository credential set
	CreateRepositoryCredentials(ctx context.Context, r *appv1.RepoCreds) (*appv1.RepoCreds, error)
	// UpdateRepositoryCredentials updates a repository credential set
	UpdateRepositoryCredentials(ctx context.Context, r *appv1.RepoCreds) (*appv1.RepoCreds, error)
	// DeleteRepositoryCredentials deletes a repository credential set from config
	DeleteRepositoryCredentials(ctx context.Context, name string) error

	// ListRepoCertificates lists all configured certificates
	ListRepoCertificates(ctx context.Context, selector *CertificateListSelector) (*appv1.RepositoryCertificateList, error)
	// CreateRepoCertificate creates a new certificate entry
	CreateRepoCertificate(ctx context.Context, certificate *appv1.RepositoryCertificateList, upsert bool) (*appv1.RepositoryCertificateList, error)
	// RemoveRepoCertificates removes certificates based upon a selector
	RemoveRepoCertificates(ctx context.Context, selector *CertificateListSelector) (*appv1.RepositoryCertificateList, error)
	// GetAllHelmRepositoryCredentials gets all repo credentials
	GetAllHelmRepositoryCredentials(ctx context.Context) ([]*appv1.RepoCreds, error)

	// ListHelmRepositories lists repositories
	ListHelmRepositories(ctx context.Context) ([]*appv1.Repository, error)

	// ListConfiguredGPGPublicKeys returns all GPG public key IDs that are configured
	ListConfiguredGPGPublicKeys(ctx context.Context) (map[string]*appv1.GnuPGPublicKey, error)
	// AddGPGPublicKey adds one or more GPG public keys to the configuration
	AddGPGPublicKey(ctx context.Context, keyData string) (map[string]*appv1.GnuPGPublicKey, []string, error)
	// DeleteGPGPublicKey removes a GPG public key from the configuration
	DeleteGPGPublicKey(ctx context.Context, keyID string) error

	// GetApplicationControllerReplicas gets the replicas of application controller
	GetApplicationControllerReplicas() int
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

// GetApplicationControllerReplicas gets the replicas of application controller
func (db *db) GetApplicationControllerReplicas() int {
	// get the replicas from application controller deployment, if the application controller deployment does not exist, check for environment variable
	applicationControllerName := env.StringFromEnv(common.EnvAppControllerName, common.DefaultApplicationControllerName)
	appControllerDeployment, err := db.kubeclientset.AppsV1().Deployments(db.settingsMgr.GetNamespace()).Get(context.Background(), applicationControllerName, metav1.GetOptions{})
	if err != nil {
		appControllerDeployment = nil
		if !kubeerrors.IsNotFound(err) {
			log.Warnf("error retrieveing Argo CD controller deployment: %s", err)
		}
	}
	if appControllerDeployment != nil && appControllerDeployment.Spec.Replicas != nil {
		return int(*appControllerDeployment.Spec.Replicas)
	}
	return env.ParseNumFromEnv(common.EnvControllerReplicas, 0, 0, math.MaxInt32)
}
