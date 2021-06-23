package db

import (
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/common"
	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/git"
)

var _ repositoryBackend = &secretsRepositoryBackend{}

type secretsRepositoryBackend struct {
	db *db
}

func (s *secretsRepositoryBackend) CreateRepository(ctx context.Context, repository *appsv1.Repository) (*appsv1.Repository, error) {
	secName := RepoURLToSecretName(repoSecretPrefix, repository.Repo)

	repositorySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secName,
		},
	}

	s.repositoryToSecret(repository, repositorySecret)

	_, err := s.db.createSecret(ctx, common.LabelValueSecretTypeRepository, repositorySecret)
	if err != nil {
		if apierr.IsAlreadyExists(err) {
			return nil, status.Errorf(codes.AlreadyExists, "repository %q already exists", repository.Repo)
		}
		return nil, err
	}

	return repository, s.db.settingsMgr.ResyncInformers()
}

func (s *secretsRepositoryBackend) GetRepository(ctx context.Context, repoURL string) (*appsv1.Repository, error) {
	secret, err := s.getRepositorySecret(repoURL)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return &appsv1.Repository{Repo: repoURL}, nil
		}

		return nil, err
	}

	repository, err := s.secretToRepository(secret)
	if err != nil {
		return nil, err
	}

	return repository, err
}

func (s *secretsRepositoryBackend) ListRepositories(ctx context.Context, repoType *string) ([]*appsv1.Repository, error) {
	var repos []*appsv1.Repository

	secrets, err := s.db.listSecretsByType(common.LabelValueSecretTypeRepository)
	if err != nil {
		return nil, err
	}

	for _, secret := range secrets {
		r, err := s.secretToRepository(secret)
		if err != nil {
			if r != nil {
				modifiedTime := metav1.Now()
				r.ConnectionState = appsv1.ConnectionState{
					Status:     appsv1.ConnectionStatusFailed,
					Message:    "Configuration error - please check the server logs",
					ModifiedAt: &modifiedTime,
				}

				log.Warnf("Error while parsing repository secret '%s': %v", secret.Name, err)
			} else {
				return nil, err
			}
		}

		if repoType == nil || *repoType == r.Type {
			repos = append(repos, r)
		}
	}

	return repos, nil
}

func (s *secretsRepositoryBackend) UpdateRepository(ctx context.Context, repository *appsv1.Repository) (*appsv1.Repository, error) {
	repositorySecret, err := s.getRepositorySecret(repository.Repo)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return s.CreateRepository(ctx, repository)
		}
		return nil, err
	}

	s.repositoryToSecret(repository, repositorySecret)

	_, err = s.db.kubeclientset.CoreV1().Secrets(s.db.ns).Update(ctx, repositorySecret, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return repository, s.db.settingsMgr.ResyncInformers()
}

func (s *secretsRepositoryBackend) DeleteRepository(ctx context.Context, repoURL string) error {
	secret, err := s.getRepositorySecret(repoURL)
	if err != nil {
		return err
	}

	if err := s.db.deleteSecret(ctx, secret); err != nil {
		return err
	}

	return s.db.settingsMgr.ResyncInformers()
}

func (s *secretsRepositoryBackend) RepositoryExists(ctx context.Context, repoURL string) (bool, error) {
	secret, err := s.getRepositorySecret(repoURL)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}

		return false, err
	}

	return secret != nil, nil
}

func (s *secretsRepositoryBackend) CreateRepoCreds(ctx context.Context, repoCreds *appsv1.RepoCreds) (*appsv1.RepoCreds, error) {
	secName := RepoURLToSecretName(credSecretPrefix, repoCreds.URL)

	repoCredsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secName,
		},
	}

	s.repoCredsToSecret(repoCreds, repoCredsSecret)

	_, err := s.db.createSecret(ctx, common.LabelValueSecretTypeRepoCreds, repoCredsSecret)
	if err != nil {
		if apierr.IsAlreadyExists(err) {
			return nil, status.Errorf(codes.AlreadyExists, "repository credentials %q already exists", repoCreds.URL)
		}
		return nil, err
	}

	return repoCreds, s.db.settingsMgr.ResyncInformers()
}

func (s *secretsRepositoryBackend) GetRepoCreds(ctx context.Context, repoURL string) (*appsv1.RepoCreds, error) {
	secret, err := s.getRepoCredsSecret(repoURL)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}

		return nil, err
	}

	return s.secretToRepoCred(secret)
}

func (s *secretsRepositoryBackend) ListRepoCreds(ctx context.Context) ([]string, error) {
	var repoURLs []string

	secrets, err := s.db.listSecretsByType(common.LabelValueSecretTypeRepoCreds)
	if err != nil {
		return nil, err
	}

	for _, secret := range secrets {
		repoURLs = append(repoURLs, string(secret.Data["url"]))
	}

	return repoURLs, nil
}

