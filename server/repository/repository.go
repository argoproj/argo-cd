package repository

import (
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/ghodss/yaml"
	"golang.org/x/net/context"
)

// Server provides a Repository service
type Server struct {
	db            db.ArgoDB
	repoClientset reposerver.Clientset
}

// NewServer returns a new instance of the Repository service
func NewServer(
	repoClientset reposerver.Clientset,
	db db.ArgoDB,
) *Server {
	return &Server{
		db:            db,
		repoClientset: repoClientset,
	}
}

// List returns list of repositories
func (s *Server) List(ctx context.Context, q *RepoQuery) (*appsv1.RepositoryList, error) {
	repoList, err := s.db.ListRepositories(ctx)
	if repoList != nil {
		for i, repo := range repoList.Items {
			repoList.Items[i] = *redact(&repo)
		}
	}
	return repoList, err
}

// ListKsonnetApps returns list of Ksonnet apps in the repo
func (s *Server) ListKsonnetApps(ctx context.Context, q *RepoKsonnetQuery) (*RepoKsonnetResponse, error) {
	repo, err := s.db.GetRepository(ctx, q.Repo)
	if err != nil {
		return nil, err
	}

	// Test the repo
	conn, repoClient, err := s.repoClientset.NewRepositoryClient()
	if err != nil {
		return nil, err
	}
	defer util.Close(conn)

	revision := q.Revision
	if revision == "" {
		revision = "HEAD"
	}

	// Verify app.yaml is functional
	req := repository.ListDirRequest{
		Repo:     repo,
		Revision: revision,
		Path:     "*app.yaml",
	}
	getRes, err := repoClient.ListDir(ctx, &req)
	if err != nil {
		return nil, err
	}

	out := make([]*KsonnetAppSpec, 0)
	for _, path := range getRes.Items {
		getFileRes, err := repoClient.GetFile(ctx, &repository.GetFileRequest{
			Repo:     repo,
			Revision: revision,
			Path:     path,
		})
		if err != nil {
			return nil, err
		}

		var appSpec KsonnetAppSpec
		err = yaml.Unmarshal(getFileRes.Data, &appSpec)
		if err == nil && appSpec.Name != "" && len(appSpec.Environments) > 0 {
			out = append(out, &appSpec)
		}
	}

	return &RepoKsonnetResponse{
		Items: out,
	}, nil
}

// Create creates a repository
func (s *Server) Create(ctx context.Context, q *RepoCreateRequest) (*appsv1.Repository, error) {
	repo, err := s.db.CreateRepository(ctx, q.Repo)
	return redact(repo), err
}

// Get returns a repository by URL
func (s *Server) Get(ctx context.Context, q *RepoQuery) (*appsv1.Repository, error) {
	repo, err := s.db.GetRepository(ctx, q.Repo)
	return redact(repo), err
}

// Update updates a repository
func (s *Server) Update(ctx context.Context, q *RepoUpdateRequest) (*appsv1.Repository, error) {
	repo, err := s.db.UpdateRepository(ctx, q.Repo)
	return redact(repo), err
}

// Delete updates a repository
func (s *Server) Delete(ctx context.Context, q *RepoQuery) (*RepoResponse, error) {
	err := s.db.DeleteRepository(ctx, q.Repo)
	return &RepoResponse{}, err
}

func redact(repo *appsv1.Repository) *appsv1.Repository {
	if repo == nil {
		return nil
	}
	repo.Password = ""
	repo.SSHPrivateKey = ""
	return repo
}
