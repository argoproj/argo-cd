package db

import (
	"context"
	"fmt"
	"hash/fnv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
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
	// The name of the project storing the project in the secret
	project = "project"
	// The name of the key storing the SSH private in the secret
	sshPrivateKey = "sshPrivateKey"
)

// repositoryBackend defines the API for types that wish to provide interaction with repository storage
type repositoryBackend interface {
	CreateRepository(ctx context.Context, r *v1alpha1.Repository) (*v1alpha1.Repository, error)
	GetRepository(ctx context.Context, repoURL, project string) (*v1alpha1.Repository, error)
	ListRepositories(ctx context.Context, repoType *string) ([]*v1alpha1.Repository, error)
	UpdateRepository(ctx context.Context, r *v1alpha1.Repository) (*v1alpha1.Repository, error)
	DeleteRepository(ctx context.Context, repoURL, project string) error
	RepositoryExists(ctx context.Context, repoURL, project string, allowFallback bool) (bool, error)

	CreateRepoCreds(ctx context.Context, r *v1alpha1.RepoCreds) (*v1alpha1.RepoCreds, error)
	GetRepoCreds(ctx context.Context, repoURL string) (*v1alpha1.RepoCreds, error)
	ListRepoCreds(ctx context.Context) ([]string, error)
	UpdateRepoCreds(ctx context.Context, r *v1alpha1.RepoCreds) (*v1alpha1.RepoCreds, error)
	DeleteRepoCreds(ctx context.Context, name string) error
	RepoCredsExists(ctx context.Context, repoURL string) (bool, error)

	GetAllHelmRepoCreds(ctx context.Context) ([]*v1alpha1.RepoCreds, error)
	GetAllOCIRepoCreds(ctx context.Context) ([]*v1alpha1.RepoCreds, error)
}

func (db *db) CreateRepository(ctx context.Context, r *v1alpha1.Repository) (*v1alpha1.Repository, error) {
	secretBackend := db.repoBackend()

	secretExists, err := secretBackend.RepositoryExists(ctx, r.Repo, r.Project, false)
	if err != nil {
		return nil, err
	}

	if secretExists {
		return nil, status.Errorf(codes.AlreadyExists, "repository %q already exists", r.Repo)
	}

	return secretBackend.CreateRepository(ctx, r)
}

func (db *db) CreateWriteRepository(ctx context.Context, r *v1alpha1.Repository) (*v1alpha1.Repository, error) {
	secretBackend := db.repoWriteBackend()
	secretExists, err := secretBackend.RepositoryExists(ctx, r.Repo, r.Project, false)
	if err != nil {
		return nil, err
	}

	if secretExists {
		return nil, status.Errorf(codes.AlreadyExists, "repository %q already exists", r.Repo)
	}

	return secretBackend.CreateRepository(ctx, r)
}

func (db *db) GetRepository(ctx context.Context, repoURL, project string) (*v1alpha1.Repository, error) {
	repository, err := db.getRepository(ctx, repoURL, project)
	if err != nil {
		return repository, fmt.Errorf("unable to get repository %q: %w", repoURL, err)
	}

	if err := db.enrichCredsToRepo(ctx, repository); err != nil {
		return repository, fmt.Errorf("unable to enrich repository %q info with credentials: %w", repoURL, err)
	}

	return repository, err
}

func (db *db) GetWriteRepository(ctx context.Context, repoURL, project string) (*v1alpha1.Repository, error) {
	repository, err := db.repoWriteBackend().GetRepository(ctx, repoURL, project)
	if err != nil {
		return repository, fmt.Errorf("unable to get write repository %q: %w", repoURL, err)
	}

	// TODO: enrich with write credentials.
	// if err := db.enrichCredsToRepo(ctx, repository); err != nil {
	//	 return repository, fmt.Errorf("unable to enrich write repository %q info with credentials: %w", repoURL, err)
	// }

	return repository, err
}

func (db *db) GetProjectRepositories(project string) ([]*v1alpha1.Repository, error) {
	return db.getRepositories(settings.ByProjectRepoIndexer, project)
}

