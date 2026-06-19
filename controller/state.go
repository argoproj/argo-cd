package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	goSync "sync"
	"time"

	synccommon "github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common"
	corev1 "k8s.io/api/core/v1"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/diff"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/health"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/sync"
	hookutil "github.com/argoproj/argo-cd/gitops-engine/pkg/sync/hook"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/sync/ignore"
	resourceutil "github.com/argoproj/argo-cd/gitops-engine/pkg/sync/resource"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/sync/syncwaves"
	kubeutil "github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"

	"github.com/argoproj/argo-cd/v3/util/sourceintegrity"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/argoproj/argo-cd/v3/common"
	statecache "github.com/argoproj/argo-cd/v3/controller/cache"
	"github.com/argoproj/argo-cd/v3/controller/metrics"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	applog "github.com/argoproj/argo-cd/v3/util/app/log"
	"github.com/argoproj/argo-cd/v3/util/app/path"
	"github.com/argoproj/argo-cd/v3/util/argo"
	argodiff "github.com/argoproj/argo-cd/v3/util/argo/diff"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
	appstatecache "github.com/argoproj/argo-cd/v3/util/cache/appstate"
	"github.com/argoproj/argo-cd/v3/util/db"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/settings"
	"github.com/argoproj/argo-cd/v3/util/stats"
)

var ErrCompareStateRepo = errors.New("failed to get repo objects")

type resourceInfoProviderStub struct{}

func (r *resourceInfoProviderStub) IsNamespaced(_ schema.GroupKind) (bool, error) {
	return false, nil
}

type managedResource struct {
	Target          *unstructured.Unstructured
	Live            *unstructured.Unstructured
	Diff            diff.DiffResult
	Group           string
	Version         string
	Kind            string
	Namespace       string
	Name            string
	Hook            bool
	ResourceVersion string
}

// AppStateManager defines methods which allow to compare application spec and actual application state.
type AppStateManager interface {
	CompareAppState(app *v1alpha1.Application, project *v1alpha1.AppProject, revisions []string, sources []v1alpha1.ApplicationSource, noCache bool, noRevisionCache bool, localObjects []string, hasMultipleSources bool) (*comparisonResult, error)
	SyncAppState(app *v1alpha1.Application, project *v1alpha1.AppProject, state *v1alpha1.OperationState)
	EvaluateAppRevisionsChanges(ctx context.Context, app *v1alpha1.Application, sources []v1alpha1.ApplicationSource, revisions []string, proj *v1alpha1.AppProject, sendRuntimeState bool, noRevisionCache bool) (bool, []string, error)
	GetRepoObjs(ctx context.Context, app *v1alpha1.Application, sources []v1alpha1.ApplicationSource, appLabelKey string, revisions []string, noCache, noRevisionCache bool, sourceIntegrity *v1alpha1.SourceIntegrity, proj *v1alpha1.AppProject, sendRuntimeState bool) ([]*unstructured.Unstructured, []*apiclient.ManifestResponse, bool, error)
}

// comparisonResult holds the state of an application after the reconciliation
type comparisonResult struct {
	syncStatus           *v1alpha1.SyncStatus
	healthStatus         health.HealthStatusCode
	resources            []v1alpha1.ResourceStatus
	managedResources     []managedResource
	reconciliationResult sync.ReconciliationResult
	diffConfig           argodiff.DiffConfig
	appSourceType        v1alpha1.ApplicationSourceType
	// appSourceTypes stores the SourceType for each application source under sources field
	appSourceTypes []v1alpha1.ApplicationSourceType
	// timings maps phases of comparison to the duration it took to complete (for statistical purposes)
	timings            map[string]time.Duration
	diffResultList     *diff.DiffResultList
	hasPostDeleteHooks bool
	hasPreDeleteHooks  bool
	// revisionsMayHaveChanges indicates if there are any possibilities that the revisions contain changes
	revisionsMayHaveChanges bool
}

func (res *comparisonResult) GetSyncStatus() *v1alpha1.SyncStatus {
	return res.syncStatus
}

func (res *comparisonResult) GetHealthStatus() health.HealthStatusCode {
	return res.healthStatus
}

// appStateManager allows to compare applications to git
type appStateManager struct {
	metricsServer         *metrics.MetricsServer
	db                    db.ArgoDB
	settingsMgr           *settings.SettingsManager
	appclientset          appclientset.Interface
	kubectl               kubeutil.Kubectl
	onKubectlRun          kubeutil.OnKubectlRunFunc
	repoClientset         apiclient.Clientset
	liveStateCache        statecache.LiveStateCache
	cache                 *appstatecache.Cache
	namespace             string
	statusRefreshTimeout  time.Duration
	resourceTracking      argo.ResourceTracking
	persistResourceHealth bool
	repoErrorCache        goSync.Map
	repoErrorGracePeriod  time.Duration
	serverSideDiff        bool
	ignoreNormalizerOpts  normalizers.IgnoreNormalizerOpts
}

// EvaluateAppRevisionsChanges checks if any source revisions have changes without generating manifests.
// If it does not, then the cached manifests are updated to the current revisions.
// Returns whether any changes were detected across all sources and the resolved revision per source (same order as sources).
func (m *appStateManager) EvaluateAppRevisionsChanges(ctx context.Context, app *v1alpha1.Application, sources []v1alpha1.ApplicationSource, revisions []string, proj *v1alpha1.AppProject, sendRuntimeState bool, noRevisionCache bool) (bool, []string, error) {
	hasChanges := false

	appLabelKey, err := m.settingsMgr.GetAppInstanceLabelKey()
	if err != nil {
		return false, nil, fmt.Errorf("failed to get app instance label key: %w", err)
	}

	trackingMethod, err := m.settingsMgr.GetTrackingMethod()
	if err != nil {
		return false, nil, fmt.Errorf("failed to get trackingMethod: %w", err)
	}

	installationID, err := m.settingsMgr.GetInstallationID()
	if err != nil {
		return false, nil, fmt.Errorf("failed to get installation ID: %w", err)
	}

	destCluster, err := argo.GetDestinationCluster(ctx, app.Spec.Destination, m.db)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get destination cluster: %w", err)
	}

	var serverVersion string
	var apiVersions []string
	if sendRuntimeState {
		var apiResources []kubeutil.APIResourceInfo
		serverVersion, apiResources, err = m.liveStateCache.GetVersionsInfo(destCluster)
		if err != nil {
			return false, nil, fmt.Errorf("failed to get cluster version for cluster %q: %w", destCluster.Server, err)
		}
		apiVersions = argo.APIResourcesToStrings(apiResources, true)
	}

	conn, repoClient, err := m.repoClientset.NewRepoServerClient()
	if err != nil {
		return false, nil, fmt.Errorf("failed to connect to repo server: %w", err)
	}
	defer utilio.Close(conn)

	refSources, err := argo.GetRefSources(ctx, sources, app.Spec.Project, m.db.GetRepository, revisions)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get ref sources: %w", err)
	}

	var syncedRefSources v1alpha1.RefTargetRevisionMapping
	if app.Spec.HasMultipleSources() {
		syncedRefSources = argo.GetSyncedRefSources(refSources, sources, app.Status.Sync.Revisions)
	}

	resolvedRevisions := make([]string, 0, len(sources))
	for i, source := range sources {
		if len(revisions) < len(sources) || revisions[i] == "" {
			revisions[i] = source.TargetRevision
		}
		resolvedRev, revisionsMayHaveChanges, err := m.evaluateRevisionChanges(ctx, app, source, i, revisions[i], refSources, syncedRefSources, noRevisionCache, trackingMethod, appLabelKey, installationID, serverVersion, apiVersions, proj, repoClient)
		if err != nil {
			return false, nil, fmt.Errorf("failed to evaluate revision changes for source %d of %d: %w", i+1, len(sources), err)
		}
		resolvedRevisions = append(resolvedRevisions, resolvedRev)

		if revisionsMayHaveChanges {
			hasChanges = true
		}
	}

	return hasChanges, resolvedRevisions, nil
}

