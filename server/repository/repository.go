package repository

import (
	"fmt"
	"reflect"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/text"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/common"
	repositorypkg "github.com/argoproj/argo-cd/pkg/apiclient/repository"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	servercache "github.com/argoproj/argo-cd/server/cache"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/errors"
	"github.com/argoproj/argo-cd/util/io"
	"github.com/argoproj/argo-cd/util/rbac"
	"github.com/argoproj/argo-cd/util/settings"
)

// Server provides a Repository service
type Server struct {
	db            db.ArgoDB
	repoClientset apiclient.Clientset
	enf           *rbac.Enforcer
	cache         *servercache.Cache
	settings      *settings.SettingsManager
}

// NewServer returns a new instance of the Repository service
func NewServer(
	repoClientset apiclient.Clientset,
	db db.ArgoDB,
	enf *rbac.Enforcer,
	cache *servercache.Cache,
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

// Get the connection state for a given repository URL by connecting to the
// repo and evaluate the results. Unless forceRefresh is set to true, the
// result may be retrieved out of the cache.
func (s *Server) getConnectionState(ctx context.Context, url string, forceRefresh bool) appsv1.ConnectionState {
	if !forceRefresh {
		if connectionState, err := s.cache.GetRepoConnectionState(url); err == nil {
			return connectionState
		}
	}
	now := metav1.Now()
	connectionState := appsv1.ConnectionState{
		Status:     appsv1.ConnectionStatusSuccessful,
		ModifiedAt: &now,
	}
	var err error
	repo, err := s.db.GetRepository(ctx, url)
	if err == nil {
		err = argo.TestRepo(repo)
	}
	if err != nil {
		connectionState.Status = appsv1.ConnectionStatusFailed
		if errors.IsCredentialsConfigurationError(err) {
			connectionState.Message = "Configuration error - please check the server logs"
			log.Warnf("could not retrieve repo: %s", err.Error())
		} else {
			connectionState.Message = fmt.Sprintf("Unable to connect to repository: %v", err)
		}
	}
	err = s.cache.SetRepoConnectionState(url, &connectionState)
	if err != nil {
		log.Warnf("getConnectionState cache set error %s: %v", url, err)
	}
	return connectionState
}

// List returns list of repositories
// Deprecated: Use ListRepositories instead
func (s *Server) List(ctx context.Context, q *repositorypkg.RepoQuery) (*appsv1.RepositoryList, error) {
	return s.ListRepositories(ctx, q)
}

// Get return the requested configured repository by URL and the state of its connections.
func (s *Server) Get(ctx context.Context, q *repositorypkg.RepoQuery) (*appsv1.Repository, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionGet, q.Repo); err != nil {
		return nil, err
	}

	repo, err := s.db.GetRepository(ctx, q.Repo)
	if err != nil {
		return nil, err
	}
	// For backwards compatibility, if we have no repo type set assume a default
	rType := repo.Type
	if rType == "" {
		rType = common.DefaultRepoType
	}
	// remove secrets
	item := appsv1.Repository{
		Repo:                       repo.Repo,
		Type:                       rType,
		Name:                       repo.Name,
		Username:                   repo.Username,
		Insecure:                   repo.IsInsecure(),
		EnableLFS:                  repo.EnableLFS,
		GithubAppId:                repo.GithubAppId,
		GithubAppInstallationId:    repo.GithubAppInstallationId,
		GitHubAppEnterpriseBaseURL: repo.GitHubAppEnterpriseBaseURL,
	}

	item.ConnectionState = s.getConnectionState(ctx, item.Repo, q.ForceRefresh)

	return &item, nil
}

// ListRepositories returns a list of all configured repositories and the state of their connections
func (s *Server) ListRepositories(ctx context.Context, q *repositorypkg.RepoQuery) (*appsv1.RepositoryList, error) {
	repos, err := s.db.ListRepositories(ctx)
	if err != nil {
		return nil, err
	}
	items := appsv1.Repositories{}
	for _, repo := range repos {
		if s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionGet, repo.Repo) {
			// For backwards compatibility, if we have no repo type set assume a default
			rType := repo.Type
			if rType == "" {
				rType = common.DefaultRepoType
			}
			// remove secrets
			items = append(items, &appsv1.Repository{
				Repo:      repo.Repo,
				Type:      rType,
				Name:      repo.Name,
				Username:  repo.Username,
				Insecure:  repo.IsInsecure(),
				EnableLFS: repo.EnableLFS,
				EnableOCI: repo.EnableOCI,
			})
		}
	}
	err = kube.RunAllAsync(len(items), func(i int) error {
		items[i].ConnectionState = s.getConnectionState(ctx, items[i].Repo, q.ForceRefresh)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &appsv1.RepositoryList{Items: items}, nil
}

func (s *Server) ListRefs(ctx context.Context, q *repositorypkg.RepoQuery) (*apiclient.Refs, error) {
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
	defer io.Close(conn)

	return repoClient.ListRefs(ctx, &apiclient.ListRefsRequest{
		Repo: repo,
	})
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
	defer io.Close(conn)

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
	if q.Source == nil {
		return nil, status.Errorf(codes.InvalidArgument, "missing payload in request")
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionGet, q.Source.RepoURL); err != nil {
		return nil, err
	}
	repo, err := s.db.GetRepository(ctx, q.Source.RepoURL)
	if err != nil {
		return nil, err
	}
	conn, repoClient, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, err
	}
	defer io.Close(conn)
	helmRepos, err := s.db.ListHelmRepositories(ctx)
	if err != nil {
		return nil, err
	}
	kustomizeSettings, err := s.settings.GetKustomizeSettings()
	if err != nil {
		return nil, err
	}
	kustomizeOptions, err := kustomizeSettings.GetOptions(*q.Source)
	if err != nil {
		return nil, err
	}
	return repoClient.GetAppDetails(ctx, &apiclient.RepoServerAppDetailsQuery{
		Repo:             repo,
		Source:           q.Source,
		Repos:            helmRepos,
		KustomizeOptions: kustomizeOptions,
	})
}

