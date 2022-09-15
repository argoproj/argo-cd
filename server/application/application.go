package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	kubecache "github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/text"
	"github.com/argoproj/pkg/sync"
	jsonpatch "github.com/evanphx/json-patch"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"

	argocommon "github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v2/util/argo"
	argoutil "github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/git"
	"github.com/argoproj/argo-cd/v2/util/glob"
	ioutil "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/lua"
	"github.com/argoproj/argo-cd/v2/util/manifeststream"
	"github.com/argoproj/argo-cd/v2/util/rbac"
	"github.com/argoproj/argo-cd/v2/util/session"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

type AppResourceTreeFn func(ctx context.Context, app *appv1.Application) (*appv1.ApplicationTree, error)

const (
	maxPodLogsToRender                 = 10
	backgroundPropagationPolicy string = "background"
	foregroundPropagationPolicy string = "foreground"
)

var (
	watchAPIBufferSize = env.ParseNumFromEnv(argocommon.EnvWatchAPIBufferSize, 1000, 0, math.MaxInt32)
)

// Server provides an Application service
type Server struct {
	ns                string
	kubeclientset     kubernetes.Interface
	appclientset      appclientset.Interface
	appLister         applisters.ApplicationLister
	appInformer       cache.SharedIndexInformer
	appBroadcaster    *broadcasterHandler
	repoClientset     apiclient.Clientset
	kubectl           kube.Kubectl
	db                db.ArgoDB
	enf               *rbac.Enforcer
	projectLock       sync.KeyLock
	auditLogger       *argo.AuditLogger
	settingsMgr       *settings.SettingsManager
	cache             *servercache.Cache
	projInformer      cache.SharedIndexInformer
	enabledNamespaces []string
}

// NewServer returns a new instance of the Application service
func NewServer(
	namespace string,
	kubeclientset kubernetes.Interface,
	appclientset appclientset.Interface,
	appLister applisters.ApplicationLister,
	appInformer cache.SharedIndexInformer,
	repoClientset apiclient.Clientset,
	cache *servercache.Cache,
	kubectl kube.Kubectl,
	db db.ArgoDB,
	enf *rbac.Enforcer,
	projectLock sync.KeyLock,
	settingsMgr *settings.SettingsManager,
	projInformer cache.SharedIndexInformer,
	enabledNamespaces []string,
) (application.ApplicationServiceServer, AppResourceTreeFn) {
	appBroadcaster := &broadcasterHandler{}
	appInformer.AddEventHandler(appBroadcaster)
	s := &Server{
		ns:                namespace,
		appclientset:      appclientset,
		appLister:         appLister,
		appInformer:       appInformer,
		appBroadcaster:    appBroadcaster,
		kubeclientset:     kubeclientset,
		cache:             cache,
		db:                db,
		repoClientset:     repoClientset,
		kubectl:           kubectl,
		enf:               enf,
		projectLock:       projectLock,
		auditLogger:       argo.NewAuditLogger(namespace, kubeclientset, "argocd-server"),
		settingsMgr:       settingsMgr,
		projInformer:      projInformer,
		enabledNamespaces: enabledNamespaces,
	}
	return s, s.getAppResources
}

// List returns list of applications
func (s *Server) List(ctx context.Context, q *application.ApplicationQuery) (*appv1.ApplicationList, error) {
	labelsMap, err := labels.ConvertSelectorToLabelsMap(q.GetSelector())
	if err != nil {
		return nil, fmt.Errorf("error converting selector to labels map: %w", err)
	}
	var apps []*appv1.Application
	if q.GetAppNamespace() == "" {
		apps, err = s.appLister.List(labelsMap.AsSelector())
	} else {
		apps, err = s.appLister.Applications(q.GetAppNamespace()).List(labelsMap.AsSelector())
	}
	if err != nil {
		return nil, fmt.Errorf("error listing apps with selectors: %w", err)
	}
	newItems := make([]appv1.Application, 0)
	for _, a := range apps {
		// Skip any application that is neither in the conrol plane's namespace
		// nor in the list of enabled namespaces.
		if a.Namespace != s.ns && !glob.MatchStringInList(s.enabledNamespaces, a.Namespace, false) {
			continue
		}
		if s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, a.RBACName(s.ns)) {
			newItems = append(newItems, *a)
		}
	}

	if q.Name != nil {
		newItems, err = argoutil.FilterByName(newItems, *q.Name)
		if err != nil {
			return nil, fmt.Errorf("error filtering applications by name: %w", err)
		}
	}

	// Filter applications by name
	newItems = argoutil.FilterByProjects(newItems, q.Projects)

	// Filter applications by source repo URL
	newItems = argoutil.FilterByRepo(newItems, q.GetRepo())

	// Sort found applications by name
	sort.Slice(newItems, func(i, j int) bool {
		return newItems[i].Name < newItems[j].Name
	})

	appList := appv1.ApplicationList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: s.appInformer.LastSyncResourceVersion(),
		},
		Items: newItems,
	}
	return &appList, nil
}

// Create creates an application
func (s *Server) Create(ctx context.Context, q *application.ApplicationCreateRequest) (*appv1.Application, error) {
	if q.GetApplication() == nil {
		return nil, fmt.Errorf("error creating application: application is nil in request")
	}
	a := q.GetApplication()

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionCreate, a.RBACName(s.ns)); err != nil {
		return nil, err
	}

	s.projectLock.RLock(a.Spec.GetProject())
	defer s.projectLock.RUnlock(a.Spec.GetProject())

	validate := true
	if q.Validate != nil {
		validate = *q.Validate
	}
	err := s.validateAndNormalizeApp(ctx, a, validate)
	if err != nil {
		return nil, fmt.Errorf("error while validating and normalizing app: %w", err)
	}

	appNs := s.appNamespaceOrDefault(a.Namespace)

	if !s.isNamespaceEnabled(appNs) {
		return nil, namespaceNotPermittedError(appNs)
	}

	created, err := s.appclientset.ArgoprojV1alpha1().Applications(appNs).Create(ctx, a, metav1.CreateOptions{})
	if err == nil {
		if a.Spec.Source.Plugin != nil && a.Spec.Source.Plugin.Name != "" {
			log.WithFields(map[string]interface{}{
				"application": a.Name,
				"plugin":      a.Spec.Source.Plugin.Name,
			}).Warnf(argocommon.ConfigMapPluginDeprecationWarning)
		}

		s.logAppEvent(created, ctx, argo.EventReasonResourceCreated, "created application")
		s.waitSync(created)
		return created, nil
	}
	if !apierr.IsAlreadyExists(err) {
		return nil, fmt.Errorf("error creating application: %w", err)
	}

	// act idempotent if existing spec matches new spec
	existing, err := s.appLister.Applications(appNs).Get(a.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to check existing application details (%s): %v", appNs, err)
	}
	equalSpecs := reflect.DeepEqual(existing.Spec, a.Spec) &&
		reflect.DeepEqual(existing.Labels, a.Labels) &&
		reflect.DeepEqual(existing.Annotations, a.Annotations) &&
		reflect.DeepEqual(existing.Finalizers, a.Finalizers)

	if equalSpecs {
		return existing, nil
	}
	if q.Upsert == nil || !*q.Upsert {
		return nil, status.Errorf(codes.InvalidArgument, "existing application spec is different, use upsert flag to force update")
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, a.RBACName(s.ns)); err != nil {
		return nil, err
	}
	updated, err := s.updateApp(existing, a, ctx, true)
	if err != nil {
		return nil, fmt.Errorf("error updating application: %w", err)
	}
	return updated, nil
}

func (s *Server) queryRepoServer(ctx context.Context, a *v1alpha1.Application, action func(
	client apiclient.RepoServerServiceClient,
	repo *appv1.Repository,
	helmRepos []*appv1.Repository,
	helmCreds []*v1alpha1.RepoCreds,
	helmOptions *v1alpha1.HelmOptions,
	kustomizeOptions *v1alpha1.KustomizeOptions,
	enabledSourceTypes map[string]bool,
) error) error {

	closer, client, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return fmt.Errorf("error creating repo server client: %w", err)
	}
	defer ioutil.Close(closer)
	repo, err := s.db.GetRepository(ctx, a.Spec.Source.RepoURL)
	if err != nil {
		return fmt.Errorf("error getting repository: %w", err)
	}
	kustomizeSettings, err := s.settingsMgr.GetKustomizeSettings()
	if err != nil {
		return fmt.Errorf("error getting kustomize settings: %w", err)
	}
	kustomizeOptions, err := kustomizeSettings.GetOptions(a.Spec.Source)
	if err != nil {
		return fmt.Errorf("error getting kustomize settings options: %w", err)
	}
	proj, err := argo.GetAppProject(a, applisters.NewAppProjectLister(s.projInformer.GetIndexer()), s.ns, s.settingsMgr, s.db, ctx)
	if err != nil {
		if apierr.IsNotFound(err) {
			return status.Errorf(codes.InvalidArgument, "application references project %s which does not exist", a.Spec.Project)
		}
		return fmt.Errorf("error getting application's project: %w", err)
	}

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
	return action(client, repo, permittedHelmRepos, permittedHelmCredentials, helmOptions, kustomizeOptions, enabledSourceTypes)
}

