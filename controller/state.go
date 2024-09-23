package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	goSync "sync"
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync"
	hookutil "github.com/argoproj/gitops-engine/pkg/sync/hook"
	"github.com/argoproj/gitops-engine/pkg/sync/ignore"
	resourceutil "github.com/argoproj/gitops-engine/pkg/sync/resource"
	"github.com/argoproj/gitops-engine/pkg/sync/syncwaves"
	kubeutil "github.com/argoproj/gitops-engine/pkg/utils/kube"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v2/common"
	statecache "github.com/argoproj/argo-cd/v2/controller/cache"
	"github.com/argoproj/argo-cd/v2/controller/metrics"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/app/path"
	"github.com/argoproj/argo-cd/v2/util/argo"
	argodiff "github.com/argoproj/argo-cd/v2/util/argo/diff"
	"github.com/argoproj/argo-cd/v2/util/argo/normalizers"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/gpg"
	"github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/argoproj/argo-cd/v2/util/stats"
)

var CompareStateRepoError = errors.New("failed to get repo objects")

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
	CompareAppState(app *v1alpha1.Application, project *v1alpha1.AppProject, revisions []string, sources []v1alpha1.ApplicationSource, noCache bool, noRevisionCache bool, localObjects []string, hasMultipleSources bool, rollback bool) (*comparisonResult, error)
	SyncAppState(app *v1alpha1.Application, state *v1alpha1.OperationState)
	GetRepoObjs(app *v1alpha1.Application, sources []v1alpha1.ApplicationSource, appLabelKey string, revisions []string, noCache, noRevisionCache, verifySignature bool, proj *v1alpha1.AppProject, rollback bool) ([]*unstructured.Unstructured, []*apiclient.ManifestResponse, bool, error)
}

// comparisonResult holds the state of an application after the reconciliation
type comparisonResult struct {
	syncStatus           *v1alpha1.SyncStatus
	healthStatus         *v1alpha1.HealthStatus
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
	revisionUpdated    bool
}

func (res *comparisonResult) GetSyncStatus() *v1alpha1.SyncStatus {
	return res.syncStatus
}

func (res *comparisonResult) GetHealthStatus() *v1alpha1.HealthStatus {
	return res.healthStatus
}