// GetRepoObjs will generate the manifests for the given application delegating the
// task to the repo-server. It returns the list of generated manifests as unstructured
// objects. It also returns the full response from all calls to the repo server as the
// second argument.
func (m *appStateManager) GetRepoObjs(ctx context.Context, app *v1alpha1.Application, sources []v1alpha1.ApplicationSource, appLabelKey string, revisions []string, noCache, noRevisionCache bool, sourceIntegrity *v1alpha1.SourceIntegrity, proj *v1alpha1.AppProject, sendRuntimeState bool) ([]*unstructured.Unstructured, []*apiclient.ManifestResponse, bool, error) {
	ts := stats.NewTimingStats()
	helmRepos, err := m.db.ListHelmRepositories(ctx)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to list Helm repositories: %w", err)
	}
	permittedHelmRepos, err := argo.GetPermittedRepos(proj, helmRepos)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to get permitted Helm repositories for project %q: %w", proj.Name, err)
	}

	ociRepos, err := m.db.ListOCIRepositories(ctx)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to list OCI repositories: %w", err)
	}
	permittedOCIRepos, err := argo.GetPermittedRepos(proj, ociRepos)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to get permitted OCI repositories for project %q: %w", proj.Name, err)
	}

	ts.AddCheckpoint("repo_ms")
	helmRepositoryCredentials, err := m.db.GetAllHelmRepositoryCredentials(ctx)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to get Helm credentials: %w", err)
	}
	permittedHelmCredentials, err := argo.GetPermittedReposCredentials(proj, helmRepositoryCredentials)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to get permitted Helm credentials for project %q: %w", proj.Name, err)
	}

	ociRepositoryCredentials, err := m.db.GetAllOCIRepositoryCredentials(ctx)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to get OCI credentials: %w", err)
	}
	permittedOCICredentials, err := argo.GetPermittedReposCredentials(proj, ociRepositoryCredentials)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to get permitted OCI credentials for project %q: %w", proj.Name, err)
	}

	enabledSourceTypes, err := m.settingsMgr.GetEnabledSourceTypes()
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to get enabled source types: %w", err)
	}
	ts.AddCheckpoint("plugins_ms")

	kustomizeSettings, err := m.settingsMgr.GetKustomizeSettings()
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to get Kustomize settings: %w", err)
	}

	helmOptions, err := m.settingsMgr.GetHelmSettings()
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to get Helm settings: %w", err)
	}

	trackingMethod, err := m.settingsMgr.GetTrackingMethod()
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to get trackingMethod: %w", err)
	}

	installationID, err := m.settingsMgr.GetInstallationID()
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to get installation ID: %w", err)
	}

	destCluster, err := argo.GetDestinationCluster(ctx, app.Spec.Destination, m.db)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to get destination cluster: %w", err)
	}

	ts.AddCheckpoint("build_options_ms")
	var serverVersion string
	var apiVersions []string
	if sendRuntimeState {
		var apiResources []kubeutil.APIResourceInfo
		serverVersion, apiResources, err = m.liveStateCache.GetVersionsInfo(destCluster)
		if err != nil {
			return nil, nil, false, fmt.Errorf("failed to get cluster version for cluster %q: %w", destCluster.Server, err)
		}
		apiVersions = argo.APIResourcesToStrings(apiResources, true)
	}
	conn, repoClient, err := m.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to connect to repo server: %w", err)
	}
	defer utilio.Close(conn)

	manifestInfos := make([]*apiclient.ManifestResponse, 0)
	targetObjs := make([]*unstructured.Unstructured, 0)

	// Store the map of all sources having ref field into a map for applications with sources field
	// If it's for a rollback process, the refSources[*].targetRevision fields are the desired
	// revisions for the rollback
	refSources, err := argo.GetRefSources(ctx, sources, app.Spec.Project, m.db.GetRepository, revisions)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to get ref sources: %w", err)
	}

	var syncedRefSources v1alpha1.RefTargetRevisionMapping
	if app.Spec.HasMultipleSources() {
		syncedRefSources = argo.GetSyncedRefSources(refSources, sources, app.Status.Sync.Revisions)
	}

	revisionsMayHaveChanges := false
	for i, source := range sources {
		if len(revisions) < len(sources) || revisions[i] == "" {
			revisions[i] = source.TargetRevision
		}
		revision := revisions[i]

		// Use evaluateRevisionChanges to check for changes and get resolved revision
		resolvedRevision, hasChanges, err := m.evaluateRevisionChanges(ctx, app, source, i, revision, refSources, syncedRefSources, noRevisionCache, trackingMethod, appLabelKey, installationID, serverVersion, apiVersions, proj, repoClient)
		if err != nil {
			return nil, nil, false, fmt.Errorf("failed to evaluate revision changes for source %d of %d: %w", i+1, len(sources), err)
		}

		if hasChanges {
			revisionsMayHaveChanges = true
		}

		// Use the resolved revision from evaluateRevisionChanges
		revision = resolvedRevision
		revisions[i] = resolvedRevision

		appNamespace := app.Spec.Destination.Namespace

		repos := permittedHelmRepos
		helmRepoCreds := permittedHelmCredentials
		// If the source is OCI, there is a potential for an OCI image to be a Helm chart and that said chart in
		// turn would have OCI dependencies. To ensure that those dependencies can be resolved, add them to the repos
		// list.
		if source.IsOCI() {
			repos = slices.Clone(permittedHelmRepos)
			helmRepoCreds = slices.Clone(permittedHelmCredentials)
			repos = append(repos, permittedOCIRepos...)
			helmRepoCreds = append(helmRepoCreds, permittedOCICredentials...)
		}

		repo, err := m.db.GetRepository(ctx, source.RepoURL, proj.Name)
		if err != nil {
			return nil, nil, false, fmt.Errorf("failed to get repo %q: %w", source.RepoURL, err)
		}

		log.Debugf("Generating Manifest for source %s revision %s", source, revision)
		manifestInfo, err := repoClient.GenerateManifest(ctx, &apiclient.ManifestRequest{
			Repo:                            repo,
			Repos:                           repos,
			Revision:                        revision,
			NoCache:                         noCache,
			NoRevisionCache:                 noRevisionCache,
			AppLabelKey:                     appLabelKey,
			AppName:                         app.InstanceName(m.namespace),
			Namespace:                       appNamespace,
			ApplicationSource:               &source,
			KustomizeOptions:                kustomizeSettings,
			KubeVersion:                     serverVersion,
			ApiVersions:                     apiVersions,
			SourceIntegrity:                 sourceIntegrity,
			VerifySignature:                 sourceIntegrity != nil, // nolint:staticcheck
			HelmRepoCreds:                   helmRepoCreds,
			TrackingMethod:                  trackingMethod,
			EnabledSourceTypes:              enabledSourceTypes,
			HelmOptions:                     helmOptions,
			HasMultipleSources:              app.Spec.HasMultipleSources(),
			RefSources:                      refSources,
			ProjectName:                     proj.Name,
			ProjectSourceRepos:              proj.Spec.SourceRepos,
			AnnotationManifestGeneratePaths: app.GetAnnotation(v1alpha1.AnnotationKeyManifestGeneratePaths),
			InstallationID:                  installationID,
		})
		if err != nil {
			genErr := fmt.Errorf("failed to generate manifest for source %d of %d: %w", i+1, len(sources), err)
			if app.Spec.SourceHydrator != nil && app.Spec.SourceHydrator.HydrateTo != nil && strings.Contains(err.Error(), path.ErrMessageAppPathDoesNotExist) {
				genErr = fmt.Errorf("%w - waiting for an external process to update %s from %s", genErr, app.Spec.SourceHydrator.SyncSource.TargetBranch, app.Spec.SourceHydrator.HydrateTo.TargetBranch)
			}
			return nil, nil, false, genErr
		}

		targetObj, err := unmarshalManifests(manifestInfo.Manifests)
		if err != nil {
			return nil, nil, false, fmt.Errorf("failed to unmarshal manifests for source %d of %d: %w", i+1, len(sources), err)
		}
		targetObjs = append(targetObjs, targetObj...)
		manifestInfos = append(manifestInfos, manifestInfo)

		// Update eventual check problems with the ID of the current source. This is so users can attribute problems to correct sources
		if len(sources) > 1 {
			var sourceId string
			if source.Name != "" {
				sourceId = "source " + source.Name
			} else {
				sourceId = fmt.Sprintf("source %d of %d", i+1, len(sources))
			}
			manifestInfo.SourceIntegrityResult.InjectSourceName(sourceId)
		}
	}

	ts.AddCheckpoint("manifests_ms")
	logCtx := log.WithFields(applog.GetAppLogFields(app))
	for k, v := range ts.Timings() {
		logCtx = logCtx.WithField(k, v.Milliseconds())
	}
	logCtx = logCtx.WithField("time_ms", time.Since(ts.StartTime).Milliseconds())
	logCtx.Info("GetRepoObjs stats")

	return targetObjs, manifestInfos, revisionsMayHaveChanges, nil
}

