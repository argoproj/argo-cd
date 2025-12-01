package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	argocommon "github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	applicationType "github.com/argoproj/argo-cd/v3/pkg/apis/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	servercache "github.com/argoproj/argo-cd/v3/server/cache"
	"github.com/argoproj/argo-cd/v3/server/deeplinks"
	applog "github.com/argoproj/argo-cd/v3/util/app/log"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/collections"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/lua"
	"github.com/argoproj/argo-cd/v3/util/rbac"
	"github.com/argoproj/argo-cd/v3/util/security"
	"github.com/argoproj/argo-cd/v3/util/session"
	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/text"
	jsonpatch "github.com/evanphx/json-patch"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"strconv"
	"strings"
	"time"
)

func (s *Server) validateAndNormalizeApp(ctx context.Context, app *v1alpha1.Application, proj *v1alpha1.AppProject, validate bool) error {
	if app.GetName() == "" {
		return errors.New("resource name may not be empty")
	}

	// ensure sources names are unique
	if app.Spec.HasMultipleSources() {
		sourceNames := make(map[string]bool)
		for _, source := range app.Spec.Sources {
			if source.Name != "" && sourceNames[source.Name] {
				return fmt.Errorf("application %s has duplicate source name: %s", app.Name, source.Name)
			}
			sourceNames[source.Name] = true
		}
	}

	appNs := s.appNamespaceOrDefault(app.Namespace)
	currApp, err := s.appclientset.ArgoprojV1alpha1().Applications(appNs).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting application by name: %w", err)
		}
		// Kubernetes go-client will return a pointer to a zero-value app instead of nil, even
		// though the API response was NotFound. This behavior was confirmed via logs.
		currApp = nil
	}
	if currApp != nil && currApp.Spec.GetProject() != app.Spec.GetProject() {
		// When changing projects, caller must have application create & update privileges in new project
		// NOTE: the update check was already verified in the caller to this function
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplications, rbac.ActionCreate, app.RBACName(s.ns)); err != nil {
			return err
		}
		// They also need 'update' privileges in the old project
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplications, rbac.ActionUpdate, currApp.RBACName(s.ns)); err != nil {
			return err
		}
		// Validate that the new project exists and the application is allowed to use it
		newProj, err := s.getAppProject(ctx, app, log.WithFields(applog.GetAppLogFields(app)))
		if err != nil {
			return err
		}
		proj = newProj
	}

	if _, err := argo.GetDestinationCluster(ctx, app.Spec.Destination, s.db); err != nil {
		return status.Errorf(codes.InvalidArgument, "application destination spec for %s is invalid: %s", app.Name, err.Error())
	}

	var conditions []v1alpha1.ApplicationCondition

	if validate {
		conditions := make([]v1alpha1.ApplicationCondition, 0)
		condition, err := argo.ValidateRepo(ctx, app, s.repoClientset, s.db, s.kubectl, proj, s.settingsMgr)
		if err != nil {
			return fmt.Errorf("error validating the repo: %w", err)
		}
		conditions = append(conditions, condition...)
		if len(conditions) > 0 {
			return status.Errorf(codes.InvalidArgument, "application spec for %s is invalid: %s", app.Name, argo.FormatAppConditions(conditions))
		}
	}

	conditions, err = argo.ValidatePermissions(ctx, &app.Spec, proj, s.db)
	if err != nil {
		return fmt.Errorf("error validating project permissions: %w", err)
	}
	if len(conditions) > 0 {
		return status.Errorf(codes.InvalidArgument, "application spec for %s is invalid: %s", app.Name, argo.FormatAppConditions(conditions))
	}

	// Validate managed-by-url annotation
	managedByURLConditions := argo.ValidateManagedByURL(app)
	if len(managedByURLConditions) > 0 {
		return status.Errorf(codes.InvalidArgument, "application spec for %s is invalid: %s", app.Name, argo.FormatAppConditions(managedByURLConditions))
	}

	app.Spec = *argo.NormalizeApplicationSpec(&app.Spec)
	return nil
}

func (s *Server) getApplicationClusterConfig(ctx context.Context, a *v1alpha1.Application) (*rest.Config, error) {
	cluster, err := argo.GetDestinationCluster(ctx, a.Spec.Destination, s.db)
	if err != nil {
		return nil, fmt.Errorf("error validating destination: %w", err)
	}
	config, err := cluster.RESTConfig()
	if err != nil {
		return nil, fmt.Errorf("error getting cluster REST config: %w", err)
	}

	return config, err
}

// getCachedAppState loads the cached state and trigger app refresh if cache is missing
func (s *Server) getCachedAppState(ctx context.Context, a *v1alpha1.Application, getFromCache func() error) error {
	err := getFromCache()
	if err != nil && errors.Is(err, servercache.ErrCacheMiss) {
		conditions := a.Status.GetConditions(map[v1alpha1.ApplicationConditionType]bool{
			v1alpha1.ApplicationConditionComparisonError:  true,
			v1alpha1.ApplicationConditionInvalidSpecError: true,
		})
		if len(conditions) > 0 {
			return errors.New(argo.FormatAppConditions(conditions))
		}
		_, err = s.Get(ctx, &application.ApplicationQuery{
			Name:         ptr.To(a.GetName()),
			AppNamespace: ptr.To(a.GetNamespace()),
			Refresh:      ptr.To(string(v1alpha1.RefreshTypeNormal)),
		})
		if err != nil {
			return fmt.Errorf("error getting application by query: %w", err)
		}
		return getFromCache()
	}
	return err
}

