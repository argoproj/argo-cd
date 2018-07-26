package db

import (
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/argoproj/argo-cd/common"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/git"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

// ListRepositories returns list of repositories
func (s *db) ListRepositories(ctx context.Context) (*appsv1.RepositoryList, error) {
	listOpts := metav1.ListOptions{}
	labelSelector := labels.NewSelector()
	req, err := labels.NewRequirement(common.LabelKeySecretType, selection.Equals, []string{common.SecretTypeRepository})
	if err != nil {
		return nil, err
	}
	labelSelector = labelSelector.Add(*req)
	listOpts.LabelSelector = labelSelector.String()
	repoSecrets, err := s.kubeclientset.CoreV1().Secrets(s.ns).List(listOpts)
	if err != nil {
		return nil, err
	}
	repoList := appsv1.RepositoryList{
		Items: make([]appsv1.Repository, len(repoSecrets.Items)),
	}
	for i, repoSec := range repoSecrets.Items {
		repoList.Items[i] = *SecretToRepo(&repoSec)
	}
	return &repoList, nil
}

// CreateRepository creates a repository
func (s *db) CreateRepository(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error) {
	shallowCopy := *r
	r = &shallowCopy
	r.Repo = git.NormalizeGitURL(r.Repo)
	r.Username = strings.TrimSpace(r.Username)
	secName := repoURLToSecretName(r.Repo)
	repoSecret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secName,
			Labels: map[string]string{
				common.LabelKeySecretType: common.SecretTypeRepository,
			},
		},
	}
	repoSecret.Data = repoToData(r)
	repoSecret.Annotations = AnnotationsFromConnectionState(&r.ConnectionState)
	repoSecret, err := s.kubeclientset.CoreV1().Secrets(s.ns).Create(repoSecret)
	if err != nil {
		if apierr.IsAlreadyExists(err) {
			return nil, status.Errorf(codes.AlreadyExists, "repository '%s' already exists", r.Repo)
		}
		return nil, err
	}
	return SecretToRepo(repoSecret), nil
}

// GetRepository returns a repository by URL
func (s *db) GetRepository(ctx context.Context, name string) (*appsv1.Repository, error) {
	repoSecret, err := s.getRepoSecret(name)
	if err != nil {
		return nil, err
	}
	return SecretToRepo(repoSecret), nil
}

// UpdateRepository updates a repository
func (s *db) UpdateRepository(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error) {
	err := git.TestRepo(r.Repo, r.Username, r.Password, r.SSHPrivateKey)
	if err != nil {
		return nil, err
	}
	repoSecret, err := s.getRepoSecret(r.Repo)
	if err != nil {
		return nil, err
	}
	repoSecret.Data = repoToData(r)
	repoSecret, err = s.kubeclientset.CoreV1().Secrets(s.ns).Update(repoSecret)
	if err != nil {
		return nil, err
	}
	return SecretToRepo(repoSecret), nil
}

// Delete updates a repository
func (s *db) DeleteRepository(ctx context.Context, name string) error {
	secName := repoURLToSecretName(name)
	return s.kubeclientset.CoreV1().Secrets(s.ns).Delete(secName, &metav1.DeleteOptions{})
}

func (s *db) getRepoSecret(repo string) (*apiv1.Secret, error) {
	secName := repoURLToSecretName(repo)
	repoSecret, err := s.kubeclientset.CoreV1().Secrets(s.ns).Get(secName, metav1.GetOptions{})
	if err != nil {
		if apierr.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "repo '%s' not found", repo)
		}
		return nil, err
	}
	return repoSecret, nil
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

// repoToData converts a repository object to secret data for serialization to a secret
func repoToData(r *appsv1.Repository) map[string][]byte {
	return map[string][]byte{
		"repository":    []byte(r.Repo),
		"username":      []byte(r.Username),
		"password":      []byte(r.Password),
		"sshPrivateKey": []byte(r.SSHPrivateKey),
	}
}

// SecretToRepo converts a secret into a repository object, optionally redacting sensitive information
func SecretToRepo(s *apiv1.Secret) *appsv1.Repository {
	repo := appsv1.Repository{
		Repo:            string(s.Data["repository"]),
		Username:        string(s.Data["username"]),
		Password:        string(s.Data["password"]),
		SSHPrivateKey:   string(s.Data["sshPrivateKey"]),
		ConnectionState: ConnectionStateFromAnnotations(s.Annotations),
	}
	return &repo
}