// appStateManager allows to compare applications to git
type appStateManager struct {
	metricsServer         *metrics.MetricsServer
	db                    db.ArgoDB
	settingsMgr           *settings.SettingsManager
	appclientset          appclientset.Interface
	projInformer          cache.SharedIndexInformer
	kubectl               kubeutil.Kubectl
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

// GetRepoObjs will generate the manifests for the given application delegating the
// task to the repo-server. It returns the list of generated manifests as unstructured
// objects. It also returns the full response from all calls to the repo server as the
// second argument.
func (m *appStateManager) GetRepoObjs(app *v1alpha1.Application, sources []v1alpha1.ApplicationSource, appLabelKey string, revisions []string, noCache, noRevisionCache, verifySignature bool, proj *v1alpha1.AppProject, rollback bool) ([]*unstructured.Unstructured, []*apiclient.ManifestResponse, bool, error) {
	ts := stats.NewTimingStats()
	helmRepos, err := m.db.ListHelmRepositories(context.Background())
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to list Helm repositories: %w", err)
	}
	permittedHelmRepos, err := argo.GetPermittedRepos(proj, helmRepos)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to get permitted Helm repositories for project %q: %w", proj.Name, err)
	}

	ts.AddCheckpoint("repo_ms")
	helmRepositoryCredentials, err := m.db.GetAllHelmRepositoryCredentials(context.Background())
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to get Helm credentials: %w", err)
	}
	permittedHelmCredentials, err := argo.GetPermittedReposCredentials(proj, helmRepositoryCredentials)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to get permitted Helm credentials for project %q: %w", proj.Name, err)
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

	ts.AddCheckpoint("build_options_ms")
	serverVersion, apiResources, err := m.liveStateCache.GetVersionsInfo(app.Spec.Destination.Server)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to get cluster version for cluster %q: %w", app.Spec.Destination.Server, err)
	}
	conn, repoClient, err := m.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to connect to repo server: %w", err)
	}
	defer io.Close(conn)

	manifestInfos := make([]*apiclient.ManifestResponse, 0)
	targetObjs := make([]*unstructured.Unstructured, 0)

	// Store the map of all sources having ref field into a map for applications with sources field
	// If it's for a rollback process, the refSources[*].targetRevision fields are the desired
	// revisions for the rollback
	refSources, err := argo.GetRefSources(context.Background(), sources, app.Spec.Project, m.db.GetRepository, revisions, rollback)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to get ref sources: %w", err)
	}

	revisionUpdated := false

	atLeastOneRevisionIsNotPossibleToBeUpdated := false

	keyManifestGenerateAnnotationVal, keyManifestGenerateAnnotationExists := app.Annotations[v1alpha1.AnnotationKeyManifestGeneratePaths]

	for i, source := range sources {
		if len(revisions) < len(sources) || revisions[i] == "" {
			revisions[i] = source.TargetRevision
		}
		repo, err := m.db.GetRepository(context.Background(), source.RepoURL, proj.Name)
		if err != nil {
			return nil, nil, false, fmt.Errorf("failed to get repo %q: %w", source.RepoURL, err)
		}
		kustomizeOptions, err := kustomizeSettings.GetOptions(source)
		if err != nil {
			return nil, nil, false, fmt.Errorf("failed to get Kustomize options for source %d of %d: %w", i+1, len(sources), err)
		}

		syncedRevision := app.Status.Sync.Revision
		if app.Spec.HasMultipleSources() {
			if i < len(app.Status.Sync.Revisions) {
				syncedRevision = app.Status.Sync.Revisions[i]
			} else {
				syncedRevision = ""
			}
		}

		revision := revisions[i]

		if !source.IsHelm() && syncedRevision != "" && keyManifestGenerateAnnotationExists && keyManifestGenerateAnnotationVal != "" {
			// Validate the manifest-generate-path annotation to avoid generating manifests if it has not changed.
			updateRevisionResult, err := repoClient.UpdateRevisionForPaths(context.Background(), &apiclient.UpdateRevisionForPathsRequest{
				Repo:               repo,
				Revision:           revision,
				SyncedRevision:     syncedRevision,
				NoRevisionCache:    noRevisionCache,
				Paths:              path.GetAppRefreshPaths(app),
				AppLabelKey:        appLabelKey,
				AppName:            app.InstanceName(m.namespace),
				Namespace:          app.Spec.Destination.Namespace,
				ApplicationSource:  &source,
				KubeVersion:        serverVersion,
				ApiVersions:        argo.APIResourcesToStrings(apiResources, true),
				TrackingMethod:     string(argo.GetTrackingMethod(m.settingsMgr)),
				RefSources:         refSources,
				HasMultipleSources: app.Spec.HasMultipleSources(),
			})
			if err != nil {
				return nil, nil, false, fmt.Errorf("failed to compare revisions for source %d of %d: %w", i+1, len(sources), err)
			}
			if updateRevisionResult.Changes {
				revisionUpdated = true
			}

			// Generate manifests should use same revision as updateRevisionForPaths, because HEAD revision may be different between these two calls
			if updateRevisionResult.Revision != "" {
				revision = updateRevisionResult.Revision
			}
		} else {
			// revisionUpdated is set to true if at least one revision is not possible to be updated,
			atLeastOneRevisionIsNotPossibleToBeUpdated = true
		}

		log.Debugf("Generating Manifest for source %s revision %s", source, revision)
		manifestInfo, err := repoClient.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
			Repo:                            repo,
			Repos:                           permittedHelmRepos,
			Revision:                        revision,
			NoCache:                         noCache,
			NoRevisionCache:                 noRevisionCache,
			AppLabelKey:                     appLabelKey,
			AppName:                         app.InstanceName(m.namespace),
			Namespace:                       app.Spec.Destination.Namespace,
			ApplicationSource:               &source,
			KustomizeOptions:                kustomizeOptions,
			KubeVersion:                     serverVersion,
			ApiVersions:                     argo.APIResourcesToStrings(apiResources, true),
			VerifySignature:                 verifySignature,
			HelmRepoCreds:                   permittedHelmCredentials,
			TrackingMethod:                  string(argo.GetTrackingMethod(m.settingsMgr)),
			EnabledSourceTypes:              enabledSourceTypes,
			HelmOptions:                     helmOptions,
			HasMultipleSources:              app.Spec.HasMultipleSources(),
			RefSources:                      refSources,
			ProjectName:                     proj.Name,
			ProjectSourceRepos:              proj.Spec.SourceRepos,
			AnnotationManifestGeneratePaths: app.GetAnnotation(v1alpha1.AnnotationKeyManifestGeneratePaths),
		})
		if err != nil {
			return nil, nil, false, fmt.Errorf("failed to generate manifest for source %d of %d: %w", i+1, len(sources), err)
		}

		targetObj, err := unmarshalManifests(manifestInfo.Manifests)
		if err != nil {
			return nil, nil, false, fmt.Errorf("failed to unmarshal manifests for source %d of %d: %w", i+1, len(sources), err)
		}
		targetObjs = append(targetObjs, targetObj...)
		manifestInfos = append(manifestInfos, manifestInfo)
	}

	ts.AddCheckpoint("manifests_ms")
	logCtx := log.WithField("application", app.QualifiedName())
	for k, v := range ts.Timings() {
		logCtx = logCtx.WithField(k, v.Milliseconds())
	}
	logCtx = logCtx.WithField("time_ms", time.Since(ts.StartTime).Milliseconds())
	logCtx.Info("GetRepoObjs stats")

	// in case if annotation not exists, we should always execute selfheal if manifests changed
	if atLeastOneRevisionIsNotPossibleToBeUpdated {
		revisionUpdated = true
	}

	return targetObjs, manifestInfos, revisionUpdated, nil
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

