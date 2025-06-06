package db

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/common"
	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/git"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

var _ repositoryBackend = &legacyRepositoryBackend{}

// legacyRepositoryBackend is a repository backend strategy that maintains backward compatibility with previous versions.
// This can be removed in a future version, once the old "argocd-cm" storage for repositories is removed.
type legacyRepositoryBackend struct {
	db *db
}

func (l *legacyRepositoryBackend) CreateRepository(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error) {
	// This strategy only kept to preserve backward compatibility, but is deprecated.
	// Therefore, no new repositories can be added with this backend.
	panic("creating new repositories is not supported for the legacy repository backend")
}

func (l *legacyRepositoryBackend) GetRepository(ctx context.Context, repoURL, project string) (*appsv1.Repository, error) {
	repository, err := l.tryGetRepository(repoURL)
	if err != nil {
		return nil, fmt.Errorf("unable to get repository: %w", err)
	}
	return repository, nil
}

func (l *legacyRepositoryBackend) ListRepositories(ctx context.Context, repoType *string) ([]*appsv1.Repository, error) {
	inRepos, err := l.db.settingsMgr.GetRepositories()
	if err != nil {
		return nil, err
	}

	var repos []*appsv1.Repository
	for _, inRepo := range inRepos {
		if repoType == nil || *repoType == inRepo.Type {
			r, err := l.tryGetRepository(inRepo.URL)
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

func (l *legacyRepositoryBackend) UpdateRepository(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error) {
	repos, err := l.db.settingsMgr.GetRepositories()
	if err != nil {
		return nil, err
	}

	index := l.getRepositoryIndex(repos, r.Repo)
	if index < 0 {
		return nil, status.Errorf(codes.NotFound, "repo '%s' not found", r.Repo)
	}

	repoInfo := repos[index]
	err = l.updateRepositorySecrets(&repoInfo, r)
	if err != nil {
		return nil, err
	}

	// Update boolean settings
	repoInfo.InsecureIgnoreHostKey = r.IsInsecure()
	repoInfo.Insecure = r.IsInsecure()
	repoInfo.EnableLFS = r.EnableLFS
	repoInfo.Proxy = r.Proxy

	repos[index] = repoInfo
	err = l.db.settingsMgr.SaveRepositories(repos)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (l *legacyRepositoryBackend) DeleteRepository(ctx context.Context, repoURL, project string) error {
	repos, err := l.db.settingsMgr.GetRepositories()
	if err != nil {
		return err
	}

	index := l.getRepositoryIndex(repos, repoURL)
	if index < 0 {
		return status.Errorf(codes.NotFound, "repo '%s' not found", repoURL)
	}
	err = l.updateRepositorySecrets(&repos[index], &appsv1.Repository{
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
	return l.db.settingsMgr.SaveRepositories(repos)
}

func (l *legacyRepositoryBackend) RepositoryExists(ctx context.Context, repoURL, project string, allowFallback bool) (bool, error) {
	repos, err := l.db.settingsMgr.GetRepositories()
	if err != nil {
		return false, fmt.Errorf("unable to get repositories: %w", err)
	}

	index := l.getRepositoryIndex(repos, repoURL)
	return index >= 0, nil
}

func (l *legacyRepositoryBackend) CreateRepoCreds(ctx context.Context, r *appsv1.RepoCreds) (*appsv1.RepoCreds, error) {
	// This strategy only kept to preserve backward compatibility, but is deprecated.
	// Therefore, no new repositories can be added with this backend.
	panic("creating new repository credentials is not supported for the legacy repository backend")
}

func (l *legacyRepositoryBackend) GetRepoCreds(ctx context.Context, repoURL string) (*appsv1.RepoCreds, error) {
	var credential *appsv1.RepoCreds

	repoCredentials, err := l.db.settingsMgr.GetRepositoryCredentials()
	if err != nil {
		return nil, err
	}
	index := getRepositoryCredentialIndex(repoCredentials, repoURL)
	if index >= 0 {
		credential, err = l.credentialsToRepositoryCredentials(repoCredentials[index])
		if err != nil {
			return nil, err
		}
	}

	return credential, err
}

func (l *legacyRepositoryBackend) ListRepoCreds(ctx context.Context) ([]string, error) {
	repos, err := l.db.settingsMgr.GetRepositoryCredentials()
	if err != nil {
		return nil, err
	}

	urls := make([]string, len(repos))
	for i := range repos {
		urls[i] = repos[i].URL
	}

	return urls, nil
}

func (l *legacyRepositoryBackend) UpdateRepoCreds(ctx context.Context, r *appsv1.RepoCreds) (*appsv1.RepoCreds, error) {
	repos, err := l.db.settingsMgr.GetRepositoryCredentials()
	if err != nil {
		return nil, err
	}

	index := getRepositoryCredentialIndex(repos, r.URL)
	if index < 0 {
		return nil, status.Errorf(codes.NotFound, "repository credentials '%s' not found", r.URL)
	}

	repoInfo := repos[index]
	err = l.updateCredentialsSecret(&repoInfo, r)
	if err != nil {
		return nil, err
	}

	repos[index] = repoInfo
	err = l.db.settingsMgr.SaveRepositoryCredentials(repos)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (l *legacyRepositoryBackend) DeleteRepoCreds(ctx context.Context, name string) error {
	repos, err := l.db.settingsMgr.GetRepositoryCredentials()
	if err != nil {
		return err
	}

	index := getRepositoryCredentialIndex(repos, name)
	if index < 0 {
		return status.Errorf(codes.NotFound, "repository credentials '%s' not found", name)
	}
	err = l.updateCredentialsSecret(&repos[index], &appsv1.RepoCreds{
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
	return l.db.settingsMgr.SaveRepositoryCredentials(repos)
}

func (l *legacyRepositoryBackend) RepoCredsExists(ctx context.Context, repoURL string) (bool, error) {
	creds, err := l.db.settingsMgr.GetRepositoryCredentials()
	if err != nil {
		return false, err
	}

	index := getRepositoryCredentialIndex(creds, repoURL)
	return index >= 0, nil
}

func (l *legacyRepositoryBackend) GetAllHelmRepoCreds(ctx context.Context) ([]*appsv1.RepoCreds, error) {
	var allCredentials []*appsv1.RepoCreds
	repoCredentials, err := l.db.settingsMgr.GetRepositoryCredentials()
	if err != nil {
		return nil, err
	}
	for _, v := range repoCredentials {
		if strings.EqualFold(v.Type, "helm") {
			credential, err := l.credentialsToRepositoryCredentials(v)
			if err != nil {
				return nil, err
			}
			allCredentials = append(allCredentials, credential)
		}
	}
	return allCredentials, err
}

func (l *legacyRepositoryBackend) updateRepositorySecrets(repoInfo *settings.Repository, r *appsv1.Repository) error {
	secretsData := make(map[string]map[string][]byte)

	repoInfo.UsernameSecret = l.setSecretData(repoSecretPrefix, r.Repo, secretsData, repoInfo.UsernameSecret, r.Username, username)
	repoInfo.PasswordSecret = l.setSecretData(repoSecretPrefix, r.Repo, secretsData, repoInfo.PasswordSecret, r.Password, password)
	repoInfo.SSHPrivateKeySecret = l.setSecretData(repoSecretPrefix, r.Repo, secretsData, repoInfo.SSHPrivateKeySecret, r.SSHPrivateKey, sshPrivateKey)
	repoInfo.TLSClientCertDataSecret = l.setSecretData(repoSecretPrefix, r.Repo, secretsData, repoInfo.TLSClientCertDataSecret, r.TLSClientCertData, tlsClientCertData)
	repoInfo.TLSClientCertKeySecret = l.setSecretData(repoSecretPrefix, r.Repo, secretsData, repoInfo.TLSClientCertKeySecret, r.TLSClientCertKey, tlsClientCertKey)
	repoInfo.GithubAppPrivateKeySecret = l.setSecretData(repoSecretPrefix, r.Repo, secretsData, repoInfo.GithubAppPrivateKeySecret, r.GithubAppPrivateKey, githubAppPrivateKey)
	repoInfo.GCPServiceAccountKey = l.setSecretData(repoSecretPrefix, r.Repo, secretsData, repoInfo.GCPServiceAccountKey, r.GCPServiceAccountKey, gcpServiceAccountKey)
	for k, v := range secretsData {
		err := l.upsertSecret(k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (l *legacyRepositoryBackend) updateCredentialsSecret(credsInfo *settings.RepositoryCredentials, c *appsv1.RepoCreds) error {
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
		GCPServiceAccountKey:       c.GCPServiceAccountKey,
	}
	secretsData := make(map[string]map[string][]byte)

	credsInfo.UsernameSecret = l.setSecretData(credSecretPrefix, r.Repo, secretsData, credsInfo.UsernameSecret, r.Username, username)
	credsInfo.PasswordSecret = l.setSecretData(credSecretPrefix, r.Repo, secretsData, credsInfo.PasswordSecret, r.Password, password)
	credsInfo.SSHPrivateKeySecret = l.setSecretData(credSecretPrefix, r.Repo, secretsData, credsInfo.SSHPrivateKeySecret, r.SSHPrivateKey, sshPrivateKey)
	credsInfo.TLSClientCertDataSecret = l.setSecretData(credSecretPrefix, r.Repo, secretsData, credsInfo.TLSClientCertDataSecret, r.TLSClientCertData, tlsClientCertData)
	credsInfo.TLSClientCertKeySecret = l.setSecretData(credSecretPrefix, r.Repo, secretsData, credsInfo.TLSClientCertKeySecret, r.TLSClientCertKey, tlsClientCertKey)
	credsInfo.GithubAppPrivateKeySecret = l.setSecretData(repoSecretPrefix, r.Repo, secretsData, credsInfo.GithubAppPrivateKeySecret, r.GithubAppPrivateKey, githubAppPrivateKey)
	credsInfo.GCPServiceAccountKey = l.setSecretData(repoSecretPrefix, r.Repo, secretsData, credsInfo.GCPServiceAccountKey, r.GCPServiceAccountKey, gcpServiceAccountKey)
	for k, v := range secretsData {
		err := l.upsertSecret(k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (l *legacyRepositoryBackend) upsertSecret(name string, data map[string][]byte) error {
	secret, err := l.db.kubeclientset.CoreV1().Secrets(l.db.ns).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if apierr.IsNotFound(err) {
			if len(data) == 0 {
				return nil
			}
			_, err = l.db.kubeclientset.CoreV1().Secrets(l.db.ns).Create(context.Background(), &apiv1.Secret{
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
			isManagedByArgo := secret.Annotations != nil && secret.Annotations[common.AnnotationKeyManagedBy] == common.AnnotationValueManagedByArgoCD
			if isManagedByArgo {
				return l.db.kubeclientset.CoreV1().Secrets(l.db.ns).Delete(context.Background(), name, metav1.DeleteOptions{})
			}
			return nil
		} else {
			_, err = l.db.kubeclientset.CoreV1().Secrets(l.db.ns).Update(context.Background(), secret, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// tryGetRepository returns a repository by URL.
// It provides the same functionality as GetRepository, with the additional behaviour of still returning a repository,
// even if an error occurred during the resolving of credentials for the repository. Otherwise this function behaves
// just as one would expect.
func (l *legacyRepositoryBackend) tryGetRepository(repoURL string) (*appsv1.Repository, error) {
	repos, err := l.db.settingsMgr.GetRepositories()
	if err != nil {
		return nil, err
	}

	repo := &appsv1.Repository{Repo: repoURL}
	index := l.getRepositoryIndex(repos, repoURL)
	if index >= 0 {
		repo, err = l.credentialsToRepository(repos[index])
		if err != nil {
			return repo, errors.NewCredentialsConfigurationError(err)
		}
	}

	return repo, err
}

func (l *legacyRepositoryBackend) credentialsToRepository(repoInfo settings.Repository) (*appsv1.Repository, error) {
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
		Proxy:                      repoInfo.Proxy,
	}
	err := l.db.unmarshalFromSecretsStr(map[*SecretMaperValidation]*apiv1.SecretKeySelector{
		{Dest: &repo.Username, Transform: StripCRLFCharacter}:             repoInfo.UsernameSecret,
		{Dest: &repo.Password, Transform: StripCRLFCharacter}:             repoInfo.PasswordSecret,
		{Dest: &repo.SSHPrivateKey, Transform: StripCRLFCharacter}:        repoInfo.SSHPrivateKeySecret,
		{Dest: &repo.TLSClientCertData, Transform: StripCRLFCharacter}:    repoInfo.TLSClientCertDataSecret,
		{Dest: &repo.TLSClientCertKey, Transform: StripCRLFCharacter}:     repoInfo.TLSClientCertKeySecret,
		{Dest: &repo.GithubAppPrivateKey, Transform: StripCRLFCharacter}:  repoInfo.GithubAppPrivateKeySecret,
		{Dest: &repo.GCPServiceAccountKey, Transform: StripCRLFCharacter}: repoInfo.GCPServiceAccountKey,
	}, make(map[string]*apiv1.Secret))
	return repo, err
}

func (l *legacyRepositoryBackend) credentialsToRepositoryCredentials(repoInfo settings.RepositoryCredentials) (*appsv1.RepoCreds, error) {
	creds := &appsv1.RepoCreds{
		URL:                        repoInfo.URL,
		GithubAppId:                repoInfo.GithubAppId,
		GithubAppInstallationId:    repoInfo.GithubAppInstallationId,
		GitHubAppEnterpriseBaseURL: repoInfo.GithubAppEnterpriseBaseURL,
		EnableOCI:                  repoInfo.EnableOCI,
	}
	err := l.db.unmarshalFromSecretsStr(map[*SecretMaperValidation]*apiv1.SecretKeySelector{
		{Dest: &creds.Username}:             repoInfo.UsernameSecret,
		{Dest: &creds.Password}:             repoInfo.PasswordSecret,
		{Dest: &creds.SSHPrivateKey}:        repoInfo.SSHPrivateKeySecret,
		{Dest: &creds.TLSClientCertData}:    repoInfo.TLSClientCertDataSecret,
		{Dest: &creds.TLSClientCertKey}:     repoInfo.TLSClientCertKeySecret,
		{Dest: &creds.GithubAppPrivateKey}:  repoInfo.GithubAppPrivateKeySecret,
		{Dest: &creds.GCPServiceAccountKey}: repoInfo.GCPServiceAccountKey,
	}, make(map[string]*apiv1.Secret))
	return creds, err
}

// Set data to be stored in a given secret used for repository credentials and templates.
// The name of the secret is a combination of the prefix given, and a calculated value
// from the repository or template URL.
func (l *legacyRepositoryBackend) setSecretData(prefix string, url string, secretsData map[string]map[string][]byte, secretKey *apiv1.SecretKeySelector, value string, defaultKeyName string) *apiv1.SecretKeySelector {
	if secretKey == nil && value != "" {
		secretKey = &apiv1.SecretKeySelector{
			LocalObjectReference: apiv1.LocalObjectReference{Name: RepoURLToSecretName(prefix, url, "")},
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

func (l *legacyRepositoryBackend) getRepositoryIndex(repos []settings.Repository, repoURL string) int {
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