func (s *Server) getAppResources(ctx context.Context, a *v1alpha1.Application) (*v1alpha1.ApplicationTree, error) {
	var tree v1alpha1.ApplicationTree
	err := s.getCachedAppState(ctx, a, func() error {
		return s.cache.GetAppResourcesTree(a.InstanceName(s.ns), &tree)
	})
	if err != nil {
		if errors.Is(err, ErrCacheMiss) {
			fmt.Println("Cache Key is missing.\nEnsure that the Redis compression setting on the Application controller and CLI is same. See --redis-compress.")
		}
		return &tree, fmt.Errorf("error getting cached app resource tree: %w", err)
	}
	return &tree, nil
}

func (s *Server) getAppLiveResource(ctx context.Context, action string, q *application.ApplicationResourceRequest) (*v1alpha1.ResourceNode, *rest.Config, *v1alpha1.Application, error) {
	fineGrainedInheritanceDisabled, err := s.settingsMgr.ApplicationFineGrainedRBACInheritanceDisabled()
	if err != nil {
		return nil, nil, nil, err
	}

	if fineGrainedInheritanceDisabled && (action == rbac.ActionDelete || action == rbac.ActionUpdate) {
		action = fmt.Sprintf("%s/%s/%s/%s/%s", action, q.GetGroup(), q.GetKind(), q.GetNamespace(), q.GetResourceName())
	}
	a, _, err := s.getApplicationEnforceRBACInformer(ctx, action, q.GetProject(), q.GetAppNamespace(), q.GetName())
	if !fineGrainedInheritanceDisabled && err != nil && errors.Is(err, argocommon.PermissionDeniedAPIError) && (action == rbac.ActionDelete || action == rbac.ActionUpdate) {
		action = fmt.Sprintf("%s/%s/%s/%s/%s", action, q.GetGroup(), q.GetKind(), q.GetNamespace(), q.GetResourceName())
		a, _, err = s.getApplicationEnforceRBACInformer(ctx, action, q.GetProject(), q.GetAppNamespace(), q.GetName())
	}
	if err != nil {
		return nil, nil, nil, err
	}

	tree, err := s.getAppResources(ctx, a)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error getting app resources: %w", err)
	}

	found := tree.FindNode(q.GetGroup(), q.GetKind(), q.GetNamespace(), q.GetResourceName())
	if found == nil || found.UID == "" {
		return nil, nil, nil, status.Errorf(codes.InvalidArgument, "%s %s %s not found as part of application %s", q.GetKind(), q.GetGroup(), q.GetResourceName(), q.GetName())
	}
	config, err := s.getApplicationClusterConfig(ctx, a)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error getting application cluster config: %w", err)
	}
	return found, config, a, nil
}

// getAppEnforceRBAC gets the Application with the given name in the given namespace. If no namespace is
// specified, the Application is fetched from the default namespace (the one in which the API server is running).
//
// If the user does not provide a "project," then we have to be very careful how we respond. If an app with the given
// name exists, and the user has access to that app in the app's project, we return the app. If the app exists but the
// user does not have access, we return "permission denied." If the app does not exist, we return "permission denied" -
// if we responded with a 404, then the user could infer that the app exists when they get "permission denied."
//
// If the user does provide a "project," we can respond more specifically. If the user does not have access to the given
// app name in the given project, we return "permission denied." If the app exists, but the project is different from
func (s *Server) getAppEnforceRBAC(ctx context.Context, action, project, namespace, name string, getApp func() (*v1alpha1.Application, error)) (*v1alpha1.Application, *v1alpha1.AppProject, error) {
	user := session.Username(ctx)
	if user == "" {
		user = "Unknown user"
	}
	logCtx := log.WithFields(map[string]any{
		"user":        user,
		"application": name,
		"namespace":   namespace,
	})
	if project != "" {
		// The user has provided everything we need to perform an initial RBAC check.
		givenRBACName := security.RBACName(s.ns, project, namespace, name)
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplications, action, givenRBACName); err != nil {
			logCtx.WithFields(map[string]any{
				"project":                project,
				argocommon.SecurityField: argocommon.SecurityMedium,
			}).Warnf("user tried to %s application which they do not have access to: %s", action, err)
			// Do a GET on the app. This ensures that the timing of a "no access" response is the same as a "yes access,
			// but the app is in a different project" response. We don't want the user inferring the existence of the
			// app from response time.
			_, _ = getApp()
			return nil, nil, argocommon.PermissionDeniedAPIError
		}
	}
	a, err := getApp()
	if err != nil {
		if apierrors.IsNotFound(err) {
			if project != "" {
				// We know that the user was allowed to get the Application, but the Application does not exist. Return 404.
				return nil, nil, status.Error(codes.NotFound, apierrors.NewNotFound(schema.GroupResource{Group: "argoproj.io", Resource: "applications"}, name).Error())
			}
			// We don't know if the user was allowed to get the Application, and we don't want to leak information about
			// the Application's existence. Return 403.
			logCtx.Warn("application does not exist")
			return nil, nil, argocommon.PermissionDeniedAPIError
		}
		logCtx.Errorf("failed to get application: %s", err)
		return nil, nil, argocommon.PermissionDeniedAPIError
	}
	// Even if we performed an initial RBAC check (because the request was fully parameterized), we still need to
	// perform a second RBAC check to ensure that the user has access to the actual Application's project (not just the
	// project they specified in the request).
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplications, action, a.RBACName(s.ns)); err != nil {
		logCtx.WithFields(map[string]any{
			"project":                a.Spec.Project,
			argocommon.SecurityField: argocommon.SecurityMedium,
		}).Warnf("user tried to %s application which they do not have access to: %s", action, err)
		if project != "" {
			// The user specified a project. We would have returned a 404 if the user had access to the app, but the app
			// did not exist. So we have to return a 404 when the app does exist, but the user does not have access.
			// Otherwise, they could infer that the app exists based on the error code.
			return nil, nil, status.Error(codes.NotFound, apierrors.NewNotFound(schema.GroupResource{Group: "argoproj.io", Resource: "applications"}, name).Error())
		}
		// The user didn't specify a project. We always return permission denied for both lack of access and lack of
		// existence.
		return nil, nil, argocommon.PermissionDeniedAPIError
	}
	effectiveProject := "default"
	if a.Spec.Project != "" {
		effectiveProject = a.Spec.Project
	}
	if project != "" && effectiveProject != project {
		logCtx.WithFields(map[string]any{
			"project":                a.Spec.Project,
			argocommon.SecurityField: argocommon.SecurityMedium,
		}).Warnf("user tried to %s application in project %s, but the application is in project %s", action, project, effectiveProject)
		// The user has access to the app, but the app is in a different project. Return 404, meaning "app doesn't
		// exist in that project".
		return nil, nil, status.Error(codes.NotFound, apierrors.NewNotFound(schema.GroupResource{Group: "argoproj.io", Resource: "applications"}, name).Error())
	}
	// Get the app's associated project, and make sure all project restrictions are enforced.
	proj, err := s.getAppProject(ctx, a, logCtx)
	if err != nil {
		return a, nil, err
	}
	return a, proj, nil
}

