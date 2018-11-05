package db

import (
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/argoproj/argo-cd/common"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/git"
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

// ListRepoURLs returns list of repositories
func (db *db) ListRepoURLs(ctx context.Context) ([]string, error) {
	s, err := db.settingsMgr.GetSettings()
	if err != nil {
		return nil, err
	}

	urls := make([]string, len(s.Repositories))
	for i := range s.Repositories {
		urls[i] = s.Repositories[i].URL
	}
	return urls, nil
}

// CreateRepository creates a repository
func (db *db) CreateRepository(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error) {
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

	repoInfo := settings.RepoCredentials{URL: r.Repo}
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
	repo := &appsv1.Repository{Repo: repoURL}

	cache := make(map[string]*apiv1.Secret)
	getSecret := func(secretName string) (*apiv1.Secret, error) {
		if _, ok := cache[secretName]; !ok {
			secret, err := db.kubeclientset.CoreV1().Secrets(db.ns).Get(secretName, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
			cache[secretName] = secret
		}
		return cache[secretName], nil
	}

	if repoInfo.UsernameSecret != nil {
		secret, err := getSecret(repoInfo.UsernameSecret.Name)
		if err != nil {
			return nil, err
		}
		repo.Username = string(secret.Data[repoInfo.UsernameSecret.Key])
	}
	if repoInfo.PasswordSecret != nil {
		secret, err := getSecret(repoInfo.PasswordSecret.Name)
		if err != nil {
			return nil, err
		}
		repo.Password = string(secret.Data[repoInfo.PasswordSecret.Key])
	}
	if repoInfo.SshPrivateKeySecret != nil {
		secret, err := getSecret(repoInfo.SshPrivateKeySecret.Name)
		if err != nil {
			return nil, err
		}
		repo.SSHPrivateKey = string(secret.Data[repoInfo.SshPrivateKeySecret.Key])
	}
	return repo, nil
}

// UpdateRepository updates a repository
func (db *db) UpdateRepository(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error) {
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
						common.ManagedByAnnotation: common.ManagedByArgoCDAnnotationValue,
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
			isManagedByArgo := (secret.Annotations != nil && secret.Annotations[common.ManagedByAnnotation] == common.ManagedByArgoCDAnnotationValue) ||
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
	repoURL = git.NormalizeGitURL(repoURL)
	for i := range s.Repositories {
		if git.NormalizeGitURL(s.Repositories[i].URL) == repoURL {
			return i
		}
	}
	return -1
}

// repoURLToSecretName hashes repo URL to the secret name using a formula.
// Part of the original repo name is incorporated for debugging purposes
func repoURLToSecretName(repo string) string {
	repo = strings.ToLower(git.NormalizeGitURL(repo))
	h := fnv.New32a()
	_, _ = h.Write([]byte(repo))
	parts := strings.Split(strings.TrimSuffix(repo, ".git"), "/")
	shortName := strings.Replace(parts[len(parts)-1], "_", "-", -1)
	return fmt.Sprintf("repo-%s-%v", shortName, h.Sum32())
}