func (db *db) GetProjectWriteRepositories(project string) ([]*v1alpha1.Repository, error) {
	return db.getRepositories(settings.ByProjectRepoWriteIndexer, project)
}

func (db *db) getRepositories(indexer, project string) ([]*v1alpha1.Repository, error) {
	informer, err := db.settingsMgr.GetSecretsInformer()
	if err != nil {
		return nil, err
	}
	secrets, err := informer.GetIndexer().ByIndex(indexer, project)
	if err != nil {
		return nil, err
	}
	var res []*v1alpha1.Repository
	for i := range secrets {
		repo, err := secretToRepository(secrets[i].(*corev1.Secret))
		if err != nil {
			return nil, err
		}
		res = append(res, repo)
	}
	return res, nil
}

func (db *db) RepositoryExists(ctx context.Context, repoURL, project string) (bool, error) {
	secretsBackend := db.repoBackend()
	return secretsBackend.RepositoryExists(ctx, repoURL, project, true)
}

func (db *db) WriteRepositoryExists(ctx context.Context, repoURL, project string) (bool, error) {
	secretsBackend := db.repoWriteBackend()
	return secretsBackend.RepositoryExists(ctx, repoURL, project, true)
}

func (db *db) getRepository(ctx context.Context, repoURL, project string) (*v1alpha1.Repository, error) {
	secretsBackend := db.repoBackend()
	exists, err := secretsBackend.RepositoryExists(ctx, repoURL, project, true)
	if err != nil {
		return nil, fmt.Errorf("unable to check if repository %q exists from secrets backend: %w", repoURL, err)
	} else if exists {
		repository, err := secretsBackend.GetRepository(ctx, repoURL, project)
		if err != nil {
			return nil, fmt.Errorf("unable to get repository %q from secrets backend: %w", repoURL, err)
		}
		return repository, nil
	}

	return &v1alpha1.Repository{Repo: repoURL}, nil
}

func (db *db) ListRepositories(ctx context.Context) ([]*v1alpha1.Repository, error) {
	return db.listRepositories(ctx, nil, false)
}

func (db *db) ListWriteRepositories(ctx context.Context) ([]*v1alpha1.Repository, error) {
	return db.listRepositories(ctx, nil, true)
}

func (db *db) listRepositories(ctx context.Context, repoType *string, writeCreds bool) ([]*v1alpha1.Repository, error) {
	var backend repositoryBackend
	if writeCreds {
		backend = db.repoWriteBackend()
	} else {
		backend = db.repoBackend()
	}
	repositories, err := backend.ListRepositories(ctx, repoType)
	if err != nil {
		return nil, err
	}
	err = db.enrichCredsToRepos(ctx, repositories)
	if err != nil {
		return nil, err
	}

	return repositories, nil
}

func (db *db) ListOCIRepositories(ctx context.Context) ([]*v1alpha1.Repository, error) {
	var result []*v1alpha1.Repository
	repos, err := db.listRepositories(ctx, ptr.To("oci"), false)
	if err != nil {
		return nil, fmt.Errorf("failed to list OCI repositories: %w", err)
	}
	result = append(result, v1alpha1.Repositories(repos).Filter(func(r *v1alpha1.Repository) bool {
		return r.Type == "oci"
	})...)
	return result, nil
}

// UpdateRepository updates a repository
func (db *db) UpdateRepository(ctx context.Context, r *v1alpha1.Repository) (*v1alpha1.Repository, error) {
	secretsBackend := db.repoBackend()
	exists, err := secretsBackend.RepositoryExists(ctx, r.Repo, r.Project, false)
	if err != nil {
		return nil, err
	} else if exists {
		return secretsBackend.UpdateRepository(ctx, r)
	}

	return nil, status.Errorf(codes.NotFound, "repo '%s' not found", r.Repo)
}

func (db *db) UpdateWriteRepository(ctx context.Context, r *v1alpha1.Repository) (*v1alpha1.Repository, error) {
	secretBackend := db.repoWriteBackend()
	exists, err := secretBackend.RepositoryExists(ctx, r.Repo, r.Project, false)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, status.Errorf(codes.NotFound, "repo '%s' not found", r.Repo)
	}

	return secretBackend.UpdateRepository(ctx, r)
}