// evaluateRevisionChanges checks if a single source revision has changes without generating manifests.
// Returns the resolved revision and whether changes were detected.
func (m *appStateManager) evaluateRevisionChanges(ctx context.Context, app *v1alpha1.Application, source v1alpha1.ApplicationSource, sourceIndex int, revision string, refSources v1alpha1.RefTargetRevisionMapping, syncedRefSources v1alpha1.RefTargetRevisionMapping, noRevisionCache bool, trackingMethod string, appLabelKey string, installationID string, serverVersion string, apiVersions []string, proj *v1alpha1.AppProject, repoClient apiclient.RepoServerServiceClient) (string, bool, error) {
	alwaysResolveRevision := false
	if revision == "" {
		revision = source.TargetRevision
	}

	// Determine the synced revision and source type for comparison
	syncedRevision := app.Status.Sync.Revision
	if app.Spec.SourceHydrator != nil {
		if drySource := app.Spec.SourceHydrator.GetDrySource(); source.Equals(&drySource) {
			// Always resolve the revision even if UpdateRevisionForPaths is not called so we can
			// correctly compare it with the syncedRevision
			alwaysResolveRevision = true
			sourceIndex = -1 // Special case allowing GetSourcePtrByIndex() to return the dry source
			// Use LastComparedDryRevision as the synced revision for cache lookups
			syncedRevision = app.Status.SourceHydrator.LastComparedDryRevision
		}
	} else if app.Spec.HasMultipleSources() {
		if sourceIndex < len(app.Status.Sync.Revisions) {
			syncedRevision = app.Status.Sync.Revisions[sourceIndex]
		} else {
			syncedRevision = ""
		}
	}

	if source.IsRef() {
		// For ref source specifically, we always return false since their change are evaluated as part of the source
		// referencing them.
		return revision, false, nil
	}

	if syncedRevision == revision && revision != "" && len(refSources) == 0 {
		// if revisions are the same (and we are not using reference sources), we know there is no changes
		// TODO: Could be optimized to not call the repo server at all if we know this specific source does not use reference.
		return revision, false, nil
	}
	repo, err := m.db.GetRepository(ctx, source.RepoURL, proj.Name)
	if err != nil {
		return "", false, fmt.Errorf("failed to get repo %q: %w", source.RepoURL, err)
	}

	keyManifestGenerateAnnotationVal := app.Annotations[v1alpha1.AnnotationKeyManifestGeneratePaths]

	if syncedRevision != "" && repo.Depth == 0 && keyManifestGenerateAnnotationVal != "" {
		updateRevisionResult, err := repoClient.UpdateRevisionForPaths(ctx, &apiclient.UpdateRevisionForPathsRequest{
			Repo:               repo,
			Revision:           revision,
			SyncedRevision:     syncedRevision,
			NoRevisionCache:    noRevisionCache,
			Paths:              path.GetSourceRefreshPaths(app, source),
			AppLabelKey:        appLabelKey,
			AppName:            app.InstanceName(m.namespace),
			Namespace:          app.Spec.Destination.Namespace,
			ApplicationSource:  &source,
			KubeVersion:        serverVersion,
			ApiVersions:        apiVersions,
			TrackingMethod:     trackingMethod,
			RefSources:         refSources,
			SyncedRefSources:   syncedRefSources,
			HasMultipleSources: app.Spec.HasMultipleSources(),
			InstallationID:     installationID,
		})
		if err != nil {
			return "", false, fmt.Errorf("failed to update revision for paths: %w", err)
		}

		resolvedRevision := revision
		if updateRevisionResult.Revision != "" {
			resolvedRevision = updateRevisionResult.Revision
		}

		return resolvedRevision, updateRevisionResult.Changes, nil
	} else if alwaysResolveRevision {
		resp, err := repoClient.ResolveRevision(ctx, &apiclient.ResolveRevisionRequest{
			Repo:              repo,
			App:               app,
			AmbiguousRevision: revision,
			SourceIndex:       int64(sourceIndex),
			NoRevisionCache:   noRevisionCache,
		})
		if err != nil {
			return "", false, fmt.Errorf("failed to resolve revision: %w", err)
		}
		revision = resp.Revision

		if syncedRevision == revision && revision != "" && len(refSources) == 0 {
			// if revisions are the same (and we are not using reference sources), we know there is no changes
			return revision, false, nil
		}
		return revision, true, nil
	}

	// For any types of sources where we cannot know if revision has changed, we return true as we cannot make assumptions.
	return revision, true, nil
}

func unmarshalManifests(manifests []string) ([]*unstructured.Unstructured, error) {
	targetObjs := make([]*unstructured.Unstructured, 0)
	for _, manifest := range manifests {
		obj, err := v1alpha1.UnmarshalToUnstructured(manifest)
		if err != nil {
			return nil, err
		}
		targetObjs = append(targetObjs, obj)
	}
	return targetObjs, nil
}