// GetManifests returns application manifests
func (s *Server) GetManifests(ctx context.Context, q *application.ApplicationManifestQuery) (*apiclient.ManifestResponse, error) {
	if q.Name == nil || *q.Name == "" {
		return nil, fmt.Errorf("invalid request: application name is missing")
	}
	appName := q.GetName()
	appNs := s.appNamespaceOrDefault(q.GetAppNamespace())
	a, err := s.appLister.Applications(appNs).Get(appName)
	if err != nil {
		return nil, fmt.Errorf("error getting application: %w", err)
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, a.RBACName(s.ns)); err != nil {
		return nil, err
	}

	if a.Spec.Source.Plugin != nil && a.Spec.Source.Plugin.Name != "" {
		log.WithFields(map[string]interface{}{
			"application": a.Name,
			"plugin":      a.Spec.Source.Plugin.Name,
		}).Warnf(argocommon.ConfigMapPluginDeprecationWarning)
	}

	if !s.isNamespaceEnabled(a.Namespace) {
		return nil, namespaceNotPermittedError(a.Namespace)
	}

	var manifestInfo *apiclient.ManifestResponse
	err = s.queryRepoServer(ctx, a, func(
		client apiclient.RepoServerServiceClient, repo *appv1.Repository, helmRepos []*appv1.Repository, helmCreds []*appv1.RepoCreds, helmOptions *appv1.HelmOptions, kustomizeOptions *appv1.KustomizeOptions, enableGenerateManifests map[string]bool) error {
		revision := a.Spec.Source.TargetRevision
		if q.GetRevision() != "" {
			revision = q.GetRevision()
		}
		appInstanceLabelKey, err := s.settingsMgr.GetAppInstanceLabelKey()
		if err != nil {
			return fmt.Errorf("error getting app instance label key from settings: %w", err)
		}

		plugins, err := s.plugins()
		if err != nil {
			return fmt.Errorf("error getting plugins: %w", err)
		}
		config, err := s.getApplicationClusterConfig(ctx, a)
		if err != nil {
			return fmt.Errorf("error getting application cluster config: %w", err)
		}

		serverVersion, err := s.kubectl.GetServerVersion(config)
		if err != nil {
			return fmt.Errorf("error getting server version: %w", err)
		}

		apiResources, err := s.kubectl.GetAPIResources(config, false, kubecache.NewNoopSettings())
		if err != nil {
			return fmt.Errorf("error getting API resources: %w", err)
		}

		manifestInfo, err = client.GenerateManifest(ctx, &apiclient.ManifestRequest{
			Repo:               repo,
			Revision:           revision,
			AppLabelKey:        appInstanceLabelKey,
			AppName:            a.InstanceName(s.ns),
			Namespace:          a.Spec.Destination.Namespace,
			ApplicationSource:  &a.Spec.Source,
			Repos:              helmRepos,
			Plugins:            plugins,
			KustomizeOptions:   kustomizeOptions,
			KubeVersion:        serverVersion,
			ApiVersions:        argo.APIResourcesToStrings(apiResources, true),
			HelmRepoCreds:      helmCreds,
			HelmOptions:        helmOptions,
			TrackingMethod:     string(argoutil.GetTrackingMethod(s.settingsMgr)),
			EnabledSourceTypes: enableGenerateManifests,
		})
		if err != nil {
			return fmt.Errorf("error generating manifests: %w", err)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	for i, manifest := range manifestInfo.Manifests {
		obj := &unstructured.Unstructured{}
		err = json.Unmarshal([]byte(manifest), obj)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling manifest into unstructured: %w", err)
		}
		if obj.GetKind() == kube.SecretKind && obj.GroupVersionKind().Group == "" {
			obj, _, err = diff.HideSecretData(obj, nil)
			if err != nil {
				return nil, fmt.Errorf("error hiding secret data: %w", err)
			}
			data, err := json.Marshal(obj)
			if err != nil {
				return nil, fmt.Errorf("error marshaling manifest: %w", err)
			}
			manifestInfo.Manifests[i] = string(data)
		}
	}

	return manifestInfo, nil
}

func (s *Server) GetManifestsWithFiles(stream application.ApplicationService_GetManifestsWithFilesServer) error {
	ctx := stream.Context()
	query, err := manifeststream.ReceiveApplicationManifestQueryWithFiles(stream)

	if err != nil {
		return fmt.Errorf("error getting query: %w", err)
	}

	if query.Name == nil || *query.Name == "" {
		return fmt.Errorf("invalid request: application name is missing")
	}

	appName := query.GetName()
	appNs := s.appNamespaceOrDefault(query.GetAppNamespace())
	a, err := s.appLister.Applications(appNs).Get(appName)

	if err != nil {
		return fmt.Errorf("error getting application: %w", err)
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, a.RBACName(s.ns)); err != nil {
		return err
	}

	var manifestInfo *apiclient.ManifestResponse
	err = s.queryRepoServer(ctx, a, func(
		client apiclient.RepoServerServiceClient, repo *appv1.Repository, helmRepos []*appv1.Repository, helmCreds []*appv1.RepoCreds, helmOptions *appv1.HelmOptions, kustomizeOptions *appv1.KustomizeOptions, enableGenerateManifests map[string]bool) error {

		appInstanceLabelKey, err := s.settingsMgr.GetAppInstanceLabelKey()
		if err != nil {
			return fmt.Errorf("error getting app instance label key from settings: %w", err)
		}

		plugins, err := s.plugins()
		if err != nil {
			return fmt.Errorf("error getting plugins: %w", err)
		}
		config, err := s.getApplicationClusterConfig(ctx, a)
		if err != nil {
			return fmt.Errorf("error getting application cluster config: %w", err)
		}

		serverVersion, err := s.kubectl.GetServerVersion(config)
		if err != nil {
			return fmt.Errorf("error getting server version: %w", err)
		}

		apiResources, err := s.kubectl.GetAPIResources(config, false, kubecache.NewNoopSettings())
		if err != nil {
			return fmt.Errorf("error getting API resources: %w", err)
		}

		req := &apiclient.ManifestRequest{
			Repo:               repo,
			Revision:           a.Spec.Source.TargetRevision,
			AppLabelKey:        appInstanceLabelKey,
			AppName:            a.Name,
			Namespace:          a.Spec.Destination.Namespace,
			ApplicationSource:  &a.Spec.Source,
			Repos:              helmRepos,
			Plugins:            plugins,
			KustomizeOptions:   kustomizeOptions,
			KubeVersion:        serverVersion,
			ApiVersions:        argo.APIResourcesToStrings(apiResources, true),
			HelmRepoCreds:      helmCreds,
			HelmOptions:        helmOptions,
			TrackingMethod:     string(argoutil.GetTrackingMethod(s.settingsMgr)),
			EnabledSourceTypes: enableGenerateManifests,
		}

		repoStreamClient, err := client.GenerateManifestWithFiles(stream.Context())
		if err != nil {
			return fmt.Errorf("error opening stream: %w", err)
		}

		err = manifeststream.SendRepoStream(repoStreamClient, stream, req, *query.Checksum)
		if err != nil {
			return fmt.Errorf("error sending repo stream: %w", err)
		}

		resp, err := repoStreamClient.CloseAndRecv()
		if err != nil {
			return fmt.Errorf("error generating manifests: %w", err)
		}

		manifestInfo = resp
		return nil
	})

	if err != nil {
		return err
	}

	for i, manifest := range manifestInfo.Manifests {
		obj := &unstructured.Unstructured{}
		err = json.Unmarshal([]byte(manifest), obj)
		if err != nil {
			return fmt.Errorf("error unmarshaling manifest into unstructured: %w", err)
		}
		if obj.GetKind() == kube.SecretKind && obj.GroupVersionKind().Group == "" {
			obj, _, err = diff.HideSecretData(obj, nil)
			if err != nil {
				return fmt.Errorf("error hiding secret data: %w", err)
			}
			data, err := json.Marshal(obj)
			if err != nil {
				return fmt.Errorf("error marshaling manifest: %w", err)
			}
			manifestInfo.Manifests[i] = string(data)
		}
	}

	stream.SendAndClose(manifestInfo)
	return nil
}

// Get returns an application by name
func (s *Server) Get(ctx context.Context, q *application.ApplicationQuery) (*appv1.Application, error) {
	appName := q.GetName()
	appNs := s.appNamespaceOrDefault(q.GetAppNamespace())

	// We must use a client Get instead of an informer Get, because it's common to call Get immediately
	// following a Watch (which is not yet powered by an informer), and the Get must reflect what was
	// previously seen by the client.
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(appNs).Get(ctx, appName, metav1.GetOptions{
		ResourceVersion: q.GetResourceVersion(),
	})

	if err != nil {
		return nil, fmt.Errorf("error getting application: %w", err)
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, a.RBACName(s.ns)); err != nil {
		return nil, err
	}

	if a.Spec.Source.Plugin != nil && a.Spec.Source.Plugin.Name != "" {
		log.WithFields(map[string]interface{}{
			"application": a.Name,
			"plugin":      a.Spec.Source.Plugin.Name,
		}).Warnf(argocommon.ConfigMapPluginDeprecationWarning)
	}

	s.inferResourcesStatusHealth(a)

	if q.Refresh == nil {
		return a, nil
	}

	refreshType := appv1.RefreshTypeNormal
	if *q.Refresh == string(appv1.RefreshTypeHard) {
		refreshType = appv1.RefreshTypeHard
	}
	appIf := s.appclientset.ArgoprojV1alpha1().Applications(appNs)

	// subscribe early with buffered channel to ensure we don't miss events
	events := make(chan *appv1.ApplicationWatchEvent, watchAPIBufferSize)
	unsubscribe := s.appBroadcaster.Subscribe(events, func(event *appv1.ApplicationWatchEvent) bool {
		return event.Application.Name == appName && event.Application.Namespace == appNs
	})
	defer unsubscribe()

	app, err := argoutil.RefreshApp(appIf, appName, refreshType)
	if err != nil {
		return nil, fmt.Errorf("error refreshing the app: %w", err)
	}

	if refreshType == appv1.RefreshTypeHard {
		// force refresh cached application details
		if err := s.queryRepoServer(ctx, a, func(
			client apiclient.RepoServerServiceClient,
			repo *appv1.Repository,
			helmRepos []*appv1.Repository,
			_ []*appv1.RepoCreds,
			helmOptions *appv1.HelmOptions,
			kustomizeOptions *appv1.KustomizeOptions,
			enabledSourceTypes map[string]bool,
		) error {
			_, err := client.GetAppDetails(ctx, &apiclient.RepoServerAppDetailsQuery{
				Repo:               repo,
				Source:             &app.Spec.Source,
				AppName:            appName,
				KustomizeOptions:   kustomizeOptions,
				Repos:              helmRepos,
				NoCache:            true,
				TrackingMethod:     string(argoutil.GetTrackingMethod(s.settingsMgr)),
				EnabledSourceTypes: enabledSourceTypes,
				HelmOptions:        helmOptions,
			})
			return err
		}); err != nil {
			log.Warnf("Failed to force refresh application details: %v", err)
		}
	}

	minVersion := 0
	if minVersion, err = strconv.Atoi(app.ResourceVersion); err != nil {
		minVersion = 0
	}

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("application refresh deadline exceeded")
		case event := <-events:
			if appVersion, err := strconv.Atoi(event.Application.ResourceVersion); err == nil && appVersion > minVersion {
				annotations := event.Application.GetAnnotations()
				if annotations == nil {
					annotations = make(map[string]string)
				}
				if _, ok := annotations[appv1.AnnotationKeyRefresh]; !ok {
					return &event.Application, nil
				}
			}
		}
	}
}

