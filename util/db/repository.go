package db

import (
	"fmt"
	"hash/fnv"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

const (
	// Prefix to use for naming repository secrets
	repoSecretPrefix = "repo"
	// Prefix to use for naming repository configuration secrets
	repoConfigSecretPrefix = "repoconfig"
	// Prefix to use for naming credential template secrets
	credSecretPrefix = "creds"
	// Prefix to use for naming credential template configuration secrets
	credConfigSecretPrefix = "repocreds"
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
	secretBackend := repositoryBackend(&secretsRepositoryBackend{db: db})
	settingBackend := repositoryBackend(&settingRepositoryBackend{db: db})

	secretExists, err := secretBackend.RepositoryExists(ctx, r.Repo)
	if err != nil {
		return nil, err
	}
	settingExists, err := settingBackend.RepositoryExists(ctx, r.Repo)
	if err != nil {
		return nil, err
	}

	if secretExists || settingExists {
		return nil, status.Errorf(codes.AlreadyExists, "repository %q already exists", r.Repo)
	}

	return secretBackend.CreateRepository(ctx, r)
}

func (db *db) GetRepository(ctx context.Context, repoURL string) (*appsv1.Repository, error) {
	secretsBackend := &secretsRepositoryBackend{db: db}
	exists, err := secretsBackend.RepositoryExists(ctx, repoURL)
	if err != nil {
		return nil, err
	} else if exists {
		return secretsBackend.GetRepository(ctx, repoURL)
	}

	settingsBackend := &settingRepositoryBackend{db: db}
	exists, err = settingsBackend.RepositoryExists(ctx, repoURL)
	if err != nil {
		return nil, err
	} else if exists {
		return settingsBackend.GetRepository(ctx, repoURL)
	}

	return &appsv1.Repository{Repo: repoURL}, nil
}

func (db *db) ListRepositories(ctx context.Context) ([]*appsv1.Repository, error) {
	return db.listRepositories(ctx, nil)
}

func (db *db) listRepositories(ctx context.Context, repoType *string) ([]*appsv1.Repository, error) {
	// TODO It would be nice to check for duplicates between secret and setting repositories and make it so that
	// 	repositories from secrets overlay repositories from settings.

	secretRepositories, err := (&secretsRepositoryBackend{db: db}).ListRepositories(ctx, repoType)
	if err != nil {
		return nil, err
	}

	settingRepositories, err := (&settingRepositoryBackend{db: db}).ListRepositories(ctx, repoType)
	if err != nil {
		return nil, err
	}

	return append(secretRepositories, settingRepositories...), nil
}

// UpdateRepository updates a repository
func (db *db) UpdateRepository(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error) {
	secretsBackend := &secretsRepositoryBackend{db: db}
	exists, err := secretsBackend.RepositoryExists(ctx, r.Repo)
	if err != nil {
		return nil, err
	} else if exists {
		return secretsBackend.UpdateRepository(ctx, r)
	}

	settingsBackend := &settingRepositoryBackend{db: db}
	exists, err = settingsBackend.RepositoryExists(ctx, r.Repo)
	if err != nil {
		return nil, err
	} else if exists {
		return settingsBackend.UpdateRepository(ctx, r)
	}

	return nil, status.Errorf(codes.NotFound, "repo '%s' not found", r.Repo)
}

func (db *db) DeleteRepository(ctx context.Context, repoURL string) error {
	secretsBackend := &secretsRepositoryBackend{db: db}
	exists, err := secretsBackend.RepositoryExists(ctx, repoURL)
	if err != nil {
		return err
	} else if exists {
		return secretsBackend.DeleteRepository(ctx, repoURL)
	}

	settingsBackend := &settingRepositoryBackend{db: db}
	exists, err = settingsBackend.RepositoryExists(ctx, repoURL)
	if err != nil {
		return err
	} else if exists {
		return settingsBackend.DeleteRepository(ctx, repoURL)
	}

	return status.Errorf(codes.NotFound, "repo '%s' not found", repoURL)
}

// ListRepositoryCredentials returns a list of URLs that contain repo credential sets
func (db *db) ListRepositoryCredentials(ctx context.Context) ([]string, error) {
	// TODO It would be nice to check for duplicates between secret and setting repositories and make it so that
	// 	repositories from secrets overlay repositories from settings.

	secretRepoCreds, err := (&secretsRepositoryBackend{db: db}).ListRepoCreds(ctx)
	if err != nil {
		return nil, err
	}

	settingRepoCreds, err := (&settingRepositoryBackend{db: db}).ListRepoCreds(ctx)
	if err != nil {
		return nil, err
	}

	return append(secretRepoCreds, settingRepoCreds...), nil
}

// GetRepositoryCredentials retrieves a repository credential set
func (db *db) GetRepositoryCredentials(ctx context.Context, repoURL string) (*appsv1.RepoCreds, error) {
	secretsBackend := &secretsRepositoryBackend{db: db}
	exists, err := secretsBackend.RepoCredsExists(ctx, repoURL)
	if err != nil {
		return nil, err
	} else if exists {
		return secretsBackend.GetRepoCreds(ctx, repoURL)
	}

	settingsBackend := &settingRepositoryBackend{db: db}
	exists, err = settingsBackend.RepoCredsExists(ctx, repoURL)
	if err != nil {
		return nil, err
	} else if exists {
		return settingsBackend.GetRepoCreds(ctx, repoURL)
	}

	return nil, nil
}

// GetAllHelmRepositoryCredentials retrieves all repository credentials
func (db *db) GetAllHelmRepositoryCredentials(ctx context.Context) ([]*appsv1.RepoCreds, error) {
	// TODO It would be nice to check for duplicates between secret and setting repositories and make it so that
	// 	repositories from secrets overlay repositories from settings.

	secretRepoCreds, err := (&secretsRepositoryBackend{db: db}).GetAllHelmRepoCreds(ctx)
	if err != nil {
		return nil, err
	}

	settingRepoCreds, err := (&settingRepositoryBackend{db: db}).GetAllHelmRepoCreds(ctx)
	if err != nil {
		return nil, err
	}

	return append(secretRepoCreds, settingRepoCreds...), nil
}

// CreateRepositoryCredentials creates a repository credential set
func (db *db) CreateRepositoryCredentials(ctx context.Context, r *appsv1.RepoCreds) (*appsv1.RepoCreds, error) {
	settingBackend := &settingRepositoryBackend{db: db}
	secretBackend := &secretsRepositoryBackend{db: db}

	secretExists, err := secretBackend.RepositoryExists(ctx, r.URL)
	if err != nil {
		return nil, err
	}
	settingExists, err := settingBackend.RepositoryExists(ctx, r.URL)
	if err != nil {
		return nil, err
	}

	if secretExists || settingExists {
		return nil, status.Errorf(codes.AlreadyExists, "repository credentials %q already exists", r.URL)
	}

	return secretBackend.CreateRepoCreds(ctx, r)
}

// UpdateRepositoryCredentials updates a repository credential set
func (db *db) UpdateRepositoryCredentials(ctx context.Context, r *appsv1.RepoCreds) (*appsv1.RepoCreds, error) {
	secretsBackend := &secretsRepositoryBackend{db: db}
	exists, err := secretsBackend.RepoCredsExists(ctx, r.URL)
	if err != nil {
		return nil, err
	} else if exists {
		return secretsBackend.UpdateRepoCreds(ctx, r)
	}

	settingsBackend := &settingRepositoryBackend{db: db}
	exists, err = settingsBackend.RepoCredsExists(ctx, r.URL)
	if err != nil {
		return nil, err
	} else if exists {
		return settingsBackend.UpdateRepoCreds(ctx, r)
	}

	return nil, status.Errorf(codes.NotFound, "repository credentials '%s' not found", r.URL)
}

// DeleteRepositoryCredentials deletes a repository credential set from config, and
// also all the secrets which actually contained the credentials.
func (db *db) DeleteRepositoryCredentials(ctx context.Context, name string) error {
	secretsBackend := &secretsRepositoryBackend{db: db}
	exists, err := secretsBackend.RepoCredsExists(ctx, name)
	if err != nil {
		return err
	} else if exists {
		return secretsBackend.DeleteRepoCreds(ctx, name)
	}

	settingsBackend := &settingRepositoryBackend{db: db}
	exists, err = settingsBackend.RepoCredsExists(ctx, name)
	if err != nil {
		return err
	} else if exists {
		return settingsBackend.DeleteRepoCreds(ctx, name)
	}

	return status.Errorf(codes.NotFound, "repository credentials '%s' not found", name)
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