func NormalizeTargetObjects(namespace string, objs []*unstructured.Unstructured, infoProvider kubeutil.ResourceInfoProvider, setAppInstance func(*unstructured.Unstructured) error) ([]*unstructured.Unstructured, []v1alpha1.ApplicationCondition, error) {
	targetByKey := make(map[kubeutil.ResourceKey][]*unstructured.Unstructured)
	for i := range objs {
		obj := objs[i]
		if obj == nil {
			continue
		}

		namespaceModified := false
		isNamespaced := kubeutil.IsNamespacedOrUnknown(infoProvider, obj.GroupVersionKind().GroupKind())
		if !isNamespaced && obj.GetNamespace() != "" {
			// If a resource is cluster scoped, set the namespace to empty.
			obj.SetNamespace("")
			namespaceModified = true
		} else if isNamespaced && obj.GetNamespace() == "" {
			// If the object does not have a namespace specified, set it to the namespace of the application.
			obj.SetNamespace(namespace)
			namespaceModified = true
		}

		if namespaceModified {
			err := setAppInstance(obj)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to set app instance label on resource %s/%s: %w", obj.GetKind(), obj.GetName(), err)
			}
		}

		key := kubeutil.GetResourceKey(obj)
		if key.Name == "" && obj.GetGenerateName() != "" {
			key.Name = fmt.Sprintf("%s%d", obj.GetGenerateName(), i)
		}
		targetByKey[key] = append(targetByKey[key], obj)
	}

	conditions := make([]v1alpha1.ApplicationCondition, 0)
	result := make([]*unstructured.Unstructured, 0)
	for key, targets := range targetByKey {
		if len(targets) > 1 {
			// If an object is duplicated in the target, we add a condition to the application.
			now := metav1.Now()
			conditions = append(conditions, v1alpha1.ApplicationCondition{
				Type:               v1alpha1.ApplicationConditionRepeatedResourceWarning,
				Message:            fmt.Sprintf("Resource %s appeared %d times among application resources.", key.String(), len(targets)),
				LastTransitionTime: &now,
			})
		}
		// Only keep the last target object to avoid duplicate resources.
		result = append(result, targets[len(targets)-1])
	}

	return result, conditions, nil
}

// getComparisonSettings will return the system level settings related to the
// diff/normalization process.
func (m *appStateManager) getComparisonSettings() (string, map[string]v1alpha1.ResourceOverride, *settings.ResourcesFilter, string, string, error) {
	resourceOverrides, err := m.settingsMgr.GetResourceOverrides()
	if err != nil {
		return "", nil, nil, "", "", err
	}
	appLabelKey, err := m.settingsMgr.GetAppInstanceLabelKey()
	if err != nil {
		return "", nil, nil, "", "", err
	}
	resFilter, err := m.settingsMgr.GetResourcesFilter()
	if err != nil {
		return "", nil, nil, "", "", err
	}
	installationID, err := m.settingsMgr.GetInstallationID()
	if err != nil {
		return "", nil, nil, "", "", err
	}
	trackingMethod, err := m.settingsMgr.GetTrackingMethod()
	if err != nil {
		return "", nil, nil, "", "", err
	}
	return appLabelKey, resourceOverrides, resFilter, installationID, trackingMethod, nil
}

func isManagedNamespace(ns *unstructured.Unstructured, app *v1alpha1.Application) bool {
	return ns != nil && ns.GetKind() == kubeutil.NamespaceKind && ns.GetName() == app.Spec.Destination.Namespace && app.Spec.SyncPolicy != nil && app.Spec.SyncPolicy.ManagedNamespaceMetadata != nil
}

// partitionTargetObjsForSync returns the manifest subset passed to gitops-engine sync, and whether
// the full manifest set declared PreDelete and/or PostDelete hooks (for finalizer handling).
// Uses isPreDeleteHook / isPostDeleteHook / hasGitOpsEngineSyncPhaseHook from hook.go.
func partitionTargetObjsForSync(targetObjs []*unstructured.Unstructured) (syncObjs []*unstructured.Unstructured, hasPreDeleteHooks, hasPostDeleteHooks bool) {
	for _, obj := range targetObjs {
		if isPreDeleteHook(obj) {
			hasPreDeleteHooks = true
			if !hasGitOpsEngineSyncPhaseHook(obj) {
				continue
			}
		}
		if isPostDeleteHook(obj) {
			hasPostDeleteHooks = true
			if !hasGitOpsEngineSyncPhaseHook(obj) {
				continue
			}
		}
		syncObjs = append(syncObjs, obj)
	}
	return syncObjs, hasPreDeleteHooks, hasPostDeleteHooks
}

