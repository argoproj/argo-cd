package db

import (
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
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

var _ repositoryBackend = &settingRepositoryBackend{}

type settingRepositoryBackend struct {
	db *db
}

func (s *settingRepositoryBackend) CreateRepository(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error) {
	/* panic("creating new repositories is not supported for the legacy repository backend") */

	repos, err := s.db.settingsMgr.GetRepositories()
	if err != nil {
		return nil, err
	}

	index := s.getRepositoryIndex(repos, r.Repo)
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
	err = s.updateRepositorySecrets(&repoInfo, r)
	if err != nil {
		return nil, err
	}

	repos = append(repos, repoInfo)
	err = s.db.settingsMgr.SaveRepositories(repos)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (s *settingRepositoryBackend) GetRepository(ctx context.Context, repoURL string) (*appsv1.Repository, error) {
	repository, err := s.tryGetRepository(ctx, repoURL)
	if err != nil {
		return nil, err
	}
	return repository, nil
}

func (s *settingRepositoryBackend) ListRepositories(ctx context.Context, repoType *string) ([]*appsv1.Repository, error) {
	inRepos, err := s.db.settingsMgr.GetRepositories()
	if err != nil {
		return nil, err
	}

	var repos []*appsv1.Repository
	for _, inRepo := range inRepos {
		if repoType == nil || *repoType == inRepo.Type {
			r, err := s.tryGetRepository(ctx, inRepo.URL)
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

func (s *settingRepositoryBackend) UpdateRepository(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error) {
	repos, err := s.db.settingsMgr.GetRepositories()
	if err != nil {
		return nil, err
	}

	index := s.getRepositoryIndex(repos, r.Repo)
	if index < 0 {
		return nil, status.Errorf(codes.NotFound, "repo '%s' not found", r.Repo)
	}

	repoInfo := repos[index]
	err = s.updateRepositorySecrets(&repoInfo, r)
	if err != nil {
		return nil, err
	}

	// Update boolean settings
	repoInfo.InsecureIgnoreHostKey = r.IsInsecure()
	repoInfo.Insecure = r.IsInsecure()
	repoInfo.EnableLFS = r.EnableLFS

	repos[index] = repoInfo
	err = s.db.settingsMgr.SaveRepositories(repos)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (s *settingRepositoryBackend) DeleteRepository(ctx context.Context, repoURL string) error {
	repos, err := s.db.settingsMgr.GetRepositories()
	if err != nil {
		return err
	}

	index := s.getRepositoryIndex(repos, repoURL)
	if index < 0 {
		return status.Errorf(codes.NotFound, "repo '%s' not found", repoURL)
	}
	err = s.updateRepositorySecrets(&repos[index], &appsv1.Repository{
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
	return s.db.settingsMgr.SaveRepositories(repos)
}

func (s *settingRepositoryBackend) CreateRepoCreds(ctx context.Context, r *appsv1.RepoCreds) (*appsv1.RepoCreds, error) {
	/* panic("creating new repository credentials is not supported for the legacy repository backend") */

	creds, err := s.db.settingsMgr.GetRepositoryCredentials()
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
		EnableOCI:                  r.EnableOCI,
		Type:                       r.Type,
	}

	err = s.updateCredentialsSecret(&repoInfo, r)
	if err != nil {
		return nil, err
	}

	creds = append(creds, repoInfo)
	err = s.db.settingsMgr.SaveRepositoryCredentials(creds)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (s *settingRepositoryBackend) GetRepoCreds(ctx context.Context, repoURL string) (*appsv1.RepoCreds, error) {
	var credential *appsv1.RepoCreds

	repoCredentials, err := s.db.settingsMgr.GetRepositoryCredentials()
	if err != nil {
		return nil, err
	}
	index := getRepositoryCredentialIndex(repoCredentials, repoURL)
	if index >= 0 {
		credential, err = s.credentialsToRepositoryCredentials(repoCredentials[index])
		if err != nil {
			return nil, err
		}
	}

	return credential, err
}

func (s *settingRepositoryBackend) ListRepoCreds(ctx context.Context) ([]string, error) {
	repos, err := s.db.settingsMgr.GetRepositoryCredentials()
	if err != nil {
		return nil, err
	}

	urls := make([]string, len(repos))
	for i := range repos {
		urls[i] = repos[i].URL
	}

	return urls, nil
}

func (s *settingRepositoryBackend) UpdateRepoCreds(ctx context.Context, r *appsv1.RepoCreds) (*appsv1.RepoCreds, error) {
	repos, err := s.db.settingsMgr.GetRepositoryCredentials()
	if err != nil {
		return nil, err
	}

	index := getRepositoryCredentialIndex(repos, r.URL)
	if index < 0 {
		return nil, status.Errorf(codes.NotFound, "repository credentials '%s' not found", r.URL)
	}

	repoInfo := repos[index]
	err = s.updateCredentialsSecret(&repoInfo, r)
	if err != nil {
		return nil, err
	}

	repos[index] = repoInfo
	err = s.db.settingsMgr.SaveRepositoryCredentials(repos)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (s *settingRepositoryBackend) DeleteRepoCreds(ctx context.Context, name string) error {
	repos, err := s.db.settingsMgr.GetRepositoryCredentials()
	if err != nil {
		return err
	}

	index := getRepositoryCredentialIndex(repos, name)
	if index < 0 {
		return status.Errorf(codes.NotFound, "repository credentials '%s' not found", name)
	}
	err = s.updateCredentialsSecret(&repos[index], &appsv1.RepoCreds{
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
	return s.db.settingsMgr.SaveRepositoryCredentials(repos)
}

func (s *settingRepositoryBackend) GetAllHelmRepoCreds(ctx context.Context) ([]*appsv1.RepoCreds, error) {
	var allCredentials []*appsv1.RepoCreds
	repoCredentials, err := s.db.settingsMgr.GetRepositoryCredentials()
	if err != nil {
		return nil, err
	}
	for _, v := range repoCredentials {
		if strings.EqualFold(v.Type, "helm") {
			credential, err := s.credentialsToRepositoryCredentials(v)
			if err != nil {
				return nil, err
			}
			allCredentials = append(allCredentials, credential)
		}
	}
	return allCredentials, err
}

func (s *settingRepositoryBackend) updateRepositorySecrets(repoInfo *settings.Repository, r *appsv1.Repository) error {
	secretsData := make(map[string]map[string][]byte)

	repoInfo.UsernameSecret = s.setSecretData(repoSecretPrefix, r.Repo, secretsData, repoInfo.UsernameSecret, r.Username, username)
	repoInfo.PasswordSecret = s.setSecretData(repoSecretPrefix, r.Repo, secretsData, repoInfo.PasswordSecret, r.Password, password)
	repoInfo.SSHPrivateKeySecret = s.setSecretData(repoSecretPrefix, r.Repo, secretsData, repoInfo.SSHPrivateKeySecret, r.SSHPrivateKey, sshPrivateKey)
	repoInfo.TLSClientCertDataSecret = s.setSecretData(repoSecretPrefix, r.Repo, secretsData, repoInfo.TLSClientCertDataSecret, r.TLSClientCertData, tlsClientCertData)
	repoInfo.TLSClientCertKeySecret = s.setSecretData(repoSecretPrefix, r.Repo, secretsData, repoInfo.TLSClientCertKeySecret, r.TLSClientCertKey, tlsClientCertKey)
	repoInfo.GithubAppPrivateKeySecret = s.setSecretData(repoSecretPrefix, r.Repo, secretsData, repoInfo.GithubAppPrivateKeySecret, r.GithubAppPrivateKey, githubAppPrivateKey)
	for k, v := range secretsData {
		err := s.upsertSecret(k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *settingRepositoryBackend) updateCredentialsSecret(credsInfo *settings.RepositoryCredentials, c *appsv1.RepoCreds) error {
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

	credsInfo.UsernameSecret = s.setSecretData(credSecretPrefix, r.Repo, secretsData, credsInfo.UsernameSecret, r.Username, username)
	credsInfo.PasswordSecret = s.setSecretData(credSecretPrefix, r.Repo, secretsData, credsInfo.PasswordSecret, r.Password, password)
	credsInfo.SSHPrivateKeySecret = s.setSecretData(credSecretPrefix, r.Repo, secretsData, credsInfo.SSHPrivateKeySecret, r.SSHPrivateKey, sshPrivateKey)
	credsInfo.TLSClientCertDataSecret = s.setSecretData(credSecretPrefix, r.Repo, secretsData, credsInfo.TLSClientCertDataSecret, r.TLSClientCertData, tlsClientCertData)
	credsInfo.TLSClientCertKeySecret = s.setSecretData(credSecretPrefix, r.Repo, secretsData, credsInfo.TLSClientCertKeySecret, r.TLSClientCertKey, tlsClientCertKey)
	credsInfo.GithubAppPrivateKeySecret = s.setSecretData(repoSecretPrefix, r.Repo, secretsData, credsInfo.GithubAppPrivateKeySecret, r.GithubAppPrivateKey, githubAppPrivateKey)
	for k, v := range secretsData {
		err := s.upsertSecret(k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *settingRepositoryBackend) upsertSecret(name string, data map[string][]byte) error {
	secret, err := s.db.kubeclientset.CoreV1().Secrets(s.db.ns).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if apierr.IsNotFound(err) {
			if len(data) == 0 {
				return nil
			}
			_, err = s.db.kubeclientset.CoreV1().Secrets(s.db.ns).Create(context.Background(), &apiv1.Secret{
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
				return s.db.kubeclientset.CoreV1().Secrets(s.db.ns).Delete(context.Background(), name, metav1.DeleteOptions{})
			}
			return nil
		} else {
			_, err = s.db.kubeclientset.CoreV1().Secrets(s.db.ns).Update(context.Background(), secret, metav1.UpdateOptions{})
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
func (s *settingRepositoryBackend) tryGetRepository(ctx context.Context, repoURL string) (*appsv1.Repository, error) {
	repos, err := s.db.settingsMgr.GetRepositories()
	if err != nil {
		return nil, err
	}

	repo := &appsv1.Repository{Repo: repoURL}
	index := s.getRepositoryIndex(repos, repoURL)
	if index >= 0 {
		repo, err = s.credentialsToRepository(repos[index])
		if err != nil {
			return repo, errors.NewCredentialsConfigurationError(err)
		}
	}

	// Check for and copy repository credentials, if repo has none configured.
	if !repo.HasCredentials() {
		creds, err := s.GetRepoCreds(ctx, repoURL)
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

func (s *settingRepositoryBackend) credentialsToRepository(repoInfo settings.Repository) (*appsv1.Repository, error) {
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
	err := s.db.unmarshalFromSecretsStr(map[*SecretMaperValidation]*apiv1.SecretKeySelector{
		&SecretMaperValidation{Dest: &repo.Username, Transform: StripCRLFCharacter}:            repoInfo.UsernameSecret,
		&SecretMaperValidation{Dest: &repo.Password, Transform: StripCRLFCharacter}:            repoInfo.PasswordSecret,
		&SecretMaperValidation{Dest: &repo.SSHPrivateKey, Transform: StripCRLFCharacter}:       repoInfo.SSHPrivateKeySecret,
		&SecretMaperValidation{Dest: &repo.TLSClientCertData, Transform: StripCRLFCharacter}:   repoInfo.TLSClientCertDataSecret,
		&SecretMaperValidation{Dest: &repo.TLSClientCertKey, Transform: StripCRLFCharacter}:    repoInfo.TLSClientCertKeySecret,
		&SecretMaperValidation{Dest: &repo.GithubAppPrivateKey, Transform: StripCRLFCharacter}: repoInfo.GithubAppPrivateKeySecret,
	}, make(map[string]*apiv1.Secret))
	return repo, err
}

func (s *settingRepositoryBackend) credentialsToRepositoryCredentials(repoInfo settings.RepositoryCredentials) (*appsv1.RepoCreds, error) {
	creds := &appsv1.RepoCreds{
		URL:                        repoInfo.URL,
		GithubAppId:                repoInfo.GithubAppId,
		GithubAppInstallationId:    repoInfo.GithubAppInstallationId,
		GitHubAppEnterpriseBaseURL: repoInfo.GithubAppEnterpriseBaseURL,
		EnableOCI:                  repoInfo.EnableOCI,
	}
	err := s.db.unmarshalFromSecretsStr(map[*SecretMaperValidation]*apiv1.SecretKeySelector{
		&SecretMaperValidation{Dest: &creds.Username}:            repoInfo.UsernameSecret,
		&SecretMaperValidation{Dest: &creds.Password}:            repoInfo.PasswordSecret,
		&SecretMaperValidation{Dest: &creds.SSHPrivateKey}:       repoInfo.SSHPrivateKeySecret,
		&SecretMaperValidation{Dest: &creds.TLSClientCertData}:   repoInfo.TLSClientCertDataSecret,
		&SecretMaperValidation{Dest: &creds.TLSClientCertKey}:    repoInfo.TLSClientCertKeySecret,
		&SecretMaperValidation{Dest: &creds.GithubAppPrivateKey}: repoInfo.GithubAppPrivateKeySecret,
	}, make(map[string]*apiv1.Secret))
	return creds, err
}

// Set data to be stored in a given secret used for repository credentials and templates.
// The name of the secret is a combination of the prefix given, and a calculated value
// from the repository or template URL.
func (s *settingRepositoryBackend) setSecretData(prefix string, url string, secretsData map[string]map[string][]byte, secretKey *apiv1.SecretKeySelector, value string, defaultKeyName string) *apiv1.SecretKeySelector {
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

func (s *settingRepositoryBackend) getRepositoryIndex(repos []settings.Repository, repoURL string) int {
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
