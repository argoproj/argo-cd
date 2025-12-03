package db

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/common"
	appsv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/git"
)

var _ repositoryBackend = &secretsRepositoryBackend{}

type secretsRepositoryBackend struct {
	db *db
	// If true, the backend will manage write only credentials. If false, it will manage only read credentials.
	writeCreds bool
}

func (s *secretsRepositoryBackend) CreateRepository(ctx context.Context, repository *appsv1.Repository) (*appsv1.Repository, error) {
	secName := RepoURLToSecretName(repoSecretPrefix, repository.Repo, repository.Project)

	repositorySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secName,
		},
	}

	updatedSecret := s.repositoryToSecret(repository, repositorySecret)

	_, err := s.db.createSecret(ctx, updatedSecret)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			hasLabel, err := s.hasRepoTypeLabel(secName)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			if !hasLabel {
				msg := fmt.Sprintf("secret %q doesn't have the proper %q label: please fix the secret or delete it", secName, common.LabelKeySecretType)
				return nil, status.Error(codes.InvalidArgument, msg)
			}
			return nil, status.Errorf(codes.AlreadyExists, "repository %q already exists", repository.Repo)
		}
		return nil, err
	}

	return repository, s.db.settingsMgr.ResyncInformers()
}

// hasRepoTypeLabel will verify if a secret with the given name exists. If so it will check if
// the secret has the proper label argocd.argoproj.io/secret-type defined. Will return true if
// the label is found and false otherwise. Will return false if no secret is found with the given
// name.
func (s *secretsRepositoryBackend) hasRepoTypeLabel(secretName string) (bool, error) {
	noCache := make(map[string]*corev1.Secret)
	sec, err := s.db.getSecret(secretName, noCache)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	_, ok := sec.GetLabels()[common.LabelKeySecretType]
	if ok {
		return true, nil
	}
	return false, nil
}

func (s *secretsRepositoryBackend) GetRepoCredsBySecretName(_ context.Context, name string) (*appsv1.RepoCreds, error) {
	secret, err := s.db.getSecret(name, map[string]*corev1.Secret{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s: %w", name, err)
	}
	return s.secretToRepoCred(secret)
}

func (s *secretsRepositoryBackend) GetRepository(_ context.Context, repoURL, project string) (*appsv1.Repository, error) {
	secret, err := s.getRepositorySecret(repoURL, project, true)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return &appsv1.Repository{Repo: repoURL}, nil
		}

		return nil, err
	}

	repository, err := secretToRepository(secret)
	if err != nil {
		return nil, err
	}

	return repository, err
}

func (s *secretsRepositoryBackend) ListRepositories(_ context.Context, repoType *string) ([]*appsv1.Repository, error) {
	var repos []*appsv1.Repository

	secrets, err := s.db.listSecretsByType(s.getSecretType())
	if err != nil {
		return nil, err
	}

	for _, secret := range secrets {
		r, err := secretToRepository(secret)
		if err != nil {
			if r == nil {
				return nil, err
			}
			modifiedTime := metav1.Now()
			r.ConnectionState = appsv1.ConnectionState{
				Status:     appsv1.ConnectionStatusFailed,
				Message:    "Configuration error - please check the server logs",
				ModifiedAt: &modifiedTime,
			}

			log.Warnf("Error while parsing repository secret '%s': %v", secret.Name, err)
		}

		if repoType == nil || *repoType == r.Type {
			repos = append(repos, r)
		}
	}

	return repos, nil
}

func (s *secretsRepositoryBackend) UpdateRepository(ctx context.Context, repository *appsv1.Repository) (*appsv1.Repository, error) {
	repositorySecret, err := s.getRepositorySecret(repository.Repo, repository.Project, false)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return s.CreateRepository(ctx, repository)
		}
		return nil, err
	}

	updatedSecret := s.repositoryToSecret(repository, repositorySecret)

	_, err = s.db.kubeclientset.CoreV1().Secrets(s.db.ns).Update(ctx, updatedSecret, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return repository, s.db.settingsMgr.ResyncInformers()
}