// getApplicationEnforceRBACInformer uses an informer to get an Application. If the app does not exist, permission is
// denied, or any other error occurs when getting the app, we return a permission denied error to obscure any sensitive
// information.
func (s *Server) getApplicationEnforceRBACInformer(ctx context.Context, action, project, namespace, name string) (*v1alpha1.Application, *v1alpha1.AppProject, error) {
	namespaceOrDefault := s.appNamespaceOrDefault(namespace)
	return s.getAppEnforceRBAC(ctx, action, project, namespaceOrDefault, name, func() (*v1alpha1.Application, error) {
		if !s.isNamespaceEnabled(namespaceOrDefault) {
			return nil, security.NamespaceNotPermittedError(namespaceOrDefault)
		}
		return s.appLister.Applications(namespaceOrDefault).Get(name)
	})
}

// getApplicationEnforceRBACClient uses a client to get an Application. If the app does not exist, permission is denied,
// or any other error occurs when getting the app, we return a permission denied error to obscure any sensitive
// information.
func (s *Server) getApplicationEnforceRBACClient(ctx context.Context, action, project, namespace, name, resourceVersion string) (*v1alpha1.Application, *v1alpha1.AppProject, error) {
	namespaceOrDefault := s.appNamespaceOrDefault(namespace)
	return s.getAppEnforceRBAC(ctx, action, project, namespaceOrDefault, name, func() (*v1alpha1.Application, error) {
		if !s.isNamespaceEnabled(namespaceOrDefault) {
			return nil, security.NamespaceNotPermittedError(namespaceOrDefault)
		}
		app, err := s.appclientset.ArgoprojV1alpha1().Applications(namespaceOrDefault).Get(ctx, name, metav1.GetOptions{
			ResourceVersion: resourceVersion,
		})
		if err != nil {
			return nil, err
		}
		return app, nil
	})
}

func (s *Server) queryRepoServer(ctx context.Context, proj *v1alpha1.AppProject, action func(
	client apiclient.RepoServerServiceClient,
	helmRepos []*v1alpha1.Repository,
	helmCreds []*v1alpha1.RepoCreds,
	ociRepos []*v1alpha1.Repository,
	ociCreds []*v1alpha1.RepoCreds,
	helmOptions *v1alpha1.HelmOptions,
	enabledSourceTypes map[string]bool,
) error,
) error {
	closer, client, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return fmt.Errorf("error creating repo server client: %w", err)
	}
	defer utilio.Close(closer)

	helmRepos, err := s.db.ListHelmRepositories(ctx)
	if err != nil {
		return fmt.Errorf("error listing helm repositories: %w", err)
	}

	permittedHelmRepos, err := argo.GetPermittedRepos(proj, helmRepos)
	if err != nil {
		return fmt.Errorf("error retrieving permitted repos: %w", err)
	}
	helmRepositoryCredentials, err := s.db.GetAllHelmRepositoryCredentials(ctx)
	if err != nil {
		return fmt.Errorf("error getting helm repository credentials: %w", err)
	}
	helmOptions, err := s.settingsMgr.GetHelmSettings()
	if err != nil {
		return fmt.Errorf("error getting helm settings: %w", err)
	}
	permittedHelmCredentials, err := argo.GetPermittedReposCredentials(proj, helmRepositoryCredentials)
	if err != nil {
		return fmt.Errorf("error getting permitted repos credentials: %w", err)
	}
	enabledSourceTypes, err := s.settingsMgr.GetEnabledSourceTypes()
	if err != nil {
		return fmt.Errorf("error getting settings enabled source types: %w", err)
	}
	ociRepos, err := s.db.ListOCIRepositories(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list OCI repositories: %w", err)
	}
	permittedOCIRepos, err := argo.GetPermittedRepos(proj, ociRepos)
	if err != nil {
		return fmt.Errorf("failed to get permitted OCI repositories for project %q: %w", proj.Name, err)
	}
	ociRepositoryCredentials, err := s.db.GetAllOCIRepositoryCredentials(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get OCI credentials: %w", err)
	}
	permittedOCICredentials, err := argo.GetPermittedReposCredentials(proj, ociRepositoryCredentials)
	if err != nil {
		return fmt.Errorf("failed to get permitted OCI credentials for project %q: %w", proj.Name, err)
	}

	return action(client, permittedHelmRepos, permittedHelmCredentials, permittedOCIRepos, permittedOCICredentials, helmOptions, enabledSourceTypes)
}