// ListResourceEvents returns a list of event resources
func (s *Server) ListResourceEvents(ctx context.Context, q *application.ApplicationResourceEventsQuery) (*v1.EventList, error) {
	appName := q.GetName()
	appNs := s.appNamespaceOrDefault(q.GetAppNamespace())
	a, err := s.appLister.Applications(appNs).Get(appName)
	if err != nil {
		return nil, fmt.Errorf("error getting application: %w", err)
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, a.RBACName(s.ns)); err != nil {
		return nil, err
	}

	if a.Spec.Source.Plugin != nil && a.Spec.Source.Plugin.Name != "" {
		log.WithFields(map[string]interface{}{
			"application": a.Name,
			"plugin":      a.Spec.Source.Plugin.Name,
		}).Warnf(argocommon.ConfigMapPluginDeprecationWarning)
	}

	var (
		kubeClientset kubernetes.Interface
		fieldSelector string
		namespace     string
	)
	// There are two places where we get events. If we are getting application events, we query
	// our own cluster. If it is events on a resource on an external cluster, then we query the
	// external cluster using its rest.Config
	if q.GetResourceName() == "" && q.GetResourceUID() == "" {
		kubeClientset = s.kubeclientset
		namespace = a.Namespace
		fieldSelector = fields.SelectorFromSet(map[string]string{
			"involvedObject.name":      a.Name,
			"involvedObject.uid":       string(a.UID),
			"involvedObject.namespace": a.Namespace,
		}).String()
	} else {
		tree, err := s.getAppResources(ctx, a)
		if err != nil {
			return nil, fmt.Errorf("error getting app resources: %w", err)
		}
		found := false
		for _, n := range append(tree.Nodes, tree.OrphanedNodes...) {
			if n.ResourceRef.UID == q.GetResourceUID() && n.ResourceRef.Name == q.GetResourceName() && n.ResourceRef.Namespace == q.GetResourceNamespace() {
				found = true
				break
			}
		}
		if !found {
			return nil, status.Errorf(codes.InvalidArgument, "%s not found as part of application %s", q.GetResourceName(), q.GetName())
		}

		namespace = q.GetResourceNamespace()
		var config *rest.Config
		config, err = s.getApplicationClusterConfig(ctx, a)
		if err != nil {
			return nil, fmt.Errorf("error getting application cluster config: %w", err)
		}
		kubeClientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			return nil, fmt.Errorf("error creating kube client: %w", err)
		}
		fieldSelector = fields.SelectorFromSet(map[string]string{
			"involvedObject.name":      q.GetResourceName(),
			"involvedObject.uid":       q.GetResourceUID(),
			"involvedObject.namespace": namespace,
		}).String()
	}
	log.Infof("Querying for resource events with field selector: %s", fieldSelector)
	opts := metav1.ListOptions{FieldSelector: fieldSelector}
	list, err := kubeClientset.CoreV1().Events(namespace).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing resource events: %w", err)
	}
	return list, nil
}

func (s *Server) validateAndUpdateApp(ctx context.Context, newApp *appv1.Application, merge bool, validate bool) (*appv1.Application, error) {
	s.projectLock.RLock(newApp.Spec.GetProject())
	defer s.projectLock.RUnlock(newApp.Spec.GetProject())

	app, err := s.appclientset.ArgoprojV1alpha1().Applications(newApp.Namespace).Get(ctx, newApp.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting application: %w", err)
	}

	err = s.validateAndNormalizeApp(ctx, newApp, validate)
	if err != nil {
		return nil, fmt.Errorf("error validating and normalizing app: %w", err)
	}

	a, err := s.updateApp(app, newApp, ctx, merge)
	if err != nil {
		return nil, fmt.Errorf("error updating application: %w", err)
	}
	return a, nil
}

func mergeStringMaps(items ...map[string]string) map[string]string {
	res := make(map[string]string)
	for _, m := range items {
		if m == nil {
			continue
		}
		for k, v := range m {
			res[k] = v
		}
	}
	return res
}

var informerSyncTimeout = 2 * time.Second

// waitSync is a helper to wait until the application informer cache is synced after create/update.
// It waits until the app in the informer, has a resource version greater than the version in the
// supplied app, or after 2 seconds, whichever comes first. Returns true if synced.
// We use an informer cache for read operations (Get, List). Since the cache is only
// eventually consistent, it is possible that it doesn't reflect an application change immediately
// after a mutating API call (create/update). This function should be called after a creates &
// update to give a probable (but not guaranteed) chance of being up-to-date after the create/update.
func (s *Server) waitSync(app *appv1.Application) {
	logCtx := log.WithField("application", app.Name)
	deadline := time.Now().Add(informerSyncTimeout)
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

func (s *Server) updateApp(app *appv1.Application, newApp *appv1.Application, ctx context.Context, merge bool) (*appv1.Application, error) {
	for i := 0; i < 10; i++ {
		app.Spec = newApp.Spec
		if merge {
			app.Labels = mergeStringMaps(app.Labels, newApp.Labels)
			app.Annotations = mergeStringMaps(app.Annotations, newApp.Annotations)
		} else {
			app.Labels = newApp.Labels
			app.Annotations = newApp.Annotations
		}

		app.Finalizers = newApp.Finalizers

		res, err := s.appclientset.ArgoprojV1alpha1().Applications(app.Namespace).Update(ctx, app, metav1.UpdateOptions{})
		if err == nil {
			s.logAppEvent(app, ctx, argo.EventReasonResourceUpdated, "updated application spec")
			s.waitSync(res)
			return res, nil
		}
		if !apierr.IsConflict(err) {
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

// Update updates an application
func (s *Server) Update(ctx context.Context, q *application.ApplicationUpdateRequest) (*appv1.Application, error) {
	if q.GetApplication() == nil {
		return nil, fmt.Errorf("error creating application: application is nil in request")
	}
	a := q.GetApplication()
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, a.RBACName(s.ns)); err != nil {
		return nil, err
	}

	if a.Spec.Source.Plugin != nil && a.Spec.Source.Plugin.Name != "" {
		log.WithFields(map[string]interface{}{
			"application": a.Name,
			"plugin":      a.Spec.Source.Plugin.Name,
		}).Warnf(argocommon.ConfigMapPluginDeprecationWarning)
	}

	validate := true
	if q.Validate != nil {
		validate = *q.Validate
	}
	return s.validateAndUpdateApp(ctx, q.Application, false, validate)
}

// UpdateSpec updates an application spec and filters out any invalid parameter overrides
func (s *Server) UpdateSpec(ctx context.Context, q *application.ApplicationUpdateSpecRequest) (*appv1.ApplicationSpec, error) {
	if q.GetSpec() == nil {
		return nil, fmt.Errorf("error updating application spec: spec is nil in request")
	}
	appName := q.GetName()
	appNs := s.appNamespaceOrDefault(q.GetAppNamespace())
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(appNs).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting application: %w", err)
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, a.RBACName(s.ns)); err != nil {
		return nil, err
	}

	if a.Spec.Source.Plugin != nil && a.Spec.Source.Plugin.Name != "" {
		log.WithFields(map[string]interface{}{
			"application": a.Name,
			"plugin":      a.Spec.Source.Plugin.Name,
		}).Warnf(argocommon.ConfigMapPluginDeprecationWarning)
	}

	a.Spec = *q.GetSpec()
	validate := true
	if q.Validate != nil {
		validate = *q.Validate
	}
	a, err = s.validateAndUpdateApp(ctx, a, false, validate)
	if err != nil {
		return nil, fmt.Errorf("error validating and updating app: %w", err)
	}
	return &a.Spec, nil
}

// Patch patches an application
func (s *Server) Patch(ctx context.Context, q *application.ApplicationPatchRequest) (*appv1.Application, error) {
	appName := q.GetName()
	appNs := s.appNamespaceOrDefault(q.GetAppNamespace())
	app, err := s.appclientset.ArgoprojV1alpha1().Applications(appNs).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting application: %w", err)
	}

	if err = s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, app.RBACName(s.ns)); err != nil {
		return nil, err
	}

	if app.Spec.Source.Plugin != nil && app.Spec.Source.Plugin.Name != "" {
		log.WithFields(map[string]interface{}{
			"application": app.Name,
			"plugin":      app.Spec.Source.Plugin.Name,
		}).Warnf(argocommon.ConfigMapPluginDeprecationWarning)
	}

	jsonApp, err := json.Marshal(app)
	if err != nil {
		return nil, fmt.Errorf("error marshaling application: %w", err)
	}

	var patchApp []byte

	switch q.GetPatchType() {
	case "json", "":
		patch, err := jsonpatch.DecodePatch([]byte(q.GetPatch()))
		if err != nil {
			return nil, fmt.Errorf("error decoding json patch: %w", err)
		}
		patchApp, err = patch.Apply(jsonApp)
		if err != nil {
			return nil, fmt.Errorf("error applying patch: %w", err)
		}
	case "merge":
		patchApp, err = jsonpatch.MergePatch(jsonApp, []byte(q.GetPatch()))
		if err != nil {
			return nil, fmt.Errorf("error calculating merge patch: %w", err)
		}
	default:
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Patch type '%s' is not supported", q.GetPatchType()))
	}

	newApp := &v1alpha1.Application{}
	err = json.Unmarshal(patchApp, newApp)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling patched app: %w", err)
	}
	return s.validateAndUpdateApp(ctx, newApp, false, true)
}

