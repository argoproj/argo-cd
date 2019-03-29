package repository

import (
	"fmt"
	"reflect"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/helm"
	"github.com/argoproj/argo-cd/util/rbac"
	"github.com/argoproj/argo-cd/util/repos"
)

// Server provides a Repository service
type Server struct {
	db            db.ArgoDB
	repoClientset reposerver.Clientset
	enf           *rbac.Enforcer
	cache         *cache.Cache
}

// NewServer returns a new instance of the Repository service
func NewServer(
	repoClientset reposerver.Clientset,
	db db.ArgoDB,
	enf *rbac.Enforcer,
	cache *cache.Cache,
) *Server {
	return &Server{
		db:            db,
		repoClientset: repoClientset,
		enf:           enf,
		cache:         cache,
	}
}

func (s *Server) populateConnectionState(ctx context.Context, repo *appsv1.Repository) {
	connectionState, err := s.cache.GetRepoConnectionState(repo.Repo)
	if err == nil {
		repo.ConnectionState = connectionState
		return
	}
	now := metav1.Now()

	switch f := repos.GetFactory(repo.Type).(type) {
	case git.RepoFactory:
		_, err = f.GetRepo(repo.Repo, repo.Username, repo.Password, repo.SSHPrivateKey, repo.InsecureIgnoreHostKey)
	case helm.RepoFactory:
		_, err = f.GetRepo(repo.Repo, repo.Name, repo.Username, repo.Password, repo.CAData, repo.CertData, repo.KeyData)
	}

	if err != nil {
		repo.ConnectionState = appsv1.ConnectionState{
			Status:     appsv1.ConnectionStatusFailed,
			Message:    fmt.Sprintf("Unable to connect to repository: %v", err),
			ModifiedAt: &now,
		}
		return
	}
	err = s.cache.SetRepoConnectionState(repo.Repo, &connectionState)
	if err != nil {
		log.Warnf("cache set error %s: %v", repo.Repo, err)
	}

	repo.ConnectionState = appsv1.ConnectionState{
		Status:     appsv1.ConnectionStatusSuccessful,
		ModifiedAt: &now,
	}
}

// List returns list of repositories
func (s *Server) List(ctx context.Context, q *RepoQuery) (*appsv1.RepositoryList, error) {
	repositories, err := s.db.ListRepositories(ctx)
	if err != nil {
		return nil, err
	}

	repositories = s.filterAllowed(ctx, repositories)

	// do this after filtering to reduce cost
	err = util.RunAllAsync(len(repositories), func(i int) error {
		s.populateConnectionState(ctx, repositories[i])
		return nil
	})

	if err != nil {
		return nil, err
	}

	items := redact(repositories)

	return &appsv1.RepositoryList{Items: items}, nil
}

func (s *Server) filterAllowed(ctx context.Context, repos []*appsv1.Repository) []*appsv1.Repository {
	filtered := make([]*appsv1.Repository, 0)
	for _, repo := range repos {
		if s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionGet, repo) {
			filtered = append(filtered, repo)
		}
	}
	return filtered
}

func redact(repos []*appsv1.Repository) []appsv1.Repository {
	items := make([]appsv1.Repository, len(repos))
	for i, repo := range repos {
		items[i] = appsv1.Repository{Repo: repo.Repo, Type: repo.Type, Name: repo.Name, ConnectionState: repo.ConnectionState}
	}
	return items
}

func (s *Server) listAppsPaths(
	ctx context.Context, repoClient repository.RepoServerServiceClient, repo *appsv1.Repository, revision string) (map[string]appsv1.ApplicationSourceType, error) {

	res, err := repoClient.ListApps(ctx, &repository.ListAppsRequest{Repo: repo, Revision: revision})
	if err != nil {
		return nil, err
	}

	output := make(map[string]appsv1.ApplicationSourceType)
	for path := range res.Apps {
		output[path] = appsv1.ApplicationSourceType(res.Apps[path])
	}

	return output, nil
}