// validateAndUpdateApp validates and updates the application. currentProject is the name of the project the app
// currently is under. If not specified, we assume that the app is under the project specified in the app spec.
func (s *Server) validateAndUpdateApp(ctx context.Context, newApp *v1alpha1.Application, merge bool, validate bool, action string, currentProject string) (*v1alpha1.Application, error) {
	s.projectLock.RLock(newApp.Spec.GetProject())
	defer s.projectLock.RUnlock(newApp.Spec.GetProject())

	app, proj, err := s.getApplicationEnforceRBACClient(ctx, action, currentProject, newApp.Namespace, newApp.Name, "")
	if err != nil {
		return nil, err
	}

	err = s.validateAndNormalizeApp(ctx, newApp, proj, validate)
	if err != nil {
		return nil, fmt.Errorf("error validating and normalizing app: %w", err)
	}

	a, err := s.updateApp(ctx, app, newApp, merge)
	if err != nil {
		return nil, fmt.Errorf("error updating application: %w", err)
	}
	return a, nil
}

// waitSync is a helper to wait until the application informer cache is synced after create/update.
// It waits until the app in the informer, has a resource version greater than the version in the
// supplied app, or after 2 seconds, whichever comes first. Returns true if synced.
// We use an informer cache for read operations (Get, List). Since the cache is only
// eventually consistent, it is possible that it doesn't reflect an application change immediately
// after a mutating API call (create/update). This function should be called after a creates &
// update to give a probable (but not guaranteed) chance of being up-to-date after the create/update.
func (s *Server) waitSync(app *v1alpha1.Application) {
	logCtx := log.WithFields(applog.GetAppLogFields(app))
	deadline := time.Now().Add(InformerSyncTimeout)
	minVersion, err := strconv.Atoi(app.ResourceVersion)
	if err != nil {
		logCtx.Warnf("waitSync failed: could not parse resource version %s", app.ResourceVersion)
		time.Sleep(50 * time.Millisecond) // sleep anyway
		return
	}
	for {
		if currApp, err := s.appLister.Applications(app.Namespace).Get(app.Name); err == nil {
			currVersion, err := strconv.Atoi(currApp.ResourceVersion)
			if err == nil && currVersion >= minVersion {
				return
			}
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	logCtx.Warnf("waitSync failed: timed out")
}

func (s *Server) updateApp(ctx context.Context, app *v1alpha1.Application, newApp *v1alpha1.Application, merge bool) (*v1alpha1.Application, error) {
	for i := 0; i < 10; i++ {
		app.Spec = newApp.Spec
		if merge {
			app.Labels = collections.Merge(app.Labels, newApp.Labels)
			app.Annotations = collections.Merge(app.Annotations, newApp.Annotations)
		} else {
			app.Labels = newApp.Labels
			app.Annotations = newApp.Annotations
		}

		app.Finalizers = newApp.Finalizers

		res, err := s.appclientset.ArgoprojV1alpha1().Applications(app.Namespace).Update(ctx, app, metav1.UpdateOptions{})
		if err == nil {
			s.logAppEvent(ctx, app, argo.EventReasonResourceUpdated, "updated application spec")
			s.waitSync(res)
			return res, nil
		}
		if !apierrors.IsConflict(err) {
			return nil, err
		}

		app, err = s.appclientset.ArgoprojV1alpha1().Applications(app.Namespace).Get(ctx, newApp.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error getting application: %w", err)
		}
		s.inferResourcesStatusHealth(app)
	}
	return nil, status.Errorf(codes.Internal, "Failed to update application. Too many conflicts")
}

func (s *Server) getAppProject(ctx context.Context, a *v1alpha1.Application, logCtx *log.Entry) (*v1alpha1.AppProject, error) {
	proj, err := argo.GetAppProject(ctx, a, applisters.NewAppProjectLister(s.projInformer.GetIndexer()), s.ns, s.settingsMgr, s.db)
	if err == nil {
		return proj, nil
	}

	// If there's a permission issue or the app doesn't exist, return a vague error to avoid letting the user enumerate project names.
	vagueError := status.Errorf(codes.InvalidArgument, "app is not allowed in project %q, or the project does not exist", a.Spec.Project)

	if apierrors.IsNotFound(err) {
		return nil, vagueError
	}

	var applicationNotAllowedToUseProjectErr *argo.ErrApplicationNotAllowedToUseProject
	if errors.As(err, &applicationNotAllowedToUseProjectErr) {
		return nil, vagueError
	}

	// Unknown error, log it but return the vague error to the user
	logCtx.WithFields(map[string]any{
		"project":                a.Spec.Project,
		argocommon.SecurityField: argocommon.SecurityMedium,
	}).Warnf("error getting app project: %s", err)
	return nil, vagueError
}

func (s *Server) isApplicationPermitted(selector labels.Selector, minVersion int, claims any, appName, appNs string, projects map[string]bool, a v1alpha1.Application) bool {
	if len(projects) > 0 && !projects[a.Spec.GetProject()] {
		return false
	}

	if appVersion, err := strconv.Atoi(a.ResourceVersion); err == nil && appVersion < minVersion {
		return false
	}
	matchedEvent := (appName == "" || (a.Name == appName && a.Namespace == appNs)) && selector.Matches(labels.Set(a.Labels))
	if !matchedEvent {
		return false
	}

	if !s.isNamespaceEnabled(a.Namespace) {
		return false
	}

	if !s.enf.Enforce(claims, rbac.ResourceApplications, rbac.ActionGet, a.RBACName(s.ns)) {
		// do not emit apps user does not have accessing
		return false
	}

	return true
}

func (s *Server) replaceSecretValues(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if obj.GetKind() == kube.SecretKind && obj.GroupVersionKind().Group == "" {
		_, obj, err := diff.HideSecretData(nil, obj, s.settingsMgr.GetSensitiveAnnotations())
		if err != nil {
			return nil, err
		}
		return obj, err
	}
	return obj, nil
}

// getAppSourceBySourceIndexAndVersionId returns the source for a specific source index and version ID. Source index and
// version ID are optional. If the source index is not specified, it defaults to 0. If the version ID is not specified,
// we use the source(s) currently configured for the app. If the version ID is specified, we find the source for that
// version ID. If the version ID is not found, we return an error. If the source index is out of bounds for whichever
// source we choose (configured sources or sources for a specific version), we return an error.
func getAppSourceBySourceIndexAndVersionId(a *v1alpha1.Application, sourceIndexMaybe *int32, versionIdMaybe *int32) (v1alpha1.ApplicationSource, error) {
	// Start with all the app's configured sources.
	sources := a.Spec.GetSources()

	// If the user specified a version, get the sources for that version. If the version is not found, return an error.
	if versionIdMaybe != nil {
		versionId := int64(*versionIdMaybe)
		var err error
		sources, err = getSourcesByVersionId(a, versionId)
		if err != nil {
			return v1alpha1.ApplicationSource{}, fmt.Errorf("error getting source by version ID: %w", err)
		}
	}

	// Start by assuming we want the first source.
	sourceIndex := 0

	// If the user specified a source index, use that instead.
	if sourceIndexMaybe != nil {
		sourceIndex = int(*sourceIndexMaybe)
		if sourceIndex >= len(sources) {
			if len(sources) == 1 {
				return v1alpha1.ApplicationSource{}, fmt.Errorf("source index %d not found because there is only 1 source", sourceIndex)
			}
			return v1alpha1.ApplicationSource{}, fmt.Errorf("source index %d not found because there are only %d sources", sourceIndex, len(sources))
		}
	}

	source := sources[sourceIndex]

	return source, nil
}

// getRevisionHistoryByVersionId returns the revision history for a specific version ID.
// If the version ID is not found, it returns an empty revision history and false.
func getRevisionHistoryByVersionId(histories v1alpha1.RevisionHistories, versionId int64) (v1alpha1.RevisionHistory, bool) {
	for _, h := range histories {
		if h.ID == versionId {
			return h, true
		}
	}
	return v1alpha1.RevisionHistory{}, false
}

// getSourcesByVersionId returns the sources for a specific version ID. If there is no history, it returns an error.
// If the version ID is not found, it returns an error. If the version ID is found, and there are multiple sources,
// it returns the sources for that version ID. If the version ID is found, and there is only one source, it returns
// a slice with just the single source.
func getSourcesByVersionId(a *v1alpha1.Application, versionId int64) ([]v1alpha1.ApplicationSource, error) {
	if len(a.Status.History) == 0 {
		return nil, fmt.Errorf("version ID %d not found because the app has no history", versionId)
	}

	h, ok := getRevisionHistoryByVersionId(a.Status.History, versionId)
	if !ok {
		return nil, fmt.Errorf("revision history not found for version ID %d", versionId)
	}

	if len(h.Sources) > 0 {
		return h.Sources, nil
	}

	return []v1alpha1.ApplicationSource{h.Source}, nil
}

func isMatchingResource(q *application.ResourcesQuery, key kube.ResourceKey) bool {
	return (q.GetName() == "" || q.GetName() == key.Name) &&
		(q.GetNamespace() == "" || q.GetNamespace() == key.Namespace) &&
		(q.GetGroup() == "" || q.GetGroup() == key.Group) &&
		(q.GetKind() == "" || q.GetKind() == key.Kind)
}

// from all of the treeNodes, get the pod who meets the criteria or whose parents meets the criteria
func getSelectedPods(treeNodes []v1alpha1.ResourceNode, q *application.ApplicationPodLogsQuery) []v1alpha1.ResourceNode {
	var pods []v1alpha1.ResourceNode
	isTheOneMap := make(map[string]bool)
	for _, treeNode := range treeNodes {
		if treeNode.Kind == kube.PodKind && treeNode.Group == "" && treeNode.UID != "" {
			if isTheSelectedOne(&treeNode, q, treeNodes, isTheOneMap) {
				pods = append(pods, treeNode)
			}
		}
	}
	return pods
}

// check is currentNode is matching with group, kind, and name, or if any of its parents matches
func isTheSelectedOne(currentNode *v1alpha1.ResourceNode, q *application.ApplicationPodLogsQuery, resourceNodes []v1alpha1.ResourceNode, isTheOneMap map[string]bool) bool {
	exist, value := isTheOneMap[currentNode.UID]
	if exist {
		return value
	}

	if (q.GetResourceName() == "" || currentNode.Name == q.GetResourceName()) &&
		(q.GetKind() == "" || currentNode.Kind == q.GetKind()) &&
		(q.GetGroup() == "" || currentNode.Group == q.GetGroup()) &&
		(q.GetNamespace() == "" || currentNode.Namespace == q.GetNamespace()) {
		isTheOneMap[currentNode.UID] = true
		return true
	}

	if len(currentNode.ParentRefs) == 0 {
		isTheOneMap[currentNode.UID] = false
		return false
	}

	for _, parentResource := range currentNode.ParentRefs {
		// look up parentResource from resourceNodes
		// then check if the parent isTheSelectedOne
		for _, resourceNode := range resourceNodes {
			if resourceNode.Namespace == parentResource.Namespace &&
				resourceNode.Name == parentResource.Name &&
				resourceNode.Group == parentResource.Group &&
				resourceNode.Kind == parentResource.Kind {
				if isTheSelectedOne(&resourceNode, q, resourceNodes, isTheOneMap) {
					isTheOneMap[currentNode.UID] = true
					return true
				}
			}
		}
	}

	isTheOneMap[currentNode.UID] = false
	return false
}

func (s *Server) resolveSourceRevisions(ctx context.Context, a *v1alpha1.Application, syncReq *application.ApplicationSyncRequest) (string, string, []string, []string, error) {
	requireOverridePrivilegeForRevisionSync, err := s.settingsMgr.RequireOverridePrivilegeForRevisionSync()
	if err != nil {
		// give up, and return the error
		return "", "", nil, nil,
			fmt.Errorf("error getting setting 'RequireOverridePrivilegeForRevisionSync' from configmap: : %w", err)
	}
	if a.Spec.HasMultipleSources() {
		numOfSources := int64(len(a.Spec.GetSources()))
		sourceRevisions := make([]string, numOfSources)
		displayRevisions := make([]string, numOfSources)
		desiredRevisions := make([]string, numOfSources)
		for i, pos := range syncReq.SourcePositions {
			if pos <= 0 || pos > numOfSources {
				return "", "", nil, nil, errors.New("source position is out of range")
			}
			desiredRevisions[pos-1] = syncReq.Revisions[i]
		}
		for index, desiredRevision := range desiredRevisions {
			if desiredRevision != "" && desiredRevision != text.FirstNonEmpty(a.Spec.GetSources()[index].TargetRevision, "HEAD") {
				// User is trying to sync to a different revision than the ones specified in the app sources
				// Enforce that they have the 'override' privilege if the setting is enabled
				if requireOverridePrivilegeForRevisionSync {
					if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplications, rbac.ActionOverride, a.RBACName(s.ns)); err != nil {
						return "", "", nil, nil, err
					}
				}
				if a.Spec.SyncPolicy != nil && a.Spec.SyncPolicy.IsAutomatedSyncEnabled() && !syncReq.GetDryRun() {
					return "", "", nil, nil, status.Errorf(codes.FailedPrecondition,
						"Cannot sync source %s to %s: auto-sync currently set to %s",
						a.Spec.GetSources()[index].RepoURL, desiredRevision, a.Spec.Sources[index].TargetRevision)
				}
			}
			revision, displayRevision, err := s.resolveRevision(ctx, a, syncReq, index)
			if err != nil {
				return "", "", nil, nil, status.Error(codes.FailedPrecondition, err.Error())
			}
			sourceRevisions[index] = revision
			displayRevisions[index] = displayRevision
		}
		return "", "", sourceRevisions, displayRevisions, nil
	}
	source := a.Spec.GetSource()
	if syncReq.GetRevision() != "" &&
		syncReq.GetRevision() != text.FirstNonEmpty(source.TargetRevision, "HEAD") {
		// User is trying to sync to a different revision than the one specified in the app spec
		// Enforce that they have the 'override' privilege if the setting is enabled
		if requireOverridePrivilegeForRevisionSync {
			if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplications, rbac.ActionOverride, a.RBACName(s.ns)); err != nil {
				return "", "", nil, nil, err
			}
		}
		if a.Spec.SyncPolicy != nil &&
			a.Spec.SyncPolicy.IsAutomatedSyncEnabled() && !syncReq.GetDryRun() {
			// If the app has auto-sync enabled, we cannot allow syncing to a different revision
			return "", "", nil, nil, status.Errorf(codes.FailedPrecondition, "Cannot sync to %s: auto-sync currently set to %s", syncReq.GetRevision(), source.TargetRevision)
		}
	}
	revision, displayRevision, err := s.resolveRevision(ctx, a, syncReq, -1)
	if err != nil {
		return "", "", nil, nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	return revision, displayRevision, nil, nil, nil
}

func getAmbiguousRevision(app *v1alpha1.Application, syncReq *application.ApplicationSyncRequest, sourceIndex int) string {
	ambiguousRevision := ""
	if app.Spec.HasMultipleSources() {
		for i, pos := range syncReq.SourcePositions {
			if pos == int64(sourceIndex+1) {
				ambiguousRevision = syncReq.Revisions[i]
			}
		}
		if ambiguousRevision == "" {
			ambiguousRevision = app.Spec.Sources[sourceIndex].TargetRevision
		}
	} else {
		ambiguousRevision = syncReq.GetRevision()
		if ambiguousRevision == "" {
			ambiguousRevision = app.Spec.GetSource().TargetRevision
		}
	}
	return ambiguousRevision
}

func (s *Server) getUnstructuredLiveResourceOrApp(ctx context.Context, rbacRequest string, q *application.ApplicationResourceRequest) (obj *unstructured.Unstructured, res *v1alpha1.ResourceNode, app *v1alpha1.Application, config *rest.Config, err error) {
	if q.GetKind() == applicationType.ApplicationKind && q.GetGroup() == applicationType.Group && q.GetName() == q.GetResourceName() {
		app, _, err = s.getApplicationEnforceRBACInformer(ctx, rbacRequest, q.GetProject(), q.GetAppNamespace(), q.GetName())
		if err != nil {
			return nil, nil, nil, nil, err
		}
		err = s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplications, rbacRequest, app.RBACName(s.ns))
		if err != nil {
			return nil, nil, nil, nil, err
		}
		config, err = s.getApplicationClusterConfig(ctx, app)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("error getting application cluster config: %w", err)
		}
		obj, err = kube.ToUnstructured(app)
	} else {
		res, config, app, err = s.getAppLiveResource(ctx, rbacRequest, q)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		obj, err = s.kubectl.GetResource(ctx, config, res.GroupKindVersion(), res.Name, res.Namespace)
	}
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("error getting resource: %w", err)
	}
	return obj, res, app, config, err
}