// Delete removes an application and all associated resources
func (s *Server) Delete(ctx context.Context, q *application.ApplicationDeleteRequest) (*application.ApplicationResponse, error) {
	appName := q.GetName()
	appNs := s.appNamespaceOrDefault(q.GetAppNamespace())
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(appNs).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting application: %w", err)
	}

	s.projectLock.RLock(a.Spec.Project)
	defer s.projectLock.RUnlock(a.Spec.Project)

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionDelete, a.RBACName(s.ns)); err != nil {
		return nil, err
	}

	if a.Spec.Source.Plugin != nil && a.Spec.Source.Plugin.Name != "" {
		log.WithFields(map[string]interface{}{
			"application": a.Name,
			"plugin":      a.Spec.Source.Plugin.Name,
		}).Warnf(argocommon.ConfigMapPluginDeprecationWarning)
	}

	if q.Cascade != nil && !*q.Cascade && q.GetPropagationPolicy() != "" {
		return nil, status.Error(codes.InvalidArgument, "cannot set propagation policy when cascading is disabled")
	}

	patchFinalizer := false
	if q.Cascade == nil || *q.Cascade {
		// validate the propgation policy
		policyFinalizer := getPropagationPolicyFinalizer(q.GetPropagationPolicy())
		if policyFinalizer == "" {
			return nil, status.Errorf(codes.InvalidArgument, "invalid propagation policy: %s", *q.PropagationPolicy)
		}
		if !a.IsFinalizerPresent(policyFinalizer) {
			a.SetCascadedDeletion(policyFinalizer)
			patchFinalizer = true
		}
	} else {
		if a.CascadedDeletion() {
			a.UnSetCascadedDeletion()
			patchFinalizer = true
		}
	}

	if patchFinalizer {
		// Although the cascaded deletion/propagation policy finalizer is not set when apps are created via
		// API, they will often be set by the user as part of declarative config. As part of a delete
		// request, we always calculate the patch to see if we need to set/unset the finalizer.
		patch, err := json.Marshal(map[string]interface{}{
			"metadata": map[string]interface{}{
				"finalizers": a.Finalizers,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("error marshaling finalizers: %w", err)
		}
		_, err = s.appclientset.ArgoprojV1alpha1().Applications(a.Namespace).Patch(ctx, a.Name, types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			return nil, fmt.Errorf("error patching application with finalizers: %w", err)
		}
	}

	err = s.appclientset.ArgoprojV1alpha1().Applications(appNs).Delete(ctx, appName, metav1.DeleteOptions{})
	if err != nil {
		return nil, fmt.Errorf("error deleting application: %w", err)
	}
	s.logAppEvent(a, ctx, argo.EventReasonResourceDeleted, "deleted application")
	return &application.ApplicationResponse{}, nil
}

func (s *Server) Watch(q *application.ApplicationQuery, ws application.ApplicationService_WatchServer) error {
	appName := q.GetName()
	appNs := s.appNamespaceOrDefault(q.GetAppNamespace())
	logCtx := log.NewEntry(log.New())
	if q.Name != nil {
		logCtx = logCtx.WithField("application", *q.Name)
	}
	projects := map[string]bool{}
	for i := range q.Projects {
		projects[q.Projects[i]] = true
	}
	claims := ws.Context().Value("claims")
	selector, err := labels.Parse(q.GetSelector())
	if err != nil {
		return fmt.Errorf("error parsing labels with selectors: %w", err)
	}
	minVersion := 0
	if q.GetResourceVersion() != "" {
		if minVersion, err = strconv.Atoi(q.GetResourceVersion()); err != nil {
			minVersion = 0
		}
	}

	// sendIfPermitted is a helper to send the application to the client's streaming channel if the
	// caller has RBAC privileges permissions to view it
	sendIfPermitted := func(a appv1.Application, eventType watch.EventType) {
		if len(projects) > 0 && !projects[a.Spec.GetProject()] {
			return
		}

		if appVersion, err := strconv.Atoi(a.ResourceVersion); err == nil && appVersion < minVersion {
			return
		}
		matchedEvent := (appName == "" || (a.Name == appName && a.Namespace == appNs)) && selector.Matches(labels.Set(a.Labels))
		if !matchedEvent {
			return
		}

		if !s.enf.Enforce(claims, rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, a.RBACName(s.ns)) {
			// do not emit apps user does not have accessing
			return
		}
		s.inferResourcesStatusHealth(&a)
		err := ws.Send(&appv1.ApplicationWatchEvent{
			Type:        eventType,
			Application: a,
		})
		if err != nil {
			logCtx.Warnf("Unable to send stream message: %v", err)
			return
		}
	}

	events := make(chan *appv1.ApplicationWatchEvent, watchAPIBufferSize)
	// Mimic watch API behavior: send ADDED events if no resource version provided
	// If watch API is executed for one application when emit event even if resource version is provided
	// This is required since single app watch API is used for during operations like app syncing and it is
	// critical to never miss events.
	if q.GetResourceVersion() == "" || q.GetName() != "" {
		apps, err := s.appLister.List(selector)
		if err != nil {
			return fmt.Errorf("error listing apps with selector: %w", err)
		}
		sort.Slice(apps, func(i, j int) bool {
			return apps[i].QualifiedName() < apps[j].QualifiedName()
		})
		for i := range apps {
			sendIfPermitted(*apps[i], watch.Added)
		}
	}
	unsubscribe := s.appBroadcaster.Subscribe(events)
	defer unsubscribe()
	for {
		select {
		case event := <-events:
			sendIfPermitted(event.Application, event.Type)
		case <-ws.Context().Done():
			return nil
		}
	}
}

func (s *Server) validateAndNormalizeApp(ctx context.Context, app *appv1.Application, validate bool) error {
	proj, err := argo.GetAppProject(app, applisters.NewAppProjectLister(s.projInformer.GetIndexer()), s.ns, s.settingsMgr, s.db, ctx)
	if err != nil {
		if apierr.IsNotFound(err) {
			return status.Errorf(codes.InvalidArgument, "application references project %s which does not exist", app.Spec.Project)
		}
		return fmt.Errorf("error getting application's project: %w", err)
	}
	if app.GetName() == "" {
		return fmt.Errorf("resource name may not be empty")
	}
	appNs := s.appNamespaceOrDefault(app.Namespace)
	currApp, err := s.appclientset.ArgoprojV1alpha1().Applications(appNs).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		if !apierr.IsNotFound(err) {
			return fmt.Errorf("error getting application by name: %w", err)
		}
		// Kubernetes go-client will return a pointer to a zero-value app instead of nil, even
		// though the API response was NotFound. This behavior was confirmed via logs.
		currApp = nil
	}
	if currApp != nil && currApp.Spec.GetProject() != app.Spec.GetProject() {
		// When changing projects, caller must have application create & update privileges in new project
		// NOTE: the update check was already verified in the caller to this function
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionCreate, app.RBACName(s.ns)); err != nil {
			return err
		}
		// They also need 'update' privileges in the old project
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, currApp.RBACName(s.ns)); err != nil {
			return err
		}
	}

	// If source is Kustomize add build options
	kustomizeSettings, err := s.settingsMgr.GetKustomizeSettings()
	if err != nil {
		return fmt.Errorf("error getting kustomize settings: %w", err)
	}
	kustomizeOptions, err := kustomizeSettings.GetOptions(app.Spec.Source)
	if err != nil {
		return fmt.Errorf("error getting kustomize options from settings: %w", err)
	}
	plugins, err := s.plugins()
	if err != nil {
		return fmt.Errorf("error getting plugins: %w", err)
	}

	if err := argo.ValidateDestination(ctx, &app.Spec.Destination, s.db); err != nil {
		return status.Errorf(codes.InvalidArgument, "application destination spec for %s is invalid: %s", app.Name, err.Error())
	}

	var conditions []appv1.ApplicationCondition
	if validate {
		conditions, err = argo.ValidateRepo(ctx, app, s.repoClientset, s.db, kustomizeOptions, plugins, s.kubectl, proj, s.settingsMgr)
		if err != nil {
			return fmt.Errorf("error validating the repo: %w", err)
		}
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

	app.Spec = *argo.NormalizeApplicationSpec(&app.Spec)
	return nil
}

func (s *Server) getApplicationClusterConfig(ctx context.Context, a *appv1.Application) (*rest.Config, error) {
	if err := argo.ValidateDestination(ctx, &a.Spec.Destination, s.db); err != nil {
		return nil, fmt.Errorf("error validating destination: %w", err)
	}
	clst, err := s.db.GetCluster(ctx, a.Spec.Destination.Server)
	if err != nil {
		return nil, fmt.Errorf("error getting cluster: %w", err)
	}
	config := clst.RESTConfig()
	return config, err
}

