package repository

import (
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/db"
	"golang.org/x/net/context"
)

// Server provides a Repository service
type Server struct {
	db db.ArgoDB
}

// NewServer returns a new instance of the Repository service
func NewServer(db db.ArgoDB) *Server {
	return &Server{
		db: db,
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

// Create creates a repository
func (s *Server) Create(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error) {
	repo, err := s.db.CreateRepository(ctx, r)
	return redact(repo), err
}

// Get returns a repository by URL
func (s *Server) Get(ctx context.Context, q *RepoQuery) (*appsv1.Repository, error) {
	repo, err := s.db.GetRepository(ctx, q.Repo)
	return redact(repo), err
}

// Update updates a repository
func (s *Server) Update(ctx context.Context, r *appsv1.Repository) (*appsv1.Repository, error) {
	repo, err := s.db.UpdateRepository(ctx, r)
	return redact(repo), err
}

// UpdateREST updates a repository (from a REST request)
func (s *Server) UpdateREST(ctx context.Context, r *RepoUpdateRequest) (*appsv1.Repository, error) {
	return s.Update(ctx, r.Repo)
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
