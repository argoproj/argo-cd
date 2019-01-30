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
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/diff"
	"github.com/argoproj/argo-cd/util/health"
	hookutil "github.com/argoproj/argo-cd/util/hook"
	kubeutil "github.com/argoproj/argo-cd/util/kube"
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

// AppStateManager defines methods which allow to compare application spec and actual application state.
type AppStateManager interface {
	CompareAppState(app *v1alpha1.Application, revision string, overrides []v1alpha1.ComponentParameter, noCache bool) (*comparisonResult, error)
	SyncAppState(app *v1alpha1.Application, state *v1alpha1.OperationState)
}

type comparisonResult struct {
	observedAt       metav1.Time
	syncStatus       *v1alpha1.SyncStatus
	healthStatus     *v1alpha1.HealthStatus
	resources        []v1alpha1.ResourceStatus
	managedResources []managedResource
	conditions       []v1alpha1.ApplicationCondition
	hooks            []*unstructured.Unstructured
}

// appStateManager allows to compare applications to git
type appStateManager struct {
	db             db.ArgoDB
	settings       *settings.ArgoCDSettings
	appclientset   appclientset.Interface
	projInformer   cache.SharedIndexInformer
	kubectl        kubeutil.Kubectl
	repoClientset  reposerver.Clientset
	liveStateCache statecache.LiveStateCache
	namespace      string
}

func (m *appStateManager) getRepoObjs(app *v1alpha1.Application, appLabelKey, revision string, overrides []v1alpha1.ComponentParameter, noCache bool) ([]*unstructured.Unstructured, []*unstructured.Unstructured, *repository.ManifestResponse, error) {
	helmRepos, err := m.db.ListHelmRepos(context.Background())
	if err != nil {
		return nil, nil, nil, err
	}
	repo := m.getRepo(app.Spec.Source.RepoURL)
	conn, repoClient, err := m.repoClientset.NewRepositoryClient()
	if err != nil {
		return nil, nil, nil, err
	}
	defer util.Close(conn)

	if revision == "" {
		revision = app.Spec.Source.TargetRevision
	}

	// Decide what overrides to compare with.
	var mfReqOverrides []*v1alpha1.ComponentParameter
	if overrides != nil {
		// If overrides is supplied, use that
		mfReqOverrides = make([]*v1alpha1.ComponentParameter, len(overrides))
		for i := range overrides {
			item := overrides[i]
			mfReqOverrides[i] = &item
		}
	} else {
		// Otherwise, use the overrides in the app spec
		mfReqOverrides = make([]*v1alpha1.ComponentParameter, len(app.Spec.Source.ComponentParameterOverrides))
		for i := range app.Spec.Source.ComponentParameterOverrides {
			item := app.Spec.Source.ComponentParameterOverrides[i]
			mfReqOverrides[i] = &item
		}
	}

	manifestInfo, err := repoClient.GenerateManifest(context.Background(), &repository.ManifestRequest{
		Repo:                        repo,
		HelmRepos:                   helmRepos,
		Revision:                    revision,
		NoCache:                     noCache,
		ComponentParameterOverrides: mfReqOverrides,
		AppLabelKey:                 appLabelKey,
		AppLabelValue:               app.Name,
		Namespace:                   app.Spec.Destination.Namespace,
		ApplicationSource:           &app.Spec.Source,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	targetObjs := make([]*unstructured.Unstructured, 0)
	hooks := make([]*unstructured.Unstructured, 0)
	for _, manifest := range manifestInfo.Manifests {
		obj, err := v1alpha1.UnmarshalToUnstructured(manifest)
		if err != nil {
			return nil, nil, nil, err
		}
		if hookutil.IsHook(obj) {
			hooks = append(hooks, obj)
		} else {
			targetObjs = append(targetObjs, obj)
		}
	}
	return targetObjs, hooks, manifestInfo, nil
}

// CompareAppState compares application git state to the live app state, using the specified
// revision and supplied overrides. If revision or overrides are empty, then compares against
// revision and overrides in the app spec.
func (m *appStateManager) CompareAppState(app *v1alpha1.Application, revision string, overrides []v1alpha1.ComponentParameter, noCache bool) (*comparisonResult, error) {
	logCtx := log.WithField("application", app.Name)
	logCtx.Infof("Comparing app state (cluster: %s, namespace: %s)", app.Spec.Destination.Server, app.Spec.Destination.Namespace)
	observedAt := metav1.Now()
	failedToLoadObjs := false
	conditions := make([]v1alpha1.ApplicationCondition, 0)
	appLabelKey := m.settings.GetAppInstanceLabelKey()
	targetObjs, hooks, manifestInfo, err := m.getRepoObjs(app, appLabelKey, revision, overrides, noCache)
	if err != nil {
		targetObjs = make([]*unstructured.Unstructured, 0)
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error()})
		failedToLoadObjs = true
	}
	logCtx.Debugf("Generated config manifests")
	liveObjByKey, err := m.liveStateCache.GetManagedLiveObjs(app, targetObjs)
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
		if namespaced, err := m.liveStateCache.IsNamespaced(app.Spec.Destination.Server, obj.GroupVersionKind()); err == nil && !namespaced {
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
	diffResults, err := diff.DiffArray(targetObjs, managedLiveObj)
	if err != nil {
		return nil, err
	}

	syncCode := v1alpha1.SyncStatusCodeSynced
	managedResources := make([]managedResource, len(targetObjs))
	resourceSummaries := make([]v1alpha1.ResourceStatus, len(targetObjs))
	for i := 0; i < len(targetObjs); i++ {
		obj := managedLiveObj[i]
		if obj == nil {
			obj = targetObjs[i]
		}
		if obj == nil {
			continue
		}
		gvk := obj.GroupVersionKind()

		resState := v1alpha1.ResourceStatus{
			Namespace: util.FirstNonEmpty(obj.GetNamespace(), app.Spec.Destination.Namespace),
			Name:      obj.GetName(),
			Kind:      gvk.Kind,
			Version:   gvk.Version,
			Group:     gvk.Group,
			Hook:      hookutil.IsHook(obj),
		}

		diffResult := diffResults.Diffs[i]
		if resState.Hook {
			// For resource hooks, don't store sync status, and do not affect overall sync status
		} else if diffResult.Modified || targetObjs[i] == nil || managedLiveObj[i] == nil {
			// Set resource state to OutOfSync since one of the following is true:
			// * target and live resource are different
			// * target resource not defined and live resource is extra
			// * target resource present but live resource is missing
			resState.Status = v1alpha1.SyncStatusCodeOutOfSync
			syncCode = v1alpha1.SyncStatusCodeOutOfSync
		} else {
			resState.Status = v1alpha1.SyncStatusCodeSynced
		}
		managedResources[i] = managedResource{
			Name:      resState.Name,
			Namespace: resState.Namespace,
			Group:     resState.Group,
			Kind:      resState.Kind,
			Version:   resState.Version,
			Live:      managedLiveObj[i],
			Target:    targetObjs[i],
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
			Source:      app.Spec.Source,
			Destination: app.Spec.Destination,
		},
		Status: syncCode,
	}
	if manifestInfo != nil {
		syncStatus.Revision = manifestInfo.Revision
	}

	healthStatus, err := health.SetApplicationHealth(resourceSummaries, GetLiveObjs(managedResources), m.settings.ResourceOverrides)
	if err != nil {
		conditions = append(conditions, appv1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error()})
	}

	compRes := comparisonResult{
		observedAt:       observedAt,
		syncStatus:       &syncStatus,
		healthStatus:     healthStatus,
		resources:        resourceSummaries,
		managedResources: managedResources,
		conditions:       conditions,
		hooks:            hooks,
	}
	return &compRes, nil
}