func (s *Server) getAvailableActions(resourceOverrides map[string]v1alpha1.ResourceOverride, obj *unstructured.Unstructured) ([]v1alpha1.ResourceAction, error) {
	luaVM := lua.VM{
		ResourceOverrides: resourceOverrides,
	}

	discoveryScripts, err := luaVM.GetResourceActionDiscovery(obj)
	if err != nil {
		return nil, fmt.Errorf("error getting Lua discovery script: %w", err)
	}
	if len(discoveryScripts) == 0 {
		return []v1alpha1.ResourceAction{}, nil
	}
	availableActions, err := luaVM.ExecuteResourceActionDiscovery(obj, discoveryScripts)
	if err != nil {
		return nil, fmt.Errorf("error executing Lua discovery script: %w", err)
	}
	return availableActions, nil
}

func (s *Server) patchResource(ctx context.Context, config *rest.Config, liveObjBytes, newObjBytes []byte, newObj *unstructured.Unstructured) (*application.ApplicationResponse, error) {
	diffBytes, err := jsonpatch.CreateMergePatch(liveObjBytes, newObjBytes)
	if err != nil {
		return nil, fmt.Errorf("error calculating merge patch: %w", err)
	}
	if string(diffBytes) == "{}" {
		return &application.ApplicationResponse{}, nil
	}

	// The following logic detects if the resource action makes a modification to status and/or spec.
	// If status was modified, we attempt to patch the status using status subresource, in case the
	// CRD is configured using the status subresource feature. See:
	// https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#status-subresource
	// If status subresource is in use, the patch has to be split into two:
	// * one to update spec (and other non-status fields)
	// * the other to update only status.
	nonStatusPatch, statusPatch, err := splitStatusPatch(diffBytes)
	if err != nil {
		return nil, fmt.Errorf("error splitting status patch: %w", err)
	}
	if statusPatch != nil {
		_, err = s.kubectl.PatchResource(ctx, config, newObj.GroupVersionKind(), newObj.GetName(), newObj.GetNamespace(), types.MergePatchType, diffBytes, "status")
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("error patching resource: %w", err)
			}
			// K8s API server returns 404 NotFound when the CRD does not support the status subresource
			// if we get here, the CRD does not use the status subresource. We will fall back to a normal patch
		} else {
			// If we get here, the CRD does use the status subresource, so we must patch status and
			// spec separately. update the diffBytes to the spec-only patch and fall through.
			diffBytes = nonStatusPatch
		}
	}
	if diffBytes != nil {
		_, err = s.kubectl.PatchResource(ctx, config, newObj.GroupVersionKind(), newObj.GetName(), newObj.GetNamespace(), types.MergePatchType, diffBytes)
		if err != nil {
			return nil, fmt.Errorf("error patching resource: %w", err)
		}
	}
	return &application.ApplicationResponse{}, nil
}