func (s *secretsRepositoryBackend) UpdateRepoCreds(ctx context.Context, repoCreds *appsv1.RepoCreds) (*appsv1.RepoCreds, error) {
	repoCredsSecret, err := s.getRepoCredsSecret(repoCreds.URL)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return s.CreateRepoCreds(ctx, repoCreds)
		}
		return nil, err
	}

	s.repoCredsToSecret(repoCreds, repoCredsSecret)

	repoCredsSecret, err = s.db.kubeclientset.CoreV1().Secrets(s.db.ns).Update(ctx, repoCredsSecret, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	updatedRepoCreds, err := s.secretToRepoCred(repoCredsSecret)
	if err != nil {
		return nil, err
	}

	return updatedRepoCreds, s.db.settingsMgr.ResyncInformers()
}

func (s *secretsRepositoryBackend) DeleteRepoCreds(ctx context.Context, name string) error {
	secret, err := s.getRepoCredsSecret(name)
	if err != nil {
		return err
	}

	if err := s.db.deleteSecret(ctx, secret); err != nil {
		return err
	}

	return s.db.settingsMgr.ResyncInformers()
}

func (s *secretsRepositoryBackend) RepoCredsExists(ctx context.Context, repoURL string) (bool, error) {
	_, err := s.getRepoCredsSecret(repoURL)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (s *secretsRepositoryBackend) GetAllHelmRepoCreds(ctx context.Context) ([]*appsv1.RepoCreds, error) {
	var helmRepoCreds []*appsv1.RepoCreds

	secrets, err := s.db.listSecretsByType(common.LabelValueSecretTypeRepoCreds)
	if err != nil {
		return nil, err
	}

	for _, secret := range secrets {
		if strings.EqualFold(string(secret.Data["type"]), "helm") {
			repoCreds, err := s.secretToRepoCred(secret)
			if err != nil {
				return nil, err
			}

			helmRepoCreds = append(helmRepoCreds, repoCreds)
		}
	}

	return helmRepoCreds, nil
}

func (s *secretsRepositoryBackend) secretToRepository(secret *corev1.Secret) (*appsv1.Repository, error) {
	repository := &appsv1.Repository{
		Name:                       string(secret.Data["name"]),
		Repo:                       string(secret.Data["url"]),
		Username:                   string(secret.Data["username"]),
		Password:                   string(secret.Data["password"]),
		SSHPrivateKey:              string(secret.Data["sshPrivateKey"]),
		TLSClientCertData:          string(secret.Data["tlsClientCertData"]),
		TLSClientCertKey:           string(secret.Data["tlsClientCertKey"]),
		Type:                       string(secret.Data["type"]),
		GithubAppPrivateKey:        string(secret.Data["githubAppPrivateKey"]),
		GitHubAppEnterpriseBaseURL: string(secret.Data["githubAppEnterpriseBaseUrl"]),
		Proxy:                      string(secret.Data["proxy"]),
	}

	insecureIgnoreHostKey, err := boolOrFalse(secret, "insecureIgnoreHostKey")
	if err != nil {
		return repository, err
	}
	repository.InsecureIgnoreHostKey = insecureIgnoreHostKey

	insecure, err := boolOrFalse(secret, "insecure")
	if err != nil {
		return repository, err
	}
	repository.Insecure = insecure

	enableLfs, err := boolOrFalse(secret, "enableLfs")
	if err != nil {
		return repository, err
	}
	repository.EnableLFS = enableLfs

	enableOCI, err := boolOrFalse(secret, "enableOCI")
	if err != nil {
		return repository, err
	}
	repository.EnableOCI = enableOCI

	githubAppID, err := intOrZero(secret, "githubAppID")
	if err != nil {
		return repository, err
	}
	repository.GithubAppId = githubAppID

	githubAppInstallationID, err := intOrZero(secret, "githubAppInstallationID")
	if err != nil {
		return repository, err
	}
	repository.GithubAppInstallationId = githubAppInstallationID

	return repository, nil
}

func (s *secretsRepositoryBackend) repositoryToSecret(repository *appsv1.Repository, secret *corev1.Secret) {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}

	updateSecretString(secret, "name", repository.Name)
	updateSecretString(secret, "url", repository.Repo)
	updateSecretString(secret, "username", repository.Username)
	updateSecretString(secret, "password", repository.Password)
	updateSecretString(secret, "sshPrivateKey", repository.SSHPrivateKey)
	updateSecretBool(secret, "enableOCI", repository.EnableOCI)
	updateSecretString(secret, "tlsClientCertData", repository.TLSClientCertData)
	updateSecretString(secret, "tlsClientCertKey", repository.TLSClientCertKey)
	updateSecretString(secret, "type", repository.Type)
	updateSecretString(secret, "githubAppPrivateKey", repository.GithubAppPrivateKey)
	updateSecretInt(secret, "githubAppID", repository.GithubAppId)
	updateSecretInt(secret, "githubAppInstallationID", repository.GithubAppInstallationId)
	updateSecretString(secret, "githubAppEnterpriseBaseUrl", repository.GitHubAppEnterpriseBaseURL)
	updateSecretBool(secret, "insecureIgnoreHostKey", repository.InsecureIgnoreHostKey)
	updateSecretBool(secret, "insecure", repository.Insecure)
	updateSecretBool(secret, "enableLfs", repository.EnableLFS)
	updateSecretString(secret, "proxy", repository.Proxy)
}

