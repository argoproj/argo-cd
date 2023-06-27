package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

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
	"github.com/argoproj/argo-cd/v2/util/argo"
	argodiff "github.com/argoproj/argo-cd/v2/util/argo/diff"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/gpg"
	"github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/argoproj/argo-cd/v2/util/stats"
)

type resourceInfoProviderStub struct {
}

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
	CompareAppState(app *v1alpha1.Application, project *v1alpha1.AppProject, revisions []string, sources []v1alpha1.ApplicationSource, noCache bool, noRevisionCache bool, localObjects []string, hasMultipleSources bool) *comparisonResult
	SyncAppState(app *v1alpha1.Application, state *v1alpha1.OperationState)
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
	timings        map[string]time.Duration
	diffResultList *diff.DiffResultList
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
}

func (m *appStateManager) getRepoObjs(app *v1alpha1.Application, sources []v1alpha1.ApplicationSource, appLabelKey string, revisions []string, noCache, noRevisionCache, verifySignature bool, proj *v1alpha1.AppProject) ([]*unstructured.Unstructured, []*apiclient.ManifestResponse, error) {

	ts := stats.NewTimingStats()
	helmRepos, err := m.db.ListHelmRepositories(context.Background())
	if err != nil {
		return nil, nil, err
	}
	permittedHelmRepos, err := argo.GetPermittedRepos(proj, helmRepos)
	if err != nil {
		return nil, nil, err
	}

	ts.AddCheckpoint("repo_ms")
	helmRepositoryCredentials, err := m.db.GetAllHelmRepositoryCredentials(context.Background())
	if err != nil {
		return nil, nil, err
	}
	permittedHelmCredentials, err := argo.GetPermittedReposCredentials(proj, helmRepositoryCredentials)
	if err != nil {
		return nil, nil, err
	}

	enabledSourceTypes, err := m.settingsMgr.GetEnabledSourceTypes()
	if err != nil {
		return nil, nil, err
	}
	ts.AddCheckpoint("plugins_ms")

	kustomizeSettings, err := m.settingsMgr.GetKustomizeSettings()
	if err != nil {
		return nil, nil, err
	}

	helmOptions, err := m.settingsMgr.GetHelmSettings()
	if err != nil {
		return nil, nil, err
	}

	ts.AddCheckpoint("build_options_ms")
	serverVersion, apiResources, err := m.liveStateCache.GetVersionsInfo(app.Spec.Destination.Server)
	if err != nil {
		return nil, nil, err
	}
	conn, repoClient, err := m.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, nil, err
	}
	defer io.Close(conn)

	manifestInfos := make([]*apiclient.ManifestResponse, 0)
	targetObjs := make([]*unstructured.Unstructured, 0)

	// Store the map of all sources having ref field into a map for applications with sources field
	refSources, err := argo.GetRefSources(context.Background(), app.Spec, m.db)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get ref sources: %v", err)
	}

	for i, source := range sources {
		if len(revisions) < len(sources) || revisions[i] == "" {
			revisions[i] = source.TargetRevision
		}
		ts.AddCheckpoint("helm_ms")
		repo, err := m.db.GetRepository(context.Background(), source.RepoURL)
		if err != nil {
			return nil, nil, err
		}
		kustomizeOptions, err := kustomizeSettings.GetOptions(source)
		if err != nil {
			return nil, nil, err
		}

		ts.AddCheckpoint("version_ms")
		log.Debugf("Generating Manifest for source %s revision %s", source, revisions[i])
		manifestInfo, err := repoClient.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
			Repo:               repo,
			Repos:              permittedHelmRepos,
			Revision:           revisions[i],
			NoCache:            noCache,
			NoRevisionCache:    noRevisionCache,
			AppLabelKey:        appLabelKey,
			AppName:            app.InstanceName(m.namespace),
			Namespace:          app.Spec.Destination.Namespace,
			ApplicationSource:  &source,
			KustomizeOptions:   kustomizeOptions,
			KubeVersion:        serverVersion,
			ApiVersions:        argo.APIResourcesToStrings(apiResources, true),
			VerifySignature:    verifySignature,
			HelmRepoCreds:      permittedHelmCredentials,
			TrackingMethod:     string(argo.GetTrackingMethod(m.settingsMgr)),
			EnabledSourceTypes: enabledSourceTypes,
			HelmOptions:        helmOptions,
			HasMultipleSources: app.Spec.HasMultipleSources(),
			RefSources:         refSources,
			ProjectName:        proj.Name,
			ProjectSourceRepos: proj.Spec.SourceRepos,
		})
		if err != nil {
			return nil, nil, err
		}

		targetObj, err := unmarshalManifests(manifestInfo.Manifests)

		if err != nil {
			return nil, nil, err
		}
		targetObjs = append(targetObjs, targetObj...)

		manifestInfos = append(manifestInfos, manifestInfo)
	}

	ts.AddCheckpoint("unmarshal_ms")
	logCtx := log.WithField("application", app.QualifiedName())
	for k, v := range ts.Timings() {
		logCtx = logCtx.WithField(k, v.Milliseconds())
	}
	logCtx = logCtx.WithField("time_ms", time.Since(ts.StartTime).Milliseconds())
	logCtx.Info("getRepoObjs stats")
	return targetObjs, manifestInfos, nil
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

