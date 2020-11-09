package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync"
	hookutil "github.com/argoproj/gitops-engine/pkg/sync/hook"
	"github.com/argoproj/gitops-engine/pkg/sync/ignore"
	resourceutil "github.com/argoproj/gitops-engine/pkg/sync/resource"
	kubeutil "github.com/argoproj/gitops-engine/pkg/utils/kube"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/common"
	statecache "github.com/argoproj/argo-cd/controller/cache"
	"github.com/argoproj/argo-cd/controller/metrics"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/gpg"
	argohealth "github.com/argoproj/argo-cd/util/health"
	"github.com/argoproj/argo-cd/util/io"
	"github.com/argoproj/argo-cd/util/settings"
	"github.com/argoproj/argo-cd/util/stats"
)

type resourceInfoProviderStub struct {
}

func (r *resourceInfoProviderStub) IsNamespaced(_ schema.GroupKind) (bool, error) {
	return false, nil
}

type managedResource struct {
	Target    *unstructured.Unstructured
	Live      *unstructured.Unstructured
	Diff      diff.DiffResult
	Group     string
	Version   string
	Kind      string
	Namespace string
	Name      string
	Hook      bool
}

func GetLiveObjsForApplicationHealth(resources []managedResource, statuses []appv1.ResourceStatus) ([]*appv1.ResourceStatus, []*unstructured.Unstructured) {
	liveObjs := make([]*unstructured.Unstructured, 0)
	resStatuses := make([]*appv1.ResourceStatus, 0)
	for i, resource := range resources {
		if resource.Target != nil && hookutil.Skip(resource.Target) {
			continue
		}

		liveObjs = append(liveObjs, resource.Live)
		resStatuses = append(resStatuses, &statuses[i])
	}
	return resStatuses, liveObjs
}

// AppStateManager defines methods which allow to compare application spec and actual application state.
type AppStateManager interface {
	CompareAppState(app *v1alpha1.Application, project *appv1.AppProject, revision string, source v1alpha1.ApplicationSource, noCache bool, localObjects []string) *comparisonResult
	SyncAppState(app *v1alpha1.Application, state *v1alpha1.OperationState)
}

type comparisonResult struct {
	syncStatus           *v1alpha1.SyncStatus
	healthStatus         *v1alpha1.HealthStatus
	resources            []v1alpha1.ResourceStatus
	managedResources     []managedResource
	reconciliationResult sync.ReconciliationResult
	diffNormalizer       diff.Normalizer
	appSourceType        v1alpha1.ApplicationSourceType
	// timings maps phases of comparison to the duration it took to complete (for statistical purposes)
	timings map[string]time.Duration
}

func (res *comparisonResult) GetSyncStatus() *v1alpha1.SyncStatus {
	return res.syncStatus
}

func (res *comparisonResult) GetHealthStatus() *v1alpha1.HealthStatus {
	return res.healthStatus
}

// appStateManager allows to compare applications to git
type appStateManager struct {
	metricsServer  *metrics.MetricsServer
	db             db.ArgoDB
	settingsMgr    *settings.SettingsManager
	appclientset   appclientset.Interface
	projInformer   cache.SharedIndexInformer
	kubectl        kubeutil.Kubectl
	repoClientset  apiclient.Clientset
	liveStateCache statecache.LiveStateCache
	namespace      string
}

