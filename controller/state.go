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

	statecache "github.com/argoproj/argo-cd/controller/cache"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/diff"
	hookutil "github.com/argoproj/argo-cd/util/hook"
	kubeutil "github.com/argoproj/argo-cd/util/kube"
)

const (
	maxHistoryCnt = 5
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
	CompareAppState(app *v1alpha1.Application, revision string, overrides []v1alpha1.ComponentParameter) (
		*v1alpha1.ComparisonResult, *repository.ManifestResponse, []managedResource, []v1alpha1.ApplicationCondition, error)
	SyncAppState(app *v1alpha1.Application, state *v1alpha1.OperationState)
	GetTargetObjs(app *v1alpha1.Application, revision string, overrides []v1alpha1.ComponentParameter) ([]*unstructured.Unstructured, *repository.ManifestResponse, error)
}

// appStateManager allows to compare application using KSonnet CLI
type appStateManager struct {
	db             db.ArgoDB
	appclientset   appclientset.Interface
	kubectl        kubeutil.Kubectl
	repoClientset  reposerver.Clientset
	liveStateCache statecache.LiveStateCache
	namespace      string
}

func (m *appStateManager) GetTargetObjs(app *v1alpha1.Application, revision string, overrides []v1alpha1.ComponentParameter) ([]*unstructured.Unstructured, *repository.ManifestResponse, error) {
	helmRepos, err := m.db.ListHelmRepos(context.Background())
	if err != nil {
		return nil, nil, err
	}
	repo := m.getRepo(app.Spec.Source.RepoURL)
	conn, repoClient, err := m.repoClientset.NewRepositoryClient()
	if err != nil {
		return nil, nil, err
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
		ComponentParameterOverrides: mfReqOverrides,
		AppLabel:                    app.Name,
		Namespace:                   app.Spec.Destination.Namespace,
		ApplicationSource:           &app.Spec.Source,
	})
	if err != nil {
		return nil, nil, err
	}

	targetObjs := make([]*unstructured.Unstructured, 0)
	for _, manifest := range manifestInfo.Manifests {
		obj, err := v1alpha1.UnmarshalToUnstructured(manifest)
		if err != nil {
			return nil, nil, err
		}
		if hookutil.IsHook(obj) {
			// Hooks are not target-state objects, so don't include them in the list
			continue
		}
		targetObjs = append(targetObjs, obj)
	}
	return targetObjs, manifestInfo, nil
}

// CompareAppState compares application git state to the live app state, using the specified
// revision and supplied overrides. If revision or overrides are empty, then compares against
// revision and overrides in the app spec.
func (m *appStateManager) CompareAppState(app *v1alpha1.Application, revision string, overrides []v1alpha1.ComponentParameter) (
	*v1alpha1.ComparisonResult, *repository.ManifestResponse, []managedResource, []v1alpha1.ApplicationCondition, error) {

	failedToLoadObjs := false
	conditions := make([]v1alpha1.ApplicationCondition, 0)
	targetObjs, manifestInfo, err := m.GetTargetObjs(app, revision, overrides)
	if err != nil {
		targetObjs = make([]*unstructured.Unstructured, 0)
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error()})
		failedToLoadObjs = true
	}

	liveObjByKey, err := m.liveStateCache.GetManagedLiveObjs(app, targetObjs)
	if err != nil {
		liveObjByKey = make(map[kubeutil.ResourceKey]*unstructured.Unstructured)
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error()})
		failedToLoadObjs = true
	}

	for _, liveObj := range liveObjByKey {
		if liveObj != nil {
			appInstanceName := kubeutil.GetAppInstanceLabel(liveObj)
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

	// Everything remaining in liveObjByKey are "extra" resources that aren't tracked in git.
	// The following adds all the extras to the managedLiveObj list and backfills the targetObj
	// list with nils, so that the lists are of equal lengths for comparison purposes.
	for _, obj := range liveObjByKey {
		targetObjs = append(targetObjs, nil)
		managedLiveObj = append(managedLiveObj, obj)
	}

	log.Infof("Comparing app %s state in cluster %s (namespace: %s)", app.ObjectMeta.Name, app.Spec.Destination.Server, app.Spec.Destination.Namespace)

	// Do the actual comparison
	diffResults, err := diff.DiffArray(targetObjs, managedLiveObj)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	comparisonStatus := v1alpha1.ComparisonStatusSynced

	managedResources := make([]managedResource, len(targetObjs))
	resourceSummaries := make([]v1alpha1.ResourceSummary, len(targetObjs))
	for i := 0; i < len(targetObjs); i++ {
		obj := managedLiveObj[i]
		if obj == nil {
			obj = targetObjs[i]
		}
		if obj == nil {
			continue
		}
		gvk := obj.GroupVersionKind()

		resState := v1alpha1.ResourceSummary{
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
			resState.Status = v1alpha1.ComparisonStatusOutOfSync
			comparisonStatus = v1alpha1.ComparisonStatusOutOfSync
		} else {
			resState.Status = v1alpha1.ComparisonStatusSynced
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
		comparisonStatus = v1alpha1.ComparisonStatusUnknown
	}

	compResult := v1alpha1.ComparisonResult{
		ComparedTo: app.Spec.Source,
		ComparedAt: metav1.Time{Time: time.Now().UTC()},
		Status:     comparisonStatus,
		Resources:  resourceSummaries,
	}

	if manifestInfo != nil {
		compResult.Revision = manifestInfo.Revision
	}
	return &compResult, manifestInfo, managedResources, conditions, nil
}

func (m *appStateManager) getRepo(repoURL string) *v1alpha1.Repository {
	repo, err := m.db.GetRepository(context.Background(), repoURL)
	if err != nil {
		// If we couldn't retrieve from the repo service, assume public repositories
		repo = &v1alpha1.Repository{Repo: repoURL}
	}
	return repo
}

func (m *appStateManager) persistDeploymentInfo(
	app *v1alpha1.Application, revision string, envParams []*v1alpha1.ComponentParameter, overrides *[]v1alpha1.ComponentParameter) error {

	params := make([]v1alpha1.ComponentParameter, len(envParams))
	for i := range envParams {
		param := *envParams[i]
		params[i] = param
	}
	var nextID int64 = 0
	if len(app.Status.History) > 0 {
		nextID = app.Status.History[len(app.Status.History)-1].ID + 1
	}
	history := append(app.Status.History, v1alpha1.DeploymentInfo{
		ComponentParameterOverrides: app.Spec.Source.ComponentParameterOverrides,
		Revision:                    revision,
		DeployedAt:                  metav1.NewTime(time.Now().UTC()),
		ID:                          nextID,
	})

	if len(history) > maxHistoryCnt {
		history = history[1 : maxHistoryCnt+1]
	}

	patch, err := json.Marshal(map[string]map[string][]v1alpha1.DeploymentInfo{
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
	liveStateCache statecache.LiveStateCache,
) AppStateManager {
	return &appStateManager{
		liveStateCache: liveStateCache,
		db:             db,
		appclientset:   appclientset,
		kubectl:        kubectl,
		repoClientset:  repoClientset,
		namespace:      namespace,
	}
}