// CompareAppState compares application git state to the live app state, using the specified
// revision and supplied source. If revision or overrides are empty, then compares against
// revision and overrides in the app spec.
func (m *appStateManager) CompareAppState(app *v1alpha1.Application, project *v1alpha1.AppProject, revisions []string, sources []v1alpha1.ApplicationSource, noCache bool, noRevisionCache bool, localManifests []string, hasMultipleSources bool) (*comparisonResult, error) {
	ts := stats.NewTimingStats()
	logCtx := log.WithFields(applog.GetAppLogFields(app))

	// Build initial sync status
	syncStatus := &v1alpha1.SyncStatus{
		ComparedTo: v1alpha1.ComparedTo{
			Destination:       app.Spec.Destination,
			IgnoreDifferences: app.Spec.IgnoreDifferences,
		},
		Status: v1alpha1.SyncStatusCodeUnknown,
	}
	if hasMultipleSources {
		syncStatus.ComparedTo.Sources = sources
		syncStatus.Revisions = revisions
	} else {
		if len(sources) > 0 {
			syncStatus.ComparedTo.Source = sources[0]
		} else {
			logCtx.Warn("CompareAppState: sources should not be empty")
		}
		if len(revisions) > 0 {
			syncStatus.Revision = revisions[0]
		}
	}

	appLabelKey, resourceOverrides, resFilter, installationID, trackingMethod, err := m.getComparisonSettings()
	ts.AddCheckpoint("settings_ms")
	if err != nil {
		log.Infof("Basic comparison settings cannot be loaded, using unknown comparison: %s", err.Error())
		return &comparisonResult{syncStatus: syncStatus, healthStatus: health.HealthStatusUnknown}, nil
	}

	// do best effort loading live and target state to present as much information about app state as possible
	failedToLoadObjs := false
	conditions := make([]v1alpha1.ApplicationCondition, 0)

	destCluster, err := argo.GetDestinationCluster(context.Background(), app.Spec.Destination, m.db)
	if err != nil {
		return nil, err
	}

	logCtx.Infof("Comparing app state (cluster: %s, namespace: %s)", app.Spec.Destination.Server, app.Spec.Destination.Namespace)

	var targetObjs []*unstructured.Unstructured
	now := metav1.Now()

	var manifestInfos []*apiclient.ManifestResponse
	targetNsExists := false

	var revisionsMayHaveChanges bool

	if len(localManifests) == 0 {
		// If the length of revisions is not same as the length of sources,
		// we take the revisions from the sources directly for all the sources.
		if len(revisions) != len(sources) {
			revisions = make([]string, 0)
			for _, source := range sources {
				revisions = append(revisions, source.TargetRevision)
			}
		}

		targetObjs, manifestInfos, revisionsMayHaveChanges, err = m.GetRepoObjs(context.Background(), app, sources, appLabelKey, revisions, noCache, noRevisionCache, project.EffectiveSourceIntegrity(), project, true)
		if err != nil {
			targetObjs = make([]*unstructured.Unstructured, 0)
			msg := "Failed to load target state: " + err.Error()
			conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: msg, LastTransitionTime: &now})
			if firstSeen, ok := m.repoErrorCache.Load(app.Name); ok {
				if time.Since(firstSeen.(time.Time)) <= m.repoErrorGracePeriod && !noRevisionCache {
					// if first seen is less than grace period and it's not a Level 3 comparison,
					// ignore error and short circuit
					logCtx.Debugf("Ignoring repo error %v, already encountered error in grace period", err.Error())
					return nil, ErrCompareStateRepo
				}
			} else if !noRevisionCache {
				logCtx.Debugf("Ignoring repo error %v, new occurrence", err.Error())
				m.repoErrorCache.Store(app.Name, time.Now())
				return nil, ErrCompareStateRepo
			}
			failedToLoadObjs = true
		} else {
			m.repoErrorCache.Delete(app.Name)
		}
	} else {
		// Prevent applying local manifests for now when source integrity is enforced
		// This is also enforced on API level, but as a last resort, we also enforce it here
		if sourceintegrity.HasCriteria(project.EffectiveSourceIntegrity(), sources...) {
			msg := "Cannot use local manifests when source integrity is enforced"
			targetObjs = make([]*unstructured.Unstructured, 0)
			conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: msg, LastTransitionTime: &now})
			failedToLoadObjs = true
		} else {
			targetObjs, err = unmarshalManifests(localManifests)
			if err != nil {
				targetObjs = make([]*unstructured.Unstructured, 0)
				msg := "Failed to load local manifests: " + err.Error()
				conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: msg, LastTransitionTime: &now})
				failedToLoadObjs = true
			}
		}
		// empty out manifestInfoMap
		manifestInfos = make([]*apiclient.ManifestResponse, 0)
	}
	ts.AddCheckpoint("git_ms")

	var infoProvider kubeutil.ResourceInfoProvider
	infoProvider, err = m.liveStateCache.GetClusterCache(destCluster)
	if err != nil {
		infoProvider = &resourceInfoProviderStub{}
	}

	targetObjs, dedupConditions, err := NormalizeTargetObjects(app.Spec.Destination.Namespace, targetObjs, infoProvider, func(u *unstructured.Unstructured) error {
		return m.resourceTracking.SetAppInstance(u, appLabelKey, app.InstanceName(m.namespace), app.Spec.Destination.Namespace, v1alpha1.TrackingMethod(trackingMethod), installationID)
	})
	if err != nil {
		msg := "Failed to normalize target state: " + err.Error()
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: msg, LastTransitionTime: &now})
	}
	conditions = append(conditions, dedupConditions...)

	for i := len(targetObjs) - 1; i >= 0; i-- {
		targetObj := targetObjs[i]
		gvk := targetObj.GroupVersionKind()
		if resFilter.IsExcludedResource(gvk.Group, gvk.Kind, destCluster.Server) {
			targetObjs = append(targetObjs[:i], targetObjs[i+1:]...)
			conditions = append(conditions, v1alpha1.ApplicationCondition{
				Type:               v1alpha1.ApplicationConditionExcludedResourceWarning,
				Message:            fmt.Sprintf("Resource %s/%s %s is excluded in the settings", gvk.Group, gvk.Kind, targetObj.GetName()),
				LastTransitionTime: &now,
			})
		}

		// If we reach this path, this means that a namespace has been both defined in Git, as well in the
		// application's managedNamespaceMetadata. We want to ensure that this manifest is the one being used instead
		// of what is present in managedNamespaceMetadata.
		if isManagedNamespace(targetObj, app) {
			targetNsExists = true
		}
	}
	ts.AddCheckpoint("dedup_ms")

	liveObjByKey, err := m.liveStateCache.GetManagedLiveObjs(destCluster, app, targetObjs)
	if err != nil {
		liveObjByKey = make(map[kubeutil.ResourceKey]*unstructured.Unstructured)
		msg := "Failed to load live state: " + err.Error()
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: msg, LastTransitionTime: &now})
		failedToLoadObjs = true
	}

	logCtx.Debugf("Retrieved live manifests")
	// filter out all resources which are not permitted in the application project
	for k, v := range liveObjByKey {
		permitted, err := project.IsLiveResourcePermitted(v, destCluster, func(project string) ([]*v1alpha1.Cluster, error) {
			clusters, err := m.db.GetProjectClusters(context.TODO(), project)
			if err != nil {
				return nil, fmt.Errorf("failed to get clusters for project %q: %w", project, err)
			}
			return clusters, nil
		})
		if err != nil {
			msg := fmt.Sprintf("Failed to check if live resource %q is permitted in project %q: %s", k.String(), app.Spec.Project, err.Error())
			conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: msg, LastTransitionTime: &now})
			failedToLoadObjs = true
			continue
		}

		if !permitted {
			delete(liveObjByKey, k)
		}
	}

	for _, liveObj := range liveObjByKey {
		if liveObj != nil {
			appInstanceName := m.resourceTracking.GetAppName(liveObj, appLabelKey, v1alpha1.TrackingMethod(trackingMethod), installationID)
			if appInstanceName != "" && appInstanceName != app.InstanceName(m.namespace) {
				fqInstanceName := strings.ReplaceAll(appInstanceName, "_", "/")
				conditions = append(conditions, v1alpha1.ApplicationCondition{
					Type:               v1alpha1.ApplicationConditionSharedResourceWarning,
					Message:            fmt.Sprintf("%s/%s is part of applications %s and %s", liveObj.GetKind(), liveObj.GetName(), app.QualifiedName(), fqInstanceName),
					LastTransitionTime: &now,
				})
			}

			// For the case when a namespace is managed with `managedNamespaceMetadata` AND it has resource tracking
			// enabled (e.g. someone manually adds resource tracking labels or annotations), we need to do some
			// bookkeeping in order to prevent the managed namespace from being pruned.
			//
			// Live namespaces which are managed namespaces (i.e. application namespaces which are managed with
			// CreateNamespace=true and has non-nil managedNamespaceMetadata) will (usually) not have a corresponding
			// entry in source control. In order for the namespace not to risk being pruned, we'll need to generate a
			// namespace which we can compare the live namespace with. For that, we'll do the same as is done in
			// gitops-engine, the difference here being that we create a managed namespace which is only used for comparison.
			//
			// targetNsExists == true implies that it already exists as a target, so no need to add the namespace to the
			// targetObjs array.
			if isManagedNamespace(liveObj, app) && !targetNsExists {
				nsSpec := &corev1.Namespace{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: kubeutil.NamespaceKind}, ObjectMeta: metav1.ObjectMeta{Name: liveObj.GetName()}}
				managedNs, err := kubeutil.ToUnstructured(nsSpec)
				if err != nil {
					conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error(), LastTransitionTime: &now})
					failedToLoadObjs = true
					continue
				}

				// No need to care about the return value here, we just want the modified managedNs
				_, err = syncNamespace(app.Spec.SyncPolicy)(managedNs, liveObj)
				if err != nil {
					conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error(), LastTransitionTime: &now})
					failedToLoadObjs = true
				} else {
					targetObjs = append(targetObjs, managedNs)
				}
			}
		}
	}
	targetObjsForSync, hasPreDeleteHooks, hasPostDeleteHooks := partitionTargetObjsForSync(targetObjs)

	reconciliation := sync.Reconcile(targetObjsForSync, liveObjByKey, app.Spec.Destination.Namespace, infoProvider)
	ts.AddCheckpoint("live_ms")

	compareOptions, err := m.settingsMgr.GetResourceCompareOptions()
	if err != nil {
		log.Warnf("Could not get compare options from ConfigMap (assuming defaults): %v", err)
		compareOptions = settings.GetDefaultDiffOptions()
	}
	manifestRevisions := make([]string, 0)

	for _, manifestInfo := range manifestInfos {
		manifestRevisions = append(manifestRevisions, manifestInfo.Revision)
	}

	serverSideDiff := m.serverSideDiff ||
		resourceutil.HasAnnotationOption(app, common.AnnotationCompareOptions, "ServerSideDiff=true")

	// This allows turning SSD off for a given app if it is enabled at the
	// controller level
	if resourceutil.HasAnnotationOption(app, common.AnnotationCompareOptions, "ServerSideDiff=false") {
		serverSideDiff = false
	}

	useDiffCache := useDiffCache(noCache, manifestInfos, sources, app, manifestRevisions, m.statusRefreshTimeout, serverSideDiff, logCtx)

	diffConfigBuilder := argodiff.NewDiffConfigBuilder().
		WithDiffSettings(app.Spec.IgnoreDifferences, resourceOverrides, compareOptions.IgnoreAggregatedRoles, m.ignoreNormalizerOpts).
		WithTracking(appLabelKey, string(trackingMethod))

	if useDiffCache {
		diffConfigBuilder.WithCache(m.cache, app.InstanceName(m.namespace))
	} else {
		diffConfigBuilder.WithNoCache()
	}

	if resourceutil.HasAnnotationOption(app, common.AnnotationCompareOptions, "IncludeMutationWebhook=true") {
		diffConfigBuilder.WithIgnoreMutationWebhook(false)
	}

	gvkParser, err := m.getGVKParser(destCluster)
	if err != nil {
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionUnknownError, Message: err.Error(), LastTransitionTime: &now})
	}
	diffConfigBuilder.WithGVKParser(gvkParser)
	diffConfigBuilder.WithManager(common.ArgoCDSSAManager)

	diffConfigBuilder.WithServerSideDiff(serverSideDiff)

	if serverSideDiff {
		applier, cleanup, err := m.getServerSideDiffDryRunApplier(destCluster)
		if err != nil {
			log.Errorf("CompareAppState error getting server side diff dry run applier: %s", err)
			conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionUnknownError, Message: err.Error(), LastTransitionTime: &now})
		} else {
			defer cleanup()
			diffConfigBuilder.WithServerSideDryRunner(diff.NewK8sServerSideDryRunner(applier))
		}
	}

	// enable structured merge diff if application syncs with server-side apply
	if app.Spec.SyncPolicy != nil && app.Spec.SyncPolicy.SyncOptions.HasOption("ServerSideApply=true") {
		diffConfigBuilder.WithStructuredMergeDiff(true)
	}

	// it is necessary to ignore the error at this point to avoid creating duplicated
	// application conditions as argo.StateDiffs will validate this diffConfig again.
	diffConfig, _ := diffConfigBuilder.Build()

	diffResults, err := argodiff.StateDiffs(reconciliation.Live, reconciliation.Target, diffConfig)
	if err != nil {
		diffResults = &diff.DiffResultList{}
		failedToLoadObjs = true
		msg := "Failed to compare desired state to live state: " + err.Error()
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: msg, LastTransitionTime: &now})
	}
	ts.AddCheckpoint("diff_ms")

	syncCode := v1alpha1.SyncStatusCodeSynced
	managedResources := make([]managedResource, len(reconciliation.Target))
	resourceSummaries := make([]v1alpha1.ResourceStatus, len(reconciliation.Target))
	for i, targetObj := range reconciliation.Target {
		liveObj := reconciliation.Live[i]
		obj := liveObj
		if obj == nil {
			obj = targetObj
		}
		if obj == nil {
			continue
		}
		gvk := obj.GroupVersionKind()

		isSelfReferencedObj := m.isSelfReferencedObj(liveObj, targetObj, app.GetName(), v1alpha1.TrackingMethod(trackingMethod), installationID)

		resState := v1alpha1.ResourceStatus{
			Namespace:                    obj.GetNamespace(),
			Name:                         obj.GetName(),
			Kind:                         gvk.Kind,
			Version:                      gvk.Version,
			Group:                        gvk.Group,
			Hook:                         isHook(obj),
			RequiresPruning:              targetObj == nil && liveObj != nil && isSelfReferencedObj,
			RequiresDeletionConfirmation: isObjRequiresDeletionConfirmation(targetObj, app) || isObjRequiresDeletionConfirmation(liveObj, app),
		}
		if targetObj != nil {
			resState.SyncWave = int64(syncwaves.Wave(targetObj))
		} else if resState.Hook {
			for _, hookObj := range reconciliation.Hooks {
				if hookObj.GetName() == liveObj.GetName() && hookObj.GetKind() == liveObj.GetKind() && hookObj.GetNamespace() == liveObj.GetNamespace() {
					resState.SyncWave = int64(syncwaves.Wave(hookObj))
					break
				}
			}
		}

		var diffResult diff.DiffResult
		if i < len(diffResults.Diffs) {
			diffResult = diffResults.Diffs[i]
		} else {
			diffResult = diff.DiffResult{Modified: false, NormalizedLive: []byte("{}"), PredictedLive: []byte("{}")}
		}

		// For the case when a namespace is managed with `managedNamespaceMetadata` AND it has resource tracking
		// enabled (e.g. someone manually adds resource tracking labels or annotations), we need to do some
		// bookkeeping in order to ensure that it's not considered `OutOfSync` (since it does not exist in source
		// control).
		//
		// This is in addition to the bookkeeping we do (see `isManagedNamespace` and its references) to prevent said
		// namespace from being pruned.
		isManagedNs := isManagedNamespace(targetObj, app) && liveObj == nil

		switch {
		case resState.Hook || ignore.Ignore(obj) || (targetObj != nil && hookutil.Skip(targetObj)) || !isSelfReferencedObj:
			// For resource hooks, skipped resources or objects that may have
			// been created by another controller with annotations copied from
			// the source object, don't store sync status, and do not affect
			// overall sync status
		case !isManagedNs && (diffResult.Modified || targetObj == nil || liveObj == nil):
			// Set resource state to OutOfSync since one of the following is true:
			// * target and live resource are different
			// * target resource not defined and live resource is extra
			// * target resource present but live resource is missing
			resState.Status = v1alpha1.SyncStatusCodeOutOfSync
			// we ignore the status if the obj needs pruning AND we have the annotation
			needsPruning := targetObj == nil && liveObj != nil
			if !needsPruning || !resourceutil.HasAnnotationOption(obj, common.AnnotationCompareOptions, "IgnoreExtraneous") {
				syncCode = v1alpha1.SyncStatusCodeOutOfSync
			}
		default:
			resState.Status = v1alpha1.SyncStatusCodeSynced
		}
		// set unknown status to all resource that are not permitted in the app project
		isNamespaced, err := m.liveStateCache.IsNamespaced(destCluster, gvk.GroupKind())
		if !project.IsGroupKindNamePermitted(gvk.GroupKind(), obj.GetName(), isNamespaced && err == nil) {
			resState.Status = v1alpha1.SyncStatusCodeUnknown
		}

		if isNamespaced && obj.GetNamespace() == "" {
			conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionInvalidSpecError, Message: fmt.Sprintf("Namespace for %s %s is missing.", obj.GetName(), gvk.String()), LastTransitionTime: &now})
		}

		// we can't say anything about the status if we were unable to get the target objects
		if failedToLoadObjs {
			resState.Status = v1alpha1.SyncStatusCodeUnknown
		}

		resourceVersion := ""
		if liveObj != nil {
			resourceVersion = liveObj.GetResourceVersion()
		}
		managedResources[i] = managedResource{
			Name:            resState.Name,
			Namespace:       resState.Namespace,
			Group:           resState.Group,
			Kind:            resState.Kind,
			Version:         resState.Version,
			Live:            liveObj,
			Target:          targetObj,
			Diff:            diffResult,
			Hook:            resState.Hook,
			ResourceVersion: resourceVersion,
		}
		resourceSummaries[i] = resState
	}

	if failedToLoadObjs {
		syncCode = v1alpha1.SyncStatusCodeUnknown
	} else if app.HasChangedManagedNamespaceMetadata() {
		syncCode = v1alpha1.SyncStatusCodeOutOfSync
	}

	syncStatus.Status = syncCode

	// Update the initial revision to the resolved manifest SHA
	if hasMultipleSources {
		syncStatus.Revisions = manifestRevisions
	} else if len(manifestRevisions) > 0 {
		syncStatus.Revision = manifestRevisions[0]
	}

	ts.AddCheckpoint("sync_ms")

	healthStatus, err := setApplicationHealth(managedResources, resourceSummaries, resourceOverrides, app, m.persistResourceHealth)
	if err != nil {
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: "error setting app health: " + err.Error(), LastTransitionTime: &now})
	}

	for _, manifestInfo := range manifestInfos {
		if manifestInfo != nil {
			if err = manifestInfo.SourceIntegrityResult.AsError(); err != nil {
				conditions = append(conditions, v1alpha1.ApplicationCondition{
					Type:               v1alpha1.ApplicationConditionComparisonError,
					Message:            err.Error(),
					LastTransitionTime: &now,
				})
			}

			// Can happen during migration when the legacy SignatureKeys are used AND the repo-server have not yet been
			// upgraded to version using Source Integrity. So the manifests comes in with verifyResult only, that we have to interpret anyway.
			if manifestInfo.SourceIntegrityResult == nil && manifestInfo.VerifyResult != "" { // nolint:staticcheck
				legacyVerifySignature := len(project.Spec.SignatureKeys) > 0 && sourceintegrity.IsGPGEnabled() // nolint:staticcheck
				if legacyVerifySignature {
					var keys []string
					for _, key := range project.Spec.SignatureKeys { // nolint:staticcheck
						keys = append(keys, key.KeyID)
					}
					condition := sourceintegrity.VerifyGnuPGSignature(manifestInfo.Revision, keys, manifestInfo.VerifyResult) // nolint:staticcheck
					if condition != nil {
						conditions = append(conditions, *condition)
					}
				}
			}
		}
	}

	compRes := comparisonResult{
		syncStatus:              syncStatus,
		healthStatus:            healthStatus,
		resources:               resourceSummaries,
		managedResources:        managedResources,
		reconciliationResult:    reconciliation,
		diffConfig:              diffConfig,
		diffResultList:          diffResults,
		hasPostDeleteHooks:      hasPostDeleteHooks,
		hasPreDeleteHooks:       hasPreDeleteHooks,
		revisionsMayHaveChanges: revisionsMayHaveChanges,
	}

	if hasMultipleSources {
		for _, manifestInfo := range manifestInfos {
			compRes.appSourceTypes = append(compRes.appSourceTypes, v1alpha1.ApplicationSourceType(manifestInfo.SourceType))
		}
	} else {
		for _, manifestInfo := range manifestInfos {
			compRes.appSourceType = v1alpha1.ApplicationSourceType(manifestInfo.SourceType)
			break
		}
	}

	app.Status.SetConditions(conditions, map[v1alpha1.ApplicationConditionType]bool{
		v1alpha1.ApplicationConditionComparisonError:         true,
		v1alpha1.ApplicationConditionSharedResourceWarning:   true,
		v1alpha1.ApplicationConditionRepeatedResourceWarning: true,
		v1alpha1.ApplicationConditionExcludedResourceWarning: true,
	})
	ts.AddCheckpoint("health_ms")
	compRes.timings = ts.Timings()
	return &compRes, nil
}