func (m *appStateManager) getRepoObjs(app *v1alpha1.Application, source v1alpha1.ApplicationSource, appLabelKey, revision string, noCache, verifySignature bool) ([]*unstructured.Unstructured, *apiclient.ManifestResponse, error) {
	ts := stats.NewTimingStats()
	helmRepos, err := m.db.ListHelmRepositories(context.Background())
	if err != nil {
		return nil, nil, err
	}
	ts.AddCheckpoint("helm_ms")
	repo, err := m.db.GetRepository(context.Background(), source.RepoURL)
	if err != nil {
		return nil, nil, err
	}
	ts.AddCheckpoint("repo_ms")
	conn, repoClient, err := m.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, nil, err
	}
	defer io.Close(conn)

	if revision == "" {
		revision = source.TargetRevision
	}

	plugins, err := m.settingsMgr.GetConfigManagementPlugins()
	if err != nil {
		return nil, nil, err
	}
	ts.AddCheckpoint("plugins_ms")
	tools := make([]*appv1.ConfigManagementPlugin, len(plugins))
	for i := range plugins {
		tools[i] = &plugins[i]
	}

	kustomizeSettings, err := m.settingsMgr.GetKustomizeSettings()
	if err != nil {
		return nil, nil, err
	}
	kustomizeOptions, err := kustomizeSettings.GetOptions(app.Spec.Source)
	if err != nil {
		return nil, nil, err
	}
	ts.AddCheckpoint("build_options_ms")
	serverVersion, apiGroups, err := m.liveStateCache.GetVersionsInfo(app.Spec.Destination.Server)
	if err != nil {
		return nil, nil, err
	}
	ts.AddCheckpoint("version_ms")
	manifestInfo, err := repoClient.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:              repo,
		Repos:             helmRepos,
		Revision:          revision,
		NoCache:           noCache,
		AppLabelKey:       appLabelKey,
		AppLabelValue:     app.Name,
		Namespace:         app.Spec.Destination.Namespace,
		ApplicationSource: &source,
		Plugins:           tools,
		KustomizeOptions:  kustomizeOptions,
		KubeVersion:       serverVersion,
		ApiVersions:       argo.APIGroupsToVersions(apiGroups),
		VerifySignature:   verifySignature,
	})
	if err != nil {
		return nil, nil, err
	}
	targetObjs, err := unmarshalManifests(manifestInfo.Manifests)

	if err != nil {
		return nil, nil, err
	}

	ts.AddCheckpoint("unmarshal_ms")
	logCtx := log.WithField("application", app.Name)
	for k, v := range ts.Timings() {
		logCtx = logCtx.WithField(k, v.Milliseconds())
	}
	logCtx = logCtx.WithField("time_ms", time.Since(ts.StartTime).Milliseconds())
	logCtx.Info("getRepoObjs stats")
	return targetObjs, manifestInfo, nil
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
			conditions = append(conditions, appv1.ApplicationCondition{
				Type:               appv1.ApplicationConditionRepeatedResourceWarning,
				Message:            fmt.Sprintf("Resource %s appeared %d times among application resources.", key.String(), len(targets)),
				LastTransitionTime: &now,
			})
		}
		result = append(result, targets[len(targets)-1])
	}

	return result, conditions, nil
}

func (m *appStateManager) getComparisonSettings(app *appv1.Application) (string, map[string]v1alpha1.ResourceOverride, diff.Normalizer, *settings.ResourcesFilter, error) {
	resourceOverrides, err := m.settingsMgr.GetResourceOverrides()
	if err != nil {
		return "", nil, nil, nil, err
	}
	appLabelKey, err := m.settingsMgr.GetAppInstanceLabelKey()
	if err != nil {
		return "", nil, nil, nil, err
	}
	diffNormalizer, err := argo.NewDiffNormalizer(app.Spec.IgnoreDifferences, resourceOverrides)
	if err != nil {
		return "", nil, nil, nil, err
	}
	resFilter, err := m.settingsMgr.GetResourcesFilter()
	if err != nil {
		return "", nil, nil, nil, err
	}
	return appLabelKey, resourceOverrides, diffNormalizer, resFilter, nil
}

