package repository

import (
	"context"
	"fmt"
	"reflect"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/text"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v2/common"
	repositorypkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/repository"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/rbac"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

// Server provides a Repository service
type Server struct {
	db            db.ArgoDB
	repoClientset apiclient.Clientset
	enf           *rbac.Enforcer
	cache         *servercache.Cache
	appLister     applisters.ApplicationLister
	projLister    cache.SharedIndexInformer
	settings      *settings.SettingsManager
	namespace     string
}

// NewServer returns a new instance of the Repository service
func NewServer(
	repoClientset apiclient.Clientset,
	db db.ArgoDB,
	enf *rbac.Enforcer,
	cache *servercache.Cache,
	appLister applisters.ApplicationLister,
	projLister cache.SharedIndexInformer,
	namespace string,
	settings *settings.SettingsManager,
) *Server {
	return &Server{
		db:            db,
		repoClientset: repoClientset,
		enf:           enf,
		cache:         cache,
		appLister:     appLister,
		projLister:    projLister,
		namespace:     namespace,
		settings:      settings,
	}
}

var (
	errPermissionDenied = status.Error(codes.PermissionDenied, "permission denied")
)

func (s *Server) getRepo(ctx context.Context, url string) (*appsv1.Repository, error) {
	repo, err := s.db.GetRepository(ctx, url)
	if err != nil {
		return nil, errPermissionDenied
	}
	return repo, nil
}

func createRBACObject(project string, repo string) string {
	if project != "" {
		return project + "/" + repo
	}
	return repo
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
		err = s.testRepo(ctx, repo)
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
	repo, err := s.getRepo(ctx, q.Repo)
	if err != nil {
		return nil, err
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionGet, createRBACObject(repo.Project, repo.Repo)); err != nil {
		return nil, err
	}

	// getRepo does not return an error for unconfigured repositories, so we are checking here
	exists, err := s.db.RepositoryExists(ctx, q.Repo)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, status.Errorf(codes.NotFound, "repo '%s' not found", q.Repo)
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
		Proxy:                      repo.Proxy,
		Project:                    repo.Project,
		InheritedCreds:             repo.InheritedCreds,
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
		if s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionGet, createRBACObject(repo.Project, repo.Repo)) {
			// For backwards compatibility, if we have no repo type set assume a default
			rType := repo.Type
			if rType == "" {
				rType = common.DefaultRepoType
			}
			// remove secrets
			items = append(items, &appsv1.Repository{
				Repo:               repo.Repo,
				Type:               rType,
				Name:               repo.Name,
				Username:           repo.Username,
				Insecure:           repo.IsInsecure(),
				EnableLFS:          repo.EnableLFS,
				EnableOCI:          repo.EnableOCI,
				Proxy:              repo.Proxy,
				Project:            repo.Project,
				ForceHttpBasicAuth: repo.ForceHttpBasicAuth,
				InheritedCreds:     repo.InheritedCreds,
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
	repo, err := s.getRepo(ctx, q.Repo)
	if err != nil {
		return nil, err
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionGet, createRBACObject(repo.Project, repo.Repo)); err != nil {
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

// ListApps performs discovery of a git repository for potential sources of applications. Used
// as a convenience to the UI for auto-complete.
func (s *Server) ListApps(ctx context.Context, q *repositorypkg.RepoAppsQuery) (*repositorypkg.RepoAppsResponse, error) {
	repo, err := s.getRepo(ctx, q.Repo)
	if err != nil {
		return nil, err
	}

	claims := ctx.Value("claims")
	if err := s.enf.EnforceErr(claims, rbacpolicy.ResourceRepositories, rbacpolicy.ActionGet, createRBACObject(repo.Project, repo.Repo)); err != nil {
		return nil, err
	}

	// This endpoint causes us to clone git repos & invoke config management tooling for the purposes
	// of app discovery. Only allow this to happen if user has privileges to create or update the
	// application which it wants to retrieve these details for.
	appRBACresource := fmt.Sprintf("%s/%s", q.AppProject, q.AppName)
	if !s.enf.Enforce(claims, rbacpolicy.ResourceApplications, rbacpolicy.ActionCreate, appRBACresource) &&
		!s.enf.Enforce(claims, rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, appRBACresource) {
		return nil, errPermissionDenied
	}
	// Also ensure the repo is actually allowed in the project in question
	if err := s.isRepoPermittedInProject(ctx, q.Repo, q.AppProject); err != nil {
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

// GetAppDetails shows parameter values to various config tools (e.g. helm/kustomize values)
// This is used by UI for parameter form fields during app create & edit pages.
// It is also used when showing history of parameters used in previous syncs in the app history.
func (s *Server) GetAppDetails(ctx context.Context, q *repositorypkg.RepoAppDetailsQuery) (*apiclient.RepoAppDetailsResponse, error) {
	if q.Source == nil {
		return nil, status.Errorf(codes.InvalidArgument, "missing payload in request")
	}
	repo, err := s.getRepo(ctx, q.Source.RepoURL)
	if err != nil {
		return nil, err
	}
	claims := ctx.Value("claims")
	if err := s.enf.EnforceErr(claims, rbacpolicy.ResourceRepositories, rbacpolicy.ActionGet, createRBACObject(repo.Project, repo.Repo)); err != nil {
		return nil, err
	}
	appName, appNs := argo.ParseFromQualifiedName(q.AppName, s.settings.GetNamespace())
	app, err := s.appLister.Applications(appNs).Get(appName)
	appRBACObj := createRBACObject(q.AppProject, q.AppName)
	// ensure caller has read privileges to app
	if err := s.enf.EnforceErr(claims, rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACObj); err != nil {
		return nil, err
	}
	if apierr.IsNotFound(err) {
		// app doesn't exist since it still is being formulated. verify they can create the app
		// before we reveal repo details
		if err := s.enf.EnforceErr(claims, rbacpolicy.ResourceApplications, rbacpolicy.ActionCreate, appRBACObj); err != nil {
			return nil, err
		}
	} else {
		// if we get here we are returning repo details of an existing app
		if q.AppProject != app.Spec.Project {
			return nil, errPermissionDenied
		}
		// verify caller is not making a request with arbitrary source values which were not in our history
		if !isSourceInHistory(app, *q.Source) {
			return nil, errPermissionDenied
		}
	}
	// Ensure the repo is actually allowed in the project in question
	if err := s.isRepoPermittedInProject(ctx, q.Source.RepoURL, q.AppProject); err != nil {
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
	helmOptions, err := s.settings.GetHelmSettings()
	if err != nil {
		return nil, err
	}
	return repoClient.GetAppDetails(ctx, &apiclient.RepoServerAppDetailsQuery{
		Repo:             repo,
		Source:           q.Source,
		Repos:            helmRepos,
		KustomizeOptions: kustomizeOptions,
		HelmOptions:      helmOptions,
		AppName:          q.AppName,
	})
}

// GetHelmCharts returns list of helm charts in the specified repository
func (s *Server) GetHelmCharts(ctx context.Context, q *repositorypkg.RepoQuery) (*apiclient.HelmChartsResponse, error) {
	repo, err := s.getRepo(ctx, q.Repo)
	if err != nil {
		return nil, err
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionGet, createRBACObject(repo.Project, repo.Repo)); err != nil {
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

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionCreate, createRBACObject(q.Repo.Project, q.Repo.Repo)); err != nil {
		return nil, err
	}

	var repo *appsv1.Repository
	var err error

	// check we can connect to the repo, copying any existing creds (not supported for project scoped repositories)
	if q.Repo.Project == "" {
		repo := q.Repo.DeepCopy()
		if !repo.HasCredentials() {
			creds, err := s.db.GetRepositoryCredentials(ctx, repo.Repo)
			if err != nil {
				return nil, err
			}
			repo.CopyCredentialsFrom(creds)
		}

		err = s.testRepo(ctx, repo)
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
			r.Project = q.Repo.Project
			return s.UpdateRepository(ctx, &repositorypkg.RepoUpdateRequest{Repo: r})
		} else {
			return nil, status.Errorf(codes.InvalidArgument, argo.GenerateSpecIsDifferentErrorMessage("repository", existing, r))
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

	repo, err := s.getRepo(ctx, q.Repo.Repo)
	if err != nil {
		return nil, err
	}

	// verify that user can do update inside project where repository is located
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionUpdate, createRBACObject(repo.Project, repo.Repo)); err != nil {
		return nil, err
	}
	// verify that user can do update inside project where repository will be located
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionUpdate, createRBACObject(q.Repo.Project, q.Repo.Repo)); err != nil {
		return nil, err
	}
	_, err = s.db.UpdateRepository(ctx, q.Repo)
	return &appsv1.Repository{Repo: q.Repo.Repo, Type: q.Repo.Type, Name: q.Repo.Name}, err
}

// Delete removes a repository from the configuration
// Deprecated: Use DeleteRepository() instead
func (s *Server) Delete(ctx context.Context, q *repositorypkg.RepoQuery) (*repositorypkg.RepoResponse, error) {
	return s.DeleteRepository(ctx, q)
}

// DeleteRepository removes a repository from the configuration
func (s *Server) DeleteRepository(ctx context.Context, q *repositorypkg.RepoQuery) (*repositorypkg.RepoResponse, error) {
	repo, err := s.getRepo(ctx, q.Repo)
	if err != nil {
		return nil, err
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionDelete, createRBACObject(repo.Project, repo.Repo)); err != nil {
		return nil, err
	}

	// invalidate cache
	if err := s.cache.SetRepoConnectionState(q.Repo, nil); err == nil {
		log.Errorf("error invalidating cache: %v", err)
	}

	err = s.db.DeleteRepository(ctx, q.Repo)
	return &repositorypkg.RepoResponse{}, err
}

// ValidateAccess checks whether access to a repository is possible with the
// given URL and credentials.
func (s *Server) ValidateAccess(ctx context.Context, q *repositorypkg.RepoAccessQuery) (*repositorypkg.RepoResponse, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionCreate, createRBACObject(q.Project, q.Repo)); err != nil {
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
		Proxy:                      q.Proxy,
		GCPServiceAccountKey:       q.GcpServiceAccountKey,
	}

	// If repo does not have credentials, check if there are credentials stored
	// for it and if yes, copy them
	if !repo.HasCredentials() {
		repoCreds, err := s.db.GetRepositoryCredentials(ctx, q.Repo)
		if err != nil {
			return nil, err
		}
		if repoCreds != nil {
			repo.CopyCredentialsFrom(repoCreds)
		}
	}
	err := s.testRepo(ctx, repo)
	if err != nil {
		return nil, err
	}
	return &repositorypkg.RepoResponse{}, nil
}

func (s *Server) testRepo(ctx context.Context, repo *appsv1.Repository) error {
	conn, repoClient, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return err
	}
	defer io.Close(conn)

	_, err = repoClient.TestRepository(ctx, &apiclient.TestRepositoryRequest{
		Repo: repo,
	})
	return err
}

func (s *Server) isRepoPermittedInProject(ctx context.Context, repo string, projName string) error {
	proj, err := argo.GetAppProjectByName(projName, applisters.NewAppProjectLister(s.projLister.GetIndexer()), s.namespace, s.settings, s.db, ctx)
	if err != nil {
		return err
	}
	if !proj.IsSourcePermitted(appsv1.ApplicationSource{RepoURL: repo}) {
		return status.Errorf(codes.PermissionDenied, "repository '%s' not permitted in project '%s'", repo, projName)
	}
	return nil
}

// isSourceInHistory checks if the supplied application source is either our current application
// source, or was something which we synced to previously.
func isSourceInHistory(app *v1alpha1.Application, source v1alpha1.ApplicationSource) bool {
	appSource := app.Spec.GetSource()
	if source.Equals(&appSource) {
		return true
	}
	appSources := app.Spec.GetSources()
	for _, s := range appSources {
		if source.Equals(&s) {
			return true
		}
	}
	// Iterate history. When comparing items in our history, use the actual synced revision to
	// compare with the supplied source.targetRevision in the request. This is because
	// history[].source.targetRevision is ambiguous (e.g. HEAD), whereas
	// history[].revision will contain the explicit SHA
	for _, h := range app.Status.History {
		h.Source.TargetRevision = h.Revision
		if source.Equals(&h.Source) {
			return true
		}
	}
	return false
}