func (db *db) DeleteRepository(ctx context.Context, repoURL, project string) error {
	secretsBackend := db.repoBackend()
	exists, err := secretsBackend.RepositoryExists(ctx, repoURL, project, false)
	if err != nil {
		return err
	} else if exists {
		return secretsBackend.DeleteRepository(ctx, repoURL, project)
	}

	return status.Errorf(codes.NotFound, "repo '%s' not found", repoURL)
}

func (db *db) DeleteWriteRepository(ctx context.Context, repoURL, project string) error {
	secretsBackend := db.repoWriteBackend()
	exists, err := secretsBackend.RepositoryExists(ctx, repoURL, project, false)
	if err != nil {
		return err
	}

	if !exists {
		return status.Errorf(codes.NotFound, "repo '%s' not found", repoURL)
	}

	return secretsBackend.DeleteRepository(ctx, repoURL, project)
}

// ListRepositoryCredentials returns a list of URLs that contain repo credential sets
func (db *db) ListRepositoryCredentials(ctx context.Context) ([]string, error) {
	secretRepoCreds, err := db.repoBackend().ListRepoCreds(ctx)
	if err != nil {
		return nil, err
	}

	return secretRepoCreds, nil
}

// ListWriteRepositoryCredentials returns a list of URLs that contain repo write credential sets
func (db *db) ListWriteRepositoryCredentials(ctx context.Context) ([]string, error) {
	secretRepoCreds, err := db.repoWriteBackend().ListRepoCreds(ctx)
	if err != nil {
		return nil, err
	}
	return secretRepoCreds, nil
}

// GetRepositoryCredentials retrieves a repository credential set
func (db *db) GetRepositoryCredentials(ctx context.Context, repoURL string) (*v1alpha1.RepoCreds, error) {
	secretsBackend := db.repoBackend()
	exists, err := secretsBackend.RepoCredsExists(ctx, repoURL)
	if err != nil {
		return nil, fmt.Errorf("unable to check if repository credentials for %q exists from secrets backend: %w", repoURL, err)
	} else if exists {
		creds, err := secretsBackend.GetRepoCreds(ctx, repoURL)
		if err != nil {
			return nil, fmt.Errorf("unable to get repository credentials for %q from secrets backend: %w", repoURL, err)
		}
		return creds, nil
	}

	return nil, nil
}

// GetWriteRepositoryCredentials retrieves a repository write credential set
func (db *db) GetWriteRepositoryCredentials(ctx context.Context, repoURL string) (*v1alpha1.RepoCreds, error) {
	secretBackend := db.repoWriteBackend()
	exists, err := secretBackend.RepoCredsExists(ctx, repoURL)
	if err != nil {
		return nil, fmt.Errorf("unable to check if repository write credentials for %q exists from secrets backend: %w", repoURL, err)
	}

	if !exists {
		return nil, nil
	}

	// TODO: enrich with write credentials.
	// if err := db.enrichCredsToRepo(ctx, repository); err != nil {
	//	 return repository, fmt.Errorf("unable to enrich write repository %q info with credentials: %w", repoURL, err)
	// }

	creds, err := secretBackend.GetRepoCreds(ctx, repoURL)
	if err != nil {
		return nil, fmt.Errorf("unable to get repository write credentials for %q from secrets backend: %w", repoURL, err)
	}

	return creds, nil
}

// GetAllHelmRepositoryCredentials retrieves all repository credentials
func (db *db) GetAllHelmRepositoryCredentials(ctx context.Context) ([]*v1alpha1.RepoCreds, error) {
	secretRepoCreds, err := db.repoBackend().GetAllHelmRepoCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all Helm repo creds: %w", err)
	}

	return secretRepoCreds, nil
}