func (s *Server) verifyResourcePermitted(destCluster *v1alpha1.Cluster, proj *v1alpha1.AppProject, obj *unstructured.Unstructured) error {
	permitted, err := proj.IsResourcePermitted(schema.GroupKind{Group: obj.GroupVersionKind().Group, Kind: obj.GroupVersionKind().Kind}, obj.GetNamespace(), destCluster, func(project string) ([]*v1alpha1.Cluster, error) {
		clusters, err := s.db.GetProjectClusters(context.TODO(), project)
		if err != nil {
			return nil, fmt.Errorf("failed to get project clusters: %w", err)
		}
		return clusters, nil
	})
	if err != nil {
		return fmt.Errorf("error checking resource permissions: %w", err)
	}
	if !permitted {
		return fmt.Errorf("application is not permitted to manage %s/%s/%s in %s", obj.GroupVersionKind().Group, obj.GroupVersionKind().Kind, obj.GetName(), obj.GetNamespace())
	}

	return nil
}

func (s *Server) createResource(ctx context.Context, config *rest.Config, newObj *unstructured.Unstructured) (*application.ApplicationResponse, error) {
	_, err := s.kubectl.CreateResource(ctx, config, newObj.GroupVersionKind(), newObj.GetName(), newObj.GetNamespace(), newObj, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("error creating resource: %w", err)
	}
	return &application.ApplicationResponse{}, nil
}