// getCachedAppState loads the cached state and trigger app refresh if cache is missing
func (s *Server) getCachedAppState(ctx context.Context, a *appv1.Application, getFromCache func() error) error {
	err := getFromCache()
	if err != nil && err == servercache.ErrCacheMiss {
		conditions := a.Status.GetConditions(map[appv1.ApplicationConditionType]bool{
			appv1.ApplicationConditionComparisonError:  true,
			appv1.ApplicationConditionInvalidSpecError: true,
		})
		if len(conditions) > 0 {
			return errors.New(argoutil.FormatAppConditions(conditions))
		}
		_, err = s.Get(ctx, &application.ApplicationQuery{
			Name:         pointer.StringPtr(a.GetName()),
			AppNamespace: pointer.StringPtr(a.GetNamespace()),
			Refresh:      pointer.StringPtr(string(appv1.RefreshTypeNormal)),
		})
		if err != nil {
			return fmt.Errorf("error getting application by query: %w", err)
		}
		return getFromCache()
	}
	return err
}

func (s *Server) getAppResources(ctx context.Context, a *appv1.Application) (*appv1.ApplicationTree, error) {
	var tree appv1.ApplicationTree
	err := s.getCachedAppState(ctx, a, func() error {
		return s.cache.GetAppResourcesTree(a.InstanceName(s.ns), &tree)
	})
	if err != nil {
		return &tree, fmt.Errorf("error getting cached app state: %w", err)
	}
	return &tree, nil
}

func (s *Server) getAppLiveResource(ctx context.Context, action string, q *application.ApplicationResourceRequest) (*appv1.ResourceNode, *rest.Config, *appv1.Application, error) {
	appName := q.GetName()
	appNs := s.appNamespaceOrDefault(q.GetAppNamespace())
	a, err := s.appLister.Applications(appNs).Get(appName)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error getting app by name: %w", err)
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, action, a.RBACName(s.ns)); err != nil {
		return nil, nil, nil, err
	}

	tree, err := s.getAppResources(ctx, a)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error getting app resources: %w", err)
	}

	found := tree.FindNode(q.GetGroup(), q.GetKind(), q.GetNamespace(), q.GetResourceName())
	if found == nil || found.ResourceRef.UID == "" {
		return nil, nil, nil, status.Errorf(codes.InvalidArgument, "%s %s %s not found as part of application %s", q.GetKind(), q.GetGroup(), q.GetResourceName(), q.GetName())
	}
	config, err := s.getApplicationClusterConfig(ctx, a)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error getting application cluster config: %w", err)
	}
	return found, config, a, nil
}

func (s *Server) GetResource(ctx context.Context, q *application.ApplicationResourceRequest) (*application.ApplicationResourceResponse, error) {
	res, config, _, err := s.getAppLiveResource(ctx, rbacpolicy.ActionGet, q)
	if err != nil {
		return nil, fmt.Errorf("error getting app live resource: %w", err)
	}

	// make sure to use specified resource version if provided
	if q.GetVersion() != "" {
		res.Version = q.GetVersion()
	}
	obj, err := s.kubectl.GetResource(ctx, config, res.GroupKindVersion(), res.Name, res.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting resource: %w", err)
	}
	obj, err = replaceSecretValues(obj)
	if err != nil {
		return nil, fmt.Errorf("error replacing secret values: %w", err)
	}
	data, err := json.Marshal(obj.Object)
	if err != nil {
		return nil, fmt.Errorf("error marshaling object: %w", err)
	}
	manifest := string(data)
	return &application.ApplicationResourceResponse{Manifest: &manifest}, nil
}

func replaceSecretValues(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if obj.GetKind() == kube.SecretKind && obj.GroupVersionKind().Group == "" {
		_, obj, err := diff.HideSecretData(nil, obj)
		if err != nil {
			return nil, err
		}
		return obj, err
	}
	return obj, nil
}

// PatchResource patches a resource
func (s *Server) PatchResource(ctx context.Context, q *application.ApplicationResourcePatchRequest) (*application.ApplicationResourceResponse, error) {
	resourceRequest := &application.ApplicationResourceRequest{
		Name:         q.Name,
		AppNamespace: q.AppNamespace,
		Namespace:    q.Namespace,
		ResourceName: q.ResourceName,
		Kind:         q.Kind,
		Version:      q.Version,
		Group:        q.Group,
	}
	res, config, a, err := s.getAppLiveResource(ctx, rbacpolicy.ActionUpdate, resourceRequest)
	if err != nil {
		return nil, fmt.Errorf("error getting app live resource: %w", err)
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionUpdate, a.RBACName(s.ns)); err != nil {
		return nil, err
	}

	manifest, err := s.kubectl.PatchResource(ctx, config, res.GroupKindVersion(), res.Name, res.Namespace, types.PatchType(q.GetPatchType()), []byte(q.GetPatch()))
	if err != nil {
		// don't expose real error for secrets since it might contain secret data
		if res.Kind == kube.SecretKind && res.Group == "" {
			return nil, fmt.Errorf("failed to patch Secret %s/%s", res.Namespace, res.Name)
		}
		return nil, fmt.Errorf("error patching resource: %w", err)
	}
	manifest, err = replaceSecretValues(manifest)
	if err != nil {
		return nil, fmt.Errorf("error replacing secret values: %w", err)
	}
	data, err := json.Marshal(manifest.Object)
	if err != nil {
		return nil, fmt.Errorf("erro marshaling manifest object: %w", err)
	}
	s.logAppEvent(a, ctx, argo.EventReasonResourceUpdated, fmt.Sprintf("patched resource %s/%s '%s'", q.GetGroup(), q.GetKind(), q.GetResourceName()))
	m := string(data)
	return &application.ApplicationResourceResponse{
		Manifest: &m,
	}, nil
}

// DeleteResource deletes a specified resource
func (s *Server) DeleteResource(ctx context.Context, q *application.ApplicationResourceDeleteRequest) (*application.ApplicationResponse, error) {
	resourceRequest := &application.ApplicationResourceRequest{
		Name:         q.Name,
		AppNamespace: q.AppNamespace,
		Namespace:    q.Namespace,
		ResourceName: q.ResourceName,
		Kind:         q.Kind,
		Version:      q.Version,
		Group:        q.Group,
	}
	res, config, a, err := s.getAppLiveResource(ctx, rbacpolicy.ActionDelete, resourceRequest)
	if err != nil {
		return nil, fmt.Errorf("error getting live resource for delete: %w", err)
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionDelete, a.RBACName(s.ns)); err != nil {
		return nil, err
	}
	var deleteOption metav1.DeleteOptions
	if q.GetOrphan() {
		propagationPolicy := metav1.DeletePropagationOrphan
		deleteOption = metav1.DeleteOptions{PropagationPolicy: &propagationPolicy}
	} else if q.GetForce() {
		propagationPolicy := metav1.DeletePropagationBackground
		zeroGracePeriod := int64(0)
		deleteOption = metav1.DeleteOptions{PropagationPolicy: &propagationPolicy, GracePeriodSeconds: &zeroGracePeriod}
	} else {
		propagationPolicy := metav1.DeletePropagationForeground
		deleteOption = metav1.DeleteOptions{PropagationPolicy: &propagationPolicy}
	}
	err = s.kubectl.DeleteResource(ctx, config, res.GroupKindVersion(), res.Name, res.Namespace, deleteOption)
	if err != nil {
		return nil, fmt.Errorf("error deleting resource: %w", err)
	}
	s.logAppEvent(a, ctx, argo.EventReasonResourceDeleted, fmt.Sprintf("deleted resource %s/%s '%s'", q.GetGroup(), q.GetKind(), q.GetResourceName()))
	return &application.ApplicationResponse{}, nil
}

func (s *Server) ResourceTree(ctx context.Context, q *application.ResourcesQuery) (*appv1.ApplicationTree, error) {
	appName := q.GetApplicationName()
	appNs := s.appNamespaceOrDefault(q.GetAppNamespace())
	a, err := s.appLister.Applications(appNs).Get(appName)
	if err != nil {
		return nil, fmt.Errorf("error getting application by name: %w", err)
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, a.RBACName(s.ns)); err != nil {
		return nil, err
	}

	if a.Spec.Source.Plugin != nil && a.Spec.Source.Plugin.Name != "" {
		log.WithFields(map[string]interface{}{
			"application": a.Name,
			"plugin":      a.Spec.Source.Plugin.Name,
		}).Warnf(argocommon.ConfigMapPluginDeprecationWarning)
	}

	return s.getAppResources(ctx, a)
}

func (s *Server) WatchResourceTree(q *application.ResourcesQuery, ws application.ApplicationService_WatchResourceTreeServer) error {
	appName := q.GetApplicationName()
	appNs := s.appNamespaceOrDefault(q.GetAppNamespace())
	a, err := s.appLister.Applications(appNs).Get(appName)
	if err != nil {
		return fmt.Errorf("error getting application by name: %w", err)
	}

	if err := s.enf.EnforceErr(ws.Context().Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, a.RBACName(s.ns)); err != nil {
		return err
	}

	return s.cache.OnAppResourcesTreeChanged(ws.Context(), q.GetApplicationName(), func() error {
		var tree appv1.ApplicationTree
		err := s.cache.GetAppResourcesTree(q.GetApplicationName(), &tree)
		if err != nil {
			return fmt.Errorf("error getting app resource tree: %w", err)
		}
		return ws.Send(&tree)
	})
}

