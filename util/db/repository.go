package db

import (
	"fmt"
	"hash/fnv"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apiv1 "k8s.io/api/core/v1"

	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

const (
	// Prefix to use for naming repository secrets
	repoSecretPrefix = "repo"
	// Prefix to use for naming credential template secrets
	credSecretPrefix = "creds"
	// The name of the key storing the username in the secret
	username = "username"
	// The name of the key storing the password in the secret
	password = "password"
	// The name of the key storing the SSH private in the secret
	sshPrivateKey = "sshPrivateKey"
	// The name of the key storing the TLS client cert data in the secret
	tlsClientCertData = "tlsClientCertData"
	// The name of the key storing the TLS client cert key in the secret
	tlsClientCertKey = "tlsClientCertKey"
	// The name of the key storing the GitHub App private key in the secret
	githubAppPrivateKey = "githubAppPrivateKey"
)

// repositoryBackend defines the API for types that wish to provide interaction with repository storage
type repositoryBackend interface {
	CreateRepository(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error)
	GetRepository(ctx context.Context, repoURL string) (*appsv1.Repository, error)
	ListRepositories(ctx context.Context, repoType *string) ([]*appsv1.Repository, error)
	UpdateRepository(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error)
	DeleteRepository(ctx context.Context, repoURL string) error
	RepositoryExists(ctx context.Context, repoURL string) (bool, error)

	CreateRepoCreds(ctx context.Context, r *appsv1.RepoCreds) (*appsv1.RepoCreds, error)
	GetRepoCreds(ctx context.Context, repoURL string) (*appsv1.RepoCreds, error)
	ListRepoCreds(ctx context.Context) ([]string, error)
	UpdateRepoCreds(ctx context.Context, r *appsv1.RepoCreds) (*appsv1.RepoCreds, error)
	DeleteRepoCreds(ctx context.Context, name string) error
	RepoCredsExists(ctx context.Context, repoURL string) (bool, error)

	GetAllHelmRepoCreds(ctx context.Context) ([]*appsv1.RepoCreds, error)
}

func (db *db) CreateRepository(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error) {
	secretBackend := db.repoBackend()
	legacyBackend := db.legacyRepoBackend()

	secretExists, err := secretBackend.RepositoryExists(ctx, r.Repo)
	if err != nil {
		return nil, err
	}
	legacyExists, err := legacyBackend.RepositoryExists(ctx, r.Repo)
	if err != nil {
		return nil, err
	}

	if secretExists || legacyExists {
		return nil, status.Errorf(codes.AlreadyExists, "repository %q already exists", r.Repo)
	}

	return secretBackend.CreateRepository(ctx, r)
}

func (db *db) GetRepository(ctx context.Context, repoURL string) (*appsv1.Repository, error) {
	repository, err := db.getRepository(ctx, repoURL)
	if err != nil {
		return repository, err
	}

	if err := db.enrichCredsToRepo(ctx, repository); err != nil {
		return repository, err
	}

	return repository, err
}

func (db *db) GetProjectRepositories(ctx context.Context, project string) ([]*appsv1.Repository, error) {
	informer, err := db.settingsMgr.GetSecretsInformer()
	if err != nil {
		return nil, err
	}
	secrets, err := informer.GetIndexer().ByIndex(settings.ByProjectRepoIndexer, project)
	if err != nil {
		return nil, err
	}
	var res []*appv1.Repository
	for i := range secrets {
		repo, err := secretToRepository(secrets[i].(*apiv1.Secret))
		if err != nil {
			return nil, err
		}
		res = append(res, repo)
	}
	return res, nil
}

func (db *db) RepositoryExists(ctx context.Context, repoURL string) (bool, error) {
	secretsBackend := db.repoBackend()
	exists, err := secretsBackend.RepositoryExists(ctx, repoURL)
	if exists || err != nil {
		return exists, err
	}

	legacyBackend := db.legacyRepoBackend()
	return legacyBackend.RepositoryExists(ctx, repoURL)
}

func (db *db) getRepository(ctx context.Context, repoURL string) (*appsv1.Repository, error) {
	secretsBackend := db.repoBackend()
	exists, err := secretsBackend.RepositoryExists(ctx, repoURL)
	if err != nil {
		return nil, err
	} else if exists {
		return secretsBackend.GetRepository(ctx, repoURL)
	}

	legacyBackend := db.legacyRepoBackend()
	exists, err = legacyBackend.RepositoryExists(ctx, repoURL)
	if err != nil {
		return nil, err
	} else if exists {
		return legacyBackend.GetRepository(ctx, repoURL)
	}

	return &appsv1.Repository{Repo: repoURL}, nil
}

func (db *db) ListRepositories(ctx context.Context) ([]*appsv1.Repository, error) {
	return db.listRepositories(ctx, nil)
}

func (db *db) listRepositories(ctx context.Context, repoType *string) ([]*appsv1.Repository, error) {
	// TODO It would be nice to check for duplicates between secret and legacy repositories and make it so that
	// 	repositories from secrets overlay repositories from legacys.

	secretRepositories, err := db.repoBackend().ListRepositories(ctx, repoType)
	if err != nil {
		return nil, err
	}

	legacyRepositories, err := db.legacyRepoBackend().ListRepositories(ctx, repoType)
	if err != nil {
		return nil, err
	}

	repositories := append(secretRepositories, legacyRepositories...)
	if err := db.enrichCredsToRepos(ctx, repositories); err != nil {
		return nil, err
	}

	return repositories, nil
}

// UpdateRepository updates a repository
func (db *db) UpdateRepository(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error) {
	secretsBackend := db.repoBackend()
	exists, err := secretsBackend.RepositoryExists(ctx, r.Repo)
	if err != nil {
		return nil, err
	} else if exists {
		return secretsBackend.UpdateRepository(ctx, r)
	}

	legacyBackend := db.legacyRepoBackend()
	exists, err = legacyBackend.RepositoryExists(ctx, r.Repo)
	if err != nil {
		return nil, err
	} else if exists {
		return legacyBackend.UpdateRepository(ctx, r)
	}

	return nil, status.Errorf(codes.NotFound, "repo '%s' not found", r.Repo)
}

func (db *db) DeleteRepository(ctx context.Context, repoURL string) error {
	secretsBackend := db.repoBackend()
	exists, err := secretsBackend.RepositoryExists(ctx, repoURL)
	if err != nil {
		return err
	} else if exists {
		return secretsBackend.DeleteRepository(ctx, repoURL)
	}

	legacyBackend := db.legacyRepoBackend()
	exists, err = legacyBackend.RepositoryExists(ctx, repoURL)
	if err != nil {
		return err
	} else if exists {
		return legacyBackend.DeleteRepository(ctx, repoURL)
	}

	return status.Errorf(codes.NotFound, "repo '%s' not found", repoURL)
}

// ListRepositoryCredentials returns a list of URLs that contain repo credential sets
func (db *db) ListRepositoryCredentials(ctx context.Context) ([]string, error) {
	// TODO It would be nice to check for duplicates between secret and legacy repositories and make it so that
	// 	repositories from secrets overlay repositories from legacys.

	secretRepoCreds, err := db.repoBackend().ListRepoCreds(ctx)
	if err != nil {
		return nil, err
	}

	legacyRepoCreds, err := db.legacyRepoBackend().ListRepoCreds(ctx)
	if err != nil {
		return nil, err
	}

	return append(secretRepoCreds, legacyRepoCreds...), nil
}

// GetRepositoryCredentials retrieves a repository credential set
func (db *db) GetRepositoryCredentials(ctx context.Context, repoURL string) (*appsv1.RepoCreds, error) {
	secretsBackend := db.repoBackend()
	exists, err := secretsBackend.RepoCredsExists(ctx, repoURL)
	if err != nil {
		return nil, err
	} else if exists {
		return secretsBackend.GetRepoCreds(ctx, repoURL)
	}

	legacyBackend := db.legacyRepoBackend()
	exists, err = legacyBackend.RepoCredsExists(ctx, repoURL)
	if err != nil {
		return nil, err
	} else if exists {
		return legacyBackend.GetRepoCreds(ctx, repoURL)
	}

	return nil, nil
}

// GetAllHelmRepositoryCredentials retrieves all repository credentials
func (db *db) GetAllHelmRepositoryCredentials(ctx context.Context) ([]*appsv1.RepoCreds, error) {
	// TODO It would be nice to check for duplicates between secret and legacy repositories and make it so that
	// 	repositories from secrets overlay repositories from legacys.

	secretRepoCreds, err := db.repoBackend().GetAllHelmRepoCreds(ctx)
	if err != nil {
		return nil, err
	}

	legacyRepoCreds, err := db.legacyRepoBackend().GetAllHelmRepoCreds(ctx)
	if err != nil {
		return nil, err
	}

	return append(secretRepoCreds, legacyRepoCreds...), nil
}

// CreateRepositoryCredentials creates a repository credential set
func (db *db) CreateRepositoryCredentials(ctx context.Context, r *appsv1.RepoCreds) (*appsv1.RepoCreds, error) {
	legacyBackend := db.legacyRepoBackend()
	secretBackend := db.repoBackend()

	secretExists, err := secretBackend.RepositoryExists(ctx, r.URL)
	if err != nil {
		return nil, err
	}
	legacyExists, err := legacyBackend.RepositoryExists(ctx, r.URL)
	if err != nil {
		return nil, err
	}

	if secretExists || legacyExists {
		return nil, status.Errorf(codes.AlreadyExists, "repository credentials %q already exists", r.URL)
	}

	return secretBackend.CreateRepoCreds(ctx, r)
}

// UpdateRepositoryCredentials updates a repository credential set
func (db *db) UpdateRepositoryCredentials(ctx context.Context, r *appsv1.RepoCreds) (*appsv1.RepoCreds, error) {
	secretsBackend := db.repoBackend()
	exists, err := secretsBackend.RepoCredsExists(ctx, r.URL)
	if err != nil {
		return nil, err
	} else if exists {
		return secretsBackend.UpdateRepoCreds(ctx, r)
	}

	legacyBackend := db.legacyRepoBackend()
	exists, err = legacyBackend.RepoCredsExists(ctx, r.URL)
	if err != nil {
		return nil, err
	} else if exists {
		return legacyBackend.UpdateRepoCreds(ctx, r)
	}

	return nil, status.Errorf(codes.NotFound, "repository credentials '%s' not found", r.URL)
}

// DeleteRepositoryCredentials deletes a repository credential set from config, and
// also all the secrets which actually contained the credentials.
func (db *db) DeleteRepositoryCredentials(ctx context.Context, name string) error {
	secretsBackend := db.repoBackend()
	exists, err := secretsBackend.RepoCredsExists(ctx, name)
	if err != nil {
		return err
	} else if exists {
		return secretsBackend.DeleteRepoCreds(ctx, name)
	}

	legacyBackend := db.legacyRepoBackend()
	exists, err = legacyBackend.RepoCredsExists(ctx, name)
	if err != nil {
		return err
	} else if exists {
		return legacyBackend.DeleteRepoCreds(ctx, name)
	}

	return status.Errorf(codes.NotFound, "repository credentials '%s' not found", name)
}

func (db *db) enrichCredsToRepos(ctx context.Context, repositories []*appsv1.Repository) error {
	for _, repository := range repositories {
		if err := db.enrichCredsToRepo(ctx, repository); err != nil {
			return err
		}
	}
	return nil
}

func (db *db) repoBackend() repositoryBackend {
	return &secretsRepositoryBackend{db: db}
}

func (db *db) legacyRepoBackend() repositoryBackend {
	return &legacyRepositoryBackend{db: db}
}

func (db *db) enrichCredsToRepo(ctx context.Context, repository *appsv1.Repository) error {
	if !repository.HasCredentials() {
		creds, err := db.GetRepositoryCredentials(ctx, repository.Repo)
		if err == nil {
			if creds != nil {
				repository.CopyCredentialsFrom(creds)
				repository.InheritedCreds = true
			}
		} else {
			return err
		}
	} else {
		log.Debugf("%s has credentials", repository.Repo)
	}

	return nil
}

// RepoURLToSecretName hashes repo URL to a secret name using a formula. This is used when
// repositories are _imperatively_ created and need its credentials to be stored in a secret.
// NOTE: this formula should not be considered stable and may change in future releases.
// Do NOT rely on this formula as a means of secret lookup, only secret creation.
func RepoURLToSecretName(prefix string, repo string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(repo))
	return fmt.Sprintf("%s-%v", prefix, h.Sum32())
}