func DeduplicateTargetObjects(
	namespace string,
	objs []*unstructured.Unstructured,
	infoProvider kubeutil.ResourceInfoProvider,
) ([]*unstructured.Unstructured, []v1alpha1.ApplicationCondition, error) {
	targetByKey := make(map[kubeutil.ResourceKey][]*unstructured.Unstructured)
	for i := range objs {
		obj := objs[i]
		if obj == nil {
			continue
		}
		isNamespaced := kubeutil.IsNamespacedOrUnknown(infoProvider, obj.GroupVersionKind().GroupKind())
		if !isNamespaced {
			obj.SetNamespace("")
		} else if obj.GetNamespace() == "" {
			obj.SetNamespace(namespace)
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
			now := metav1.Now()
			conditions = append(conditions, v1alpha1.ApplicationCondition{
				Type:               v1alpha1.ApplicationConditionRepeatedResourceWarning,
				Message:            fmt.Sprintf("Resource %s appeared %d times among application resources.", key.String(), len(targets)),
				LastTransitionTime: &now,
			})
		}
		result = append(result, targets[len(targets)-1])
	}

	return result, conditions, nil
}

// getComparisonSettings will return the system level settings related to the
// diff/normalization process.
func (m *appStateManager) getComparisonSettings() (string, map[string]v1alpha1.ResourceOverride, *settings.ResourcesFilter, error) {
	resourceOverrides, err := m.settingsMgr.GetResourceOverrides()
	if err != nil {
		return "", nil, nil, err
	}
	appLabelKey, err := m.settingsMgr.GetAppInstanceLabelKey()
	if err != nil {
		return "", nil, nil, err
	}
	resFilter, err := m.settingsMgr.GetResourcesFilter()
	if err != nil {
		return "", nil, nil, err
	}
	return appLabelKey, resourceOverrides, resFilter, nil
}

