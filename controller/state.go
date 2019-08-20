package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/common"
	statecache "github.com/argoproj/argo-cd/controller/cache"
	"github.com/argoproj/argo-cd/controller/metrics"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/diff"
	"github.com/argoproj/argo-cd/util/health"
	hookutil "github.com/argoproj/argo-cd/util/hook"
	kubeutil "github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/resource"
	"github.com/argoproj/argo-cd/util/settings"
)

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

func GetLiveObjs(res []managedResource) []*unstructured.Unstructured {
	objs := make([]*unstructured.Unstructured, len(res))
	for i := range res {
		objs[i] = res[i].Live
	}
	return objs
}

type ResourceInfoProvider interface {
	IsNamespaced(server string, obj *unstructured.Unstructured) (bool, error)
}

// AppStateManager defines methods which allow to compare application spec and actual application state.
type AppStateManager interface {
	CompareAppState(app *v1alpha1.Application, revision string, source v1alpha1.ApplicationSource, noCache bool, localObjects []string) *comparisonResult
	SyncAppState(app *v1alpha1.Application, state *v1alpha1.OperationState)
}

type comparisonResult struct {
	reconciledAt     metav1.Time
	syncStatus       *v1alpha1.SyncStatus
	healthStatus     *v1alpha1.HealthStatus
	resources        []v1alpha1.ResourceStatus
	managedResources []managedResource
	conditions       []v1alpha1.ApplicationCondition
	hooks            []*unstructured.Unstructured
	diffNormalizer   diff.Normalizer
	appSourceType    v1alpha1.ApplicationSourceType
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

func (m *appStateManager) getRepoObjs(app *v1alpha1.Application, source v1alpha1.ApplicationSource, appLabelKey, revision string, noCache bool) ([]*unstructured.Unstructured, []*unstructured.Unstructured, *apiclient.ManifestResponse, error) {
	helmRepos, err := m.db.ListHelmRepos(context.Background())
	if err != nil {
		return nil, nil, nil, err
	}
	repo, err := m.db.GetRepository(context.Background(), source.RepoURL)
	if err != nil {
		return nil, nil, nil, err
	}
	conn, repoClient, err := m.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, nil, nil, err
	}
	defer util.Close(conn)

	if revision == "" {
		revision = source.TargetRevision
	}

	plugins, err := m.settingsMgr.GetConfigManagementPlugins()
	if err != nil {
		return nil, nil, nil, err
	}

	tools := make([]*appv1.ConfigManagementPlugin, len(plugins))
	for i := range plugins {
		tools[i] = &plugins[i]
	}

	buildOptions, err := m.settingsMgr.GetKustomizeBuildOptions()
	if err != nil {
		return nil, nil, nil, err
	}
	manifestInfo, err := repoClient.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:              repo,
		HelmRepos:         helmRepos,
		Revision:          revision,
		NoCache:           noCache,
		AppLabelKey:       appLabelKey,
		AppLabelValue:     app.Name,
		Namespace:         app.Spec.Destination.Namespace,
		ApplicationSource: &source,
		Plugins:           tools,
		KustomizeOptions: &appv1.KustomizeOptions{
			BuildOptions: buildOptions,
		},
	})
	if err != nil {
		return nil, nil, nil, err
	}
	targetObjs, hooks, err := unmarshalManifests(manifestInfo.Manifests)
	if err != nil {
		return nil, nil, nil, err
	}
	return targetObjs, hooks, manifestInfo, nil
}

func unmarshalManifests(manifests []string) ([]*unstructured.Unstructured, []*unstructured.Unstructured, error) {
	targetObjs := make([]*unstructured.Unstructured, 0)
	hooks := make([]*unstructured.Unstructured, 0)
	for _, manifest := range manifests {
		obj, err := v1alpha1.UnmarshalToUnstructured(manifest)
		if err != nil {
			return nil, nil, err
		}
		if resource.Ignore(obj) {
			continue
		}
		if hookutil.IsHook(obj) {
			hooks = append(hooks, obj)
		} else {
			targetObjs = append(targetObjs, obj)
		}
	}
	return targetObjs, hooks, nil
}

