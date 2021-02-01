package repocreds

import (
	"reflect"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	repocredspkg "github.com/argoproj/argo-cd/pkg/apiclient/repocreds"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/rbac"
	"github.com/argoproj/argo-cd/util/settings"
)

// Server provides a Repository service
type Server struct {
	db            db.ArgoDB
	repoClientset apiclient.Clientset
	enf           *rbac.Enforcer
	settings      *settings.SettingsManager
}

// NewServer returns a new instance of the Repository service
func NewServer(
	repoClientset apiclient.Clientset,
	db db.ArgoDB,
	enf *rbac.Enforcer,
	settings *settings.SettingsManager,
) *Server {
	return &Server{
		db:            db,
		repoClientset: repoClientset,
		enf:           enf,
		settings:      settings,
	}
}

// ListRepositoryCredentials returns a list of all configured repository credential sets
func (s *Server) ListRepositoryCredentials(ctx context.Context, q *repocredspkg.RepoCredsQuery) (*appsv1.RepoCredsList, error) {
	urls, err := s.db.ListRepositoryCredentials(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]appsv1.RepoCreds, 0)
	for _, url := range urls {
		if s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionGet, url) {
			repo, err := s.db.GetRepositoryCredentials(ctx, url)
			if err != nil {
				return nil, err
			}
			if repo != nil {
				items = append(items, appsv1.RepoCreds{
					URL:      url,
					Username: repo.Username,
				})
			}
		}
	}
	return &appsv1.RepoCredsList{Items: items}, nil
}

// CreateRepositoryCredentials creates a new credential set in the configuration
func (s *Server) CreateRepositoryCredentials(ctx context.Context, q *repocredspkg.RepoCredsCreateRequest) (*appsv1.RepoCreds, error) {
	if q.Creds == nil {
		return nil, status.Errorf(codes.InvalidArgument, "missing payload in request")
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionCreate, q.Creds.URL); err != nil {
		return nil, err
	}

	r := q.Creds

	if r.URL == "" {
		return nil, status.Errorf(codes.InvalidArgument, "must specify URL")
	}

	_, err := s.db.CreateRepositoryCredentials(ctx, r)
	if status.Convert(err).Code() == codes.AlreadyExists {
		// act idempotent if existing spec matches new spec
		existing, getErr := s.db.GetRepositoryCredentials(ctx, r.URL)
		if getErr != nil {
			return nil, status.Errorf(codes.Internal, "unable to check existing repository credentials details: %v", getErr)
		}

		if reflect.DeepEqual(existing, r) {
			err = nil
		} else if q.Upsert {
			return s.UpdateRepositoryCredentials(ctx, &repocredspkg.RepoCredsUpdateRequest{Creds: r})
		} else {
			return nil, status.Errorf(codes.InvalidArgument, "existing repository credentials spec is different; use upsert flag to force update")
		}
	}
	return &appsv1.RepoCreds{URL: r.URL}, err
}

// UpdateRepositoryCredentials updates a repository credential set
func (s *Server) UpdateRepositoryCredentials(ctx context.Context, q *repocredspkg.RepoCredsUpdateRequest) (*appsv1.RepoCreds, error) {
	if q.Creds == nil {
		return nil, status.Errorf(codes.InvalidArgument, "missing payload in request")
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionUpdate, q.Creds.URL); err != nil {
		return nil, err
	}
	_, err := s.db.UpdateRepositoryCredentials(ctx, q.Creds)
	return &appsv1.RepoCreds{URL: q.Creds.URL}, err
}

// DeleteRepositoryCredentials removes a credential set from the configuration
func (s *Server) DeleteRepositoryCredentials(ctx context.Context, q *repocredspkg.RepoCredsDeleteRequest) (*repocredspkg.RepoCredsResponse, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionDelete, q.Url); err != nil {
		return nil, err
	}

	err := s.db.DeleteRepositoryCredentials(ctx, q.Url)
	return &repocredspkg.RepoCredsResponse{}, err
}