// verifyGnuPGSignature verifies the result of a GnuPG operation for a given git
// revision.
func verifyGnuPGSignature(revision string, project *v1alpha1.AppProject, manifestInfo *apiclient.ManifestResponse) []v1alpha1.ApplicationCondition {
	now := metav1.Now()
	conditions := make([]v1alpha1.ApplicationCondition, 0)
	// We need to have some data in the verification result to parse, otherwise there was no signature
	if manifestInfo.VerifyResult != "" {
		verifyResult := gpg.ParseGitCommitVerification(manifestInfo.VerifyResult)
		switch verifyResult.Result {
		case gpg.VerifyResultGood:
			// This is the only case we allow to sync to, but we need to make sure signing key is allowed
			validKey := false
			for _, k := range project.Spec.SignatureKeys {
				if gpg.KeyID(k.KeyID) == gpg.KeyID(verifyResult.KeyID) && gpg.KeyID(k.KeyID) != "" {
					validKey = true
					break
				}
			}
			if !validKey {
				msg := fmt.Sprintf("Found good signature made with %s key %s, but this key is not allowed in AppProject",
					verifyResult.Cipher, verifyResult.KeyID)
				conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: msg, LastTransitionTime: &now})
			}
		case gpg.VerifyResultInvalid:
			msg := fmt.Sprintf("Found signature made with %s key %s, but verification result was invalid: '%s'",
				verifyResult.Cipher, verifyResult.KeyID, verifyResult.Message)
			conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: msg, LastTransitionTime: &now})
		default:
			msg := fmt.Sprintf("Could not verify commit signature on revision '%s', check logs for more information.", revision)
			conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: msg, LastTransitionTime: &now})
		}
	} else {
		msg := fmt.Sprintf("Target revision %s in Git is not signed, but a signature is required", revision)
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: msg, LastTransitionTime: &now})
	}

	return conditions
}

func isManagedNamespace(ns *unstructured.Unstructured, app *v1alpha1.Application) bool {
	return ns != nil && ns.GetKind() == kubeutil.NamespaceKind && ns.GetName() == app.Spec.Destination.Namespace && app.Spec.SyncPolicy != nil && app.Spec.SyncPolicy.ManagedNamespaceMetadata != nil
}