// verifyGnuPGSignature verifies the result of a GnuPG operation for a given git
// revision.
func verifyGnuPGSignature(revision string, project *appv1.AppProject, manifestInfo *apiclient.ManifestResponse) []appv1.ApplicationCondition {
	now := metav1.Now()
	conditions := make([]appv1.ApplicationCondition, 0)
	// We need to have some data in the verification result to parse, otherwise there was no signature
	if manifestInfo.VerifyResult != "" {
		verifyResult, err := gpg.ParseGitCommitVerification(manifestInfo.VerifyResult)
		if err != nil {
			conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error(), LastTransitionTime: &now})
			log.Errorf("Error while verifying git commit for revision %s: %s", revision, err.Error())
		} else {
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
func (m *appStateManager) CompareAppState(app *v1alpha1.Application, project *appv1.AppProject, revision string, source v1alpha1.ApplicationSource, noCache bool, localManifests []string) *comparisonResult {
	ts := stats.NewTimingStats()
	appLabelKey, resourceOverrides, diffNormalizer, resFilter, err := m.getComparisonSettings(app)
	ts.AddCheckpoint("settings_ms")

	// return unknown comparison result if basic comparison settings cannot be loaded
	if err != nil {
		return &comparisonResult{
			syncStatus: &v1alpha1.SyncStatus{
				ComparedTo: appv1.ComparedTo{Source: source, Destination: app.Spec.Destination},
				Status:     appv1.SyncStatusCodeUnknown,
			},
			healthStatus: &appv1.HealthStatus{Status: health.HealthStatusUnknown},
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

	logCtx := log.WithField("application", app.Name)
	logCtx.Infof("Comparing app state (cluster: %s, namespace: %s)", app.Spec.Destination.Server, app.Spec.Destination.Namespace)

	var targetObjs []*unstructured.Unstructured
	var manifestInfo *apiclient.ManifestResponse
	now := metav1.Now()

	if len(localManifests) == 0 {
		targetObjs, manifestInfo, err = m.getRepoObjs(app, source, appLabelKey, revision, noCache, verifySignature)
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
		manifestInfo = nil
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
	logCtx.Debugf("Retrieved lived manifests")

	// filter out all resources which are not permitted in the application project
	for k, v := range liveObjByKey {
		if !project.IsLiveResourcePermitted(v, app.Spec.Destination.Server) {
			delete(liveObjByKey, k)
		}
	}

	for _, liveObj := range liveObjByKey {
		if liveObj != nil {
			appInstanceName := kubeutil.GetAppInstanceLabel(liveObj, appLabelKey)
			if appInstanceName != "" && appInstanceName != app.Name {
				conditions = append(conditions, v1alpha1.ApplicationCondition{
					Type:               v1alpha1.ApplicationConditionSharedResourceWarning,
					Message:            fmt.Sprintf("%s/%s is part of applications %s and %s", liveObj.GetKind(), liveObj.GetName(), app.Name, appInstanceName),
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

	logCtx.Debugf("built managed objects list")
	// Do the actual comparison
	diffResults, err := diff.DiffArray(
		reconciliation.Target, reconciliation.Live,
		diff.WithNormalizer(diffNormalizer),
		diff.IgnoreAggregatedRoles(compareOptions.IgnoreAggregatedRoles))
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

		resState := v1alpha1.ResourceStatus{
			Namespace:       obj.GetNamespace(),
			Name:            obj.GetName(),
			Kind:            gvk.Kind,
			Version:         gvk.Version,
			Group:           gvk.Group,
			Hook:            hookutil.IsHook(obj),
			RequiresPruning: targetObj == nil && liveObj != nil,
		}

		var diffResult diff.DiffResult
		if i < len(diffResults.Diffs) {
			diffResult = diffResults.Diffs[i]
		} else {
			diffResult = diff.DiffResult{Modified: false, NormalizedLive: []byte("{}"), PredictedLive: []byte("{}")}
		}
		if resState.Hook || ignore.Ignore(obj) || (targetObj != nil && hookutil.Skip(targetObj)) {
			// For resource hooks or skipped resources, don't store sync status, and do not affect overall sync status
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
			conditions = append(conditions, appv1.ApplicationCondition{Type: v1alpha1.ApplicationConditionInvalidSpecError, Message: fmt.Sprintf("Namespace for %s %s is missing.", obj.GetName(), gvk.String()), LastTransitionTime: &now})
		}

		// we can't say anything about the status if we were unable to get the target objects
		if failedToLoadObjs {
			resState.Status = v1alpha1.SyncStatusCodeUnknown
		}
		managedResources[i] = managedResource{
			Name:      resState.Name,
			Namespace: resState.Namespace,
			Group:     resState.Group,
			Kind:      resState.Kind,
			Version:   resState.Version,
			Live:      liveObj,
			Target:    targetObj,
			Diff:      diffResult,
			Hook:      resState.Hook,
		}
		resourceSummaries[i] = resState
	}

	if failedToLoadObjs {
		syncCode = v1alpha1.SyncStatusCodeUnknown
	}
	syncStatus := v1alpha1.SyncStatus{
		ComparedTo: appv1.ComparedTo{
			Source:      source,
			Destination: app.Spec.Destination,
		},
		Status: syncCode,
	}
	if manifestInfo != nil {
		syncStatus.Revision = manifestInfo.Revision
	}
	ts.AddCheckpoint("sync_ms")

	resSumForAppHealth, liveObjsForAppHealth := GetLiveObjsForApplicationHealth(managedResources, resourceSummaries)
	healthStatus, err := argohealth.SetApplicationHealth(resSumForAppHealth, liveObjsForAppHealth, resourceOverrides, func(obj *unstructured.Unstructured) bool {
		return !isSelfReferencedApp(app, kubeutil.GetObjectRef(obj))
	})

	if err != nil {
		conditions = append(conditions, appv1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error(), LastTransitionTime: &now})
	}

	// Git has already performed the signature verification via its GPG interface, and the result is available
	// in the manifest info received from the repository server. We now need to form our opinion about the result
	// and stop processing if we do not agree about the outcome.
	if gpg.IsGPGEnabled() && verifySignature && manifestInfo != nil {
		conditions = append(conditions, verifyGnuPGSignature(revision, project, manifestInfo)...)
	}

	compRes := comparisonResult{
		syncStatus:           &syncStatus,
		healthStatus:         healthStatus,
		resources:            resourceSummaries,
		managedResources:     managedResources,
		reconciliationResult: reconciliation,
		diffNormalizer:       diffNormalizer,
	}
	if manifestInfo != nil {
		compRes.appSourceType = v1alpha1.ApplicationSourceType(manifestInfo.SourceType)
	}
	app.Status.SetConditions(conditions, map[appv1.ApplicationConditionType]bool{
		appv1.ApplicationConditionComparisonError:         true,
		appv1.ApplicationConditionSharedResourceWarning:   true,
		appv1.ApplicationConditionRepeatedResourceWarning: true,
		appv1.ApplicationConditionExcludedResourceWarning: true,
	})
	ts.AddCheckpoint("health_ms")
	compRes.timings = ts.Timings()
	return &compRes
}

func (m *appStateManager) persistRevisionHistory(app *v1alpha1.Application, revision string, source v1alpha1.ApplicationSource, startedAt metav1.Time) error {
	var nextID int64
	if len(app.Status.History) > 0 {
		nextID = app.Status.History.LastRevisionHistory().ID + 1
	}
	app.Status.History = append(app.Status.History, v1alpha1.RevisionHistory{
		Revision:        revision,
		DeployedAt:      metav1.NewTime(time.Now().UTC()),
		DeployStartedAt: &startedAt,
		ID:              nextID,
		Source:          source,
	})

	app.Status.History = app.Status.History.Trunc(app.Spec.GetRevisionHistoryLimit())

	patch, err := json.Marshal(map[string]map[string][]v1alpha1.RevisionHistory{
		"status": {
			"history": app.Status.History,
		},
	})
	if err != nil {
		return err
	}
	_, err = m.appclientset.ArgoprojV1alpha1().Applications(m.namespace).Patch(context.Background(), app.Name, types.MergePatchType, patch, metav1.PatchOptions{})
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
) AppStateManager {
	return &appStateManager{
		liveStateCache: liveStateCache,
		db:             db,
		appclientset:   appclientset,
		kubectl:        kubectl,
		repoClientset:  repoClientset,
		namespace:      namespace,
		settingsMgr:    settingsMgr,
		projInformer:   projInformer,
		metricsServer:  metricsServer,
	}
}