func (s *secretsRepositoryBackend) DeleteRepository(ctx context.Context, repoURL, project string) error {
	secret, err := s.getRepositorySecret(repoURL, project, false)
	if err != nil {
		return err
	}

	if err := s.db.deleteSecret(ctx, secret); err != nil {
		return err
	}

	return s.db.settingsMgr.ResyncInformers()
}

func (s *secretsRepositoryBackend) RepositoryExists(_ context.Context, repoURL, project string, allowFallback bool) (bool, error) {
	secret, err := s.getRepositorySecret(repoURL, project, allowFallback)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}

		return false, fmt.Errorf("failed to get repository secret for %q: %w", repoURL, err)
	}

	return secret != nil, nil
}

func (s *secretsRepositoryBackend) CreateRepoCreds(ctx context.Context, repoCreds *appsv1.RepoCreds) (*appsv1.RepoCreds, error) {
	secName := RepoURLToSecretName(credSecretPrefix, repoCreds.URL, "")

	repoCredsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secName,
		},
	}

	updatedSecret := s.repoCredsToSecret(repoCreds, repoCredsSecret)

	_, err := s.db.createSecret(ctx, updatedSecret)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil, status.Errorf(codes.AlreadyExists, "repository credentials %q already exists", repoCreds.URL)
		}
		return nil, err
	}

	return repoCreds, s.db.settingsMgr.ResyncInformers()
}

func (s *secretsRepositoryBackend) GetRepoCreds(_ context.Context, repoURL string) (*appsv1.RepoCreds, error) {
	secret, err := s.getRepoCredsSecret(repoURL)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}

		return nil, err
	}

	return s.secretToRepoCred(secret)
}