// ListApps returns list of apps in the repo
func (s *Server) ListApps(ctx context.Context, q *RepoAppsQuery) (*RepoAppsResponse, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionGet, q.Repo); err != nil {
		return nil, err
	}
	repo, err := s.db.GetRepository(ctx, q.Repo)
	if err != nil {
		if errStatus, ok := status.FromError(err); ok && errStatus.Code() == codes.NotFound {
			repo = &appsv1.Repository{
				Repo: q.Repo,
			}
		} else {
			return nil, err
		}
	}

	// Test the repo
	conn, repoClient, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, err
	}
	defer util.Close(conn)

	revision := q.Revision

	paths, err := s.listAppsPaths(ctx, repoClient, repo, revision)
	if err != nil {
		return nil, err
	}
	items := make([]*AppInfo, 0)
	for appPath, appType := range paths {
		items = append(items, &AppInfo{Path: appPath, Type: string(appType)})
	}
	return &RepoAppsResponse{Items: items}, nil
}

func (s *Server) GetAppDetails(ctx context.Context, q *RepoAppDetailsQuery) (*repository.RepoAppDetailsResponse, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionGet, q.Repo); err != nil {
		return nil, err
	}
	repo, err := s.db.GetRepository(ctx, q.Repo)
	if err != nil {
		if errStatus, ok := status.FromError(err); ok && errStatus.Code() == codes.NotFound {
			repo = &appsv1.Repository{
				Repo: q.Repo,
			}
		} else {
			return nil, err
		}
	}
	conn, repoClient, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, err
	}
	defer util.Close(conn)
	repositories, err := s.db.ListRepositories(ctx)
	if err != nil {
		return nil, err
	}
	return repoClient.GetAppDetails(ctx, &repository.RepoServerAppDetailsQuery{
		Repo:     repo,
		Revision: q.Revision,
		Path:     q.Path,
		Repos:    repositories,
		Helm:     q.Helm,
	})
}

// Create creates a repository
func (s *Server) Create(ctx context.Context, q *RepoCreateRequest) (*appsv1.Repository, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionCreate, q.Repo.Repo); err != nil {
		return nil, err
	}
	r := q.Repo

	var err error
	switch f := repos.GetFactory(r.Type).(type) {
	case git.RepoFactory:
		_, err = f.GetRepo(r.Repo, r.Username, r.Password, r.SSHPrivateKey, r.InsecureIgnoreHostKey)
	case helm.RepoFactory:
		_, err = f.GetRepo(r.Repo, r.Name, r.Username, r.Password, r.CAData, r.CertData, r.KeyData)
	}
	if err != nil {
		return nil, err
	}

	r.ConnectionState = appsv1.ConnectionState{Status: appsv1.ConnectionStatusSuccessful}
	repo, err := s.db.CreateRepository(ctx, r)
	if status.Convert(err).Code() == codes.AlreadyExists {
		// act idempotent if existing spec matches new spec
		existing, getErr := s.db.GetRepository(ctx, r.Repo)
		if getErr != nil {
			return nil, status.Errorf(codes.Internal, "unable to check existing repository details: %v", getErr)
		}

		// repository ConnectionState may differ, so make consistent before testing
		existing.ConnectionState = r.ConnectionState
		if reflect.DeepEqual(existing, r) {
			repo, err = existing, nil
		} else if q.Upsert {
			return s.Update(ctx, &RepoUpdateRequest{Repo: r})
		} else {
			return nil, status.Errorf(codes.InvalidArgument, "existing repository spec is different; use upsert flag to force update")
		}
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create %s", err)
	}
	return &appsv1.Repository{Repo: repo.Repo, Type: repo.Type, Name: repo.Name}, err
}

// Update updates a repository
func (s *Server) Update(ctx context.Context, q *RepoUpdateRequest) (*appsv1.Repository, error) {
	// TODO - allow update of type?
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionUpdate, q.Repo.Repo); err != nil {
		return nil, err
	}
	_, err := s.db.UpdateRepository(ctx, q.Repo)
	return &appsv1.Repository{Repo: q.Repo.Repo}, err
}

// Delete updates a repository
func (s *Server) Delete(ctx context.Context, q *RepoQuery) (*RepoResponse, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionDelete, q.Repo); err != nil {
		return nil, err
	}

	// invalidate cache
	if err := s.cache.SetRepoConnectionState(q.Repo, nil); err == nil {
		log.Errorf("error invalidating cache: %v", err)
	}

	err := s.db.DeleteRepository(ctx, q.Repo)
	return &RepoResponse{}, err
}