// splitStatusPatch splits a patch into two: one for a non-status patch, and the status-only patch.
// Returns nil for either if the patch doesn't have modifications to non-status, or status, respectively.
func splitStatusPatch(patch []byte) ([]byte, []byte, error) {
	var obj map[string]any
	err := json.Unmarshal(patch, &obj)
	if err != nil {
		return nil, nil, err
	}
	var nonStatusPatch, statusPatch []byte
	if statusVal, ok := obj["status"]; ok {
		// calculate the status-only patch
		statusObj := map[string]any{
			"status": statusVal,
		}
		statusPatch, err = json.Marshal(statusObj)
		if err != nil {
			return nil, nil, err
		}
		// remove status, and calculate the non-status patch
		delete(obj, "status")
		if len(obj) > 0 {
			nonStatusPatch, err = json.Marshal(obj)
			if err != nil {
				return nil, nil, err
			}
		}
	} else {
		// status was not modified in patch
		nonStatusPatch = patch
	}
	return nonStatusPatch, statusPatch, nil
}

func (s *Server) inferResourcesStatusHealth(app *v1alpha1.Application) {
	if app.Status.ResourceHealthSource == v1alpha1.ResourceHealthLocationAppTree {
		tree := &v1alpha1.ApplicationTree{}
		if err := s.cache.GetAppResourcesTree(app.InstanceName(s.ns), tree); err == nil {
			healthByKey := map[kube.ResourceKey]*v1alpha1.HealthStatus{}
			for _, node := range tree.Nodes {
				if node.Health != nil {
					healthByKey[kube.NewResourceKey(node.Group, node.Kind, node.Namespace, node.Name)] = node.Health
				} else if node.ResourceVersion == "" && node.UID == "" && node.CreatedAt == nil {
					healthByKey[kube.NewResourceKey(node.Group, node.Kind, node.Namespace, node.Name)] = &v1alpha1.HealthStatus{
						Status:  health.HealthStatusMissing,
						Message: "Resource has not been created",
					}
				}
			}
			for i, res := range app.Status.Resources {
				res.Health = healthByKey[kube.NewResourceKey(res.Group, res.Kind, res.Namespace, res.Name)]
				app.Status.Resources[i] = res
			}
		}
	}
}