func (s *secretsRepositoryBackend) ListRepoCreds(_ context.Context) ([]string, error) {
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

	updatedSecret := s.repoCredsToSecret(repoCreds, repoCredsSecret)

	repoCredsSecret, err = s.db.kubeclientset.CoreV1().Secrets(s.db.ns).Update(ctx, updatedSecret, metav1.UpdateOptions{})
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

func (s *secretsRepositoryBackend) RepoCredsExists(_ context.Context, repoURL string) (bool, error) {
	_, err := s.getRepoCredsSecret(repoURL)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (s *secretsRepositoryBackend) GetAllHelmRepoCreds(_ context.Context) ([]*appsv1.RepoCreds, error) {
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

func (s *secretsRepositoryBackend) GetAllOCIRepoCreds(_ context.Context) ([]*appsv1.RepoCreds, error) {
	var ociRepoCreds []*appsv1.RepoCreds

	secrets, err := s.db.listSecretsByType(common.LabelValueSecretTypeRepoCreds)
	if err != nil {
		return nil, err
	}

	for _, secret := range secrets {
		if strings.EqualFold(string(secret.Data["type"]), "oci") {
			repoCreds, err := s.secretToRepoCred(secret)
			if err != nil {
				return nil, err
			}

			ociRepoCreds = append(ociRepoCreds, repoCreds)
		}
	}

	return ociRepoCreds, nil
}

func secretToRepository(secret *corev1.Secret) (*appsv1.Repository, error) {
	secretCopy := secret.DeepCopy()

	repository := &appsv1.Repository{
		Name:                       string(secretCopy.Data["name"]),
		Repo:                       string(secretCopy.Data["url"]),
		Username:                   string(secretCopy.Data["username"]),
		Password:                   string(secretCopy.Data["password"]),
		BearerToken:                string(secretCopy.Data["bearerToken"]),
		SSHPrivateKey:              string(secretCopy.Data["sshPrivateKey"]),
		TLSClientCertData:          string(secretCopy.Data["tlsClientCertData"]),
		TLSClientCertKey:           string(secretCopy.Data["tlsClientCertKey"]),
		Type:                       string(secretCopy.Data["type"]),
		GithubAppPrivateKey:        string(secretCopy.Data["githubAppPrivateKey"]),
		GitHubAppEnterpriseBaseURL: string(secretCopy.Data["githubAppEnterpriseBaseUrl"]),
		Proxy:                      string(secretCopy.Data["proxy"]),
		NoProxy:                    string(secretCopy.Data["noProxy"]),
		Project:                    string(secretCopy.Data["project"]),
		GCPServiceAccountKey:       string(secretCopy.Data["gcpServiceAccountKey"]),
	}

	insecureIgnoreHostKey, err := boolOrFalse(secretCopy, "insecureIgnoreHostKey")
	if err != nil {
		return repository, err
	}
	repository.InsecureIgnoreHostKey = insecureIgnoreHostKey

	insecure, err := boolOrFalse(secretCopy, "insecure")
	if err != nil {
		return repository, err
	}
	repository.Insecure = insecure

	enableLfs, err := boolOrFalse(secretCopy, "enableLfs")
	if err != nil {
		return repository, err
	}
	repository.EnableLFS = enableLfs

	enableOCI, err := boolOrFalse(secretCopy, "enableOCI")
	if err != nil {
		return repository, err
	}
	repository.EnableOCI = enableOCI

	insecureOCIForceHTTP, err := boolOrFalse(secret, "insecureOCIForceHttp")
	if err != nil {
		return repository, err
	}
	repository.InsecureOCIForceHttp = insecureOCIForceHTTP

	githubAppID, err := intOrZero(secretCopy, "githubAppID")
	if err != nil {
		return repository, err
	}
	repository.GithubAppId = githubAppID

	githubAppInstallationID, err := intOrZero(secretCopy, "githubAppInstallationID")
	if err != nil {
		return repository, err
	}
	repository.GithubAppInstallationId = githubAppInstallationID

	forceBasicAuth, err := boolOrFalse(secretCopy, "forceHttpBasicAuth")
	if err != nil {
		return repository, err
	}
	repository.ForceHttpBasicAuth = forceBasicAuth

	useAzureWorkloadIdentity, err := boolOrFalse(secret, "useAzureWorkloadIdentity")
	if err != nil {
		return repository, err
	}
	repository.UseAzureWorkloadIdentity = useAzureWorkloadIdentity

	depth, err := intOrZero(secret, "depth")
	if err != nil {
		return repository, err
	}
	repository.Depth = depth

	enablePartialClone, err := boolOrFalse(secret, "enablePartialClone")
	if err != nil {
		return repository, err
	}
	repository.EnablePartialClone = enablePartialClone

	repository.SparsePaths = stringArrayOrEmpty(secret, "sparsePaths")

	return repository, nil
}

func (s *secretsRepositoryBackend) repositoryToSecret(repository *appsv1.Repository, secret *corev1.Secret) *corev1.Secret {
	secretCopy := secret.DeepCopy()

	if secretCopy.Data == nil {
		secretCopy.Data = make(map[string][]byte)
	}

	updateSecretString(secretCopy, "name", repository.Name)
	updateSecretString(secretCopy, "project", repository.Project)
	updateSecretString(secretCopy, "url", repository.Repo)
	updateSecretString(secretCopy, "username", repository.Username)
	updateSecretString(secretCopy, "password", repository.Password)
	updateSecretString(secretCopy, "bearerToken", repository.BearerToken)
	updateSecretString(secretCopy, "sshPrivateKey", repository.SSHPrivateKey)
	updateSecretBool(secretCopy, "enableOCI", repository.EnableOCI)
	updateSecretBool(secretCopy, "insecureOCIForceHttp", repository.InsecureOCIForceHttp)
	updateSecretString(secretCopy, "tlsClientCertData", repository.TLSClientCertData)
	updateSecretString(secretCopy, "tlsClientCertKey", repository.TLSClientCertKey)
	updateSecretString(secretCopy, "type", repository.Type)
	updateSecretString(secretCopy, "githubAppPrivateKey", repository.GithubAppPrivateKey)
	updateSecretInt(secretCopy, "githubAppID", repository.GithubAppId)
	updateSecretInt(secretCopy, "githubAppInstallationID", repository.GithubAppInstallationId)
	updateSecretString(secretCopy, "githubAppEnterpriseBaseUrl", repository.GitHubAppEnterpriseBaseURL)
	updateSecretBool(secretCopy, "insecureIgnoreHostKey", repository.InsecureIgnoreHostKey)
	updateSecretBool(secretCopy, "insecure", repository.Insecure)
	updateSecretBool(secretCopy, "enableLfs", repository.EnableLFS)
	updateSecretString(secretCopy, "proxy", repository.Proxy)
	updateSecretString(secretCopy, "noProxy", repository.NoProxy)
	updateSecretString(secretCopy, "gcpServiceAccountKey", repository.GCPServiceAccountKey)
	updateSecretBool(secretCopy, "forceHttpBasicAuth", repository.ForceHttpBasicAuth)
	updateSecretBool(secretCopy, "useAzureWorkloadIdentity", repository.UseAzureWorkloadIdentity)
	updateSecretInt(secretCopy, "depth", repository.Depth)
	updateSecretBool(secretCopy, "enablePartialClone", repository.EnablePartialClone)
	updateSecretStringArray(secretCopy, "sparsePaths", repository.SparsePaths)
	addSecretMetadata(secretCopy, s.getSecretType())

	return secretCopy
}

func (s *secretsRepositoryBackend) secretToRepoCred(secret *corev1.Secret) (*appsv1.RepoCreds, error) {
	secretCopy := secret.DeepCopy()

	repository := &appsv1.RepoCreds{
		URL:                        string(secretCopy.Data["url"]),
		Username:                   string(secretCopy.Data["username"]),
		Password:                   string(secretCopy.Data["password"]),
		BearerToken:                string(secretCopy.Data["bearerToken"]),
		SSHPrivateKey:              string(secretCopy.Data["sshPrivateKey"]),
		TLSClientCertData:          string(secretCopy.Data["tlsClientCertData"]),
		TLSClientCertKey:           string(secretCopy.Data["tlsClientCertKey"]),
		Type:                       string(secretCopy.Data["type"]),
		GithubAppPrivateKey:        string(secretCopy.Data["githubAppPrivateKey"]),
		GitHubAppEnterpriseBaseURL: string(secretCopy.Data["githubAppEnterpriseBaseUrl"]),
		GCPServiceAccountKey:       string(secretCopy.Data["gcpServiceAccountKey"]),
		Proxy:                      string(secretCopy.Data["proxy"]),
		NoProxy:                    string(secretCopy.Data["noProxy"]),
	}

	enableOCI, err := boolOrFalse(secretCopy, "enableOCI")
	if err != nil {
		return repository, err
	}
	repository.EnableOCI = enableOCI

	insecureOCIForceHTTP, err := boolOrFalse(secret, "insecureOCIForceHttp")
	if err != nil {
		return repository, err
	}
	repository.InsecureOCIForceHttp = insecureOCIForceHTTP

	githubAppID, err := intOrZero(secretCopy, "githubAppID")
	if err != nil {
		return repository, err
	}
	repository.GithubAppId = githubAppID

	githubAppInstallationID, err := intOrZero(secretCopy, "githubAppInstallationID")
	if err != nil {
		return repository, err
	}
	repository.GithubAppInstallationId = githubAppInstallationID

	forceBasicAuth, err := boolOrFalse(secretCopy, "forceHttpBasicAuth")
	if err != nil {
		return repository, err
	}
	repository.ForceHttpBasicAuth = forceBasicAuth

	useAzureWorkloadIdentity, err := boolOrFalse(secret, "useAzureWorkloadIdentity")
	if err != nil {
		return repository, err
	}
	repository.UseAzureWorkloadIdentity = useAzureWorkloadIdentity

	return repository, nil
}

func (s *secretsRepositoryBackend) repoCredsToSecret(repoCreds *appsv1.RepoCreds, secret *corev1.Secret) *corev1.Secret {
	secretCopy := secret.DeepCopy()

	if secretCopy.Data == nil {
		secretCopy.Data = make(map[string][]byte)
	}

	updateSecretString(secretCopy, "url", repoCreds.URL)
	updateSecretString(secretCopy, "username", repoCreds.Username)
	updateSecretString(secretCopy, "password", repoCreds.Password)
	updateSecretString(secretCopy, "bearerToken", repoCreds.BearerToken)
	updateSecretString(secretCopy, "sshPrivateKey", repoCreds.SSHPrivateKey)
	updateSecretBool(secretCopy, "enableOCI", repoCreds.EnableOCI)
	updateSecretBool(secretCopy, "insecureOCIForceHttp", repoCreds.InsecureOCIForceHttp)
	updateSecretString(secretCopy, "tlsClientCertData", repoCreds.TLSClientCertData)
	updateSecretString(secretCopy, "tlsClientCertKey", repoCreds.TLSClientCertKey)
	updateSecretString(secretCopy, "type", repoCreds.Type)
	updateSecretString(secretCopy, "githubAppPrivateKey", repoCreds.GithubAppPrivateKey)
	updateSecretInt(secretCopy, "githubAppID", repoCreds.GithubAppId)
	updateSecretInt(secretCopy, "githubAppInstallationID", repoCreds.GithubAppInstallationId)
	updateSecretString(secretCopy, "githubAppEnterpriseBaseUrl", repoCreds.GitHubAppEnterpriseBaseURL)
	updateSecretString(secretCopy, "gcpServiceAccountKey", repoCreds.GCPServiceAccountKey)
	updateSecretString(secretCopy, "proxy", repoCreds.Proxy)
	updateSecretString(secretCopy, "noProxy", repoCreds.NoProxy)
	updateSecretBool(secretCopy, "forceHttpBasicAuth", repoCreds.ForceHttpBasicAuth)
	updateSecretBool(secretCopy, "useAzureWorkloadIdentity", repoCreds.UseAzureWorkloadIdentity)
	addSecretMetadata(secretCopy, s.getRepoCredSecretType())

	return secretCopy
}

func (s *secretsRepositoryBackend) getRepositorySecret(repoURL, project string, allowFallback bool) (*corev1.Secret, error) {
	secrets, err := s.db.listSecretsByType(s.getSecretType())
	if err != nil {
		return nil, fmt.Errorf("failed to list repository secrets: %w", err)
	}

	var foundSecret *corev1.Secret
	for _, secret := range secrets {
		if git.SameURL(string(secret.Data["url"]), repoURL) {
			projectSecret := string(secret.Data["project"])
			if project == projectSecret {
				if foundSecret != nil {
					log.Warnf("Found multiple credentials for repoURL: %s", repoURL)
				}

				return secret, nil
			}

			if projectSecret == "" && allowFallback {
				if foundSecret != nil {
					log.Warnf("Found multiple credentials for repoURL: %s", repoURL)
				}

				foundSecret = secret
			}
		}
	}

	if foundSecret != nil {
		return foundSecret, nil
	}

	return nil, status.Errorf(codes.NotFound, "repository %q not found", repoURL)
}

func (s *secretsRepositoryBackend) getRepoCredsSecret(repoURL string) (*corev1.Secret, error) {
	secrets, err := s.db.listSecretsByType(s.getRepoCredSecretType())
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
	maxLen, idx := 0, -1
	repoURL = git.NormalizeGitURL(repoURL)
	for i, cred := range repoCredentials {
		credURL := git.NormalizeGitURL(string(cred.Data["url"]))
		if strings.HasPrefix(repoURL, credURL) {
			if len(credURL) == maxLen {
				log.Warnf("Found multiple credentials for repoURL: %s", repoURL)
			}
			if len(credURL) > maxLen {
				maxLen = len(credURL)
				idx = i
			}
		}
	}
	return idx
}

func (s *secretsRepositoryBackend) getSecretType() string {
	if s.writeCreds {
		return common.LabelValueSecretTypeRepositoryWrite
	}
	return common.LabelValueSecretTypeRepository
}

func (s *secretsRepositoryBackend) getRepoCredSecretType() string {
	if s.writeCreds {
		return common.LabelValueSecretTypeRepoCredsWrite
	}
	return common.LabelValueSecretTypeRepoCreds
}