func (s *Server) RevisionMetadata(ctx context.Context, q *application.RevisionMetadataQuery) (*v1alpha1.RevisionMetadata, error) {
	appName := q.GetName()
	appNs := s.appNamespaceOrDefault(q.GetAppNamespace())
	a, err := s.appLister.Applications(appNs).Get(appName)
	if err != nil {
		return nil, fmt.Errorf("error getting app by name: %w", err)
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, a.RBACName(s.ns)); err != nil {
		return nil, err
	}

	if a.Spec.Source.Plugin != nil && a.Spec.Source.Plugin.Name != "" {
		log.WithFields(map[string]interface{}{
			"application": a.Name,
			"plugin":      a.Spec.Source.Plugin.Name,
		}).Warnf(argocommon.ConfigMapPluginDeprecationWarning)
	}

	repo, err := s.db.GetRepository(ctx, a.Spec.Source.RepoURL)
	if err != nil {
		return nil, fmt.Errorf("error getting repository by URL: %w", err)
	}
	// We need to get some information with the project associated to the app,
	// so we'll know whether GPG signatures are enforced.
	proj, err := argo.GetAppProject(a, applisters.NewAppProjectLister(s.projInformer.GetIndexer()), s.ns, s.settingsMgr, s.db, ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting app project: %w", err)
	}
	conn, repoClient, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, fmt.Errorf("error creating repo server client: %w", err)
	}
	defer ioutil.Close(conn)
	return repoClient.GetRevisionMetadata(ctx, &apiclient.RepoServerRevisionMetadataRequest{
		Repo:           repo,
		Revision:       q.GetRevision(),
		CheckSignature: len(proj.Spec.SignatureKeys) > 0,
	})
}

func isMatchingResource(q *application.ResourcesQuery, key kube.ResourceKey) bool {
	return (q.GetName() == "" || q.GetName() == key.Name) &&
		(q.GetNamespace() == "" || q.GetNamespace() == key.Namespace) &&
		(q.GetGroup() == "" || q.GetGroup() == key.Group) &&
		(q.GetKind() == "" || q.GetKind() == key.Kind)
}

func (s *Server) ManagedResources(ctx context.Context, q *application.ResourcesQuery) (*application.ManagedResourcesResponse, error) {
	appName := q.GetApplicationName()
	appNs := s.appNamespaceOrDefault(q.GetAppNamespace())
	a, err := s.appLister.Applications(appNs).Get(appName)
	if err != nil {
		return nil, fmt.Errorf("error getting application: %w", err)
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, a.RBACName(s.ns)); err != nil {
		return nil, fmt.Errorf("error verifying rbac: %w", err)
	}

	if a.Spec.Source.Plugin != nil && a.Spec.Source.Plugin.Name != "" {
		log.WithFields(map[string]interface{}{
			"application": a.Name,
			"plugin":      a.Spec.Source.Plugin.Name,
		}).Warnf(argocommon.ConfigMapPluginDeprecationWarning)
	}

	items := make([]*appv1.ResourceDiff, 0)
	err = s.getCachedAppState(ctx, a, func() error {
		return s.cache.GetAppManagedResources(a.InstanceName(s.ns), &items)
	})
	if err != nil {
		return nil, fmt.Errorf("error getting cached app state: %w", err)
	}
	res := &application.ManagedResourcesResponse{}
	for i := range items {
		item := items[i]
		if isMatchingResource(q, kube.ResourceKey{Name: item.Name, Namespace: item.Namespace, Kind: item.Kind, Group: item.Group}) {
			res.Items = append(res.Items, item)
		}
	}

	return res, nil
}

func (s *Server) PodLogs(q *application.ApplicationPodLogsQuery, ws application.ApplicationService_PodLogsServer) error {
	if q.PodName != nil {
		podKind := "Pod"
		q.Kind = &podKind
		q.ResourceName = q.PodName
	}

	var sinceSeconds, tailLines *int64
	if q.GetSinceSeconds() > 0 {
		sinceSeconds = pointer.Int64(q.GetSinceSeconds())
	}
	if q.GetTailLines() > 0 {
		tailLines = pointer.Int64(q.GetTailLines())
	}
	var untilTime *metav1.Time
	if q.GetUntilTime() != "" {
		if val, err := time.Parse(time.RFC3339Nano, q.GetUntilTime()); err != nil {
			return fmt.Errorf("invalid untilTime parameter value: %v", err)
		} else {
			untilTimeVal := metav1.NewTime(val)
			untilTime = &untilTimeVal
		}
	}

	literal := ""
	inverse := false
	if q.GetFilter() != "" {
		literal = *q.Filter
		if literal[0] == '!' {
			literal = literal[1:]
			inverse = true
		}
	}

	appName := q.GetName()
	appNs := s.appNamespaceOrDefault(q.GetAppNamespace())
	a, err := s.appLister.Applications(appNs).Get(appName)
	if err != nil {
		return fmt.Errorf("error getting application by name: %w", err)
	}

	if err := s.enf.EnforceErr(ws.Context().Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, a.RBACName(s.ns)); err != nil {
		return err
	}

	if a.Spec.Source.Plugin != nil && a.Spec.Source.Plugin.Name != "" {
		log.WithFields(map[string]interface{}{
			"application": a.Name,
			"plugin":      a.Spec.Source.Plugin.Name,
		}).Warnf(argocommon.ConfigMapPluginDeprecationWarning)
	}

	// Logs RBAC will be enforced only if an internal var serverRBACLogEnforceEnable (representing server.rbac.log.enforce.enable env var)
	// is defined and has a "true" value
	// Otherwise, no RBAC enforcement for logs will take place (meaning, PodLogs will return the logs,
	// even if there is no explicit RBAC allow, or if there is an explicit RBAC deny)
	serverRBACLogEnforceEnable, err := s.settingsMgr.GetServerRBACLogEnforceEnable()
	if err != nil {
		return fmt.Errorf("error getting RBAC log enforce enable: %w", err)
	}

	if serverRBACLogEnforceEnable {
		if err := s.enf.EnforceErr(ws.Context().Value("claims"), rbacpolicy.ResourceLogs, rbacpolicy.ActionGet, a.RBACName(s.ns)); err != nil {
			return err
		}
	}

	tree, err := s.getAppResources(ws.Context(), a)
	if err != nil {
		return fmt.Errorf("error getting app resource tree: %w", err)
	}

	config, err := s.getApplicationClusterConfig(ws.Context(), a)
	if err != nil {
		return fmt.Errorf("error getting application cluster config: %w", err)
	}

	kubeClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating kube client: %w", err)
	}

	// from the tree find pods which match query of kind, group, and resource name
	pods := getSelectedPods(tree.Nodes, q)
	if len(pods) == 0 {
		return nil
	}

	if len(pods) > maxPodLogsToRender {
		return errors.New("Max pods to view logs are reached. Please provide more granular query.")
	}

	var streams []chan logEntry

	for _, pod := range pods {
		stream, err := kubeClientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{
			Container:    q.GetContainer(),
			Follow:       q.GetFollow(),
			Timestamps:   true,
			SinceSeconds: sinceSeconds,
			SinceTime:    q.GetSinceTime(),
			TailLines:    tailLines,
			Previous:     q.GetPrevious(),
		}).Stream(ws.Context())
		podName := pod.Name
		logStream := make(chan logEntry)
		if err == nil {
			defer ioutil.Close(stream)
		}

		streams = append(streams, logStream)
		go func() {
			// if k8s failed to start steaming logs (typically because Pod is not ready yet)
			// then the error should be shown in the UI so that user know the reason
			if err != nil {
				logStream <- logEntry{line: err.Error()}
			} else {
				parseLogsStream(podName, stream, logStream)
			}
			close(logStream)
		}()
	}

	logStream := mergeLogStreams(streams, time.Millisecond*100)
	sentCount := int64(0)
	done := make(chan error)
	go func() {
		for entry := range logStream {
			if entry.err != nil {
				done <- entry.err
				return
			} else {
				if q.Filter != nil {
					lineContainsFilter := strings.Contains(entry.line, literal)
					if (inverse && lineContainsFilter) || (!inverse && !lineContainsFilter) {
						continue
					}
				}
				ts := metav1.NewTime(entry.timeStamp)
				if untilTime != nil && entry.timeStamp.After(untilTime.Time) {
					done <- ws.Send(&application.LogEntry{
						Last:         pointer.Bool(true),
						PodName:      &entry.podName,
						Content:      &entry.line,
						TimeStampStr: pointer.String(entry.timeStamp.Format(time.RFC3339Nano)),
						TimeStamp:    &ts,
					})
					return
				} else {
					sentCount++
					if err := ws.Send(&application.LogEntry{
						PodName:      &entry.podName,
						Content:      &entry.line,
						TimeStampStr: pointer.String(entry.timeStamp.Format(time.RFC3339Nano)),
						TimeStamp:    &ts,
						Last:         pointer.Bool(false),
					}); err != nil {
						done <- err
						break
					}
				}
			}
		}
		now := time.Now()
		nowTS := metav1.NewTime(now)
		done <- ws.Send(&application.LogEntry{
			Last:         pointer.Bool(true),
			PodName:      pointer.String(""),
			Content:      pointer.String(""),
			TimeStampStr: pointer.String(now.Format(time.RFC3339Nano)),
			TimeStamp:    &nowTS,
		})
	}()

	select {
	case err := <-done:
		return err
	case <-ws.Context().Done():
		log.WithField("application", q.Name).Debug("k8s pod logs reader completed due to closed grpc context")
		return nil
	}
}