// GetAllOCIRepositoryCredentials retrieves all repository credentials
func (db *db) GetAllOCIRepositoryCredentials(ctx context.Context) ([]*v1alpha1.RepoCreds, error) {
	secretRepoCreds, err := db.repoBackend().GetAllOCIRepoCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all Helm repo creds: %w", err)
	}

	return secretRepoCreds, nil
}

// CreateRepositoryCredentials creates a repository credential set
func (db *db) CreateRepositoryCredentials(ctx context.Context, r *v1alpha1.RepoCreds) (*v1alpha1.RepoCreds, error) {
	secretBackend := db.repoBackend()

	secretExists, err := secretBackend.RepositoryExists(ctx, r.URL, "", false)
	if err != nil {
		return nil, err
	}

	if secretExists {
		return nil, status.Errorf(codes.AlreadyExists, "repository credentials %q already exists", r.URL)
	}

	return secretBackend.CreateRepoCreds(ctx, r)
}

// CreateWriteRepositoryCredentials creates a repository write credential set
func (db *db) CreateWriteRepositoryCredentials(ctx context.Context, r *v1alpha1.RepoCreds) (*v1alpha1.RepoCreds, error) {
	secretBackend := db.repoWriteBackend()
	secretExists, err := secretBackend.RepoCredsExists(ctx, r.URL)
	if err != nil {
		return nil, err
	}

	if secretExists {
		return nil, status.Errorf(codes.AlreadyExists, "write repository credentials %q already exists", r.URL)
	}

	return secretBackend.CreateRepoCreds(ctx, r)
}

// UpdateRepositoryCredentials updates a repository credential set
func (db *db) UpdateRepositoryCredentials(ctx context.Context, r *v1alpha1.RepoCreds) (*v1alpha1.RepoCreds, error) {
	secretsBackend := db.repoBackend()
	exists, err := secretsBackend.RepoCredsExists(ctx, r.URL)
	if err != nil {
		return nil, err
	} else if exists {
		return secretsBackend.UpdateRepoCreds(ctx, r)
	}

	return nil, status.Errorf(codes.NotFound, "repository credentials '%s' not found", r.URL)
}

// UpdateWriteRepositoryCredentials updates a repository write credential set
func (db *db) UpdateWriteRepositoryCredentials(ctx context.Context, r *v1alpha1.RepoCreds) (*v1alpha1.RepoCreds, error) {
	secretBackend := db.repoWriteBackend()
	exists, err := secretBackend.RepoCredsExists(ctx, r.URL)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, status.Errorf(codes.NotFound, "write repository credentials '%s' not found", r.URL)
	}

	return secretBackend.UpdateRepoCreds(ctx, r)
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

	return status.Errorf(codes.NotFound, "repository credentials '%s' not found", name)
}

// DeleteWriteRepositoryCredentials deletes a repository write credential set from config, and
// also all the secrets which actually contained the credentials.
func (db *db) DeleteWriteRepositoryCredentials(ctx context.Context, name string) error {
	secretBackend := db.repoWriteBackend()
	exists, err := secretBackend.RepoCredsExists(ctx, name)
	if err != nil {
		return err
	} else if exists {
		return secretBackend.DeleteRepoCreds(ctx, name)
	}
	return status.Errorf(codes.NotFound, "write repository credentials '%s' not found", name)
}

func (db *db) enrichCredsToRepos(ctx context.Context, repositories []*v1alpha1.Repository) error {
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

func (db *db) repoWriteBackend() repositoryBackend {
	return &secretsRepositoryBackend{db: db, writeCreds: true}
}

func (db *db) enrichCredsToRepo(ctx context.Context, repository *v1alpha1.Repository) error {
	if !repository.HasCredentials() {
		creds, err := db.GetRepositoryCredentials(ctx, repository.Repo)
		if err != nil {
			return fmt.Errorf("failed to get repository credentials for %q: %w", repository.Repo, err)
		}
		if creds != nil {
			repository.CopyCredentialsFrom(creds)
			repository.InheritedCreds = true
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
func RepoURLToSecretName(prefix string, repo string, project string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(repo))
	_, _ = h.Write([]byte(project))
	return fmt.Sprintf("%s-%v", prefix, h.Sum32())
}
