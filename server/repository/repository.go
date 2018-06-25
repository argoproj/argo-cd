package repository

import (
	"fmt"
	"reflect"

	"github.com/ghodss/yaml"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/rbac"
)

// Server provides a Repository service
type Server struct {
	db            db.ArgoDB
	repoClientset reposerver.Clientset
	enf           *rbac.Enforcer
}

// NewServer returns a new instance of the Repository service
func NewServer(
	repoClientset reposerver.Clientset,
	db db.ArgoDB,
	enf *rbac.Enforcer,
) *Server {
	return &Server{
		db:            db,
		repoClientset: repoClientset,
		enf:           enf,
	}
}

// repoRBACName formats fully qualified repository name for RBAC check
func repoRBACName(repo *appsv1.Repository) string {
	return fmt.Sprintf("*/%s", repo.Repo)
}

// List returns list of repositories
func (s *Server) List(ctx context.Context, q *RepoQuery) (*appsv1.RepositoryList, error) {
	repoList, err := s.db.ListRepositories(ctx)
	if repoList != nil {
		newItems := make([]appsv1.Repository, 0)
		for _, repo := range repoList.Items {
			if s.enf.EnforceClaims(ctx.Value("claims"), "repositories", "get", fmt.Sprintf("*/%s", repo.Repo)) {
				newItems = append(newItems, *redact(&repo))
			}
		}
		repoList.Items = newItems
	}
	return repoList, err
}

// ListKsonnetApps returns list of Ksonnet apps in the repo
func (s *Server) ListKsonnetApps(ctx context.Context, q *RepoKsonnetQuery) (*RepoKsonnetResponse, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "repositories/apps", "get", fmt.Sprintf("*/%s", q.Repo)) {
		return nil, grpc.ErrPermissionDenied
	}
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
		appSpec.Path = path
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
	if !s.enf.EnforceClaims(ctx.Value("claims"), "repositories", "create", repoRBACName(q.Repo)) {
		return nil, grpc.ErrPermissionDenied
	}
	r := q.Repo
	repo, err := s.db.CreateRepository(ctx, r)
	if status.Convert(err).Code() == codes.AlreadyExists {
		// act idempotent if existing spec matches new spec
		existing, getErr := s.db.GetRepository(ctx, r.Repo)
		if getErr != nil {
			return nil, status.Errorf(codes.Internal, "unable to check existing repository details: %v", err)
		}
		if q.Upsert {
			if !s.enf.EnforceClaims(ctx.Value("claims"), "repositories", "update", repoRBACName(r)) {
				return nil, grpc.ErrPermissionDenied
			}
			repo, err = s.db.UpdateRepository(ctx, r)
		} else {
			// repository ConnectionState may differ, so make consistent before testing
			r.ConnectionState = existing.ConnectionState
			if reflect.DeepEqual(existing, r) {
				repo, err = existing, nil
			} else {
				return nil, status.Errorf(codes.InvalidArgument, "existing repository spec is different; use upsert flag to force update")
			}
		}

	}
	return redact(repo), err
}

// Get returns a repository by URL
func (s *Server) Get(ctx context.Context, q *RepoQuery) (*appsv1.Repository, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "repositories", "get", fmt.Sprintf("*/%s", q.Repo)) {
		return nil, grpc.ErrPermissionDenied
	}
	repo, err := s.db.GetRepository(ctx, q.Repo)
	return redact(repo), err
}

// Update updates a repository
func (s *Server) Update(ctx context.Context, q *RepoUpdateRequest) (*appsv1.Repository, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "repositories", "update", fmt.Sprintf("*/%s", q.Repo.Repo)) {
		return nil, grpc.ErrPermissionDenied
	}
	repo, err := s.db.UpdateRepository(ctx, q.Repo)
	return redact(repo), err
}

// Delete updates a repository
func (s *Server) Delete(ctx context.Context, q *RepoQuery) (*RepoResponse, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "repositories", "delete", fmt.Sprintf("*/%s", q.Repo)) {
		return nil, grpc.ErrPermissionDenied
	}
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