func convertSyncWindows(w *v1alpha1.SyncWindows) []*application.ApplicationSyncWindow {
	if w != nil {
		var windows []*application.ApplicationSyncWindow
		for _, w := range *w {
			nw := &application.ApplicationSyncWindow{
				Kind:       &w.Kind,
				Schedule:   &w.Schedule,
				Duration:   &w.Duration,
				ManualSync: &w.ManualSync,
			}
			windows = append(windows, nw)
		}
		if len(windows) > 0 {
			return windows
		}
	}
	return nil
}

func getPropagationPolicyFinalizer(policy string) string {
	switch strings.ToLower(policy) {
	case backgroundPropagationPolicy:
		return v1alpha1.BackgroundPropagationPolicyFinalizer
	case foregroundPropagationPolicy:
		return v1alpha1.ForegroundPropagationPolicyFinalizer
	case "":
		return v1alpha1.ResourcesFinalizerName
	default:
		return ""
	}
}

func (s *Server) appNamespaceOrDefault(appNs string) string {
	if appNs == "" {
		return s.ns
	}
	return appNs
}

func (s *Server) isNamespaceEnabled(namespace string) bool {
	return security.IsNamespaceEnabled(namespace, s.ns, s.enabledNamespaces)
}

// getProjectsFromApplicationQuery gets the project names from a query. If the legacy "project" field was specified, use
// that. Otherwise, use the newer "projects" field.
func getProjectsFromApplicationQuery(q application.ApplicationQuery) []string {
	if q.Project != nil {
		return q.Project
	}
	return q.Projects
}

func (s *Server) logAppEvent(ctx context.Context, a *v1alpha1.Application, reason string, action string) {
	eventInfo := argo.EventInfo{Type: corev1.EventTypeNormal, Reason: reason}
	user := session.Username(ctx)
	if user == "" {
		user = "Unknown user"
	}
	message := fmt.Sprintf("%s %s", user, action)
	eventLabels := argo.GetAppEventLabels(ctx, a, applisters.NewAppProjectLister(s.projInformer.GetIndexer()), s.ns, s.settingsMgr, s.db)
	s.auditLogger.LogAppEvent(a, eventInfo, message, user, eventLabels)
}

func (s *Server) logResourceEvent(ctx context.Context, res *v1alpha1.ResourceNode, reason string, action string) {
	eventInfo := argo.EventInfo{Type: corev1.EventTypeNormal, Reason: reason}
	user := session.Username(ctx)
	if user == "" {
		user = "Unknown user"
	}
	message := fmt.Sprintf("%s %s", user, action)
	s.auditLogger.LogResourceEvent(res, eventInfo, message, user)
}

func (s *Server) getObjectsForDeepLinks(ctx context.Context, app *v1alpha1.Application, proj *v1alpha1.AppProject) (cluster *unstructured.Unstructured, project *unstructured.Unstructured, err error) {
	// sanitize project jwt tokens
	proj.Status = v1alpha1.AppProjectStatus{}

	project, err = kube.ToUnstructured(proj)
	if err != nil {
		return nil, nil, err
	}

	getProjectClusters := func(project string) ([]*v1alpha1.Cluster, error) {
		return s.db.GetProjectClusters(ctx, project)
	}

	destCluster, err := argo.GetDestinationCluster(ctx, app.Spec.Destination, s.db)
	if err != nil {
		log.WithFields(applog.GetAppLogFields(app)).
			WithFields(map[string]any{
				"destination": app.Spec.Destination,
			}).Warnf("cannot validate cluster, error=%v", err.Error())
		return nil, nil, nil
	}

	permitted, err := proj.IsDestinationPermitted(destCluster, app.Spec.Destination.Namespace, getProjectClusters)
	if err != nil {
		return nil, nil, err
	}
	if !permitted {
		return nil, nil, errors.New("error getting destination cluster")
	}
	// sanitize cluster, remove cluster config creds and other unwanted fields
	cluster, err = deeplinks.SanitizeCluster(destCluster)
	return cluster, project, err
}
