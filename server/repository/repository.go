package repository

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
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
	"github.com/argoproj/argo-cd/util/kustomize"
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

	config := repos.Config{Url: repo.Repo, RepoType: string(repo.Type), Username: repo.Username, Password: repo.Password, SshPrivateKey: repo.SSHPrivateKey, CAData: repo.CAData, CertData: repo.CertData, KeyData: repo.KeyData}
	err = repos.TestRepo(config)

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
	repos, err := s.db.ListRepositories(ctx)
	if err != nil {
		return nil, err
	}

	repos = s.filterAllowed(ctx, repos)

	// do this after filtering to reduce cost
	err = util.RunAllAsync(len(repos), func(i int) error {
		s.populateConnectionState(ctx, repos[i])
		return nil
	})

	if err != nil {
		return nil, err
	}

	items := redact(repos)

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
	ctx context.Context, repoClient repository.RepoServerServiceClient, repo *appsv1.Repository, revision string, subPath string) (map[string]appsv1.ApplicationSourceType, error) {

	if revision == "" {
		revision = "HEAD"
	}

	ksonnetRes, err := repoClient.ListDir(ctx, &repository.ListDirRequest{Repo: repo, Revision: revision, Path: path.Join(subPath, "*app.yaml")})
	if err != nil {
		return nil, err
	}
	componentRes, err := repoClient.ListDir(ctx, &repository.ListDirRequest{Repo: repo, Revision: revision, Path: path.Join(subPath, "*components/params.libsonnet")})
	if err != nil {
		return nil, err
	}

	helmRes, err := repoClient.ListDir(ctx, &repository.ListDirRequest{Repo: repo, Revision: revision, Path: path.Join(subPath, "*Chart.yaml")})
	if err != nil {
		return nil, err
	}

	kustomizationRes, err := getKustomizationRes(ctx, repoClient, repo, revision, subPath)
	if err != nil {
		return nil, err
	}

	componentDirs := make(map[string]interface{})
	for _, i := range componentRes.Items {
		d := filepath.Dir(filepath.Dir(i))
		componentDirs[d] = struct{}{}
	}

	pathToType := make(map[string]appsv1.ApplicationSourceType)
	for _, i := range ksonnetRes.Items {
		d := filepath.Dir(i)
		if _, ok := componentDirs[d]; ok {
			pathToType[i] = appsv1.ApplicationSourceTypeKsonnet
		}
	}

	for i := range helmRes.Items {
		pathToType[helmRes.Items[i]] = appsv1.ApplicationSourceTypeHelm
	}

	for i := range kustomizationRes.Items {
		pathToType[kustomizationRes.Items[i]] = appsv1.ApplicationSourceTypeKustomize
	}
	return pathToType, nil
}

func getKustomizationRes(ctx context.Context, repoClient repository.RepoServerServiceClient, repo *appsv1.Repository, revision string, subPath string) (*repository.FileList, error) {
	for _, kustomization := range kustomize.KustomizationNames {
		request := repository.ListDirRequest{Repo: repo, Revision: revision, Path: path.Join(subPath, "*"+kustomization)}
		kustomizationRes, err := repoClient.ListDir(ctx, &request)
		if err != nil {
			return nil, err
		}
		return kustomizationRes, nil
	}
	return nil, errors.New("could not find kustomization")
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
	if revision == "" {
		revision = "HEAD"
	}

	paths, err := s.listAppsPaths(ctx, repoClient, repo, revision, "")
	if err != nil {
		return nil, err
	}
	items := make([]*AppInfo, 0)
	for appFilePath, appType := range paths {
		items = append(items, &AppInfo{Path: path.Dir(appFilePath), Type: string(appType)})
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
	repos, err := s.db.ListRepositories(ctx)
	if err != nil {
		return nil, err
	}
	return repoClient.GetAppDetails(ctx, &repository.RepoServerAppDetailsQuery{
		Repo:     repo,
		Revision: q.Revision,
		Path:     q.Path,
		Repos:    repos,
		Helm:     q.Helm,
	})
}

// Create creates a repository
func (s *Server) Create(ctx context.Context, q *RepoCreateRequest) (*appsv1.Repository, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionCreate, q.Repo.Repo); err != nil {
		return nil, err
	}
	r := q.Repo
	config := repos.Config{Url: r.Repo, RepoType: string(r.Type), Name: r.Name, Username: r.Username, Password: r.Password, SshPrivateKey: r.SSHPrivateKey, CAData: r.CAData, CertData: r.CertData, KeyData: r.KeyData}
	err := repos.TestRepo(config)
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