func DeduplicateTargetObjects(
	server string,
	namespace string,
	objs []*unstructured.Unstructured,
	infoProvider ResourceInfoProvider,
) ([]*unstructured.Unstructured, []v1alpha1.ApplicationCondition, error) {

	targetByKey := make(map[kubeutil.ResourceKey][]*unstructured.Unstructured)
	for i := range objs {
		obj := objs[i]
		isNamespaced, err := infoProvider.IsNamespaced(server, obj)
		if err != nil {
			return objs, nil, err
		}
		if !isNamespaced {
			obj.SetNamespace("")
		} else if obj.GetNamespace() == "" {
			obj.SetNamespace(namespace)
		}
		key := kubeutil.GetResourceKey(obj)
		targetByKey[key] = append(targetByKey[key], obj)
	}
	conditions := make([]v1alpha1.ApplicationCondition, 0)
	result := make([]*unstructured.Unstructured, 0)
	for key, targets := range targetByKey {
		if len(targets) > 1 {
			conditions = append(conditions, appv1.ApplicationCondition{
				Type:    appv1.ApplicationConditionRepeatedResourceWarning,
				Message: fmt.Sprintf("Resource %s appeared %d times among application resources.", key.String(), len(targets)),
			})
		}
		result = append(result, targets[len(targets)-1])
	}

	return result, conditions, nil
}

// dedupLiveResources handles removes live resource duplicates with the same UID. Duplicates are created in a separate resource groups.
// E.g. apps/Deployment produces duplicate in extensions/Deployment, authorization.openshift.io/ClusterRole produces duplicate in rbac.authorization.k8s.io/ClusterRole etc.
// The method removes such duplicates unless it was defined in git ( exists in target resources list ). At least one duplicate stays.
// If non of duplicates are in git at random one stays
func dedupLiveResources(targetObjs []*unstructured.Unstructured, liveObjsByKey map[kubeutil.ResourceKey]*unstructured.Unstructured) {
	targetObjByKey := make(map[kubeutil.ResourceKey]*unstructured.Unstructured)
	for i := range targetObjs {
		targetObjByKey[kubeutil.GetResourceKey(targetObjs[i])] = targetObjs[i]
	}
	liveObjsById := make(map[types.UID][]*unstructured.Unstructured)
	for k := range liveObjsByKey {
		obj := liveObjsByKey[k]
		if obj != nil {
			liveObjsById[obj.GetUID()] = append(liveObjsById[obj.GetUID()], obj)
		}
	}
	for id := range liveObjsById {
		objs := liveObjsById[id]

		if len(objs) > 1 {
			duplicatesLeft := len(objs)
			for i := range objs {
				obj := objs[i]
				resourceKey := kubeutil.GetResourceKey(obj)
				if _, ok := targetObjByKey[resourceKey]; !ok {
					delete(liveObjsByKey, resourceKey)
					duplicatesLeft--
					if duplicatesLeft == 1 {
						break
					}
				}
			}
		}
	}
}

func (m *appStateManager) getComparisonSettings(app *appv1.Application) (string, map[string]v1alpha1.ResourceOverride, diff.Normalizer, error) {
	resourceOverrides, err := m.settingsMgr.GetResourceOverrides()
	if err != nil {
		return "", nil, nil, err
	}
	appLabelKey, err := m.settingsMgr.GetAppInstanceLabelKey()
	if err != nil {
		return "", nil, nil, err
	}
	diffNormalizer, err := argo.NewDiffNormalizer(app.Spec.IgnoreDifferences, resourceOverrides)
	if err != nil {
		return "", nil, nil, err
	}
	return appLabelKey, resourceOverrides, diffNormalizer, nil
}

