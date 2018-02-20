package repository

import (
	"github.com/argoproj/argo-cd/common"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util/git"
	"golang.org/x/net/context"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
)

// Server provides a Repository service
type Server struct {
	ns            string
	kubeclientset kubernetes.Interface
	appclientset  appclientset.Interface
}

// NewServer returns a new instance of the Repository service
func NewServer(namespace string, kubeclientset kubernetes.Interface, appclientset appclientset.Interface) *Server {
	return &Server{
		ns:            namespace,
		appclientset:  appclientset,
		kubeclientset: kubeclientset,
	}
}

// List returns list of repositories
func (s *Server) List(ctx context.Context, q *RepoQuery) (*appsv1.RespositoryList, error) {
	listOpts := metav1.ListOptions{}
	labelSelector := labels.NewSelector()
	req, _ := labels.NewRequirement(common.LabelKeyRepo, selection.Exists, nil)
	labelSelector = labelSelector.Add(*req)
	listOpts.LabelSelector = labelSelector.String()
	repoSecrets, err := s.kubeclientset.CoreV1().Secrets(s.ns).List(listOpts)
	if err != nil {
		return nil, err
	}
	repoList := appsv1.RespositoryList{
		Items: make([]appsv1.Respository, len(repoSecrets.Items)),
	}
	for i, repoSec := range repoSecrets.Items {
		repoList.Items[i] = *secretToRepo(&repoSec)
	}
	return &repoList, nil
}

// Create creates a repository
func (s *Server) Create(ctx context.Context, r *appsv1.Respository) (*appsv1.Respository, error) {
	repoURL := git.NormalizeGitURL(r.Repo)
	cmName := repoURLToSecretName(repoURL)
	repoSecret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: cmName,
			Labels: map[string]string{
				common.LabelKeyRepo: repoURL,
			},
		},
	}
	repoSecret.StringData = make(map[string]string)
	repoSecret.StringData["username"] = r.Username
	repoSecret.StringData["password"] = r.Password
	repoSecret, err := s.kubeclientset.CoreV1().Secrets(s.ns).Create(repoSecret)
	if err != nil {
		return nil, err
	}
	return secretToRepo(repoSecret), nil
}

// Get returns a repository by URL
func (s *Server) Get(ctx context.Context, q *RepoQuery) (*appsv1.Respository, error) {
	cmName := repoURLToSecretName(q.Repo)
	repoSecret, err := s.kubeclientset.CoreV1().Secrets(s.ns).Get(cmName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return secretToRepo(repoSecret), nil
}

// Update updates a repository
func (s *Server) Update(ctx context.Context, r *appsv1.Respository) (*appsv1.Respository, error) {
	cmName := repoURLToSecretName(r.Repo)
	repoSecret, err := s.kubeclientset.CoreV1().Secrets(s.ns).Get(cmName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	repoSecret.StringData = make(map[string]string)
	repoSecret.StringData["username"] = r.Username
	repoSecret.StringData["password"] = r.Password
	repoSecret, err = s.kubeclientset.CoreV1().Secrets(s.ns).Update(repoSecret)
	if err != nil {
		return nil, err
	}
	return secretToRepo(repoSecret), nil
}

// UpdateREST updates a repository (from a REST request)
func (s *Server) UpdateREST(ctx context.Context, r *RepoUpdateRequest) (*appsv1.Respository, error) {
	return s.Update(ctx, r.Repo)
}

// Delete updates a repository
func (s *Server) Delete(ctx context.Context, q *RepoQuery) (*RepoResponse, error) {
	cmName := repoURLToSecretName(q.Repo)
	err := s.kubeclientset.CoreV1().Secrets(s.ns).Delete(cmName, &metav1.DeleteOptions{})
	return &RepoResponse{}, err

}

// repoURLToSecretName converts a repo
func repoURLToSecretName(repo string) string {
	repoURL := git.NormalizeGitURL(repo)
	return repoURL
}

// secretToRepo converts a secret into a repository object
func secretToRepo(s *apiv1.Secret) *appsv1.Respository {
	repo := appsv1.Respository{
		Repo:     s.ObjectMeta.Labels[common.LabelKeyRepo],
		Username: string(s.Data["username"]),
		Password: string(s.Data["password"]),
	}
	return &repo
}