// useDiffCache will determine if the diff should be calculated based
// on the existing live state cache or not.
func useDiffCache(noCache bool, manifestInfos []*apiclient.ManifestResponse, sources []v1alpha1.ApplicationSource, app *v1alpha1.Application, manifestRevisions []string, statusRefreshTimeout time.Duration, serverSideDiff bool, log *log.Entry) bool {
	if noCache {
		log.WithField("useDiffCache", "false").Debug("noCache is true")
		return false
	}
	refreshType, refreshRequested := app.IsRefreshRequested()
	if refreshRequested {
		log.WithField("useDiffCache", "false").Debugf("refresh type %s requested", string(refreshType))
		return false
	}
	// serverSideDiff should still use cache even if status is expired.
	// This is an attempt to avoid hitting k8s API server too frequently during
	// app refresh with serverSideDiff is enabled. If there are negative side
	// effects identified with this approach, the serverSideDiff should be removed
	// from this condition.
	if app.Status.Expired(statusRefreshTimeout) && !serverSideDiff {
		log.WithField("useDiffCache", "false").Debug("app.status.expired")
		return false
	}

	if len(manifestInfos) != len(sources) {
		log.WithField("useDiffCache", "false").Debug("manifestInfos len != sources len")
		return false
	}

	revisionChanged := !reflect.DeepEqual(app.Status.GetRevisions(), manifestRevisions)
	if revisionChanged {
		log.WithField("useDiffCache", "false").Debug("revisionChanged")
		return false
	}

	if !specEqualsCompareTo(app.Spec, sources, app.Status.Sync.ComparedTo) {
		log.WithField("useDiffCache", "false").Debug("specChanged")
		return false
	}

	log.WithField("useDiffCache", "true").Debug("using diff cache")
	return true
}