// GetHelmCharts returns list of helm charts in the specified repository
func (s *Server) GetHelmCharts(ctx context.Context, q *repositorypkg.RepoQuery) (*apiclient.HelmChartsResponse, error) {
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
	defer io.Close(conn)
	return repoClient.GetHelmCharts(ctx, &apiclient.HelmChartsRequest{Repo: repo})
}

// Create creates a repository or repository credential set
// Deprecated: Use CreateRepository() instead
func (s *Server) Create(ctx context.Context, q *repositorypkg.RepoCreateRequest) (*appsv1.Repository, error) {
	return s.CreateRepository(ctx, q)
}

// CreateRepository creates a repository configuration
func (s *Server) CreateRepository(ctx context.Context, q *repositorypkg.RepoCreateRequest) (*appsv1.Repository, error) {
	if q.Repo == nil {
		return nil, status.Errorf(codes.InvalidArgument, "missing payload in request")
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionCreate, q.Repo.Repo); err != nil {
		return nil, err
	}

	var repo *appsv1.Repository
	var err error

	// check we can connect to the repo, copying any existing creds
	{
		repo := q.Repo.DeepCopy()
		if !repo.HasCredentials() {
			creds, err := s.db.GetRepositoryCredentials(ctx, repo.Repo)
			if err != nil {
				return nil, err
			}
			repo.CopyCredentialsFrom(creds)
		}
		err = argo.TestRepo(repo)
		if err != nil {
			return nil, err
		}
	}

	r := q.Repo
	r.ConnectionState = appsv1.ConnectionState{Status: appsv1.ConnectionStatusSuccessful}
	repo, err = s.db.CreateRepository(ctx, r)
	if status.Convert(err).Code() == codes.AlreadyExists {
		// act idempotent if existing spec matches new spec
		existing, getErr := s.db.GetRepository(ctx, r.Repo)
		if getErr != nil {
			return nil, status.Errorf(codes.Internal, "unable to check existing repository details: %v", getErr)
		}

		existing.Type = text.FirstNonEmpty(existing.Type, "git")
		// repository ConnectionState may differ, so make consistent before testing
		existing.ConnectionState = r.ConnectionState
		if reflect.DeepEqual(existing, r) {
			repo, err = existing, nil
		} else if q.Upsert {
			return s.UpdateRepository(ctx, &repositorypkg.RepoUpdateRequest{Repo: r})
		} else {
			return nil, status.Errorf(codes.InvalidArgument, "existing repository spec is different; use upsert flag to force update")
		}
	}
	if err != nil {
		return nil, err
	}
	return &appsv1.Repository{Repo: repo.Repo, Type: repo.Type, Name: repo.Name}, nil
}

// Update updates a repository or credential set
// Deprecated: Use UpdateRepository() instead
func (s *Server) Update(ctx context.Context, q *repositorypkg.RepoUpdateRequest) (*appsv1.Repository, error) {
	return s.UpdateRepository(ctx, q)
}

// UpdateRepository updates a repository configuration
func (s *Server) UpdateRepository(ctx context.Context, q *repositorypkg.RepoUpdateRequest) (*appsv1.Repository, error) {
	if q.Repo == nil {
		return nil, status.Errorf(codes.InvalidArgument, "missing payload in request")
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionUpdate, q.Repo.Repo); err != nil {
		return nil, err
	}
	_, err := s.db.UpdateRepository(ctx, q.Repo)
	return &appsv1.Repository{Repo: q.Repo.Repo, Type: q.Repo.Type, Name: q.Repo.Name}, err
}

// Delete removes a repository from the configuration
// Deprecated: Use DeleteRepository() instead
func (s *Server) Delete(ctx context.Context, q *repositorypkg.RepoQuery) (*repositorypkg.RepoResponse, error) {
	return s.DeleteRepository(ctx, q)
}

// DeleteRepository removes a repository from the configuration
func (s *Server) DeleteRepository(ctx context.Context, q *repositorypkg.RepoQuery) (*repositorypkg.RepoResponse, error) {
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
		Repo:                       q.Repo,
		Type:                       q.Type,
		Name:                       q.Name,
		Username:                   q.Username,
		Password:                   q.Password,
		SSHPrivateKey:              q.SshPrivateKey,
		Insecure:                   q.Insecure,
		TLSClientCertData:          q.TlsClientCertData,
		TLSClientCertKey:           q.TlsClientCertKey,
		EnableOCI:                  q.EnableOci,
		GithubAppPrivateKey:        q.GithubAppPrivateKey,
		GithubAppId:                q.GithubAppID,
		GithubAppInstallationId:    q.GithubAppInstallationID,
		GitHubAppEnterpriseBaseURL: q.GithubAppEnterpriseBaseUrl,
	}

	var repoCreds *appsv1.RepoCreds
	var err error

	// If repo does not have credentials, check if there are credentials stored
	// for it and if yes, copy them
	if !repo.HasCredentials() {
		repoCreds, err = s.db.GetRepositoryCredentials(ctx, q.Repo)
		if err != nil {
			return nil, err
		}
		if repoCreds != nil {
			repo.CopyCredentialsFrom(repoCreds)
		}
	}
	err = argo.TestRepo(repo)
	if err != nil {
		return nil, err
	}
	return &repositorypkg.RepoResponse{}, nil
}
