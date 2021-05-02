package db

import (
	"fmt"
	"hash/fnv"
	"strings"

	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/git"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"golang.org/x/net/context"
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
	return (&settingRepositoryBackend{}).CreateRepository(ctx, r)
}

func (db *db) GetRepository(ctx context.Context, repoURL string) (*appsv1.Repository, error) {
	return (&settingRepositoryBackend{}).GetRepository(ctx, repoURL)
}

func (db *db) ListRepositories(ctx context.Context) ([]*appsv1.Repository, error) {
	return (&settingRepositoryBackend{}).ListRepositories(ctx, nil)
}

func (db *db) UpdateRepository(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error) {
	return (&settingRepositoryBackend{}).UpdateRepository(ctx, r)
}

func (db *db) DeleteRepository(ctx context.Context, repoURL string) error {
	return (&settingRepositoryBackend{}).DeleteRepository(ctx, repoURL)
}

// ListRepositoryCredentials returns a list of URLs that contain repo credential sets
func (db *db) ListRepositoryCredentials(ctx context.Context) ([]string, error) {
	return (&settingRepositoryBackend{}).ListRepoCreds(ctx)
}

// GetRepositoryCredentials retrieves a repository credential set
func (db *db) GetRepositoryCredentials(ctx context.Context, repoURL string) (*appsv1.RepoCreds, error) {
	return (&settingRepositoryBackend{}).GetRepoCreds(ctx, repoURL)
}

// GetAllHelmRepositoryCredentials retrieves all repository credentials
func (db *db) GetAllHelmRepositoryCredentials(ctx context.Context) ([]*appsv1.RepoCreds, error) {
	return (&settingRepositoryBackend{}).GetAllHelmRepoCreds(ctx)
}

// CreateRepositoryCredentials creates a repository credential set
func (db *db) CreateRepositoryCredentials(ctx context.Context, r *appsv1.RepoCreds) (*appsv1.RepoCreds, error) {
	return (&settingRepositoryBackend{}).CreateRepoCreds(ctx, r)
}

// UpdateRepositoryCredentials updates a repository credential set
func (db *db) UpdateRepositoryCredentials(ctx context.Context, r *appsv1.RepoCreds) (*appsv1.RepoCreds, error) {
	return (&settingRepositoryBackend{}).UpdateRepoCreds(ctx, r)
}

// DeleteRepositoryCredentials deletes a repository credential set from config, and
// also all the secrets which actually contained the credentials.
func (db *db) DeleteRepositoryCredentials(ctx context.Context, name string) error {
	return (&settingRepositoryBackend{}).DeleteRepoCreds(ctx, name)
}

// getRepositoryCredentialIndex returns the index of the best matching repository credential
// configuration, i.e. the one with the longest match
func getRepositoryCredentialIndex(repoCredentials []settings.RepositoryCredentials, repoURL string) int {
	var max, idx int = 0, -1
	repoURL = git.NormalizeGitURL(repoURL)
	for i, cred := range repoCredentials {
		credUrl := git.NormalizeGitURL(cred.URL)
		if strings.HasPrefix(repoURL, credUrl) {
			if len(credUrl) > max {
				max = len(credUrl)
				idx = i
			}
		}
	}
	return idx
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