// specEqualsCompareTo compares the application spec to the comparedTo status. It normalizes the destination to match
// the comparedTo destination before comparing. It does not mutate the original spec or comparedTo.
func specEqualsCompareTo(spec v1alpha1.ApplicationSpec, sources []v1alpha1.ApplicationSource, comparedTo v1alpha1.ComparedTo) bool {
	// Make a copy to be sure we don't mutate the original.
	specCopy := spec.DeepCopy()
	compareToSpec := specCopy.BuildComparedToStatus(sources)
	return reflect.DeepEqual(comparedTo, compareToSpec)
}

func isObjRequiresDeletionConfirmation(obj *unstructured.Unstructured, app *v1alpha1.Application) bool {
	if obj == nil {
		return false
	}
	deleteOption := resourceutil.GetAnnotationOptionValue(obj, synccommon.AnnotationSyncOptions, synccommon.SyncOptionDelete)
	if deleteOption == nil && app.Spec.SyncPolicy != nil {
		deleteOption = app.Spec.SyncPolicy.SyncOptions.GetOptionValue(synccommon.SyncOptionDelete)
	}
	if deleteOption != nil && *deleteOption == synccommon.SyncValueConfirm {
		return true
	}

	pruneOption := resourceutil.GetAnnotationOptionValue(obj, synccommon.AnnotationSyncOptions, synccommon.SyncOptionPrune)
	if pruneOption == nil && app.Spec.SyncPolicy != nil {
		pruneOption = app.Spec.SyncPolicy.SyncOptions.GetOptionValue(synccommon.SyncOptionPrune)
	}
	if pruneOption != nil && *pruneOption == synccommon.SyncValueConfirm {
		return true
	}

	return false
}