// CompareAppState compares application git state to the live app state, using the specified
// revision and supplied source. If revision or overrides are empty, then compares against
// revision and overrides in the app spec.
func (m *appStateManager) CompareAppState(app *v1alpha1.Application, project *v1alpha1.AppProject, revisions []string, sources []v1alpha1.ApplicationSource, noCache bool, noRevisionCache bool, localManifests []string, hasMultipleSources bool) *comparisonResult {
	ts := stats.NewTimingStats()
	appLabelKey, resourceOverrides, resFilter, err := m.getComparisonSettings()

	ts.AddCheckpoint("settings_ms")

	// return unknown comparison result if basic comparison settings cannot be loaded
	if err != nil {
		if hasMultipleSources {
			return &comparisonResult{
				syncStatus: &v1alpha1.SyncStatus{
					ComparedTo: v1alpha1.ComparedTo{Destination: app.Spec.Destination, Sources: sources},
					Status:     v1alpha1.SyncStatusCodeUnknown,
					Revisions:  revisions,
				},
				healthStatus: &v1alpha1.HealthStatus{Status: health.HealthStatusUnknown},
			}
		} else {
			return &comparisonResult{
				syncStatus: &v1alpha1.SyncStatus{
					ComparedTo: v1alpha1.ComparedTo{Source: sources[0], Destination: app.Spec.Destination},
					Status:     v1alpha1.SyncStatusCodeUnknown,
					Revision:   revisions[0],
				},
				healthStatus: &v1alpha1.HealthStatus{Status: health.HealthStatusUnknown},
			}
		}
	}

	// When signature keys are defined in the project spec, we need to verify the signature on the Git revision
	verifySignature := false
	if project.Spec.SignatureKeys != nil && len(project.Spec.SignatureKeys) > 0 && gpg.IsGPGEnabled() {
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

	if len(localManifests) == 0 {
		// If the length of revisions is not same as the length of sources,
		// we take the revisions from the sources directly for all the sources.
		if len(revisions) != len(sources) {
			revisions = make([]string, 0)
			for _, source := range sources {
				revisions = append(revisions, source.TargetRevision)
			}
		}

		targetObjs, manifestInfos, err = m.getRepoObjs(app, sources, appLabelKey, revisions, noCache, noRevisionCache, verifySignature, project)
		if err != nil {
			targetObjs = make([]*unstructured.Unstructured, 0)
			conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error(), LastTransitionTime: &now})
			failedToLoadObjs = true
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
				conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error(), LastTransitionTime: &now})
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
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error(), LastTransitionTime: &now})
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
	}
	ts.AddCheckpoint("dedup_ms")

	liveObjByKey, err := m.liveStateCache.GetManagedLiveObjs(app, targetObjs)
	if err != nil {
		liveObjByKey = make(map[kubeutil.ResourceKey]*unstructured.Unstructured)
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error(), LastTransitionTime: &now})
		failedToLoadObjs = true
	}

	logCtx.Debugf("Retrieved live manifests")

	// filter out all resources which are not permitted in the application project
	for k, v := range liveObjByKey {
		permitted, err := project.IsLiveResourcePermitted(v, app.Spec.Destination.Server, app.Spec.Destination.Name, func(project string) ([]*v1alpha1.Cluster, error) {
			return m.db.GetProjectClusters(context.TODO(), project)
		})

		if err != nil {
			conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error(), LastTransitionTime: &now})
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

	// restore comparison using cached diff result if previous comparison was performed for the same revision
	revisionChanged := len(manifestInfos) != len(sources) || !reflect.DeepEqual(app.Status.Sync.Revisions, manifestRevisions)
	specChanged := !reflect.DeepEqual(app.Status.Sync.ComparedTo, v1alpha1.ComparedTo{Source: app.Spec.GetSource(), Destination: app.Spec.Destination, Sources: sources})

	_, refreshRequested := app.IsRefreshRequested()
	noCache = noCache || refreshRequested || app.Status.Expired(m.statusRefreshTimeout) || specChanged || revisionChanged

	diffConfigBuilder := argodiff.NewDiffConfigBuilder().
		WithDiffSettings(app.Spec.IgnoreDifferences, resourceOverrides, compareOptions.IgnoreAggregatedRoles).
		WithTracking(appLabelKey, string(trackingMethod))

	if noCache {
		diffConfigBuilder.WithNoCache()
	} else {
		diffConfigBuilder.WithCache(m.cache, app.GetName())
	}

	gvkParser, err := m.getGVKParser(app.Spec.Destination.Server)
	if err != nil {
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionUnknownError, Message: err.Error(), LastTransitionTime: &now})
	}
	diffConfigBuilder.WithGVKParser(gvkParser)
	diffConfigBuilder.WithManager(common.ArgoCDSSAManager)

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
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error(), LastTransitionTime: &now})
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
			Hook:            hookutil.IsHook(obj),
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
		if resState.Hook || ignore.Ignore(obj) || (targetObj != nil && hookutil.Skip(targetObj)) || !isSelfReferencedObj {
			// For resource hooks, skipped resources or objects that may have
			// been created by another controller with annotations copied from
			// the source object, don't store sync status, and do not affect
			// overall sync status
		} else if diffResult.Modified || targetObj == nil || liveObj == nil {
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
				Destination: app.Spec.Destination,
				Sources:     sources,
			},
			Status:    syncCode,
			Revisions: manifestRevisions,
		}
	} else {
		syncStatus = v1alpha1.SyncStatus{
			ComparedTo: v1alpha1.ComparedTo{
				Destination: app.Spec.Destination,
				Source:      app.Spec.GetSource(),
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
	return &compRes
}

func (m *appStateManager) persistRevisionHistory(app *v1alpha1.Application, revision string, source v1alpha1.ApplicationSource, revisions []string, sources []v1alpha1.ApplicationSource, hasMultipleSources bool, startedAt metav1.Time) error {
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
		})
	} else {
		app.Status.History = append(app.Status.History, v1alpha1.RevisionHistory{
			Revision:        revision,
			DeployedAt:      metav1.NewTime(time.Now().UTC()),
			DeployStartedAt: &startedAt,
			ID:              nextID,
			Source:          source,
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