// from all of the treeNodes, get the pod who meets the criteria or whose parents meets the criteria
func getSelectedPods(treeNodes []appv1.ResourceNode, q *application.ApplicationPodLogsQuery) []appv1.ResourceNode {
	var pods []appv1.ResourceNode
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
func isTheSelectedOne(currentNode *appv1.ResourceNode, q *application.ApplicationPodLogsQuery, resourceNodes []appv1.ResourceNode, isTheOneMap map[string]bool) bool {
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
		//look up parentResource from resourceNodes
		//then check if the parent isTheSelectedOne
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

// Sync syncs an application to its target state
func (s *Server) Sync(ctx context.Context, syncReq *application.ApplicationSyncRequest) (*appv1.Application, error) {
	appName := syncReq.GetName()
	appNs := s.appNamespaceOrDefault(syncReq.GetAppNamespace())
	appIf := s.appclientset.ArgoprojV1alpha1().Applications(appNs)
	a, err := appIf.Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting application by name: %w", err)
	}

	proj, err := argo.GetAppProject(a, applisters.NewAppProjectLister(s.projInformer.GetIndexer()), s.ns, s.settingsMgr, s.db, ctx)
	if err != nil {
		if apierr.IsNotFound(err) {
			return a, status.Errorf(codes.InvalidArgument, "application references project %s which does not exist", a.Spec.Project)
		}
		return a, fmt.Errorf("error getting app project: %w", err)
	}

	s.inferResourcesStatusHealth(a)

	if !proj.Spec.SyncWindows.Matches(a).CanSync(true) {
		return a, status.Errorf(codes.PermissionDenied, "cannot sync: blocked by sync window")
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionSync, a.RBACName(s.ns)); err != nil {
		return nil, err
	}

	if a.Spec.Source.Plugin != nil && a.Spec.Source.Plugin.Name != "" {
		log.WithFields(map[string]interface{}{
			"application": a.Name,
			"plugin":      a.Spec.Source.Plugin.Name,
		}).Warnf(argocommon.ConfigMapPluginDeprecationWarning)
	}

	if syncReq.Manifests != nil {
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionOverride, a.RBACName(s.ns)); err != nil {
			return nil, err
		}
		if a.Spec.SyncPolicy != nil && a.Spec.SyncPolicy.Automated != nil && !syncReq.GetDryRun() {
			return nil, status.Error(codes.FailedPrecondition, "cannot use local sync when Automatic Sync Policy is enabled unless for dry run")
		}
	}
	if a.DeletionTimestamp != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "application is deleting")
	}
	if a.Spec.SyncPolicy != nil && a.Spec.SyncPolicy.Automated != nil {
		if syncReq.GetRevision() != "" && syncReq.GetRevision() != text.FirstNonEmpty(a.Spec.Source.TargetRevision, "HEAD") {
			return nil, status.Errorf(codes.FailedPrecondition, "Cannot sync to %s: auto-sync currently set to %s", syncReq.GetRevision(), a.Spec.Source.TargetRevision)
		}
	}
	revision, displayRevision, err := s.resolveRevision(ctx, a, syncReq)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, err.Error())
	}

	var retry *appv1.RetryStrategy
	var syncOptions appv1.SyncOptions
	if a.Spec.SyncPolicy != nil {
		syncOptions = a.Spec.SyncPolicy.SyncOptions
		retry = a.Spec.SyncPolicy.Retry
	}
	if syncReq.RetryStrategy != nil {
		retry = syncReq.RetryStrategy
	}
	if syncReq.SyncOptions != nil {
		syncOptions = syncReq.SyncOptions.Items
	}

	// We cannot use local manifests if we're only allowed to sync to signed commits
	if syncReq.Manifests != nil && len(proj.Spec.SignatureKeys) > 0 {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot use local sync when signature keys are required.")
	}

	resources := []appv1.SyncOperationResource{}
	if syncReq.GetResources() != nil {
		for _, r := range syncReq.GetResources() {
			if r != nil {
				resources = append(resources, *r)
			}
		}
	}
	op := appv1.Operation{
		Sync: &appv1.SyncOperation{
			Revision:     revision,
			Prune:        syncReq.GetPrune(),
			DryRun:       syncReq.GetDryRun(),
			SyncOptions:  syncOptions,
			SyncStrategy: syncReq.Strategy,
			Resources:    resources,
			Manifests:    syncReq.Manifests,
		},
		InitiatedBy: appv1.OperationInitiator{Username: session.Username(ctx)},
		Info:        syncReq.Infos,
	}
	if retry != nil {
		op.Retry = *retry
	}

	a, err = argo.SetAppOperation(appIf, appName, &op)
	if err != nil {
		return nil, fmt.Errorf("error setting app operation: %w", err)
	}
	partial := ""
	if len(syncReq.Resources) > 0 {
		partial = "partial "
	}
	reason := fmt.Sprintf("initiated %ssync to %s", partial, displayRevision)
	if syncReq.Manifests != nil {
		reason = fmt.Sprintf("initiated %ssync locally", partial)
	}
	s.logAppEvent(a, ctx, argo.EventReasonOperationStarted, reason)
	return a, nil
}

func (s *Server) Rollback(ctx context.Context, rollbackReq *application.ApplicationRollbackRequest) (*appv1.Application, error) {
	appName := rollbackReq.GetName()
	appNs := s.appNamespaceOrDefault(rollbackReq.GetAppNamespace())
	appIf := s.appclientset.ArgoprojV1alpha1().Applications(appNs)
	a, err := appIf.Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting application by name: %w", err)
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionSync, a.RBACName(s.ns)); err != nil {
		return nil, err
	}

	if a.Spec.Source.Plugin != nil && a.Spec.Source.Plugin.Name != "" {
		log.WithFields(map[string]interface{}{
			"application": a.Name,
			"plugin":      a.Spec.Source.Plugin.Name,
		}).Warnf(argocommon.ConfigMapPluginDeprecationWarning)
	}

	s.inferResourcesStatusHealth(a)

	if a.DeletionTimestamp != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "application is deleting")
	}
	if a.Spec.SyncPolicy != nil && a.Spec.SyncPolicy.Automated != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "rollback cannot be initiated when auto-sync is enabled")
	}

	var deploymentInfo *appv1.RevisionHistory
	for _, info := range a.Status.History {
		if info.ID == rollbackReq.GetId() {
			deploymentInfo = &info
			break
		}
	}
	if deploymentInfo == nil {
		return nil, status.Errorf(codes.InvalidArgument, "application %s does not have deployment with id %v", a.QualifiedName(), rollbackReq.GetId())
	}
	if deploymentInfo.Source.IsZero() {
		// Since source type was introduced to history starting with v0.12, and is now required for
		// rollback, we cannot support rollback to revisions deployed using Argo CD v0.11 or below
		return nil, status.Errorf(codes.FailedPrecondition, "cannot rollback to revision deployed with Argo CD v0.11 or lower. sync to revision instead.")
	}

	var syncOptions appv1.SyncOptions
	if a.Spec.SyncPolicy != nil {
		syncOptions = a.Spec.SyncPolicy.SyncOptions
	}

	// Rollback is just a convenience around Sync
	op := appv1.Operation{
		Sync: &appv1.SyncOperation{
			Revision:     deploymentInfo.Revision,
			DryRun:       rollbackReq.GetDryRun(),
			Prune:        rollbackReq.GetPrune(),
			SyncOptions:  syncOptions,
			SyncStrategy: &appv1.SyncStrategy{Apply: &appv1.SyncStrategyApply{}},
			Source:       &deploymentInfo.Source,
		},
		InitiatedBy: appv1.OperationInitiator{Username: session.Username(ctx)},
	}
	a, err = argo.SetAppOperation(appIf, appName, &op)
	if err != nil {
		return nil, fmt.Errorf("error setting app operation: %w", err)
	}
	s.logAppEvent(a, ctx, argo.EventReasonOperationStarted, fmt.Sprintf("initiated rollback to %d", rollbackReq.GetId()))
	return a, nil
}

// resolveRevision resolves the revision specified either in the sync request, or the
// application source, into a concrete revision that will be used for a sync operation.
func (s *Server) resolveRevision(ctx context.Context, app *appv1.Application, syncReq *application.ApplicationSyncRequest) (string, string, error) {
	if syncReq.Manifests != nil {
		return "", "", nil
	}
	ambiguousRevision := syncReq.GetRevision()
	if ambiguousRevision == "" {
		ambiguousRevision = app.Spec.Source.TargetRevision
	}
	repo, err := s.db.GetRepository(ctx, app.Spec.Source.RepoURL)
	if err != nil {
		return "", "", fmt.Errorf("error getting repository by URL: %w", err)
	}
	conn, repoClient, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return "", "", fmt.Errorf("error getting repo server client: %w", err)
	}
	defer ioutil.Close(conn)

	if !app.Spec.Source.IsHelm() {
		if git.IsCommitSHA(ambiguousRevision) {
			// If it's already a commit SHA, then no need to look it up
			return ambiguousRevision, ambiguousRevision, nil
		}
	}

	resolveRevisionResponse, err := repoClient.ResolveRevision(ctx, &apiclient.ResolveRevisionRequest{
		Repo:              repo,
		App:               app,
		AmbiguousRevision: ambiguousRevision,
	})
	if err != nil {
		return "", "", fmt.Errorf("error resolving repo revision: %w", err)
	}
	return resolveRevisionResponse.Revision, resolveRevisionResponse.AmbiguousRevision, nil
}

func (s *Server) TerminateOperation(ctx context.Context, termOpReq *application.OperationTerminateRequest) (*application.OperationTerminateResponse, error) {
	appName := termOpReq.GetName()
	appNs := s.appNamespaceOrDefault(termOpReq.GetAppNamespace())
	a, err := s.appclientset.ArgoprojV1alpha1().Applications(appNs).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting application by name: %w", err)
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionSync, a.RBACName(s.ns)); err != nil {
		return nil, err
	}

	for i := 0; i < 10; i++ {
		if a.Operation == nil || a.Status.OperationState == nil {
			return nil, status.Errorf(codes.InvalidArgument, "Unable to terminate operation. No operation is in progress")
		}
		a.Status.OperationState.Phase = common.OperationTerminating
		updated, err := s.appclientset.ArgoprojV1alpha1().Applications(appNs).Update(ctx, a, metav1.UpdateOptions{})
		if err == nil {
			s.waitSync(updated)
			s.logAppEvent(a, ctx, argo.EventReasonResourceUpdated, "terminated running operation")
			return &application.OperationTerminateResponse{}, nil
		}
		if !apierr.IsConflict(err) {
			return nil, fmt.Errorf("error updating application: %w", err)
		}
		log.Warnf("failed to set operation for app %q due to update conflict. retrying again...", *termOpReq.Name)
		time.Sleep(100 * time.Millisecond)
		a, err = s.appclientset.ArgoprojV1alpha1().Applications(appNs).Get(ctx, appName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error getting application by name: %w", err)
		}
	}
	return nil, status.Errorf(codes.Internal, "Failed to terminate app. Too many conflicts")
}

