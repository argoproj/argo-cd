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

	CreateRepoCreds(ctx context.Context, r *appsv1.RepoCreds) (*appsv1.RepoCreds, error)
	GetRepoCreds(ctx context.Context, repoURL string) (*appsv1.RepoCreds, error)
	ListRepoCreds(ctx context.Context) ([]string, error)
	UpdateRepoCreds(ctx context.Context, r *appsv1.RepoCreds) (*appsv1.RepoCreds, error)
	DeleteRepoCreds(ctx context.Context, name string) error

	GetAllHelmRepoCreds(ctx context.Context) ([]*appsv1.RepoCreds, error)
}

func (db *db) CreateRepository(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error) {
	return (&secretsRepositoryBackend{db: db}).CreateRepository(ctx, r)
}

func (db *db) GetRepository(ctx context.Context, repoURL string) (*appsv1.Repository, error) {
	repo, err := (&secretsRepositoryBackend{db: db}).GetRepository(ctx, repoURL)
	if status.Code(err) == codes.NotFound {
		return (&settingRepositoryBackend{db: db}).GetRepository(ctx, repoURL)
	}

	return repo, err
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
	repo, err := (&secretsRepositoryBackend{db: db}).UpdateRepository(ctx, r)
	if status.Code(err) == codes.NotFound {
		return (&settingRepositoryBackend{db: db}).UpdateRepository(ctx, r)
	}

	return repo, err
}

func (db *db) DeleteRepository(ctx context.Context, repoURL string) error {
	err := (&secretsRepositoryBackend{db: db}).DeleteRepository(ctx, repoURL)
	if status.Code(err) == codes.NotFound {
		return (&settingRepositoryBackend{db: db}).DeleteRepository(ctx, repoURL)
	}

	return err
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

	return append(secretRepoCreds, settingRepoCreds...), err
}

// GetRepositoryCredentials retrieves a repository credential set
func (db *db) GetRepositoryCredentials(ctx context.Context, repoURL string) (*appsv1.RepoCreds, error) {
	repoCreds, err := (&secretsRepositoryBackend{db: db}).GetRepoCreds(ctx, repoURL)
	if status.Code(err) == codes.NotFound {
		return (&settingRepositoryBackend{db: db}).GetRepoCreds(ctx, repoURL)
	}

	return repoCreds, err
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
	return (&secretsRepositoryBackend{db: db}).CreateRepoCreds(ctx, r)
}

// UpdateRepositoryCredentials updates a repository credential set
func (db *db) UpdateRepositoryCredentials(ctx context.Context, r *appsv1.RepoCreds) (*appsv1.RepoCreds, error) {
	return (&settingRepositoryBackend{db: db}).UpdateRepoCreds(ctx, r)
}

// DeleteRepositoryCredentials deletes a repository credential set from config, and
// also all the secrets which actually contained the credentials.
func (db *db) DeleteRepositoryCredentials(ctx context.Context, name string) error {
	return (&settingRepositoryBackend{db: db}).DeleteRepoCreds(ctx, name)
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