func (m *appStateManager) persistRevisionHistory(
	app *v1alpha1.Application,
	revision string,
	source v1alpha1.ApplicationSource,
	revisions []string,
	sources []v1alpha1.ApplicationSource,
	hasMultipleSources bool,
	startedAt metav1.Time,
	initiatedBy v1alpha1.OperationInitiator,
) error {
	var nextID int64
	if len(app.Status.History) > 0 {
		nextID = app.Status.History.LastRevisionHistory().ID + 1
	}

	if hasMultipleSources {
		app.Status.History = append(app.Status.History, v1alpha1.RevisionHistory{
			DeployedAt:      metav1.NewTime(time.Now().UTC()),
			DeployStartedAt: &startedAt,
			ID:              nextID,
			Sources:         sources,
			Revisions:       revisions,
			InitiatedBy:     initiatedBy,
		})
	} else {
		app.Status.History = append(app.Status.History, v1alpha1.RevisionHistory{
			Revision:        revision,
			DeployedAt:      metav1.NewTime(time.Now().UTC()),
			DeployStartedAt: &startedAt,
			ID:              nextID,
			Source:          source,
			InitiatedBy:     initiatedBy,
		})
	}

	app.Status.History = app.Status.History.Trunc(app.Spec.GetRevisionHistoryLimit())

	patch, err := json.Marshal(map[string]map[string][]v1alpha1.RevisionHistory{
		"status": {
			"history": app.Status.History,
		},
	})
	if err != nil {
		return fmt.Errorf("error marshaling revision history patch: %w", err)
	}
	_, err = m.appclientset.ArgoprojV1alpha1().Applications(app.Namespace).Patch(context.Background(), app.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

// NewAppStateManager creates new instance of AppStateManager
func NewAppStateManager(
	db db.ArgoDB,
	appclientset appclientset.Interface,
	repoClientset apiclient.Clientset,
	namespace string,
	kubectl kubeutil.Kubectl,
	onKubectlRun kubeutil.OnKubectlRunFunc,
	settingsMgr *settings.SettingsManager,
	liveStateCache statecache.LiveStateCache,
	metricsServer *metrics.MetricsServer,
	cache *appstatecache.Cache,
	statusRefreshTimeout time.Duration,
	resourceTracking argo.ResourceTracking,
	persistResourceHealth bool,
	repoErrorGracePeriod time.Duration,
	serverSideDiff bool,
	ignoreNormalizerOpts normalizers.IgnoreNormalizerOpts,
) AppStateManager {
	return &appStateManager{
		liveStateCache:        liveStateCache,
		cache:                 cache,
		db:                    db,
		appclientset:          appclientset,
		kubectl:               kubectl,
		onKubectlRun:          onKubectlRun,
		repoClientset:         repoClientset,
		namespace:             namespace,
		settingsMgr:           settingsMgr,
		metricsServer:         metricsServer,
		statusRefreshTimeout:  statusRefreshTimeout,
		resourceTracking:      resourceTracking,
		persistResourceHealth: persistResourceHealth,
		repoErrorGracePeriod:  repoErrorGracePeriod,
		serverSideDiff:        serverSideDiff,
		ignoreNormalizerOpts:  ignoreNormalizerOpts,
	}
}

// isSelfReferencedObj returns whether the given obj is managed by the application
// according to the values of the tracking id (aka app instance value) annotation.
// It returns true when all of the properties of the tracking id (app name, namespace,
// group and kind) match the properties of the live object, or if the tracking method
// used does not provide the required properties for matching.
// Reference: https://github.com/argoproj/argo-cd/issues/8683
func (m *appStateManager) isSelfReferencedObj(live, config *unstructured.Unstructured, appName string, trackingMethod v1alpha1.TrackingMethod, installationID string) bool {
	if live == nil {
		return true
	}

	// If tracking method doesn't contain required metadata for this check,
	// we are not able to determine and just assume the object to be managed.
	if trackingMethod == v1alpha1.TrackingMethodLabel {
		return true
	}

	// config != nil is the best-case scenario for constructing an accurate
	// Tracking ID. `config` is the "desired state" (from git/helm/etc.).
	// Using the desired state is important when there is an ApiGroup upgrade.
	// When upgrading, the comparison must be made with the new tracking ID.
	// Example:
	//     live resource annotation will be:
	//        ingress-app:extensions/Ingress:default/some-ingress
	//     when it should be:
	//        ingress-app:networking.k8s.io/Ingress:default/some-ingress
	// More details in: https://github.com/argoproj/argo-cd/pull/11012
	var aiv argo.AppInstanceValue
	if config != nil {
		aiv = argo.UnstructuredToAppInstanceValue(config, appName, "")
		return isSelfReferencedObj(live, aiv)
	}

	// If config is nil then compare the live resource with the value
	// of the annotation. In this case, in order to validate if obj is
	// managed by this application, the values from the annotation have
	// to match the properties from the live object. Cluster scoped objects
	// carry the app's destination namespace in the tracking annotation,
	// but are unique in GVK + name combination.
	appInstance := m.resourceTracking.GetAppInstance(live, trackingMethod, installationID)
	if appInstance != nil {
		return isSelfReferencedObj(live, *appInstance)
	}
	return true
}

// isSelfReferencedObj returns true if the given Tracking ID (`aiv`) matches
// the given object. It returns false when the ID doesn't match. This sometimes
// happens when a tracking label or annotation gets accidentally copied to a
// different resource.
func isSelfReferencedObj(obj *unstructured.Unstructured, aiv argo.AppInstanceValue) bool {
	return (obj.GetNamespace() == aiv.Namespace || obj.GetNamespace() == "") &&
		obj.GetName() == aiv.Name &&
		obj.GetObjectKind().GroupVersionKind().Group == aiv.Group &&
		obj.GetObjectKind().GroupVersionKind().Kind == aiv.Kind
}
