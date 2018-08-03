package repository

import (
	"path"
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
	"github.com/argoproj/argo-cd/util/git"
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

// List returns list of repositories
func (s *Server) List(ctx context.Context, q *RepoQuery) (*appsv1.RepositoryList, error) {
	repoList, err := s.db.ListRepositories(ctx)
	if repoList != nil {
		newItems := make([]appsv1.Repository, 0)
		for _, repo := range repoList.Items {
			if s.enf.EnforceClaims(ctx.Value("claims"), "repositories", "get", repo.Repo) {
				newItems = append(newItems, *redact(&repo))
			}
		}
		repoList.Items = newItems
	}
	return repoList, err
}

// ListKsonnetApps returns list of Ksonnet apps in the repo
func (s *Server) ListApps(ctx context.Context, q *RepoAppsQuery) (*RepoAppsResponse, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "repositories/apps", "get", q.Repo) {
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

	ksonnetRes, err := repoClient.ListDir(ctx, &repository.ListDirRequest{Repo: repo, Revision: revision, Path: "*app.yaml"})
	if err != nil {
		return nil, err
	}

	helmRes, err := repoClient.ListDir(ctx, &repository.ListDirRequest{Repo: repo, Revision: revision, Path: "*Chart.yaml"})
	if err != nil {
		return nil, err
	}

	items := make([]*AppInfo, 0)

	for i := range ksonnetRes.Items {
		items = append(items, &AppInfo{Type: string(repository.AppSourceKsonnet), Path: ksonnetRes.Items[i]})
	}

	for i := range helmRes.Items {
		items = append(items, &AppInfo{Type: string(repository.AppSourceHelm), Path: helmRes.Items[i]})
	}

	return &RepoAppsResponse{Items: items}, nil
}

func (s *Server) GetAppDetails(ctx context.Context, q *RepoAppDetailsQuery) (*RepoAppDetailsResponse, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "repositories/apps", "get", q.Repo) {
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

	appSpecRes, err := repoClient.GetFile(ctx, &repository.GetFileRequest{
		Repo:     repo,
		Revision: revision,
		Path:     q.Path,
	})
	if err != nil {
		return nil, err
	}

	appSourceType := repository.IdentifyAppSourceTypeByAppPath(q.Path)
	switch appSourceType {
	case repository.AppSourceKsonnet:
		var appSpec KsonnetAppSpec
		appSpec.Path = q.Path
		err = yaml.Unmarshal(appSpecRes.Data, &appSpec)
		if err != nil {
			return nil, err
		}
		return &RepoAppDetailsResponse{
			Type:    string(appSourceType),
			Ksonnet: &appSpec,
		}, nil
	case repository.AppSourceHelm:
		var appSpec HelmAppSpec
		appSpec.Path = q.Path
		err = yaml.Unmarshal(appSpecRes.Data, &appSpec)
		if err != nil {
			return nil, err
		}
		valuesFilesRes, err := repoClient.ListDir(ctx, &repository.ListDirRequest{
			Revision: revision,
			Repo:     repo,
			Path:     path.Join(path.Dir(q.Path), "*values*.yaml"),
		})
		if err != nil {
			return nil, err
		}
		appSpec.ValueFiles = valuesFilesRes.Items
		return &RepoAppDetailsResponse{
			Type: string(appSourceType),
			Helm: &appSpec,
		}, nil

	}

	return nil, status.Errorf(codes.InvalidArgument, "specified application path is not supported")
}

// Create creates a repository
func (s *Server) Create(ctx context.Context, q *RepoCreateRequest) (*appsv1.Repository, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "repositories", "create", q.Repo.Repo) {
		return nil, grpc.ErrPermissionDenied
	}
	r := q.Repo
	err := git.TestRepo(git.NormalizeGitURL(r.Repo), r.Username, r.Password, r.SSHPrivateKey)
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
	}
	return redact(repo), err
}

// Get returns a repository by URL
func (s *Server) Get(ctx context.Context, q *RepoQuery) (*appsv1.Repository, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "repositories", "get", q.Repo) {
		return nil, grpc.ErrPermissionDenied
	}
	repo, err := s.db.GetRepository(ctx, q.Repo)
	return redact(repo), err
}

// Update updates a repository
func (s *Server) Update(ctx context.Context, q *RepoUpdateRequest) (*appsv1.Repository, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "repositories", "update", q.Repo.Repo) {
		return nil, grpc.ErrPermissionDenied
	}
	repo, err := s.db.UpdateRepository(ctx, q.Repo)
	return redact(repo), err
}

// Delete updates a repository
func (s *Server) Delete(ctx context.Context, q *RepoQuery) (*RepoResponse, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "repositories", "delete", q.Repo) {
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
