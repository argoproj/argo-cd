package db

import (
	"fmt"
	"hash/fnv"
	"strings"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/common"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/errors"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/settings"
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

func (db *db) CreateRepository(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error) {
	repos, err := db.settingsMgr.GetRepositories()
	if err != nil {
		return nil, err
	}

	index := getRepositoryIndex(repos, r.Repo)
	if index > -1 {
		return nil, status.Errorf(codes.AlreadyExists, "repository '%s' already exists", r.Repo)
	}

	data := make(map[string][]byte)
	if r.Username != "" {
		data[username] = []byte(r.Username)
	}
	if r.Password != "" {
		data[password] = []byte(r.Password)
	}
	if r.SSHPrivateKey != "" {
		data[sshPrivateKey] = []byte(r.SSHPrivateKey)
	}
	if r.GithubAppPrivateKey != "" {
		data[githubAppPrivateKey] = []byte(r.GithubAppPrivateKey)
	}

	repoInfo := settings.Repository{
		URL:                        r.Repo,
		Type:                       r.Type,
		Name:                       r.Name,
		InsecureIgnoreHostKey:      r.IsInsecure(),
		Insecure:                   r.IsInsecure(),
		EnableLFS:                  r.EnableLFS,
		EnableOci:                  r.EnableOCI,
		GithubAppId:                r.GithubAppId,
		GithubAppInstallationId:    r.GithubAppInstallationId,
		GithubAppEnterpriseBaseURL: r.GitHubAppEnterpriseBaseURL,
	}
	err = db.updateRepositorySecrets(&repoInfo, r)
	if err != nil {
		return nil, err
	}

	repos = append(repos, repoInfo)
	err = db.settingsMgr.SaveRepositories(repos)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// GetRepository returns a repository by URL. If the repository doesn't have any
// credentials attached to it, checks if a credential set for the repo's URL is
// configured and copies them to the returned repository data.
func (db *db) GetRepository(ctx context.Context, repoURL string) (*appsv1.Repository, error) {
	repository, err := db.tryGetRepository(ctx, repoURL)
	if err != nil {
		return nil, err
	}
	return repository, nil
}

// tryGetRepository returns a repository by URL.
// It provides the same functionality as GetRepository, with the additional behaviour of still returning a repository,
// even if an error occurred during the resolving of credentials for the repository. Otherwise this function behaves
// just as one would expect.
func (db *db) tryGetRepository(ctx context.Context, repoURL string) (*appsv1.Repository, error) {
	repos, err := db.settingsMgr.GetRepositories()
	if err != nil {
		return nil, err
	}

	repo := &appsv1.Repository{Repo: repoURL}
	index := getRepositoryIndex(repos, repoURL)
	if index >= 0 {
		repo, err = db.credentialsToRepository(repos[index])
		if err != nil {
			return repo, errors.NewCredentialsConfigurationError(err)
		}
	}

	// Check for and copy repository credentials, if repo has none configured.
	if !repo.HasCredentials() {
		creds, err := db.GetRepositoryCredentials(ctx, repoURL)
		if err == nil {
			if creds != nil {
				repo.CopyCredentialsFrom(creds)
				repo.InheritedCreds = true
			}
		} else {
			return repo, err
		}
	} else {
		log.Debugf("%s has credentials", repo.Repo)
	}

	return repo, err
}

func (db *db) ListRepositories(ctx context.Context) ([]*appsv1.Repository, error) {
	return db.listRepositories(ctx, nil)
}

func (db *db) listRepositories(ctx context.Context, repoType *string) ([]*appsv1.Repository, error) {
	inRepos, err := db.settingsMgr.GetRepositories()
	if err != nil {
		return nil, err
	}

	var repos []*appsv1.Repository
	for _, inRepo := range inRepos {
		if repoType == nil || *repoType == inRepo.Type {
			r, err := db.tryGetRepository(ctx, inRepo.URL)
			if err != nil {
				if r != nil && errors.IsCredentialsConfigurationError(err) {
					modifiedTime := metav1.Now()
					r.ConnectionState = appsv1.ConnectionState{
						Status:     appsv1.ConnectionStatusFailed,
						Message:    "Configuration error - please check the server logs",
						ModifiedAt: &modifiedTime,
					}

					log.Warnf("could not retrieve repo: %s", err.Error())
				} else {
					return nil, err
				}
			}
			repos = append(repos, r)
		}
	}
	return repos, nil
}

func (db *db) credentialsToRepository(repoInfo settings.Repository) (*appsv1.Repository, error) {
	repo := &appsv1.Repository{
		Repo:                       repoInfo.URL,
		Type:                       repoInfo.Type,
		Name:                       repoInfo.Name,
		InsecureIgnoreHostKey:      repoInfo.InsecureIgnoreHostKey,
		Insecure:                   repoInfo.Insecure,
		EnableLFS:                  repoInfo.EnableLFS,
		EnableOCI:                  repoInfo.EnableOci,
		GithubAppId:                repoInfo.GithubAppId,
		GithubAppInstallationId:    repoInfo.GithubAppInstallationId,
		GitHubAppEnterpriseBaseURL: repoInfo.GithubAppEnterpriseBaseURL,
	}
	err := db.unmarshalFromSecretsStr(map[*string]*apiv1.SecretKeySelector{
		&repo.Username:            repoInfo.UsernameSecret,
		&repo.Password:            repoInfo.PasswordSecret,
		&repo.SSHPrivateKey:       repoInfo.SSHPrivateKeySecret,
		&repo.TLSClientCertData:   repoInfo.TLSClientCertDataSecret,
		&repo.TLSClientCertKey:    repoInfo.TLSClientCertKeySecret,
		&repo.GithubAppPrivateKey: repoInfo.GithubAppPrivateKeySecret,
	}, make(map[string]*apiv1.Secret))
	return repo, err
}

func (db *db) credentialsToRepositoryCredentials(repoInfo settings.RepositoryCredentials) (*appsv1.RepoCreds, error) {
	creds := &appsv1.RepoCreds{
		URL:                        repoInfo.URL,
		GithubAppId:                repoInfo.GithubAppId,
		GithubAppInstallationId:    repoInfo.GithubAppInstallationId,
		GitHubAppEnterpriseBaseURL: repoInfo.GithubAppEnterpriseBaseURL,
	}
	err := db.unmarshalFromSecretsStr(map[*string]*apiv1.SecretKeySelector{
		&creds.Username:            repoInfo.UsernameSecret,
		&creds.Password:            repoInfo.PasswordSecret,
		&creds.SSHPrivateKey:       repoInfo.SSHPrivateKeySecret,
		&creds.TLSClientCertData:   repoInfo.TLSClientCertDataSecret,
		&creds.TLSClientCertKey:    repoInfo.TLSClientCertKeySecret,
		&creds.GithubAppPrivateKey: repoInfo.GithubAppPrivateKeySecret,
	}, make(map[string]*apiv1.Secret))
	return creds, err
}

// UpdateRepository updates a repository
func (db *db) UpdateRepository(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error) {
	repos, err := db.settingsMgr.GetRepositories()
	if err != nil {
		return nil, err
	}

	index := getRepositoryIndex(repos, r.Repo)
	if index < 0 {
		return nil, status.Errorf(codes.NotFound, "repo '%s' not found", r.Repo)
	}

	repoInfo := repos[index]
	err = db.updateRepositorySecrets(&repoInfo, r)
	if err != nil {
		return nil, err
	}

	// Update boolean settings
	repoInfo.InsecureIgnoreHostKey = r.IsInsecure()
	repoInfo.Insecure = r.IsInsecure()
	repoInfo.EnableLFS = r.EnableLFS

	repos[index] = repoInfo
	err = db.settingsMgr.SaveRepositories(repos)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// Delete updates a repository
func (db *db) DeleteRepository(ctx context.Context, repoURL string) error {
	repos, err := db.settingsMgr.GetRepositories()
	if err != nil {
		return err
	}

	index := getRepositoryIndex(repos, repoURL)
	if index < 0 {
		return status.Errorf(codes.NotFound, "repo '%s' not found", repoURL)
	}
	err = db.updateRepositorySecrets(&repos[index], &appsv1.Repository{
		SSHPrivateKey:       "",
		Password:            "",
		Username:            "",
		TLSClientCertData:   "",
		TLSClientCertKey:    "",
		GithubAppPrivateKey: "",
	})
	if err != nil {
		return err
	}
	repos = append(repos[:index], repos[index+1:]...)
	return db.settingsMgr.SaveRepositories(repos)
}

// ListRepositoryCredentials returns a list of URLs that contain repo credential sets
func (db *db) ListRepositoryCredentials(ctx context.Context) ([]string, error) {
	repos, err := db.settingsMgr.GetRepositoryCredentials()
	if err != nil {
		return nil, err
	}

	urls := make([]string, len(repos))
	for i := range repos {
		urls[i] = repos[i].URL
	}

	return urls, nil
}

// GetRepositoryCredentials retrieves a repository credential set
func (db *db) GetRepositoryCredentials(ctx context.Context, repoURL string) (*appsv1.RepoCreds, error) {
	var credential *appsv1.RepoCreds

	repoCredentials, err := db.settingsMgr.GetRepositoryCredentials()
	if err != nil {
		return nil, err
	}
	index := getRepositoryCredentialIndex(repoCredentials, repoURL)
	if index >= 0 {
		credential, err = db.credentialsToRepositoryCredentials(repoCredentials[index])
		if err != nil {
			return nil, err
		}
	}

	return credential, err
}

// CreateRepositoryCredentials creates a repository credential set
func (db *db) CreateRepositoryCredentials(ctx context.Context, r *appsv1.RepoCreds) (*appsv1.RepoCreds, error) {
	creds, err := db.settingsMgr.GetRepositoryCredentials()
	if err != nil {
		return nil, err
	}

	index := getRepositoryCredentialIndex(creds, r.URL)
	if index > -1 {
		return nil, status.Errorf(codes.AlreadyExists, "repository credentials for '%s' already exists", r.URL)
	}

	repoInfo := settings.RepositoryCredentials{
		URL:                        r.URL,
		GithubAppId:                r.GithubAppId,
		GithubAppInstallationId:    r.GithubAppInstallationId,
		GithubAppEnterpriseBaseURL: r.GitHubAppEnterpriseBaseURL,
	}

	err = db.updateCredentialsSecret(&repoInfo, r)
	if err != nil {
		return nil, err
	}

	creds = append(creds, repoInfo)
	err = db.settingsMgr.SaveRepositoryCredentials(creds)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// UpdateRepositoryCredentials updates a repository credential set
func (db *db) UpdateRepositoryCredentials(ctx context.Context, r *appsv1.RepoCreds) (*appsv1.RepoCreds, error) {
	repos, err := db.settingsMgr.GetRepositoryCredentials()
	if err != nil {
		return nil, err
	}

	index := getRepositoryCredentialIndex(repos, r.URL)
	if index < 0 {
		return nil, status.Errorf(codes.NotFound, "repository credentials '%s' not found", r.URL)
	}

	repoInfo := repos[index]
	err = db.updateCredentialsSecret(&repoInfo, r)
	if err != nil {
		return nil, err
	}

	repos[index] = repoInfo
	err = db.settingsMgr.SaveRepositoryCredentials(repos)
	if err != nil {
		return nil, err
	}
	return r, nil

}

// DeleteRepositoryCredentials deletes a repository credential set from config, and
// also all the secrets which actually contained the credentials.
func (db *db) DeleteRepositoryCredentials(ctx context.Context, name string) error {
	repos, err := db.settingsMgr.GetRepositoryCredentials()
	if err != nil {
		return err
	}

	index := getRepositoryCredentialIndex(repos, name)
	if index < 0 {
		return status.Errorf(codes.NotFound, "repository credentials '%s' not found", name)
	}
	err = db.updateCredentialsSecret(&repos[index], &appsv1.RepoCreds{
		SSHPrivateKey:       "",
		Password:            "",
		Username:            "",
		TLSClientCertData:   "",
		TLSClientCertKey:    "",
		GithubAppPrivateKey: "",
	})
	if err != nil {
		return err
	}
	repos = append(repos[:index], repos[index+1:]...)
	return db.settingsMgr.SaveRepositoryCredentials(repos)
}

func (db *db) updateCredentialsSecret(credsInfo *settings.RepositoryCredentials, c *appsv1.RepoCreds) error {
	r := &appsv1.Repository{
		Repo:                       c.URL,
		Username:                   c.Username,
		Password:                   c.Password,
		SSHPrivateKey:              c.SSHPrivateKey,
		TLSClientCertData:          c.TLSClientCertData,
		TLSClientCertKey:           c.TLSClientCertKey,
		GithubAppPrivateKey:        c.GithubAppPrivateKey,
		GithubAppId:                c.GithubAppId,
		GithubAppInstallationId:    c.GithubAppInstallationId,
		GitHubAppEnterpriseBaseURL: c.GitHubAppEnterpriseBaseURL,
	}
	secretsData := make(map[string]map[string][]byte)

	credsInfo.UsernameSecret = setSecretData(credSecretPrefix, r.Repo, secretsData, credsInfo.UsernameSecret, r.Username, username)
	credsInfo.PasswordSecret = setSecretData(credSecretPrefix, r.Repo, secretsData, credsInfo.PasswordSecret, r.Password, password)
	credsInfo.SSHPrivateKeySecret = setSecretData(credSecretPrefix, r.Repo, secretsData, credsInfo.SSHPrivateKeySecret, r.SSHPrivateKey, sshPrivateKey)
	credsInfo.TLSClientCertDataSecret = setSecretData(credSecretPrefix, r.Repo, secretsData, credsInfo.TLSClientCertDataSecret, r.TLSClientCertData, tlsClientCertData)
	credsInfo.TLSClientCertKeySecret = setSecretData(credSecretPrefix, r.Repo, secretsData, credsInfo.TLSClientCertKeySecret, r.TLSClientCertKey, tlsClientCertKey)
	credsInfo.GithubAppPrivateKeySecret = setSecretData(repoSecretPrefix, r.Repo, secretsData, credsInfo.GithubAppPrivateKeySecret, r.GithubAppPrivateKey, githubAppPrivateKey)
	for k, v := range secretsData {
		err := db.upsertSecret(k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *db) updateRepositorySecrets(repoInfo *settings.Repository, r *appsv1.Repository) error {
	secretsData := make(map[string]map[string][]byte)

	repoInfo.UsernameSecret = setSecretData(repoSecretPrefix, r.Repo, secretsData, repoInfo.UsernameSecret, r.Username, username)
	repoInfo.PasswordSecret = setSecretData(repoSecretPrefix, r.Repo, secretsData, repoInfo.PasswordSecret, r.Password, password)
	repoInfo.SSHPrivateKeySecret = setSecretData(repoSecretPrefix, r.Repo, secretsData, repoInfo.SSHPrivateKeySecret, r.SSHPrivateKey, sshPrivateKey)
	repoInfo.TLSClientCertDataSecret = setSecretData(repoSecretPrefix, r.Repo, secretsData, repoInfo.TLSClientCertDataSecret, r.TLSClientCertData, tlsClientCertData)
	repoInfo.TLSClientCertKeySecret = setSecretData(repoSecretPrefix, r.Repo, secretsData, repoInfo.TLSClientCertKeySecret, r.TLSClientCertKey, tlsClientCertKey)
	repoInfo.GithubAppPrivateKeySecret = setSecretData(repoSecretPrefix, r.Repo, secretsData, repoInfo.GithubAppPrivateKeySecret, r.GithubAppPrivateKey, githubAppPrivateKey)
	for k, v := range secretsData {
		err := db.upsertSecret(k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

// Set data to be stored in a given secret used for repository credentials and templates.
// The name of the secret is a combination of the prefix given, and a calculated value
// from the repository or template URL.
func setSecretData(prefix string, url string, secretsData map[string]map[string][]byte, secretKey *apiv1.SecretKeySelector, value string, defaultKeyName string) *apiv1.SecretKeySelector {
	if secretKey == nil && value != "" {
		secretKey = &apiv1.SecretKeySelector{
			LocalObjectReference: apiv1.LocalObjectReference{Name: RepoURLToSecretName(prefix, url)},
			Key:                  defaultKeyName,
		}
	}

	if secretKey != nil {
		data, ok := secretsData[secretKey.Name]
		if !ok {
			data = map[string][]byte{}
		}
		if value != "" {
			data[secretKey.Key] = []byte(value)
		}
		secretsData[secretKey.Name] = data
	}

	if value == "" {
		secretKey = nil
	}

	return secretKey
}

func (db *db) upsertSecret(name string, data map[string][]byte) error {
	secret, err := db.kubeclientset.CoreV1().Secrets(db.ns).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if apierr.IsNotFound(err) {
			if len(data) == 0 {
				return nil
			}
			_, err = db.kubeclientset.CoreV1().Secrets(db.ns).Create(context.Background(), &apiv1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Annotations: map[string]string{
						common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
					},
				},
				Data: data,
			}, metav1.CreateOptions{})
			if err != nil {
				return err
			}
		}
	} else {
		for _, key := range []string{username, password, sshPrivateKey, tlsClientCertData, tlsClientCertKey, githubAppPrivateKey} {
			if secret.Data == nil {
				secret.Data = make(map[string][]byte)
			}
			if val, ok := data[key]; ok && len(val) > 0 {
				secret.Data[key] = val
			} else {
				delete(secret.Data, key)
			}
		}
		if len(secret.Data) == 0 {
			isManagedByArgo := (secret.Annotations != nil && secret.Annotations[common.AnnotationKeyManagedBy] == common.AnnotationValueManagedByArgoCD) ||
				(secret.Labels != nil && secret.Labels[common.LabelKeySecretType] == "repository")
			if isManagedByArgo {
				return db.kubeclientset.CoreV1().Secrets(db.ns).Delete(context.Background(), name, metav1.DeleteOptions{})
			}
			return nil
		} else {
			_, err = db.kubeclientset.CoreV1().Secrets(db.ns).Update(context.Background(), secret, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func getRepositoryIndex(repos []settings.Repository, repoURL string) int {
	for i, repo := range repos {
		if git.SameURL(repo.URL, repoURL) {
			return i
		}
	}
	return -1
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
