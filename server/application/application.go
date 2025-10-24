package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"

	kubecache "github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/pkg/v2/sync"
	jsonpatch "github.com/evanphx/json-patch"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"

	argocommon "github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"

	appclientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
	applisters "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	servercache "github.com/argoproj/argo-cd/v3/server/cache"
	"github.com/argoproj/argo-cd/v3/server/deeplinks"
	applog "github.com/argoproj/argo-cd/v3/util/app/log"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/env"
	"github.com/argoproj/argo-cd/v3/util/git"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/lua"
	"github.com/argoproj/argo-cd/v3/util/manifeststream"
	"github.com/argoproj/argo-cd/v3/util/rbac"
	"github.com/argoproj/argo-cd/v3/util/security"
	"github.com/argoproj/argo-cd/v3/util/session"
	"github.com/argoproj/argo-cd/v3/util/settings"

	resourceutil "github.com/argoproj/gitops-engine/pkg/sync/resource"

	argodiff "github.com/argoproj/argo-cd/v3/util/argo/diff"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
	kubeutil "github.com/argoproj/argo-cd/v3/util/kube"
)

type AppResourceTreeFn func(ctx context.Context, app *v1alpha1.Application) (*v1alpha1.ApplicationTree, error)

const (
	backgroundPropagationPolicy string = "background"
	foregroundPropagationPolicy string = "foreground"
)

var (
	InformerSyncTimeout = 2 * time.Second
	ErrCacheMiss        = cacheutil.ErrCacheMiss
	watchAPIBufferSize  = env.ParseNumFromEnv(argocommon.EnvWatchAPIBufferSize, 1000, 0, math.MaxInt32)
)

// Server provides an Application service
type Server struct {
	ns                     string
	kubeclientset          kubernetes.Interface
	appclientset           appclientset.Interface
	appLister              applisters.ApplicationLister
	appInformer            cache.SharedIndexInformer
	appBroadcaster         Broadcaster
	repoClientset          apiclient.Clientset
	kubectl                kube.Kubectl
	db                     db.ArgoDB
	enf                    *rbac.Enforcer
	projectLock            sync.KeyLock
	auditLogger            *argo.AuditLogger
	settingsMgr            *settings.SettingsManager
	cache                  *servercache.Cache
	projInformer           cache.SharedIndexInformer
	enabledNamespaces      []string
	syncWithReplaceAllowed bool
}

// NewServer returns a new instance of the Application service
func NewServer(
	namespace string,
	kubeclientset kubernetes.Interface,
	appclientset appclientset.Interface,
	appLister applisters.ApplicationLister,
	appInformer cache.SharedIndexInformer,
	appBroadcaster Broadcaster,
	repoClientset apiclient.Clientset,
	cache *servercache.Cache,
	kubectl kube.Kubectl,
	db db.ArgoDB,
	enf *rbac.Enforcer,
	projectLock sync.KeyLock,
	settingsMgr *settings.SettingsManager,
	projInformer cache.SharedIndexInformer,
	enabledNamespaces []string,
	enableK8sEvent []string,
	syncWithReplaceAllowed bool,
) (application.ApplicationServiceServer, AppResourceTreeFn) {
	if appBroadcaster == nil {
		appBroadcaster = &broadcasterHandler{}
	}
	_, err := appInformer.AddEventHandler(appBroadcaster)
	if err != nil {
		log.Error(err)
	}
	s := &Server{
		ns:                     namespace,
		appclientset:           &deepCopyAppClientset{appclientset},
		appLister:              &deepCopyApplicationLister{appLister},
		appInformer:            appInformer,
		appBroadcaster:         appBroadcaster,
		kubeclientset:          kubeclientset,
		cache:                  cache,
		db:                     db,
		repoClientset:          repoClientset,
		kubectl:                kubectl,
		enf:                    enf,
		projectLock:            projectLock,
		auditLogger:            argo.NewAuditLogger(kubeclientset, "argocd-server", enableK8sEvent),
		settingsMgr:            settingsMgr,
		projInformer:           projInformer,
		enabledNamespaces:      enabledNamespaces,
		syncWithReplaceAllowed: syncWithReplaceAllowed,
	}
	return s, s.getAppResources
}

// List returns list of applications
func (s *Server) List(ctx context.Context, q *application.ApplicationQuery) (*v1alpha1.ApplicationList, error) {
	selector, err := labels.Parse(q.GetSelector())
	if err != nil {
		return nil, fmt.Errorf("error parsing the selector: %w", err)
	}
	var apps []*v1alpha1.Application
	if q.GetAppNamespace() == "" {
		apps, err = s.appLister.List(selector)
	} else {
		apps, err = s.appLister.Applications(q.GetAppNamespace()).List(selector)
	}
	if err != nil {
		return nil, fmt.Errorf("error listing apps with selectors: %w", err)
	}

	filteredApps := apps
	// Filter applications by name
	if q.Name != nil {
		filteredApps = argo.FilterByNameP(filteredApps, *q.Name)
	}

	// Filter applications by projects
	filteredApps = argo.FilterByProjectsP(filteredApps, getProjectsFromApplicationQuery(*q))

	// Filter applications by source repo URL
	filteredApps = argo.FilterByRepoP(filteredApps, q.GetRepo())

	newItems := make([]v1alpha1.Application, 0)
	for _, a := range filteredApps {
		// Skip any application that is neither in the control plane's namespace
		// nor in the list of enabled namespaces.
		if !s.isNamespaceEnabled(a.Namespace) {
			continue
		}
		if s.enf.Enforce(ctx.Value("claims"), rbac.ResourceApplications, rbac.ActionGet, a.RBACName(s.ns)) {
			// Create a deep copy to ensure all metadata fields including annotations are preserved
			appCopy := a.DeepCopy()
			// Explicitly copy annotations in case DeepCopy does not preserve them
			if a.Annotations != nil {
				appCopy.Annotations = a.Annotations
			}
			newItems = append(newItems, *appCopy)
		}
	}

	// Sort found applications by name
	sort.Slice(newItems, func(i, j int) bool {
		return newItems[i].Name < newItems[j].Name
	})

	appList := v1alpha1.ApplicationList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: s.appInformer.LastSyncResourceVersion(),
		},
		Items: newItems,
	}
	return &appList, nil
}

// Create creates an application
func (s *Server) Create(ctx context.Context, q *application.ApplicationCreateRequest) (*v1alpha1.Application, error) {
	if q.GetApplication() == nil {
		return nil, errors.New("error creating application: application is nil in request")
	}
	a := q.GetApplication()

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplications, rbac.ActionCreate, a.RBACName(s.ns)); err != nil {
		return nil, err
	}

	s.projectLock.RLock(a.Spec.GetProject())
	defer s.projectLock.RUnlock(a.Spec.GetProject())

	validate := true
	if q.Validate != nil {
		validate = *q.Validate
	}

	proj, err := s.getAppProject(ctx, a, log.WithFields(applog.GetAppLogFields(a)))
	if err != nil {
		return nil, err
	}

	err = s.validateAndNormalizeApp(ctx, a, proj, validate)
	if err != nil {
		return nil, fmt.Errorf("error while validating and normalizing app: %w", err)
	}

	appNs := s.appNamespaceOrDefault(a.Namespace)

	if !s.isNamespaceEnabled(appNs) {
		return nil, security.NamespaceNotPermittedError(appNs)
	}

	// Don't let the app creator set the operation explicitly. Those requests should always go through the Sync API.
	if a.Operation != nil {
		log.WithFields(applog.GetAppLogFields(a)).
			WithFields(log.Fields{
				argocommon.SecurityField: argocommon.SecurityLow,
			}).Warn("User attempted to set operation on application creation. This could have allowed them to bypass branch protection rules by setting manifests directly. Ignoring the set operation.")
		a.Operation = nil
	}

	created, err := s.appclientset.ArgoprojV1alpha1().Applications(appNs).Create(ctx, a, metav1.CreateOptions{})
	if err == nil {
		s.logAppEvent(ctx, created, argo.EventReasonResourceCreated, "created application")
		s.waitSync(created)
		return created, nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("error creating application: %w", err)
	}

	// act idempotent if existing spec matches new spec
	existing, err := s.appLister.Applications(appNs).Get(a.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to check existing application details (%s): %v", appNs, err)
	}

	equalSpecs := reflect.DeepEqual(existing.Spec.Destination, a.Spec.Destination) &&
		reflect.DeepEqual(existing.Spec, a.Spec) &&
		reflect.DeepEqual(existing.Labels, a.Labels) &&
		reflect.DeepEqual(existing.Annotations, a.Annotations) &&
		reflect.DeepEqual(existing.Finalizers, a.Finalizers)

	if equalSpecs {
		return existing, nil
	}
	if q.Upsert == nil || !*q.Upsert {
		return nil, status.Errorf(codes.InvalidArgument, "existing application spec is different, use upsert flag to force update")
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplications, rbac.ActionUpdate, a.RBACName(s.ns)); err != nil {
		return nil, err
	}
	updated, err := s.updateApp(ctx, existing, a, true)
	if err != nil {
		return nil, fmt.Errorf("error updating application: %w", err)
	}
	return updated, nil
}

