package repository

import (
	"fmt"
	"reflect"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	repositorypkg "github.com/argoproj/argo-cd/pkg/apiclient/repository"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/rbac"
	"github.com/argoproj/argo-cd/util/repo/factory"
	"github.com/argoproj/argo-cd/util/repo/metrics"
	"github.com/argoproj/argo-cd/util/settings"
)

// Server provides a Repository service
type Server struct {
	db            db.ArgoDB
	repoClientset apiclient.Clientset
	enf           *rbac.Enforcer
	cache         *cache.Cache
	settings      *settings.SettingsManager
}

// NewServer returns a new instance of the Repository service
func NewServer(
	repoClientset apiclient.Clientset,
	db db.ArgoDB,
	enf *rbac.Enforcer,
	cache *cache.Cache,
	settings *settings.SettingsManager,
) *Server {
	return &Server{
		db:            db,
		repoClientset: repoClientset,
		enf:           enf,
		cache:         cache,
		settings:      settings,
	}
}

func (s *Server) getConnectionState(ctx context.Context, url string) appsv1.ConnectionState {
	if connectionState, err := s.cache.GetRepoConnectionState(url); err == nil {
		return connectionState
	}
	now := metav1.Now()
	connectionState := appsv1.ConnectionState{
		Status:     appsv1.ConnectionStatusSuccessful,
		ModifiedAt: &now,
	}
	var err error
	repo, err := s.db.GetRepository(ctx, url)
	if err == nil {
		_, err = factory.NewFactory().NewRepo(repo, metrics.NopReporter)
	}
	if err != nil {
		connectionState.Status = appsv1.ConnectionStatusFailed
		connectionState.Message = fmt.Sprintf("Unable to connect to repository: %v", err)
	}
	err = s.cache.SetRepoConnectionState(url, &connectionState)
	if err != nil {
		log.Warnf("getConnectionState cache set error %s: %v", url, err)
	}
	return connectionState
}

// List returns list of repositories
func (s *Server) List(ctx context.Context, q *repositorypkg.RepoQuery) (*appsv1.RepositoryList, error) {
	repos, err := s.db.ListRepositories(ctx)
	if err != nil {
		return nil, err
	}
	items := appsv1.Repositories{}
	for _, repo := range repos {
		if s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionGet, repo.Repo) {
			// remove secrets
			items = append(items, &appsv1.Repository{
				Repo:      repo.Repo,
				Type:      repo.Type,
				Name:      repo.Name,
				Username:  repo.Username,
				Insecure:  repo.IsInsecure(),
				EnableLFS: repo.EnableLFS,
			})
		}
	}
	err = util.RunAllAsync(len(items), func(i int) error {
		items[i].ConnectionState = s.getConnectionState(ctx, items[i].Repo)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &appsv1.RepositoryList{Items: items}, nil
}

// ListApps returns list of apps in the repo
func (s *Server) ListApps(ctx context.Context, q *repositorypkg.RepoAppsQuery) (*repositorypkg.RepoAppsResponse, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionGet, q.Repo); err != nil {
		return nil, err
	}
	repo, err := s.db.GetRepository(ctx, q.Repo)
	if err != nil {
		return nil, err
	}

	// Test the repo
	conn, repoClient, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, err
	}
	defer util.Close(conn)

	apps, err := repoClient.ListApps(ctx, &apiclient.ListAppsRequest{
		Repo:     repo,
		Revision: q.Revision,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*repositorypkg.AppInfo, 0)
	for app, appType := range apps.Apps {
		items = append(items, &repositorypkg.AppInfo{Path: app, Type: appType})
	}
	return &repositorypkg.RepoAppsResponse{Items: items}, nil
}

func (s *Server) GetAppDetails(ctx context.Context, q *repositorypkg.RepoAppDetailsQuery) (*apiclient.RepoAppDetailsResponse, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionGet, q.Repo); err != nil {
		return nil, err
	}
	repo, err := s.db.GetRepository(ctx, q.Repo)
	if err != nil {
		return nil, err
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
	buildOptions, err := s.settings.GetKustomizeBuildOptions()
	if err != nil {
		return nil, err
	}
	return repoClient.GetAppDetails(ctx, &apiclient.RepoServerAppDetailsQuery{
		Repo:     repo,
		Revision: q.Revision,
		App:      q.Path,
		Repos:    repos,
		Helm:     q.Helm,
		Ksonnet:  q.Ksonnet,
		KustomizeOptions: &appsv1.KustomizeOptions{
			BuildOptions: buildOptions,
		},
	})
}

// Create creates a repository
func (s *Server) Create(ctx context.Context, q *repositorypkg.RepoCreateRequest) (*appsv1.Repository, error) {
	err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionCreate, q.Repo.Repo)
	if err != nil {
		return nil, err
	}

	// check we can connect to the repo, copying any existing creds
	{
		repo := q.Repo.DeepCopy()
		creds, err := s.db.GetRepository(ctx, repo.Repo)
		if err != nil {
			return nil, err
		}
		repo.CopyCredentialsFrom(creds)
		_, err = factory.NewFactory().NewRepo(q.Repo, metrics.NopReporter)
		if err != nil {
			return nil, err
		}
	}

	r := q.Repo
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
			return s.Update(ctx, &repositorypkg.RepoUpdateRequest{Repo: r})
		} else {
			return nil, status.Errorf(codes.InvalidArgument, "existing repository spec is different; use upsert flag to force update")
		}
	}
	return &appsv1.Repository{Repo: repo.Repo, Type: repo.Type, Name: repo.Name}, err
}

// Update updates a repository
func (s *Server) Update(ctx context.Context, q *repositorypkg.RepoUpdateRequest) (*appsv1.Repository, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionUpdate, q.Repo.Repo); err != nil {
		return nil, err
	}
	_, err := s.db.UpdateRepository(ctx, q.Repo)
	return &appsv1.Repository{Repo: q.Repo.Repo, Type: q.Repo.Type, Name: q.Repo.Name}, err
}

// Delete updates a repository
func (s *Server) Delete(ctx context.Context, q *repositorypkg.RepoQuery) (*repositorypkg.RepoResponse, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionDelete, q.Repo); err != nil {
		return nil, err
	}

	// invalidate cache
	if err := s.cache.SetRepoConnectionState(q.Repo, nil); err == nil {
		log.Errorf("error invalidating cache: %v", err)
	}

	err := s.db.DeleteRepository(ctx, q.Repo)
	return &repositorypkg.RepoResponse{}, err
}

// ValidateAccess checks whether access to a repository is possible with the
// given URL and credentials.
func (s *Server) ValidateAccess(ctx context.Context, q *repositorypkg.RepoAccessQuery) (*repositorypkg.RepoResponse, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionCreate, q.Repo); err != nil {
		return nil, err
	}

	repo := &appsv1.Repository{
		Repo:              q.Repo,
		Type:              q.Type,
		Name:              q.Name,
		Username:          q.Username,
		Password:          q.Password,
		SSHPrivateKey:     q.SshPrivateKey,
		Insecure:          q.Insecure,
		TLSClientCertData: q.TlsClientCertData,
		TLSClientCertKey:  q.TlsClientCertKey,
		TLSClientCAData:   q.TlsClientCAData,
	}
	_, err := factory.NewFactory().NewRepo(repo, metrics.NopReporter)
	if err != nil {
		return nil, err
	}
	return &repositorypkg.RepoResponse{}, nil
}