// CompareAppState compares application git state to the live app state, using the specified
// revision and supplied source. If revision or overrides are empty, then compares against
// revision and overrides in the app spec.
func (m *appStateManager) CompareAppState(app *v1alpha1.Application, project *v1alpha1.AppProject, revisions []string, sources []v1alpha1.ApplicationSource, noCache bool, noRevisionCache bool, localManifests []string, hasMultipleSources bool, rollback bool) (*comparisonResult, error) {
	ts := stats.NewTimingStats()
	appLabelKey, resourceOverrides, resFilter, err := m.getComparisonSettings()

	ts.AddCheckpoint("settings_ms")

	// return unknown comparison result if basic comparison settings cannot be loaded
	if err != nil {
		if hasMultipleSources {
			return &comparisonResult{
				syncStatus: &v1alpha1.SyncStatus{
					ComparedTo: v1alpha1.ComparedTo{Destination: app.Spec.Destination, Sources: sources, IgnoreDifferences: app.Spec.IgnoreDifferences},
					Status:     v1alpha1.SyncStatusCodeUnknown,
					Revisions:  revisions,
				},
				healthStatus: &v1alpha1.HealthStatus{Status: health.HealthStatusUnknown},
			}, nil
		} else {
			return &comparisonResult{
				syncStatus: &v1alpha1.SyncStatus{
					ComparedTo: v1alpha1.ComparedTo{Source: sources[0], Destination: app.Spec.Destination, IgnoreDifferences: app.Spec.IgnoreDifferences},
					Status:     v1alpha1.SyncStatusCodeUnknown,
					Revision:   revisions[0],
				},
				healthStatus: &v1alpha1.HealthStatus{Status: health.HealthStatusUnknown},
			}, nil
		}
	}

	// When signature keys are defined in the project spec, we need to verify the signature on the Git revision
	verifySignature := false
	if len(project.Spec.SignatureKeys) > 0 && gpg.IsGPGEnabled() {
		verifySignature = true
	}

	// do best effort loading live and target state to present as much information about app state as possible
	failedToLoadObjs := false
	conditions := make([]v1alpha1.ApplicationCondition, 0)

	logCtx := log.WithField("application", app.QualifiedName())
	logCtx.Infof("Comparing app state (cluster: %s, namespace: %s)", app.Spec.Destination.Server, app.Spec.Destination.Namespace)

	var targetObjs []*unstructured.Unstructured
	now := metav1.Now()

	var manifestInfos []*apiclient.ManifestResponse
	targetNsExists := false

	var revisionUpdated bool

	if len(localManifests) == 0 {
		// If the length of revisions is not same as the length of sources,
		// we take the revisions from the sources directly for all the sources.
		if len(revisions) != len(sources) {
			revisions = make([]string, 0)
			for _, source := range sources {
				revisions = append(revisions, source.TargetRevision)
			}
		}

		targetObjs, manifestInfos, revisionUpdated, err = m.GetRepoObjs(app, sources, appLabelKey, revisions, noCache, noRevisionCache, verifySignature, project, rollback)
		if err != nil {
			targetObjs = make([]*unstructured.Unstructured, 0)
			msg := fmt.Sprintf("Failed to load target state: %s", err.Error())
			conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: msg, LastTransitionTime: &now})
			if firstSeen, ok := m.repoErrorCache.Load(app.Name); ok {
				if time.Since(firstSeen.(time.Time)) <= m.repoErrorGracePeriod && !noRevisionCache {
					// if first seen is less than grace period and it's not a Level 3 comparison,
					// ignore error and short circuit
					logCtx.Debugf("Ignoring repo error %v, already encountered error in grace period", err.Error())
					return nil, CompareStateRepoError
				}
			} else if !noRevisionCache {
				logCtx.Debugf("Ignoring repo error %v, new occurrence", err.Error())
				m.repoErrorCache.Store(app.Name, time.Now())
				return nil, CompareStateRepoError
			}
			failedToLoadObjs = true
		} else {
			m.repoErrorCache.Delete(app.Name)
		}
	} else {
		// Prevent applying local manifests for now when signature verification is enabled
		// This is also enforced on API level, but as a last resort, we also enforce it here
		if gpg.IsGPGEnabled() && verifySignature {
			msg := "Cannot use local manifests when signature verification is required"
			targetObjs = make([]*unstructured.Unstructured, 0)
			conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: msg, LastTransitionTime: &now})
			failedToLoadObjs = true
		} else {
			targetObjs, err = unmarshalManifests(localManifests)
			if err != nil {
				targetObjs = make([]*unstructured.Unstructured, 0)
				msg := fmt.Sprintf("Failed to load local manifests: %s", err.Error())
				conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: msg, LastTransitionTime: &now})
				failedToLoadObjs = true
			}
		}
		// empty out manifestInfoMap
		manifestInfos = make([]*apiclient.ManifestResponse, 0)
	}
	ts.AddCheckpoint("git_ms")

	var infoProvider kubeutil.ResourceInfoProvider
	infoProvider, err = m.liveStateCache.GetClusterCache(app.Spec.Destination.Server)
	if err != nil {
		infoProvider = &resourceInfoProviderStub{}
	}
	targetObjs, dedupConditions, err := DeduplicateTargetObjects(app.Spec.Destination.Namespace, targetObjs, infoProvider)
	if err != nil {
		msg := fmt.Sprintf("Failed to deduplicate target state: %s", err.Error())
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: msg, LastTransitionTime: &now})
	}
	conditions = append(conditions, dedupConditions...)
	for i := len(targetObjs) - 1; i >= 0; i-- {
		targetObj := targetObjs[i]
		gvk := targetObj.GroupVersionKind()
		if resFilter.IsExcludedResource(gvk.Group, gvk.Kind, app.Spec.Destination.Server) {
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

	liveObjByKey, err := m.liveStateCache.GetManagedLiveObjs(app, targetObjs)
	if err != nil {
		liveObjByKey = make(map[kubeutil.ResourceKey]*unstructured.Unstructured)
		msg := fmt.Sprintf("Failed to load live state: %s", err.Error())
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: msg, LastTransitionTime: &now})
		failedToLoadObjs = true
	}

	logCtx.Debugf("Retrieved live manifests")

	// filter out all resources which are not permitted in the application project
	for k, v := range liveObjByKey {
		permitted, err := project.IsLiveResourcePermitted(v, app.Spec.Destination.Server, app.Spec.Destination.Name, func(project string) ([]*v1alpha1.Cluster, error) {
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

	trackingMethod := argo.GetTrackingMethod(m.settingsMgr)

	for _, liveObj := range liveObjByKey {
		if liveObj != nil {
			appInstanceName := m.resourceTracking.GetAppName(liveObj, appLabelKey, trackingMethod)
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
				nsSpec := &v1.Namespace{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: kubeutil.NamespaceKind}, ObjectMeta: metav1.ObjectMeta{Name: liveObj.GetName()}}
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
	hasPostDeleteHooks := false
	for _, obj := range targetObjs {
		if isPostDeleteHook(obj) {
			hasPostDeleteHooks = true
		}
	}

	reconciliation := sync.Reconcile(targetObjs, liveObjByKey, app.Spec.Destination.Namespace, infoProvider)
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

	gvkParser, err := m.getGVKParser(app.Spec.Destination.Server)
	if err != nil {
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionUnknownError, Message: err.Error(), LastTransitionTime: &now})
	}
	diffConfigBuilder.WithGVKParser(gvkParser)
	diffConfigBuilder.WithManager(common.ArgoCDSSAManager)

	diffConfigBuilder.WithServerSideDiff(serverSideDiff)

	if serverSideDiff {
		resourceOps, cleanup, err := m.getResourceOperations(app.Spec.Destination.Server)
		if err != nil {
			log.Errorf("CompareAppState error getting resource operations: %s", err)
			conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionUnknownError, Message: err.Error(), LastTransitionTime: &now})
		}
		defer cleanup()
		diffConfigBuilder.WithServerSideDryRunner(diff.NewK8sServerSideDryRunner(resourceOps))
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
		msg := fmt.Sprintf("Failed to compare desired state to live state: %s", err.Error())
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

		isSelfReferencedObj := m.isSelfReferencedObj(liveObj, targetObj, app.GetName(), appLabelKey, trackingMethod)

		resState := v1alpha1.ResourceStatus{
			Namespace:       obj.GetNamespace(),
			Name:            obj.GetName(),
			Kind:            gvk.Kind,
			Version:         gvk.Version,
			Group:           gvk.Group,
			Hook:            isHook(obj),
			RequiresPruning: targetObj == nil && liveObj != nil && isSelfReferencedObj,
		}
		if targetObj != nil {
			resState.SyncWave = int64(syncwaves.Wave(targetObj))
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

		if resState.Hook || ignore.Ignore(obj) || (targetObj != nil && hookutil.Skip(targetObj)) || !isSelfReferencedObj {
			// For resource hooks, skipped resources or objects that may have
			// been created by another controller with annotations copied from
			// the source object, don't store sync status, and do not affect
			// overall sync status
		} else if !isManagedNs && (diffResult.Modified || targetObj == nil || liveObj == nil) {
			// Set resource state to OutOfSync since one of the following is true:
			// * target and live resource are different
			// * target resource not defined and live resource is extra
			// * target resource present but live resource is missing
			resState.Status = v1alpha1.SyncStatusCodeOutOfSync
			// we ignore the status if the obj needs pruning AND we have the annotation
			needsPruning := targetObj == nil && liveObj != nil
			if !(needsPruning && resourceutil.HasAnnotationOption(obj, common.AnnotationCompareOptions, "IgnoreExtraneous")) {
				syncCode = v1alpha1.SyncStatusCodeOutOfSync
			}
		} else {
			resState.Status = v1alpha1.SyncStatusCodeSynced
		}
		// set unknown status to all resource that are not permitted in the app project
		isNamespaced, err := m.liveStateCache.IsNamespaced(app.Spec.Destination.Server, gvk.GroupKind())
		if !project.IsGroupKindPermitted(gvk.GroupKind(), isNamespaced && err == nil) {
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
	var revision string

	if !hasMultipleSources && len(manifestRevisions) > 0 {
		revision = manifestRevisions[0]
	}
	var syncStatus v1alpha1.SyncStatus
	if hasMultipleSources {
		syncStatus = v1alpha1.SyncStatus{
			ComparedTo: v1alpha1.ComparedTo{
				Destination:       app.Spec.Destination,
				Sources:           sources,
				IgnoreDifferences: app.Spec.IgnoreDifferences,
			},
			Status:    syncCode,
			Revisions: manifestRevisions,
		}
	} else {
		syncStatus = v1alpha1.SyncStatus{
			ComparedTo: v1alpha1.ComparedTo{
				Destination:       app.Spec.Destination,
				Source:            app.Spec.GetSource(),
				IgnoreDifferences: app.Spec.IgnoreDifferences,
			},
			Status:   syncCode,
			Revision: revision,
		}
	}

	ts.AddCheckpoint("sync_ms")

	healthStatus, err := setApplicationHealth(managedResources, resourceSummaries, resourceOverrides, app, m.persistResourceHealth)
	if err != nil {
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: fmt.Sprintf("error setting app health: %s", err.Error()), LastTransitionTime: &now})
	}

	// Git has already performed the signature verification via its GPG interface, and the result is available
	// in the manifest info received from the repository server. We now need to form our opinion about the result
	// and stop processing if we do not agree about the outcome.
	for _, manifestInfo := range manifestInfos {
		if gpg.IsGPGEnabled() && verifySignature && manifestInfo != nil {
			conditions = append(conditions, verifyGnuPGSignature(manifestInfo.Revision, project, manifestInfo)...)
		}
	}

	compRes := comparisonResult{
		syncStatus:           &syncStatus,
		healthStatus:         healthStatus,
		resources:            resourceSummaries,
		managedResources:     managedResources,
		reconciliationResult: reconciliation,
		diffConfig:           diffConfig,
		diffResultList:       diffResults,
		hasPostDeleteHooks:   hasPostDeleteHooks,
		revisionUpdated:      revisionUpdated,
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

	currentSpec := app.BuildComparedToStatus()
	specChanged := !reflect.DeepEqual(app.Status.Sync.ComparedTo, currentSpec)
	if specChanged {
		log.WithField("useDiffCache", "false").Debug("specChanged")
		return false
	}

	log.WithField("useDiffCache", "true").Debug("using diff cache")
	return true
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
	settingsMgr *settings.SettingsManager,
	liveStateCache statecache.LiveStateCache,
	projInformer cache.SharedIndexInformer,
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
		repoClientset:         repoClientset,
		namespace:             namespace,
		settingsMgr:           settingsMgr,
		projInformer:          projInformer,
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
func (m *appStateManager) isSelfReferencedObj(live, config *unstructured.Unstructured, appName, appLabelKey string, trackingMethod v1alpha1.TrackingMethod) bool {
	if live == nil {
		return true
	}

	// If tracking method doesn't contain required metadata for this check,
	// we are not able to determine and just assume the object to be managed.
	if trackingMethod == argo.TrackingMethodLabel {
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
	appInstance := m.resourceTracking.GetAppInstance(live, appLabelKey, trackingMethod)
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