// CompareAppState compares application git state to the live app state, using the specified
// revision and supplied source. If revision or overrides are empty, then compares against
// revision and overrides in the app spec.
func (m *appStateManager) CompareAppState(app *v1alpha1.Application, revision string, source v1alpha1.ApplicationSource, noCache bool, localManifests []string) *comparisonResult {
	reconciledAt := metav1.Now()
	appLabelKey, resourceOverrides, diffNormalizer, err := m.getComparisonSettings(app)

	// return unknown comparison result if basic comparison settings cannot be loaded
	if err != nil {
		return &comparisonResult{
			reconciledAt: reconciledAt,
			syncStatus: &v1alpha1.SyncStatus{
				ComparedTo: appv1.ComparedTo{Source: source, Destination: app.Spec.Destination},
				Status:     appv1.SyncStatusCodeUnknown,
			},
			healthStatus: &appv1.HealthStatus{Status: appv1.HealthStatusUnknown},
		}
	}

	// do best effort loading live and target state to present as much information about app state as possible
	failedToLoadObjs := false
	conditions := make([]v1alpha1.ApplicationCondition, 0)

	logCtx := log.WithField("application", app.Name)
	logCtx.Infof("Comparing app state (cluster: %s, namespace: %s)", app.Spec.Destination.Server, app.Spec.Destination.Namespace)

	var targetObjs []*unstructured.Unstructured
	var hooks []*unstructured.Unstructured
	var manifestInfo *apiclient.ManifestResponse

	if len(localManifests) == 0 {
		targetObjs, hooks, manifestInfo, err = m.getRepoObjs(app, source, appLabelKey, revision, noCache)
		if err != nil {
			targetObjs = make([]*unstructured.Unstructured, 0)
			conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error()})
			failedToLoadObjs = true
		}
	} else {
		targetObjs, hooks, err = unmarshalManifests(localManifests)
		if err != nil {
			targetObjs = make([]*unstructured.Unstructured, 0)
			conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error()})
			failedToLoadObjs = true
		}
		manifestInfo = nil
	}

	targetObjs, dedupConditions, err := DeduplicateTargetObjects(app.Spec.Destination.Server, app.Spec.Destination.Namespace, targetObjs, m.liveStateCache)
	if err != nil {
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error()})
	}
	conditions = append(conditions, dedupConditions...)

	resFilter, err := m.settingsMgr.GetResourcesFilter()
	if err != nil {
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error()})
	} else {
		for i := len(targetObjs) - 1; i >= 0; i-- {
			targetObj := targetObjs[i]
			gvk := targetObj.GroupVersionKind()
			if resFilter.IsExcludedResource(gvk.Group, gvk.Kind, app.Spec.Destination.Server) {
				targetObjs = append(targetObjs[:i], targetObjs[i+1:]...)
				conditions = append(conditions, v1alpha1.ApplicationCondition{
					Type:    v1alpha1.ApplicationConditionExcludedResourceWarning,
					Message: fmt.Sprintf("Resource %s/%s %s is excluded in the settings", gvk.Group, gvk.Kind, targetObj.GetName()),
				})
			}
		}
	}

	logCtx.Debugf("Generated config manifests")
	liveObjByKey, err := m.liveStateCache.GetManagedLiveObjs(app, targetObjs)
	dedupLiveResources(targetObjs, liveObjByKey)
	if err != nil {
		liveObjByKey = make(map[kubeutil.ResourceKey]*unstructured.Unstructured)
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error()})
		failedToLoadObjs = true
	}
	logCtx.Debugf("Retrieved lived manifests")
	for _, liveObj := range liveObjByKey {
		if liveObj != nil {
			appInstanceName := kubeutil.GetAppInstanceLabel(liveObj, appLabelKey)
			if appInstanceName != "" && appInstanceName != app.Name {
				conditions = append(conditions, v1alpha1.ApplicationCondition{
					Type:    v1alpha1.ApplicationConditionSharedResourceWarning,
					Message: fmt.Sprintf("%s/%s is part of a different application: %s", liveObj.GetKind(), liveObj.GetName(), appInstanceName),
				})
			}
		}
	}

	managedLiveObj := make([]*unstructured.Unstructured, len(targetObjs))
	for i, obj := range targetObjs {
		gvk := obj.GroupVersionKind()
		ns := util.FirstNonEmpty(obj.GetNamespace(), app.Spec.Destination.Namespace)
		if namespaced, err := m.liveStateCache.IsNamespaced(app.Spec.Destination.Server, obj); err == nil && !namespaced {
			ns = ""
		}
		key := kubeutil.NewResourceKey(gvk.Group, gvk.Kind, ns, obj.GetName())
		if liveObj, ok := liveObjByKey[key]; ok {
			managedLiveObj[i] = liveObj
			delete(liveObjByKey, key)
		} else {
			managedLiveObj[i] = nil
		}
	}
	logCtx.Debugf("built managed objects list")
	// Everything remaining in liveObjByKey are "extra" resources that aren't tracked in git.
	// The following adds all the extras to the managedLiveObj list and backfills the targetObj
	// list with nils, so that the lists are of equal lengths for comparison purposes.
	for _, obj := range liveObjByKey {
		targetObjs = append(targetObjs, nil)
		managedLiveObj = append(managedLiveObj, obj)
	}

	// Do the actual comparison
	diffResults, err := diff.DiffArray(targetObjs, managedLiveObj, diffNormalizer)
	if err != nil {
		diffResults = &diff.DiffResultList{}
		failedToLoadObjs = true
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error()})
	}

	syncCode := v1alpha1.SyncStatusCodeSynced
	managedResources := make([]managedResource, len(targetObjs))
	resourceSummaries := make([]v1alpha1.ResourceStatus, len(targetObjs))
	for i, targetObj := range targetObjs {
		liveObj := managedLiveObj[i]
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

		diffResult := diffResults.Diffs[i]
		if resState.Hook || resource.Ignore(obj) {
			// For resource hooks, don't store sync status, and do not affect overall sync status
		} else if diffResult.Modified || targetObj == nil || liveObj == nil {
			// Set resource state to OutOfSync since one of the following is true:
			// * target and live resource are different
			// * target resource not defined and live resource is extra
			// * target resource present but live resource is missing
			resState.Status = v1alpha1.SyncStatusCodeOutOfSync
			// we ignore the status if the obj needs pruning AND we have the annotation
			needsPruning := targetObj == nil && liveObj != nil
			if !(needsPruning && resource.HasAnnotationOption(obj, common.AnnotationCompareOptions, "IgnoreExtraneous")) {
				syncCode = v1alpha1.SyncStatusCodeOutOfSync
			}
		} else {
			resState.Status = v1alpha1.SyncStatusCodeSynced
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

	healthStatus, err := health.SetApplicationHealth(resourceSummaries, GetLiveObjs(managedResources), resourceOverrides, func(obj *unstructured.Unstructured) bool {
		return !isSelfReferencedApp(app, kubeutil.GetObjectRef(obj))
	})

	if err != nil {
		conditions = append(conditions, appv1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error()})
	}

	compRes := comparisonResult{
		reconciledAt:     reconciledAt,
		syncStatus:       &syncStatus,
		healthStatus:     healthStatus,
		resources:        resourceSummaries,
		managedResources: managedResources,
		conditions:       conditions,
		hooks:            hooks,
		diffNormalizer:   diffNormalizer,
	}
	if manifestInfo != nil {
		compRes.appSourceType = v1alpha1.ApplicationSourceType(manifestInfo.SourceType)
	}
	return &compRes
}

func (m *appStateManager) persistRevisionHistory(app *v1alpha1.Application, revision string, source v1alpha1.ApplicationSource) error {
	var nextID int64
	if len(app.Status.History) > 0 {
		nextID = app.Status.History[len(app.Status.History)-1].ID + 1
	}
	history := append(app.Status.History, v1alpha1.RevisionHistory{
		Revision:   revision,
		DeployedAt: metav1.NewTime(time.Now().UTC()),
		ID:         nextID,
		Source:     source,
	})

	if len(history) > common.RevisionHistoryLimit {
		history = history[1 : common.RevisionHistoryLimit+1]
	}

	patch, err := json.Marshal(map[string]map[string][]v1alpha1.RevisionHistory{
		"status": {
			"history": history,
		},
	})
	if err != nil {
		return err
	}
	_, err = m.appclientset.ArgoprojV1alpha1().Applications(m.namespace).Patch(app.Name, types.MergePatchType, patch)
	return err
}

// NewAppStateManager creates new instance of Ksonnet app comparator
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
