package db

import (
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/argoproj/argo-cd/common"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/repos"
	"github.com/argoproj/argo-cd/util/settings"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	username      = "username"
	password      = "password"
	sshPrivateKey = "sshPrivateKey"
)

// ListRepositories returns list of repositories
func (db *db) ListRepositories(ctx context.Context) ([]*appsv1.Repository, error) {
	s, err := db.settingsMgr.GetSettings()
	if err != nil {
		return nil, err
	}

	repos := make([]*appsv1.Repository, len(s.Repositories))
	for i, repo := range s.Repositories {
		repo, err := db.GetRepository(ctx, repo.URL)
		if err != nil {
			return nil, err
		}
		repos[i] = repo
	}

	return repos, nil
}

// CreateRepository creates a repository
func (db *db) CreateRepository(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error) {
	err := r.Validate()
	if err != nil {
		return nil, err
	}

	s, err := db.settingsMgr.GetSettings()
	if err != nil {
		return nil, err
	}

	index := getRepoCredIndex(s, r.Repo)
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

	repoInfo := settings.RepoCredentials{Name: r.Name, URL: r.Repo, Type: settings.RepoType(r.Type)}
	err = db.updateSecrets(&repoInfo, r)
	if err != nil {
		return nil, err
	}

	s.Repositories = append(s.Repositories, repoInfo)
	err = db.settingsMgr.SaveSettings(s)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// GetRepository returns a repository by URL
func (db *db) GetRepository(ctx context.Context, repoURL string) (*appsv1.Repository, error) {
	s, err := db.settingsMgr.GetSettings()
	if err != nil {
		return nil, err
	}

	index := getRepoCredIndex(s, repoURL)
	if index < 0 {
		return nil, status.Errorf(codes.NotFound, "repo '%s' not found", repoURL)
	}
	repoInfo := s.Repositories[index]
	repo := &appsv1.Repository{Name: repoInfo.Name, Repo: repoInfo.URL, Type: appsv1.RepoType(repoInfo.Type)}

	cache := make(map[string]*apiv1.Secret)
	err = db.unmarshalFromSecretsBytes(map[*[]byte]*apiv1.SecretKeySelector{
		&repo.CAData:   repoInfo.CASecret,
		&repo.CertData: repoInfo.CertSecret,
		&repo.KeyData:  repoInfo.KeySecret,
	}, cache)
	if err != nil {
		return nil, err
	}

	err = db.unmarshalFromSecretsStr(map[*string]*apiv1.SecretKeySelector{
		&repo.Username:      repoInfo.UsernameSecret,
		&repo.Password:      repoInfo.PasswordSecret,
		&repo.SSHPrivateKey: repoInfo.SshPrivateKeySecret,
	}, make(map[string]*apiv1.Secret))
	if err != nil {
		return nil, err
	}

	return repo, nil
}

// UpdateRepository updates a repository
func (db *db) UpdateRepository(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error) {

	err := r.Validate()
	if err != nil {
		return nil, err
	}

	s, err := db.settingsMgr.GetSettings()
	if err != nil {
		return nil, err
	}

	index := getRepoCredIndex(s, r.Repo)
	if index < 0 {
		return nil, status.Errorf(codes.NotFound, "repo '%s' not found", r.Repo)
	}

	repoInfo := s.Repositories[index]
	err = db.updateSecrets(&repoInfo, r)
	if err != nil {
		return nil, err
	}
	s.Repositories[index] = repoInfo
	err = db.settingsMgr.SaveSettings(s)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// Delete updates a repository
func (db *db) DeleteRepository(ctx context.Context, repoURL string) error {
	s, err := db.settingsMgr.GetSettings()
	if err != nil {
		return err
	}

	index := getRepoCredIndex(s, repoURL)
	if index < 0 {
		return status.Errorf(codes.NotFound, "repo '%s' not found", repoURL)
	}
	err = db.updateSecrets(&s.Repositories[index], &appsv1.Repository{
		SSHPrivateKey: "",
		Password:      "",
		Username:      "",
	})
	if err != nil {
		return err
	}
	s.Repositories = append(s.Repositories[:index], s.Repositories[index+1:]...)
	return db.settingsMgr.SaveSettings(s)
}

func (db *db) updateSecrets(repoInfo *settings.RepoCredentials, r *appsv1.Repository) error {
	secretsData := make(map[string]map[string][]byte)

	setSecretData := func(secretKey *apiv1.SecretKeySelector, value string, defaultKeyName string) *apiv1.SecretKeySelector {
		if secretKey == nil && value != "" {
			secretKey = &apiv1.SecretKeySelector{
				LocalObjectReference: apiv1.LocalObjectReference{Name: repoURLToSecretName(r.Repo)},
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

	repoInfo.UsernameSecret = setSecretData(repoInfo.UsernameSecret, r.Username, username)
	repoInfo.PasswordSecret = setSecretData(repoInfo.PasswordSecret, r.Password, password)
	repoInfo.SshPrivateKeySecret = setSecretData(repoInfo.SshPrivateKeySecret, r.SSHPrivateKey, sshPrivateKey)
	for k, v := range secretsData {
		err := db.upsertSecret(k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *db) upsertSecret(name string, data map[string][]byte) error {
	secret, err := db.kubeclientset.CoreV1().Secrets(db.ns).Get(name, metav1.GetOptions{})
	if err != nil {
		if apierr.IsNotFound(err) {
			if len(data) == 0 {
				return nil
			}
			_, err = db.kubeclientset.CoreV1().Secrets(db.ns).Create(&apiv1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Annotations: map[string]string{
						common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
					},
				},
				Data: data,
			})
			if err != nil {
				return err
			}
		}
	} else {
		for _, key := range []string{username, password, sshPrivateKey} {
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
				return db.kubeclientset.CoreV1().Secrets(db.ns).Delete(name, &metav1.DeleteOptions{})
			}
			return nil
		} else {
			_, err = db.kubeclientset.CoreV1().Secrets(db.ns).Update(secret)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func getRepoCredIndex(s *settings.ArgoCDSettings, repoURL string) int {
	for i, cred := range s.Repositories {
		if repos.SameURL(cred.URL, repoURL) {
			return i
		}
	}
	return -1
}

// repoURLToSecretName hashes repo URL to a secret name using a formula. This is used when
// repositories are _imperatively_ created and need its credentials to be stored in a secret.
// NOTE: this formula should not be considered stable and may change in future releases.
// Do NOT rely on this formula as a means of secret lookup, only secret creation.
func repoURLToSecretName(repo string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(repo))
	// Part of the original repo name is incorporated into the secret name for debugging purposes
	parts := strings.Split(strings.TrimSuffix(repo, ".git"), "/")
	shortName := strings.ToLower(strings.Replace(parts[len(parts)-1], "_", "-", -1))
	return fmt.Sprintf("repo-%s-%v", shortName, h.Sum32())
}