func (m *appStateManager) getRepo(repoURL string) *v1alpha1.Repository {
	repo, err := m.db.GetRepository(context.Background(), repoURL)
	if err != nil {
		// If we couldn't retrieve from the repo service, assume public repositories
		repo = &v1alpha1.Repository{Repo: repoURL}
	}
	return repo
}

func (m *appStateManager) persistRevisionHistory(app *v1alpha1.Application, revision string, overrides []v1alpha1.ComponentParameter) error {

	var nextID int64 = 0
	if len(app.Status.History) > 0 {
		nextID = app.Status.History[len(app.Status.History)-1].ID + 1
	}
	if overrides == nil {
		overrides = app.Spec.Source.ComponentParameterOverrides
	}
	history := append(app.Status.History, v1alpha1.RevisionHistory{
		ComponentParameterOverrides: overrides,
		Revision:                    revision,
		DeployedAt:                  metav1.NewTime(time.Now().UTC()),
		ID:                          nextID,
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
	repoClientset reposerver.Clientset,
	namespace string,
	kubectl kubeutil.Kubectl,
	settings *settings.ArgoCDSettings,
	liveStateCache statecache.LiveStateCache,
	projInformer cache.SharedIndexInformer,
) AppStateManager {
	return &appStateManager{
		liveStateCache: liveStateCache,
		db:             db,
		appclientset:   appclientset,
		kubectl:        kubectl,
		repoClientset:  repoClientset,
		namespace:      namespace,
		settings:       settings,
		projInformer:   projInformer,
	}
}