func (s *secretsRepositoryBackend) secretToRepoCred(secret *corev1.Secret) (*appsv1.RepoCreds, error) {
	repository := &appsv1.RepoCreds{
		URL:                        string(secret.Data["url"]),
		Username:                   string(secret.Data["username"]),
		Password:                   string(secret.Data["password"]),
		SSHPrivateKey:              string(secret.Data["sshPrivateKey"]),
		TLSClientCertData:          string(secret.Data["tlsClientCertData"]),
		TLSClientCertKey:           string(secret.Data["tlsClientCertKey"]),
		Type:                       string(secret.Data["type"]),
		GithubAppPrivateKey:        string(secret.Data["githubAppPrivateKey"]),
		GitHubAppEnterpriseBaseURL: string(secret.Data["githubAppEnterpriseBaseUrl"]),
	}

	enableOCI, err := boolOrFalse(secret, "enableOCI")
	if err != nil {
		return repository, err
	}
	repository.EnableOCI = enableOCI

	githubAppID, err := intOrZero(secret, "githubAppID")
	if err != nil {
		return repository, err
	}
	repository.GithubAppId = githubAppID

	githubAppInstallationID, err := intOrZero(secret, "githubAppInstallationID")
	if err != nil {
		return repository, err
	}
	repository.GithubAppInstallationId = githubAppInstallationID

	return repository, nil
}

func (s *secretsRepositoryBackend) repoCredsToSecret(repoCreds *appsv1.RepoCreds, secret *corev1.Secret) {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}

	updateSecretString(secret, "url", repoCreds.URL)
	updateSecretString(secret, "username", repoCreds.Username)
	updateSecretString(secret, "password", repoCreds.Password)
	updateSecretString(secret, "sshPrivateKey", repoCreds.SSHPrivateKey)
	updateSecretBool(secret, "enableOCI", repoCreds.EnableOCI)
	updateSecretString(secret, "tlsClientCertData", repoCreds.TLSClientCertData)
	updateSecretString(secret, "tlsClientCertKey", repoCreds.TLSClientCertKey)
	updateSecretString(secret, "type", repoCreds.Type)
	updateSecretString(secret, "githubAppPrivateKey", repoCreds.GithubAppPrivateKey)
	updateSecretInt(secret, "githubAppID", repoCreds.GithubAppId)
	updateSecretInt(secret, "githubAppInstallationID", repoCreds.GithubAppInstallationId)
	updateSecretString(secret, "githubAppEnterpriseBaseUrl", repoCreds.GitHubAppEnterpriseBaseURL)
}

func (s *secretsRepositoryBackend) getRepositorySecret(repoURL string) (*corev1.Secret, error) {
	secrets, err := s.db.listSecretsByType(common.LabelValueSecretTypeRepository)
	if err != nil {
		return nil, err
	}

	for _, secret := range secrets {
		if git.SameURL(string(secret.Data["url"]), repoURL) {
			return secret, nil
		}
	}

	return nil, status.Errorf(codes.NotFound, "repository %q not found", repoURL)
}

func (s *secretsRepositoryBackend) getRepoCredsSecret(repoURL string) (*corev1.Secret, error) {
	secrets, err := s.db.listSecretsByType(common.LabelValueSecretTypeRepoCreds)
	if err != nil {
		return nil, err
	}

	index := s.getRepositoryCredentialIndex(secrets, repoURL)
	if index < 0 {
		return nil, status.Errorf(codes.NotFound, "repository credentials %q not found", repoURL)
	}

	return secrets[index], nil
}

func (s *secretsRepositoryBackend) getRepositoryCredentialIndex(repoCredentials []*corev1.Secret, repoURL string) int {
	var max, idx = 0, -1
	repoURL = git.NormalizeGitURL(repoURL)
	for i, cred := range repoCredentials {
		credUrl := git.NormalizeGitURL(string(cred.Data["url"]))
		if strings.HasPrefix(repoURL, credUrl) {
			if len(credUrl) > max {
				max = len(credUrl)
				idx = i
			}
		}
	}
	return idx
}