// Get returns an application by name

func (s *Server) Get(ctx context.Context, q *application.ApplicationQuery) (*v1alpha1.Application, error) {
	appName := q.GetName()
	appNs := s.appNamespaceOrDefault(q.GetAppNamespace())

	project := ""
	projects := getProjectsFromApplicationQuery(*q)
	if len(projects) == 1 {
		project = projects[0]
	} else if len(projects) > 1 {
		return nil, status.Errorf(codes.InvalidArgument, "multiple projects specified - the get endpoint accepts either zero or one project")
	}

	// We must use a client Get instead of an informer Get, because it's common to call Get immediately
	// following a Watch (which is not yet powered by an informer), and the Get must reflect what was
	// previously seen by the client.
	a, proj, err := s.getApplicationEnforceRBACClient(ctx, rbac.ActionGet, project, appNs, appName, q.GetResourceVersion())
	if err != nil {
		return nil, err
	}

	if q.Refresh == nil {
		s.inferResourcesStatusHealth(a)
		return a.DeepCopy(), nil
	}

	refreshType := v1alpha1.RefreshTypeNormal
	if *q.Refresh == string(v1alpha1.RefreshTypeHard) {
		refreshType = v1alpha1.RefreshTypeHard
	}
	appIf := s.appclientset.ArgoprojV1alpha1().Applications(appNs)

	// subscribe early with buffered channel to ensure we don't miss events
	events := make(chan *v1alpha1.ApplicationWatchEvent, watchAPIBufferSize)
	unsubscribe := s.appBroadcaster.Subscribe(events, func(event *v1alpha1.ApplicationWatchEvent) bool {
		return event.Application.Name == appName && event.Application.Namespace == appNs
	})
	defer unsubscribe()

	app, err := argo.RefreshApp(appIf, appName, refreshType, true)
	if err != nil {
		return nil, fmt.Errorf("error refreshing the app: %w", err)
	}

	if refreshType == v1alpha1.RefreshTypeHard {
		// force refresh cached application details
		if err := s.queryRepoServer(ctx, proj, func(
			client apiclient.RepoServerServiceClient,
			helmRepos []*v1alpha1.Repository,
			_ []*v1alpha1.RepoCreds,
			_ []*v1alpha1.Repository,
			_ []*v1alpha1.RepoCreds,
			helmOptions *v1alpha1.HelmOptions,
			enabledSourceTypes map[string]bool,
		) error {
			source := app.Spec.GetSource()
			repo, err := s.db.GetRepository(ctx, a.Spec.GetSource().RepoURL, proj.Name)
			if err != nil {
				return fmt.Errorf("error getting repository: %w", err)
			}
			kustomizeSettings, err := s.settingsMgr.GetKustomizeSettings()
			if err != nil {
				return fmt.Errorf("error getting kustomize settings: %w", err)
			}
			trackingMethod, err := s.settingsMgr.GetTrackingMethod()
			if err != nil {
				return fmt.Errorf("error getting trackingMethod from settings: %w", err)
			}
			_, err = client.GetAppDetails(ctx, &apiclient.RepoServerAppDetailsQuery{
				Repo:               repo,
				Source:             &source,
				AppName:            appName,
				KustomizeOptions:   kustomizeSettings,
				Repos:              helmRepos,
				NoCache:            true,
				TrackingMethod:     trackingMethod,
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
			return nil, errors.New("application refresh deadline exceeded")
		case event := <-events:
			if appVersion, err := strconv.Atoi(event.Application.ResourceVersion); err == nil && appVersion > minVersion {
				annotations := event.Application.GetAnnotations()
				if annotations == nil {
					annotations = make(map[string]string)
				}
				if _, ok := annotations[v1alpha1.AnnotationKeyRefresh]; !ok {
					refreshedApp := event.Application.DeepCopy()
					s.inferResourcesStatusHealth(refreshedApp)
					return refreshedApp, nil
				}
			}
		}
	}
}

// GetManifests returns application manifests
func (s *Server) GetManifests(ctx context.Context, q *application.ApplicationManifestQuery) (*apiclient.ManifestResponse, error) {
	if q.Name == nil || *q.Name == "" {
		return nil, errors.New("invalid request: application name is missing")
	}
	a, proj, err := s.getApplicationEnforceRBACInformer(ctx, rbac.ActionGet, q.GetProject(), q.GetAppNamespace(), q.GetName())
	if err != nil {
		return nil, err
	}

	if !s.isNamespaceEnabled(a.Namespace) {
		return nil, security.NamespaceNotPermittedError(a.Namespace)
	}

	manifestInfos := make([]*apiclient.ManifestResponse, 0)
	err = s.queryRepoServer(ctx, proj, func(
		client apiclient.RepoServerServiceClient, helmRepos []*v1alpha1.Repository, helmCreds []*v1alpha1.RepoCreds, ociRepos []*v1alpha1.Repository, ociCreds []*v1alpha1.RepoCreds, helmOptions *v1alpha1.HelmOptions, enableGenerateManifests map[string]bool,
	) error {
		appInstanceLabelKey, err := s.settingsMgr.GetAppInstanceLabelKey()
		if err != nil {
			return fmt.Errorf("error getting app instance label key from settings: %w", err)
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

		sources := make([]v1alpha1.ApplicationSource, 0)
		appSpec := a.Spec
		if a.Spec.HasMultipleSources() {
			numOfSources := int64(len(a.Spec.GetSources()))
			for i, pos := range q.SourcePositions {
				if pos <= 0 || pos > numOfSources {
					return errors.New("source position is out of range")
				}
				appSpec.Sources[pos-1].TargetRevision = q.Revisions[i]
			}
			sources = appSpec.GetSources()
		} else {
			source := a.Spec.GetSource()
			if q.GetRevision() != "" {
				source.TargetRevision = q.GetRevision()
			}
			sources = append(sources, source)
		}

		// Store the map of all sources having ref field into a map for applications with sources field
		refSources, err := argo.GetRefSources(context.Background(), sources, appSpec.Project, s.db.GetRepository, []string{})
		if err != nil {
			return fmt.Errorf("failed to get ref sources: %w", err)
		}

		for _, source := range sources {
			repo, err := s.db.GetRepository(ctx, source.RepoURL, proj.Name)
			if err != nil {
				return fmt.Errorf("error getting repository: %w", err)
			}

			kustomizeSettings, err := s.settingsMgr.GetKustomizeSettings()
			if err != nil {
				return fmt.Errorf("error getting kustomize settings: %w", err)
			}

			installationID, err := s.settingsMgr.GetInstallationID()
			if err != nil {
				return fmt.Errorf("error getting installation ID: %w", err)
			}
			trackingMethod, err := s.settingsMgr.GetTrackingMethod()
			if err != nil {
				return fmt.Errorf("error getting trackingMethod from settings: %w", err)
			}

			repos := helmRepos
			helmRepoCreds := helmCreds
			// If the source is OCI, there is a potential for an OCI image to be a Helm chart and that said chart in
			// turn would have OCI dependencies. To ensure that those dependencies can be resolved, add them to the repos
			// list.
			if source.IsOCI() {
				repos = slices.Clone(helmRepos)
				helmRepoCreds = slices.Clone(helmCreds)
				repos = append(repos, ociRepos...)
				helmRepoCreds = append(helmRepoCreds, ociCreds...)
			}

			manifestInfo, err := client.GenerateManifest(ctx, &apiclient.ManifestRequest{
				Repo:                            repo,
				Revision:                        source.TargetRevision,
				AppLabelKey:                     appInstanceLabelKey,
				AppName:                         a.InstanceName(s.ns),
				Namespace:                       a.Spec.Destination.Namespace,
				ApplicationSource:               &source,
				Repos:                           repos,
				KustomizeOptions:                kustomizeSettings,
				KubeVersion:                     serverVersion,
				ApiVersions:                     argo.APIResourcesToStrings(apiResources, true),
				HelmRepoCreds:                   helmRepoCreds,
				HelmOptions:                     helmOptions,
				TrackingMethod:                  trackingMethod,
				EnabledSourceTypes:              enableGenerateManifests,
				ProjectName:                     proj.Name,
				ProjectSourceRepos:              proj.Spec.SourceRepos,
				HasMultipleSources:              a.Spec.HasMultipleSources(),
				RefSources:                      refSources,
				AnnotationManifestGeneratePaths: a.GetAnnotation(v1alpha1.AnnotationKeyManifestGeneratePaths),
				InstallationID:                  installationID,
				NoCache:                         q.NoCache != nil && *q.NoCache,
			})
			if err != nil {
				return fmt.Errorf("error generating manifests: %w", err)
			}
			manifestInfos = append(manifestInfos, manifestInfo)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	manifests := &apiclient.ManifestResponse{}
	for _, manifestInfo := range manifestInfos {
		for i, manifest := range manifestInfo.Manifests {
			obj := &unstructured.Unstructured{}
			err = json.Unmarshal([]byte(manifest), obj)
			if err != nil {
				return nil, fmt.Errorf("error unmarshaling manifest into unstructured: %w", err)
			}
			if obj.GetKind() == kube.SecretKind && obj.GroupVersionKind().Group == "" {
				obj, _, err = diff.HideSecretData(obj, nil, s.settingsMgr.GetSensitiveAnnotations())
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
		manifests.Manifests = append(manifests.Manifests, manifestInfo.Manifests...)
	}

	return manifests, nil
}

func (s *Server) GetManifestsWithFiles(stream application.ApplicationService_GetManifestsWithFilesServer) error {
	ctx := stream.Context()
	query, err := manifeststream.ReceiveApplicationManifestQueryWithFiles(stream)
	if err != nil {
		return fmt.Errorf("error getting query: %w", err)
	}

	if query.Name == nil || *query.Name == "" {
		return errors.New("invalid request: application name is missing")
	}

	a, proj, err := s.getApplicationEnforceRBACInformer(ctx, rbac.ActionGet, query.GetProject(), query.GetAppNamespace(), query.GetName())
	if err != nil {
		return err
	}

	var manifestInfo *apiclient.ManifestResponse
	err = s.queryRepoServer(ctx, proj, func(
		client apiclient.RepoServerServiceClient, helmRepos []*v1alpha1.Repository, helmCreds []*v1alpha1.RepoCreds, _ []*v1alpha1.Repository, _ []*v1alpha1.RepoCreds, helmOptions *v1alpha1.HelmOptions, enableGenerateManifests map[string]bool,
	) error {
		appInstanceLabelKey, err := s.settingsMgr.GetAppInstanceLabelKey()
		if err != nil {
			return fmt.Errorf("error getting app instance label key from settings: %w", err)
		}

		trackingMethod, err := s.settingsMgr.GetTrackingMethod()
		if err != nil {
			return fmt.Errorf("error getting trackingMethod from settings: %w", err)
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

		source := a.Spec.GetSource()

		proj, err := argo.GetAppProject(ctx, a, applisters.NewAppProjectLister(s.projInformer.GetIndexer()), s.ns, s.settingsMgr, s.db)
		if err != nil {
			return fmt.Errorf("error getting app project: %w", err)
		}

		repo, err := s.db.GetRepository(ctx, a.Spec.GetSource().RepoURL, proj.Name)
		if err != nil {
			return fmt.Errorf("error getting repository: %w", err)
		}

		kustomizeSettings, err := s.settingsMgr.GetKustomizeSettings()
		if err != nil {
			return fmt.Errorf("error getting kustomize settings: %w", err)
		}

		req := &apiclient.ManifestRequest{
			Repo:                            repo,
			Revision:                        source.TargetRevision,
			AppLabelKey:                     appInstanceLabelKey,
			AppName:                         a.Name,
			Namespace:                       a.Spec.Destination.Namespace,
			ApplicationSource:               &source,
			Repos:                           helmRepos,
			KustomizeOptions:                kustomizeSettings,
			KubeVersion:                     serverVersion,
			ApiVersions:                     argo.APIResourcesToStrings(apiResources, true),
			HelmRepoCreds:                   helmCreds,
			HelmOptions:                     helmOptions,
			TrackingMethod:                  trackingMethod,
			EnabledSourceTypes:              enableGenerateManifests,
			ProjectName:                     proj.Name,
			ProjectSourceRepos:              proj.Spec.SourceRepos,
			AnnotationManifestGeneratePaths: a.GetAnnotation(v1alpha1.AnnotationKeyManifestGeneratePaths),
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
			obj, _, err = diff.HideSecretData(obj, nil, s.settingsMgr.GetSensitiveAnnotations())
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

// ListResourceEvents returns a list of event resources
func (s *Server) ListResourceEvents(ctx context.Context, q *application.ApplicationResourceEventsQuery) (*corev1.EventList, error) {
	a, _, err := s.getApplicationEnforceRBACInformer(ctx, rbac.ActionGet, q.GetProject(), q.GetAppNamespace(), q.GetName())
	if err != nil {
		return nil, err
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
			if n.UID == q.GetResourceUID() && n.Name == q.GetResourceName() && n.Namespace == q.GetResourceNamespace() {
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
	return list.DeepCopy(), nil
}

// Update updates an application
func (s *Server) Update(ctx context.Context, q *application.ApplicationUpdateRequest) (*v1alpha1.Application, error) {
	if q.GetApplication() == nil {
		return nil, errors.New("error updating application: application is nil in request")
	}
	a := q.GetApplication()
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplications, rbac.ActionUpdate, a.RBACName(s.ns)); err != nil {
		return nil, err
	}

	validate := true
	if q.Validate != nil {
		validate = *q.Validate
	}
	return s.validateAndUpdateApp(ctx, q.Application, false, validate, rbac.ActionUpdate, q.GetProject())
}

// UpdateSpec updates an application spec and filters out any invalid parameter overrides
func (s *Server) UpdateSpec(ctx context.Context, q *application.ApplicationUpdateSpecRequest) (*v1alpha1.ApplicationSpec, error) {
	if q.GetSpec() == nil {
		return nil, errors.New("error updating application spec: spec is nil in request")
	}
	a, _, err := s.getApplicationEnforceRBACClient(ctx, rbac.ActionUpdate, q.GetProject(), q.GetAppNamespace(), q.GetName(), "")
	if err != nil {
		return nil, err
	}

	a.Spec = *q.GetSpec()
	validate := true
	if q.Validate != nil {
		validate = *q.Validate
	}
	a, err = s.validateAndUpdateApp(ctx, a, false, validate, rbac.ActionUpdate, q.GetProject())
	if err != nil {
		return nil, fmt.Errorf("error validating and updating app: %w", err)
	}
	return &a.Spec, nil
}

// Patch patches an application
func (s *Server) Patch(ctx context.Context, q *application.ApplicationPatchRequest) (*v1alpha1.Application, error) {
	app, _, err := s.getApplicationEnforceRBACClient(ctx, rbac.ActionGet, q.GetProject(), q.GetAppNamespace(), q.GetName(), "")
	if err != nil {
		return nil, err
	}

	err = s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplications, rbac.ActionUpdate, app.RBACName(s.ns))
	if err != nil {
		return nil, err
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
	return s.validateAndUpdateApp(ctx, newApp, false, true, rbac.ActionUpdate, q.GetProject())
}

// Delete removes an application and all associated resources
func (s *Server) Delete(ctx context.Context, q *application.ApplicationDeleteRequest) (*application.ApplicationResponse, error) {
	appName := q.GetName()
	appNs := s.appNamespaceOrDefault(q.GetAppNamespace())
	a, _, err := s.getApplicationEnforceRBACClient(ctx, rbac.ActionGet, q.GetProject(), appNs, appName, "")
	if err != nil {
		return nil, err
	}

	s.projectLock.RLock(a.Spec.Project)
	defer s.projectLock.RUnlock(a.Spec.Project)

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplications, rbac.ActionDelete, a.RBACName(s.ns)); err != nil {
		return nil, err
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
	} else if a.CascadedDeletion() {
		a.UnSetCascadedDeletion()
		patchFinalizer = true
	}

	if patchFinalizer {
		// Although the cascaded deletion/propagation policy finalizer is not set when apps are created via
		// API, they will often be set by the user as part of declarative config. As part of a delete
		// request, we always calculate the patch to see if we need to set/unset the finalizer.
		patch, err := json.Marshal(map[string]any{
			"metadata": map[string]any{
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
	s.logAppEvent(ctx, a, argo.EventReasonResourceDeleted, "deleted application")
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
	for _, project := range getProjectsFromApplicationQuery(*q) {
		projects[project] = true
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
	sendIfPermitted := func(a v1alpha1.Application, eventType watch.EventType) {
		permitted := s.isApplicationPermitted(selector, minVersion, claims, appName, appNs, projects, a)
		if !permitted {
			return
		}
		s.inferResourcesStatusHealth(&a)
		err := ws.Send(&v1alpha1.ApplicationWatchEvent{
			Type:        eventType,
			Application: a,
		})
		if err != nil {
			logCtx.Warnf("Unable to send stream message: %v", err)
			return
		}
	}

	events := make(chan *v1alpha1.ApplicationWatchEvent, watchAPIBufferSize)
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

func (s *Server) GetResource(ctx context.Context, q *application.ApplicationResourceRequest) (*application.ApplicationResourceResponse, error) {
	res, config, _, err := s.getAppLiveResource(ctx, rbac.ActionGet, q)
	if err != nil {
		return nil, err
	}

	// make sure to use specified resource version if provided
	if q.GetVersion() != "" {
		res.Version = q.GetVersion()
	}
	obj, err := s.kubectl.GetResource(ctx, config, res.GroupKindVersion(), res.Name, res.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting resource: %w", err)
	}
	obj, err = s.replaceSecretValues(obj)
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
		Project:      q.Project,
	}
	res, config, a, err := s.getAppLiveResource(ctx, rbac.ActionUpdate, resourceRequest)
	if err != nil {
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
	if manifest == nil {
		return nil, errors.New("failed to patch resource: manifest was nil")
	}
	manifest, err = s.replaceSecretValues(manifest)
	if err != nil {
		return nil, fmt.Errorf("error replacing secret values: %w", err)
	}
	data, err := json.Marshal(manifest.Object)
	if err != nil {
		return nil, fmt.Errorf("erro marshaling manifest object: %w", err)
	}
	s.logAppEvent(ctx, a, argo.EventReasonResourceUpdated, fmt.Sprintf("patched resource %s/%s '%s'", q.GetGroup(), q.GetKind(), q.GetResourceName()))
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
		Project:      q.Project,
	}
	res, config, a, err := s.getAppLiveResource(ctx, rbac.ActionDelete, resourceRequest)
	if err != nil {
		return nil, err
	}
	var deleteOption metav1.DeleteOptions
	switch {
	case q.GetOrphan():
		propagationPolicy := metav1.DeletePropagationOrphan
		deleteOption = metav1.DeleteOptions{PropagationPolicy: &propagationPolicy}
	case q.GetForce():
		propagationPolicy := metav1.DeletePropagationBackground
		zeroGracePeriod := int64(0)
		deleteOption = metav1.DeleteOptions{PropagationPolicy: &propagationPolicy, GracePeriodSeconds: &zeroGracePeriod}
	default:
		propagationPolicy := metav1.DeletePropagationForeground
		deleteOption = metav1.DeleteOptions{PropagationPolicy: &propagationPolicy}
	}
	err = s.kubectl.DeleteResource(ctx, config, res.GroupKindVersion(), res.Name, res.Namespace, deleteOption)
	if err != nil {
		return nil, fmt.Errorf("error deleting resource: %w", err)
	}
	s.logAppEvent(ctx, a, argo.EventReasonResourceDeleted, fmt.Sprintf("deleted resource %s/%s '%s'", q.GetGroup(), q.GetKind(), q.GetResourceName()))
	return &application.ApplicationResponse{}, nil
}

func (s *Server) ResourceTree(ctx context.Context, q *application.ResourcesQuery) (*v1alpha1.ApplicationTree, error) {
	a, _, err := s.getApplicationEnforceRBACInformer(ctx, rbac.ActionGet, q.GetProject(), q.GetAppNamespace(), q.GetApplicationName())
	if err != nil {
		return nil, err
	}

	return s.getAppResources(ctx, a)
}

func (s *Server) WatchResourceTree(q *application.ResourcesQuery, ws application.ApplicationService_WatchResourceTreeServer) error {
	_, _, err := s.getApplicationEnforceRBACInformer(ws.Context(), rbac.ActionGet, q.GetProject(), q.GetAppNamespace(), q.GetApplicationName())
	if err != nil {
		return err
	}

	cacheKey := argo.AppInstanceName(q.GetApplicationName(), q.GetAppNamespace(), s.ns)
	return s.cache.OnAppResourcesTreeChanged(ws.Context(), cacheKey, func() error {
		var tree v1alpha1.ApplicationTree
		err := s.cache.GetAppResourcesTree(cacheKey, &tree)
		if err != nil {
			return fmt.Errorf("error getting app resource tree: %w", err)
		}
		return ws.Send(&tree)
	})
}

func (s *Server) RevisionMetadata(ctx context.Context, q *application.RevisionMetadataQuery) (*v1alpha1.RevisionMetadata, error) {
	a, proj, err := s.getApplicationEnforceRBACInformer(ctx, rbac.ActionGet, q.GetProject(), q.GetAppNamespace(), q.GetName())
	if err != nil {
		return nil, err
	}

	source, err := getAppSourceBySourceIndexAndVersionId(a, q.SourceIndex, q.VersionId)
	if err != nil {
		return nil, fmt.Errorf("error getting app source by source index and version ID: %w", err)
	}

	repo, err := s.db.GetRepository(ctx, source.RepoURL, proj.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting repository by URL: %w", err)
	}
	conn, repoClient, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, fmt.Errorf("error creating repo server client: %w", err)
	}
	defer utilio.Close(conn)
	return repoClient.GetRevisionMetadata(ctx, &apiclient.RepoServerRevisionMetadataRequest{
		Repo:           repo,
		Revision:       q.GetRevision(),
		CheckSignature: len(proj.Spec.SignatureKeys) > 0,
	})
}

// RevisionChartDetails returns the helm chart metadata, as fetched from the reposerver
func (s *Server) RevisionChartDetails(ctx context.Context, q *application.RevisionMetadataQuery) (*v1alpha1.ChartDetails, error) {
	a, _, err := s.getApplicationEnforceRBACInformer(ctx, rbac.ActionGet, q.GetProject(), q.GetAppNamespace(), q.GetName())
	if err != nil {
		return nil, err
	}

	source, err := getAppSourceBySourceIndexAndVersionId(a, q.SourceIndex, q.VersionId)
	if err != nil {
		return nil, fmt.Errorf("error getting app source by source index and version ID: %w", err)
	}

	if source.Chart == "" {
		return nil, fmt.Errorf("no chart found for application: %v", q.GetName())
	}
	repo, err := s.db.GetRepository(ctx, source.RepoURL, a.Spec.Project)
	if err != nil {
		return nil, fmt.Errorf("error getting repository by URL: %w", err)
	}
	conn, repoClient, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, fmt.Errorf("error creating repo server client: %w", err)
	}
	defer utilio.Close(conn)
	return repoClient.GetRevisionChartDetails(ctx, &apiclient.RepoServerRevisionChartDetailsRequest{
		Repo:     repo,
		Name:     source.Chart,
		Revision: q.GetRevision(),
	})
}

func (s *Server) GetOCIMetadata(ctx context.Context, q *application.RevisionMetadataQuery) (*v1alpha1.OCIMetadata, error) {
	a, proj, err := s.getApplicationEnforceRBACInformer(ctx, rbac.ActionGet, q.GetProject(), q.GetAppNamespace(), q.GetName())
	if err != nil {
		return nil, err
	}

	source, err := getAppSourceBySourceIndexAndVersionId(a, q.SourceIndex, q.VersionId)
	if err != nil {
		return nil, fmt.Errorf("error getting app source by source index and version ID: %w", err)
	}

	repo, err := s.db.GetRepository(ctx, source.RepoURL, proj.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting repository by URL: %w", err)
	}
	conn, repoClient, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, fmt.Errorf("error creating repo server client: %w", err)
	}
	defer utilio.Close(conn)

	return repoClient.GetOCIMetadata(ctx, &apiclient.RepoServerRevisionChartDetailsRequest{
		Repo:     repo,
		Name:     source.Chart,
		Revision: q.GetRevision(),
	})
}

func (s *Server) ManagedResources(ctx context.Context, q *application.ResourcesQuery) (*application.ManagedResourcesResponse, error) {
	a, _, err := s.getApplicationEnforceRBACInformer(ctx, rbac.ActionGet, q.GetProject(), q.GetAppNamespace(), q.GetApplicationName())
	if err != nil {
		return nil, err
	}

	items := make([]*v1alpha1.ResourceDiff, 0)
	err = s.getCachedAppState(ctx, a, func() error {
		return s.cache.GetAppManagedResources(a.InstanceName(s.ns), &items)
	})
	if err != nil {
		return nil, fmt.Errorf("error getting cached app managed resources: %w", err)
	}
	res := &application.ManagedResourcesResponse{}
	for i := range items {
		item := items[i]
		if !item.Hook && isMatchingResource(q, kube.ResourceKey{Name: item.Name, Namespace: item.Namespace, Kind: item.Kind, Group: item.Group}) {
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
		sinceSeconds = ptr.To(q.GetSinceSeconds())
	}
	if q.GetTailLines() > 0 {
		tailLines = ptr.To(q.GetTailLines())
	}
	var untilTime *metav1.Time
	if q.GetUntilTime() != "" {
		val, err := time.Parse(time.RFC3339Nano, q.GetUntilTime())
		if err != nil {
			return fmt.Errorf("invalid untilTime parameter value: %w", err)
		}
		untilTimeVal := metav1.NewTime(val)
		untilTime = &untilTimeVal
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

	a, _, err := s.getApplicationEnforceRBACInformer(ws.Context(), rbac.ActionGet, q.GetProject(), q.GetAppNamespace(), q.GetName())
	if err != nil {
		return err
	}

	if err := s.enf.EnforceErr(ws.Context().Value("claims"), rbac.ResourceLogs, rbac.ActionGet, a.RBACName(s.ns)); err != nil {
		return err
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

	maxPodLogsToRender, err := s.settingsMgr.GetMaxPodLogsToRender()
	if err != nil {
		return fmt.Errorf("error getting MaxPodLogsToRender config: %w", err)
	}

	if int64(len(pods)) > maxPodLogsToRender {
		return status.Error(codes.InvalidArgument, "max pods to view logs are reached. Please provide more granular query")
	}

	var streams []chan logEntry

	for _, pod := range pods {
		stream, err := kubeClientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
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
			defer utilio.Close(stream)
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
			}
			if q.Filter != nil {
				var lineContainsFilter bool
				if q.GetMatchCase() {
					lineContainsFilter = strings.Contains(entry.line, literal)
				} else {
					lineContainsFilter = strings.Contains(strings.ToLower(entry.line), strings.ToLower(literal))
				}

				if (inverse && lineContainsFilter) || (!inverse && !lineContainsFilter) {
					continue
				}
			}
			ts := metav1.NewTime(entry.timeStamp)
			if untilTime != nil && entry.timeStamp.After(untilTime.Time) {
				done <- ws.Send(&application.LogEntry{
					Last:         ptr.To(true),
					PodName:      &entry.podName,
					Content:      &entry.line,
					TimeStampStr: ptr.To(entry.timeStamp.Format(time.RFC3339Nano)),
					TimeStamp:    &ts,
				})
				return
			}
			sentCount++
			if err := ws.Send(&application.LogEntry{
				PodName:      &entry.podName,
				Content:      &entry.line,
				TimeStampStr: ptr.To(entry.timeStamp.Format(time.RFC3339Nano)),
				TimeStamp:    &ts,
				Last:         ptr.To(false),
			}); err != nil {
				done <- err
				break
			}
		}
		now := time.Now()
		nowTS := metav1.NewTime(now)
		done <- ws.Send(&application.LogEntry{
			Last:         ptr.To(true),
			PodName:      ptr.To(""),
			Content:      ptr.To(""),
			TimeStampStr: ptr.To(now.Format(time.RFC3339Nano)),
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

// Sync syncs an application to its target state
func (s *Server) Sync(ctx context.Context, syncReq *application.ApplicationSyncRequest) (*v1alpha1.Application, error) {
	a, proj, err := s.getApplicationEnforceRBACClient(ctx, rbac.ActionGet, syncReq.GetProject(), syncReq.GetAppNamespace(), syncReq.GetName(), "")
	if err != nil {
		return nil, err
	}

	s.inferResourcesStatusHealth(a)

	canSync, err := proj.Spec.SyncWindows.Matches(a).CanSync(true)
	if err != nil {
		return a, status.Errorf(codes.PermissionDenied, "cannot sync: invalid sync window: %v", err)
	}
	if !canSync {
		return a, status.Errorf(codes.PermissionDenied, "cannot sync: blocked by sync window")
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplications, rbac.ActionSync, a.RBACName(s.ns)); err != nil {
		return nil, err
	}

	if syncReq.Manifests != nil {
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplications, rbac.ActionOverride, a.RBACName(s.ns)); err != nil {
			return nil, err
		}
		if a.Spec.SyncPolicy != nil && a.Spec.SyncPolicy.IsAutomatedSyncEnabled() && !syncReq.GetDryRun() {
			return nil, status.Error(codes.FailedPrecondition, "cannot use local sync when Automatic Sync Policy is enabled unless for dry run")
		}
	}
	if a.DeletionTimestamp != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "application is deleting")
	}

	revision, displayRevision, sourceRevisions, displayRevisions, err := s.resolveSourceRevisions(ctx, a, syncReq)
	if err != nil {
		return nil, err
	}

	var retry *v1alpha1.RetryStrategy
	var syncOptions v1alpha1.SyncOptions
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

	if syncOptions.HasOption(common.SyncOptionReplace) && !s.syncWithReplaceAllowed {
		return nil, status.Error(codes.FailedPrecondition, "sync with replace was disabled on the API Server level via the server configuration")
	}

	// We cannot use local manifests if we're only allowed to sync to signed commits
	if syncReq.Manifests != nil && len(proj.Spec.SignatureKeys) > 0 {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot use local sync when signature keys are required.")
	}

	resources := []v1alpha1.SyncOperationResource{}
	if syncReq.GetResources() != nil {
		for _, r := range syncReq.GetResources() {
			if r != nil {
				resources = append(resources, *r)
			}
		}
	}

	var source *v1alpha1.ApplicationSource
	if !a.Spec.HasMultipleSources() {
		source = ptr.To(a.Spec.GetSource())
	}

	op := v1alpha1.Operation{
		Sync: &v1alpha1.SyncOperation{
			Source:       source,
			Revision:     revision,
			Prune:        syncReq.GetPrune(),
			DryRun:       syncReq.GetDryRun(),
			SyncOptions:  syncOptions,
			SyncStrategy: syncReq.Strategy,
			Resources:    resources,
			Manifests:    syncReq.Manifests,
			Sources:      a.Spec.Sources,
			Revisions:    sourceRevisions,
		},
		InitiatedBy: v1alpha1.OperationInitiator{Username: session.Username(ctx)},
		Info:        syncReq.Infos,
	}
	if retry != nil {
		op.Retry = *retry
	}

	appName := syncReq.GetName()
	appNs := s.appNamespaceOrDefault(syncReq.GetAppNamespace())
	appIf := s.appclientset.ArgoprojV1alpha1().Applications(appNs)
	a, err = argo.SetAppOperation(appIf, appName, &op)
	if err != nil {
		return nil, fmt.Errorf("error setting app operation: %w", err)
	}
	partial := ""
	if len(syncReq.Resources) > 0 {
		partial = "partial "
	}
	var reason string
	if a.Spec.HasMultipleSources() {
		reason = fmt.Sprintf("initiated %ssync to %s", partial, strings.Join(displayRevisions, ","))
	} else {
		reason = fmt.Sprintf("initiated %ssync to %s", partial, displayRevision)
	}
	if syncReq.Manifests != nil {
		reason = fmt.Sprintf("initiated %ssync locally", partial)
	}
	s.logAppEvent(ctx, a, argo.EventReasonOperationStarted, reason)
	return a, nil
}

func (s *Server) Rollback(ctx context.Context, rollbackReq *application.ApplicationRollbackRequest) (*v1alpha1.Application, error) {
	a, _, err := s.getApplicationEnforceRBACClient(ctx, rbac.ActionSync, rollbackReq.GetProject(), rollbackReq.GetAppNamespace(), rollbackReq.GetName(), "")
	if err != nil {
		return nil, err
	}

	s.inferResourcesStatusHealth(a)

	if a.DeletionTimestamp != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "application is deleting")
	}
	if a.Spec.SyncPolicy != nil && a.Spec.SyncPolicy.IsAutomatedSyncEnabled() {
		return nil, status.Errorf(codes.FailedPrecondition, "rollback cannot be initiated when auto-sync is enabled")
	}

	var deploymentInfo *v1alpha1.RevisionHistory
	for _, info := range a.Status.History {
		if info.ID == rollbackReq.GetId() {
			deploymentInfo = &info
			break
		}
	}
	if deploymentInfo == nil {
		return nil, status.Errorf(codes.InvalidArgument, "application %s does not have deployment with id %v", a.QualifiedName(), rollbackReq.GetId())
	}
	if deploymentInfo.Source.IsZero() && deploymentInfo.Sources.IsZero() {
		// Since source type was introduced to history starting with v0.12, and is now required for
		// rollback, we cannot support rollback to revisions deployed using Argo CD v0.11 or below
		// As multi source doesn't use app.Source, we need to check to the Sources length
		return nil, status.Errorf(codes.FailedPrecondition, "cannot rollback to revision deployed with Argo CD v0.11 or lower. sync to revision instead.")
	}

	var syncOptions v1alpha1.SyncOptions
	if a.Spec.SyncPolicy != nil {
		syncOptions = a.Spec.SyncPolicy.SyncOptions
	}

	// Rollback is just a convenience around Sync
	op := v1alpha1.Operation{
		Sync: &v1alpha1.SyncOperation{
			Revision:     deploymentInfo.Revision,
			Revisions:    deploymentInfo.Revisions,
			DryRun:       rollbackReq.GetDryRun(),
			Prune:        rollbackReq.GetPrune(),
			SyncOptions:  syncOptions,
			SyncStrategy: &v1alpha1.SyncStrategy{Apply: &v1alpha1.SyncStrategyApply{}},
			Source:       &deploymentInfo.Source,
			Sources:      deploymentInfo.Sources,
		},
		InitiatedBy: v1alpha1.OperationInitiator{Username: session.Username(ctx)},
	}
	appName := rollbackReq.GetName()
	appNs := s.appNamespaceOrDefault(rollbackReq.GetAppNamespace())
	appIf := s.appclientset.ArgoprojV1alpha1().Applications(appNs)
	a, err = argo.SetAppOperation(appIf, appName, &op)
	if err != nil {
		return nil, fmt.Errorf("error setting app operation: %w", err)
	}
	s.logAppEvent(ctx, a, argo.EventReasonOperationStarted, fmt.Sprintf("initiated rollback to %d", rollbackReq.GetId()))
	return a, nil
}

func (s *Server) ListLinks(ctx context.Context, req *application.ListAppLinksRequest) (*application.LinksResponse, error) {
	a, proj, err := s.getApplicationEnforceRBACClient(ctx, rbac.ActionGet, req.GetProject(), req.GetNamespace(), req.GetName(), "")
	if err != nil {
		return nil, err
	}

	obj, err := kube.ToUnstructured(a)
	if err != nil {
		return nil, fmt.Errorf("error getting application: %w", err)
	}

	deepLinks, err := s.settingsMgr.GetDeepLinks(settings.ApplicationDeepLinks)
	if err != nil {
		return nil, fmt.Errorf("failed to read application deep links from configmap: %w", err)
	}

	clstObj, _, err := s.getObjectsForDeepLinks(ctx, a, proj)
	if err != nil {
		return nil, err
	}

	// Create deep links object with managed-by URL
	deepLinksObject := deeplinks.CreateDeepLinksObject(nil, obj, clstObj, nil)

	// If no managed-by URL is set, use the current instance's URL
	if deepLinksObject[deeplinks.ManagedByURLKey] == nil {
		settings, err := s.settingsMgr.GetSettings()
		if err != nil {
			log.Warnf("Failed to get settings: %v", err)
		} else if settings.URL != "" {
			deepLinksObject[deeplinks.ManagedByURLKey] = settings.URL
		}
	}

	finalList, errorList := deeplinks.EvaluateDeepLinksResponse(deepLinksObject, obj.GetName(), deepLinks)
	if len(errorList) > 0 {
		log.Errorf("errorList while evaluating application deep links, %v", strings.Join(errorList, ", "))
	}

	return finalList, nil
}

func (s *Server) ListResourceLinks(ctx context.Context, req *application.ApplicationResourceRequest) (*application.LinksResponse, error) {
	obj, _, app, _, err := s.getUnstructuredLiveResourceOrApp(ctx, rbac.ActionGet, req)
	if err != nil {
		return nil, err
	}
	deepLinks, err := s.settingsMgr.GetDeepLinks(settings.ResourceDeepLinks)
	if err != nil {
		return nil, fmt.Errorf("failed to read application deep links from configmap: %w", err)
	}

	obj, err = s.replaceSecretValues(obj)
	if err != nil {
		return nil, fmt.Errorf("error replacing secret values: %w", err)
	}

	appObj, err := kube.ToUnstructured(app)
	if err != nil {
		return nil, err
	}

	proj, err := s.getAppProject(ctx, app, log.WithFields(applog.GetAppLogFields(app)))
	if err != nil {
		return nil, err
	}

	clstObj, projObj, err := s.getObjectsForDeepLinks(ctx, app, proj)
	if err != nil {
		return nil, err
	}

	deepLinksObject := deeplinks.CreateDeepLinksObject(obj, appObj, clstObj, projObj)
	finalList, errorList := deeplinks.EvaluateDeepLinksResponse(deepLinksObject, obj.GetName(), deepLinks)
	if len(errorList) > 0 {
		log.Errorf("errors while evaluating resource deep links, %v", strings.Join(errorList, ", "))
	}

	return finalList, nil
}

// resolveRevision resolves the revision specified either in the sync request, or the
// application source, into a concrete revision that will be used for a sync operation.
func (s *Server) resolveRevision(ctx context.Context, app *v1alpha1.Application, syncReq *application.ApplicationSyncRequest, sourceIndex int) (string, string, error) {
	if syncReq.Manifests != nil {
		return "", "", nil
	}

	ambiguousRevision := getAmbiguousRevision(app, syncReq, sourceIndex)

	repoURL := app.Spec.GetSource().RepoURL
	if app.Spec.HasMultipleSources() {
		repoURL = app.Spec.Sources[sourceIndex].RepoURL
	}

	repo, err := s.db.GetRepository(ctx, repoURL, app.Spec.Project)
	if err != nil {
		return "", "", fmt.Errorf("error getting repository by URL: %w", err)
	}
	conn, repoClient, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return "", "", fmt.Errorf("error getting repo server client: %w", err)
	}
	defer utilio.Close(conn)

	source := app.Spec.GetSourcePtrByIndex(sourceIndex)
	if !source.IsHelm() {
		if git.IsCommitSHA(ambiguousRevision) {
			// If it's already a commit SHA, then no need to look it up
			return ambiguousRevision, ambiguousRevision, nil
		}
	}

	resolveRevisionResponse, err := repoClient.ResolveRevision(ctx, &apiclient.ResolveRevisionRequest{
		Repo:              repo,
		App:               app,
		AmbiguousRevision: ambiguousRevision,
		SourceIndex:       int64(sourceIndex),
	})
	if err != nil {
		return "", "", fmt.Errorf("error resolving repo revision: %w", err)
	}
	return resolveRevisionResponse.Revision, resolveRevisionResponse.AmbiguousRevision, nil
}

func (s *Server) TerminateOperation(ctx context.Context, termOpReq *application.OperationTerminateRequest) (*application.OperationTerminateResponse, error) {
	appName := termOpReq.GetName()
	appNs := s.appNamespaceOrDefault(termOpReq.GetAppNamespace())
	a, _, err := s.getApplicationEnforceRBACClient(ctx, rbac.ActionSync, termOpReq.GetProject(), appNs, appName, "")
	if err != nil {
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
			s.logAppEvent(ctx, a, argo.EventReasonResourceUpdated, "terminated running operation")
			return &application.OperationTerminateResponse{}, nil
		}
		if !apierrors.IsConflict(err) {
			return nil, fmt.Errorf("error updating application: %w", err)
		}
		log.Warnf("failed to set operation for app %q due to update conflict. retrying again...", *termOpReq.Name)
		time.Sleep(100 * time.Millisecond)
		_, err = s.appclientset.ArgoprojV1alpha1().Applications(appNs).Get(ctx, appName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error getting application by name: %w", err)
		}
	}
	return nil, status.Errorf(codes.Internal, "Failed to terminate app. Too many conflicts")
}

func (s *Server) ListResourceActions(ctx context.Context, q *application.ApplicationResourceRequest) (*application.ResourceActionsListResponse, error) {
	obj, _, _, _, err := s.getUnstructuredLiveResourceOrApp(ctx, rbac.ActionGet, q)
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
	actionsPtr := []*v1alpha1.ResourceAction{}
	for i := range availableActions {
		actionsPtr = append(actionsPtr, &availableActions[i])
	}

	return &application.ResourceActionsListResponse{Actions: actionsPtr}, nil
}

// RunResourceAction runs a resource action on a live resource
//
// Deprecated: use RunResourceActionV2 instead. This version does not support resource action parameters but is
// maintained for backward compatibility. It will be removed in a future release.
func (s *Server) RunResourceAction(ctx context.Context, q *application.ResourceActionRunRequest) (*application.ApplicationResponse, error) {
	log.WithFields(log.Fields{
		"action":        q.Action,
		"application":   q.Name,
		"app-namespace": q.AppNamespace,
		"project":       q.Project,
		"user":          session.Username(ctx),
	}).Warn("RunResourceAction was called. RunResourceAction is deprecated and will be removed in a future release. Use RunResourceActionV2 instead.")
	qV2 := &application.ResourceActionRunRequestV2{
		Name:         q.Name,
		AppNamespace: q.AppNamespace,
		Namespace:    q.Namespace,
		ResourceName: q.ResourceName,
		Kind:         q.Kind,
		Version:      q.Version,
		Group:        q.Group,
		Action:       q.Action,
		Project:      q.Project,
	}
	return s.RunResourceActionV2(ctx, qV2)
}

func (s *Server) RunResourceActionV2(ctx context.Context, q *application.ResourceActionRunRequestV2) (*application.ApplicationResponse, error) {
	resourceRequest := &application.ApplicationResourceRequest{
		Name:         q.Name,
		AppNamespace: q.AppNamespace,
		Namespace:    q.Namespace,
		ResourceName: q.ResourceName,
		Kind:         q.Kind,
		Version:      q.Version,
		Group:        q.Group,
		Project:      q.Project,
	}
	actionRequest := fmt.Sprintf("%s/%s/%s/%s", rbac.ActionAction, q.GetGroup(), q.GetKind(), q.GetAction())
	liveObj, res, a, config, err := s.getUnstructuredLiveResourceOrApp(ctx, actionRequest, resourceRequest)
	if err != nil {
		return nil, err
	}

	liveObjBytes, err := json.Marshal(liveObj)
	if err != nil {
		return nil, fmt.Errorf("error marshaling live object: %w", err)
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

	newObjects, err := luaVM.ExecuteResourceAction(liveObj, action.ActionLua, q.GetResourceActionParameters())
	if err != nil {
		return nil, fmt.Errorf("error executing Lua resource action: %w", err)
	}

	var app *v1alpha1.Application
	// Only bother getting the app if we know we're going to need it for a resource permission check.
	if len(newObjects) > 0 {
		// No need for an RBAC check, we checked above that the user is allowed to run this action.
		app, err = s.appLister.Applications(s.appNamespaceOrDefault(q.GetAppNamespace())).Get(q.GetName())
		if err != nil {
			return nil, err
		}
	}

	proj, err := s.getAppProject(ctx, a, log.WithFields(applog.GetAppLogFields(a)))
	if err != nil {
		return nil, err
	}

	destCluster, err := argo.GetDestinationCluster(ctx, app.Spec.Destination, s.db)
	if err != nil {
		return nil, err
	}

	// First, make sure all the returned resources are permitted, for each operation.
	// Also perform create with dry-runs for all create-operation resources.
	// This is performed separately to reduce the risk of only some of the resources being successfully created later.
	// TODO: when apply/delete operations would be supported for custom actions,
	// the dry-run for relevant apply/delete operation would have to be invoked as well.
	for _, impactedResource := range newObjects {
		newObj := impactedResource.UnstructuredObj
		err := s.verifyResourcePermitted(destCluster, proj, newObj)
		if err != nil {
			return nil, err
		}
		if impactedResource.K8SOperation == lua.CreateOperation {
			createOptions := metav1.CreateOptions{DryRun: []string{"All"}}
			_, err := s.kubectl.CreateResource(ctx, config, newObj.GroupVersionKind(), newObj.GetName(), newObj.GetNamespace(), newObj, createOptions)
			if err != nil {
				return nil, err
			}
		}
	}

	// Now, perform the actual operations.
	// The creation itself is not transactional.
	// TODO: maybe create a k8s list representation of the resources,
	// and invoke create on this list resource to make it semi-transactional (there is still patch operation that is separate,
	// thus can fail separately from create).
	for _, impactedResource := range newObjects {
		newObj := impactedResource.UnstructuredObj
		newObjBytes, err := json.Marshal(newObj)
		if err != nil {
			return nil, fmt.Errorf("error marshaling new object: %w", err)
		}

		switch impactedResource.K8SOperation {
		// No default case since a not supported operation would have failed upon unmarshaling earlier
		case lua.PatchOperation:
			_, err := s.patchResource(ctx, config, liveObjBytes, newObjBytes, newObj)
			if err != nil {
				return nil, err
			}
		case lua.CreateOperation:
			_, err := s.createResource(ctx, config, newObj)
			if err != nil {
				return nil, err
			}
		}
	}

	if res == nil {
		s.logAppEvent(ctx, a, argo.EventReasonResourceActionRan, "ran action "+q.GetAction())
	} else {
		s.logAppEvent(ctx, a, argo.EventReasonResourceActionRan, fmt.Sprintf("ran action %s on resource %s/%s/%s", q.GetAction(), res.Group, res.Kind, res.Name))
		s.logResourceEvent(ctx, res, argo.EventReasonResourceActionRan, "ran action "+q.GetAction())
	}
	return &application.ApplicationResponse{}, nil
}

func (s *Server) GetApplicationSyncWindows(ctx context.Context, q *application.ApplicationSyncWindowsQuery) (*application.ApplicationSyncWindowsResponse, error) {
	a, proj, err := s.getApplicationEnforceRBACClient(ctx, rbac.ActionGet, q.GetProject(), q.GetAppNamespace(), q.GetName(), "")
	if err != nil {
		return nil, err
	}

	windows := proj.Spec.SyncWindows.Matches(a)
	sync, err := windows.CanSync(true)
	if err != nil {
		return nil, fmt.Errorf("invalid sync windows: %w", err)
	}

	activeWindows, err := windows.Active()
	if err != nil {
		return nil, fmt.Errorf("invalid sync windows: %w", err)
	}
	res := &application.ApplicationSyncWindowsResponse{
		ActiveWindows:   convertSyncWindows(activeWindows),
		AssignedWindows: convertSyncWindows(windows),
		CanSync:         &sync,
	}

	return res, nil
}

// ServerSideDiff gets the destination cluster and creates a server-side dry run applier and performs the diff
// It returns the diff result in the form of a list of ResourceDiffs.
func (s *Server) ServerSideDiff(ctx context.Context, q *application.ApplicationServerSideDiffQuery) (*application.ApplicationServerSideDiffResponse, error) {
	a, _, err := s.getApplicationEnforceRBACInformer(ctx, rbac.ActionGet, q.GetProject(), q.GetAppNamespace(), q.GetAppName())
	if err != nil {
		return nil, fmt.Errorf("error getting application: %w", err)
	}

	argoSettings, err := s.settingsMgr.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("error getting ArgoCD settings: %w", err)
	}

	resourceOverrides, err := s.settingsMgr.GetResourceOverrides()
	if err != nil {
		return nil, fmt.Errorf("error getting resource overrides: %w", err)
	}

	// Convert to map format expected by DiffConfigBuilder
	overrides := make(map[string]v1alpha1.ResourceOverride)
	for k, v := range resourceOverrides {
		overrides[k] = v
	}

	// Get cluster connection for server-side dry run
	cluster, err := argo.GetDestinationCluster(ctx, a.Spec.Destination, s.db)
	if err != nil {
		return nil, fmt.Errorf("error getting destination cluster: %w", err)
	}

	clusterConfig, err := cluster.RawRestConfig()
	if err != nil {
		return nil, fmt.Errorf("error getting cluster raw REST config: %w", err)
	}

	// Create server-side diff dry run applier
	openAPISchema, gvkParser, err := s.kubectl.LoadOpenAPISchema(clusterConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenAPI schema: %w", err)
	}

	applier, cleanup, err := kubeutil.ManageServerSideDiffDryRuns(clusterConfig, openAPISchema, func(_ string) (kube.CleanupFunc, error) {
		return func() {}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("error creating server-side dry run applier: %w", err)
	}
	defer cleanup()

	dryRunner := diff.NewK8sServerSideDryRunner(applier)

	appLabelKey, err := s.settingsMgr.GetAppInstanceLabelKey()
	if err != nil {
		return nil, fmt.Errorf("error getting app instance label key: %w", err)
	}

	// Build diff config like the CLI does, but with server-side diff enabled
	ignoreAggregatedRoles := false
	diffConfig, err := argodiff.NewDiffConfigBuilder().
		WithDiffSettings(a.Spec.IgnoreDifferences, overrides, ignoreAggregatedRoles, normalizers.IgnoreNormalizerOpts{}).
		WithTracking(appLabelKey, argoSettings.TrackingMethod).
		WithNoCache().
		WithManager(argocommon.ArgoCDSSAManager).
		WithServerSideDiff(true).
		WithServerSideDryRunner(dryRunner).
		WithGVKParser(gvkParser).
		WithIgnoreMutationWebhook(!resourceutil.HasAnnotationOption(a, argocommon.AnnotationCompareOptions, "IncludeMutationWebhook=true")).
		Build()
	if err != nil {
		return nil, fmt.Errorf("error building diff config: %w", err)
	}

	// Convert live resources to unstructured objects
	liveObjs := make([]*unstructured.Unstructured, 0, len(q.GetLiveResources()))
	for _, liveResource := range q.GetLiveResources() {
		if liveResource.LiveState != "" && liveResource.LiveState != "null" {
			liveObj := &unstructured.Unstructured{}
			err := json.Unmarshal([]byte(liveResource.LiveState), liveObj)
			if err != nil {
				return nil, fmt.Errorf("error unmarshaling live state for %s/%s: %w", liveResource.Kind, liveResource.Name, err)
			}
			liveObjs = append(liveObjs, liveObj)
		} else {
			liveObjs = append(liveObjs, nil)
		}
	}

	// Convert target manifests to unstructured objects
	targetObjs := make([]*unstructured.Unstructured, 0, len(q.GetTargetManifests()))
	for i, manifestStr := range q.GetTargetManifests() {
		obj, err := v1alpha1.UnmarshalToUnstructured(manifestStr)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling target manifest %d: %w", i, err)
		}
		targetObjs = append(targetObjs, obj)
	}

	diffResults, err := argodiff.StateDiffs(liveObjs, targetObjs, diffConfig)
	if err != nil {
		return nil, fmt.Errorf("error performing state diffs: %w", err)
	}

	// Convert StateDiffs results to ResourceDiff format for API response
	responseDiffs := make([]*v1alpha1.ResourceDiff, 0, len(diffResults.Diffs))
	modified := false

	for i, diffRes := range diffResults.Diffs {
		if diffRes.Modified {
			modified = true
		}

		// Extract resource metadata for the diff result. Resources should be pre-aligned by the CLI.
		var group, kind, namespace, name string
		var hook bool
		var resourceVersion string

		// Extract resource metadata for the ResourceDiff response. The CLI sends aligned arrays
		// of live resources and target manifests, but individual resources may only exist in one
		// array depending on the operation
		switch {
		case i < len(q.GetLiveResources()):
			// A live resource exists at this index
			lr := q.GetLiveResources()[i]
			group = lr.Group
			kind = lr.Kind
			namespace = lr.Namespace
			name = lr.Name
			hook = lr.Hook
			resourceVersion = lr.ResourceVersion
		case i < len(targetObjs) && targetObjs[i] != nil:
			// A target resource exists at this index, but no live resource exists at this index
			obj := targetObjs[i]
			group = obj.GroupVersionKind().Group
			kind = obj.GroupVersionKind().Kind
			namespace = obj.GetNamespace()
			name = obj.GetName()
			hook = false
			resourceVersion = ""
		default:
			return nil, fmt.Errorf("diff result index %d out of bounds: live resources (%d), target objects (%d)",
				i, len(q.GetLiveResources()), len(targetObjs))
		}

		// Create ResourceDiff with StateDiffs results
		// TargetState = PredictedLive (what the target should be after applying)
		// LiveState = NormalizedLive (current normalized live state)
		responseDiffs = append(responseDiffs, &v1alpha1.ResourceDiff{
			Group:           group,
			Kind:            kind,
			Namespace:       namespace,
			Name:            name,
			TargetState:     string(diffRes.PredictedLive),
			LiveState:       string(diffRes.NormalizedLive),
			Diff:            "", // Diff string is generated client-side
			Hook:            hook,
			Modified:        diffRes.Modified,
			ResourceVersion: resourceVersion,
		})
	}

	log.Infof("ServerSideDiff completed with %d results, overall modified: %t", len(responseDiffs), modified)

	return &application.ApplicationServerSideDiffResponse{
		Items:    responseDiffs,
		Modified: &modified,
	}, nil
}