func (s *Server) logAppEvent(a *appv1.Application, ctx context.Context, reason string, action string) {
	eventInfo := argo.EventInfo{Type: v1.EventTypeNormal, Reason: reason}
	user := session.Username(ctx)
	if user == "" {
		user = "Unknown user"
	}
	message := fmt.Sprintf("%s %s", user, action)
	s.auditLogger.LogAppEvent(a, eventInfo, message)
}

func (s *Server) logResourceEvent(res *appv1.ResourceNode, ctx context.Context, reason string, action string) {
	eventInfo := argo.EventInfo{Type: v1.EventTypeNormal, Reason: reason}
	user := session.Username(ctx)
	if user == "" {
		user = "Unknown user"
	}
	message := fmt.Sprintf("%s %s", user, action)
	s.auditLogger.LogResourceEvent(res, eventInfo, message)
}

func (s *Server) ListResourceActions(ctx context.Context, q *application.ApplicationResourceRequest) (*application.ResourceActionsListResponse, error) {
	obj, _, _, _, err := s.getUnstructuredLiveResourceOrApp(ctx, rbacpolicy.ActionGet, q)
	if err != nil {
		return nil, err
	}
	resourceOverrides, err := s.settingsMgr.GetResourceOverrides()
	if err != nil {
		return nil, fmt.Errorf("error getting resource overrides: %w", err)
	}

	availableActions, err := s.getAvailableActions(resourceOverrides, obj)
	if err != nil {
		return nil, fmt.Errorf("error getting available actions: %w", err)
	}
	actionsPtr := []*appv1.ResourceAction{}
	for i := range availableActions {
		actionsPtr = append(actionsPtr, &availableActions[i])
	}

	return &application.ResourceActionsListResponse{Actions: actionsPtr}, nil
}

func (s *Server) getUnstructuredLiveResourceOrApp(ctx context.Context, rbacRequest string, q *application.ApplicationResourceRequest) (obj *unstructured.Unstructured, res *appv1.ResourceNode, app *appv1.Application, config *rest.Config, err error) {
	if q.GetKind() == "Application" && q.GetGroup() == "argoproj.io" && q.GetName() == q.GetResourceName() {
		namespace := s.appNamespaceOrDefault(q.GetAppNamespace())
		app, err = s.appLister.Applications(namespace).Get(q.GetName())
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("error getting app by name: %w", err)
		}
		if err = s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacRequest, app.RBACName(s.ns)); err != nil {
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
			return nil, nil, nil, nil, fmt.Errorf("error getting app live resource: %w", err)
		}
		obj, err = s.kubectl.GetResource(ctx, config, res.GroupKindVersion(), res.Name, res.Namespace)
	}
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("error getting resource: %w", err)
	}
	return
}

func (s *Server) getAvailableActions(resourceOverrides map[string]appv1.ResourceOverride, obj *unstructured.Unstructured) ([]appv1.ResourceAction, error) {
	luaVM := lua.VM{
		ResourceOverrides: resourceOverrides,
	}

	discoveryScript, err := luaVM.GetResourceActionDiscovery(obj)
	if err != nil {
		return nil, fmt.Errorf("error getting Lua discovery script: %w", err)
	}
	if discoveryScript == "" {
		return []appv1.ResourceAction{}, nil
	}
	availableActions, err := luaVM.ExecuteResourceActionDiscovery(obj, discoveryScript)
	if err != nil {
		return nil, fmt.Errorf("error executing Lua discovery script: %w", err)
	}
	return availableActions, nil

}

func (s *Server) RunResourceAction(ctx context.Context, q *application.ResourceActionRunRequest) (*application.ApplicationResponse, error) {
	resourceRequest := &application.ApplicationResourceRequest{
		Name:         q.Name,
		AppNamespace: q.AppNamespace,
		Namespace:    q.Namespace,
		ResourceName: q.ResourceName,
		Kind:         q.Kind,
		Version:      q.Version,
		Group:        q.Group,
	}
	actionRequest := fmt.Sprintf("%s/%s/%s/%s", rbacpolicy.ActionAction, q.GetGroup(), q.GetKind(), q.GetAction())
	liveObj, res, a, config, err := s.getUnstructuredLiveResourceOrApp(ctx, actionRequest, resourceRequest)
	if err != nil {
		return nil, err
	}

	resourceOverrides, err := s.settingsMgr.GetResourceOverrides()
	if err != nil {
		return nil, fmt.Errorf("error getting resource overrides: %w", err)
	}

	luaVM := lua.VM{
		ResourceOverrides: resourceOverrides,
	}
	action, err := luaVM.GetResourceAction(liveObj, q.GetAction())
	if err != nil {
		return nil, fmt.Errorf("error getting Lua resource action: %w", err)
	}

	newObj, err := luaVM.ExecuteResourceAction(liveObj, action.ActionLua)
	if err != nil {
		return nil, fmt.Errorf("error executing Lua resource action: %w", err)
	}

	newObjBytes, err := json.Marshal(newObj)
	if err != nil {
		return nil, fmt.Errorf("error marshaling new object: %w", err)
	}

	liveObjBytes, err := json.Marshal(liveObj)
	if err != nil {
		return nil, fmt.Errorf("error marshaling live object: %w", err)
	}

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
			if !apierr.IsNotFound(err) {
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

	if res == nil {
		s.logAppEvent(a, ctx, argo.EventReasonResourceActionRan, fmt.Sprintf("ran action %s", q.GetAction()))
	} else {
		s.logAppEvent(a, ctx, argo.EventReasonResourceActionRan, fmt.Sprintf("ran action %s on resource %s/%s/%s", q.GetAction(), res.Group, res.Kind, res.Name))
		s.logResourceEvent(res, ctx, argo.EventReasonResourceActionRan, fmt.Sprintf("ran action %s", q.GetAction()))
	}
	return &application.ApplicationResponse{}, nil
}

// splitStatusPatch splits a patch into two: one for a non-status patch, and the status-only patch.
// Returns nil for either if the patch doesn't have modifications to non-status, or status, respectively.
func splitStatusPatch(patch []byte) ([]byte, []byte, error) {
	var obj map[string]interface{}
	err := json.Unmarshal(patch, &obj)
	if err != nil {
		return nil, nil, err
	}
	var nonStatusPatch, statusPatch []byte
	if statusVal, ok := obj["status"]; ok {
		// calculate the status-only patch
		statusObj := map[string]interface{}{
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

func (s *Server) plugins() ([]*v1alpha1.ConfigManagementPlugin, error) {
	plugins, err := s.settingsMgr.GetConfigManagementPlugins()
	if err != nil {
		return nil, fmt.Errorf("error getting config management plugin: %w", err)
	}
	tools := make([]*v1alpha1.ConfigManagementPlugin, len(plugins))
	for i, p := range plugins {
		p := p
		tools[i] = &p
	}
	return tools, nil
}

func (s *Server) GetApplicationSyncWindows(ctx context.Context, q *application.ApplicationSyncWindowsQuery) (*application.ApplicationSyncWindowsResponse, error) {
	appName := q.GetName()
	appNs := s.appNamespaceOrDefault(q.GetAppNamespace())
	appIf := s.appclientset.ArgoprojV1alpha1().Applications(appNs)
	a, err := appIf.Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting application by name: %w", err)
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, a.RBACName(s.ns)); err != nil {
		return nil, err
	}

	if a.Spec.Source.Plugin != nil && a.Spec.Source.Plugin.Name != "" {
		log.WithFields(map[string]interface{}{
			"application": a.Name,
			"plugin":      a.Spec.Source.Plugin.Name,
		}).Warnf(argocommon.ConfigMapPluginDeprecationWarning)
	}

	proj, err := argo.GetAppProject(a, applisters.NewAppProjectLister(s.projInformer.GetIndexer()), s.ns, s.settingsMgr, s.db, ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting app project: %w", err)
	}

	windows := proj.Spec.SyncWindows.Matches(a)
	sync := windows.CanSync(true)

	res := &application.ApplicationSyncWindowsResponse{
		ActiveWindows:   convertSyncWindows(windows.Active()),
		AssignedWindows: convertSyncWindows(windows),
		CanSync:         &sync,
	}

	return res, nil
}

func (s *Server) inferResourcesStatusHealth(app *v1alpha1.Application) {
	if app.Status.ResourceHealthSource == v1alpha1.ResourceHealthLocationAppTree {
		tree := &v1alpha1.ApplicationTree{}
		if err := s.cache.GetAppResourcesTree(app.Name, tree); err == nil {
			healthByKey := map[kube.ResourceKey]*v1alpha1.HealthStatus{}
			for _, node := range tree.Nodes {
				healthByKey[kube.NewResourceKey(node.Group, node.Kind, node.Namespace, node.Name)] = node.Health
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
		return appv1.BackgroundPropagationPolicyFinalizer
	case foregroundPropagationPolicy:
		return appv1.ForegroundPropagationPolicyFinalizer
	case "":
		return appv1.ResourcesFinalizerName
	default:
		return ""
	}
}

func (s *Server) appNamespaceOrDefault(appNs string) string {
	if appNs == "" {
		return s.ns
	} else {
		return appNs
	}
}

func (s *Server) isNamespaceEnabled(namespace string) bool {
	return namespace == s.ns || glob.MatchStringInList(s.enabledNamespaces, namespace, false)
}

func namespaceNotPermittedError(namespace string) error {
	return fmt.Errorf("namespace '%s' is not permitted", namespace)
}
