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
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/diff"
	kubeutil "github.com/argoproj/argo-cd/util/kube"
)

const (
	maxHistoryCnt = 5
)

// AppStateManager defines methods which allow to compare application spec and actual application state.
type AppStateManager interface {
	CompareAppState(app *v1alpha1.Application, revision string, overrides []v1alpha1.ComponentParameter) (
		*v1alpha1.ComparisonResult, *repository.ManifestResponse, []v1alpha1.ApplicationCondition, error)
	SyncAppState(app *v1alpha1.Application, state *v1alpha1.OperationState)
}

// ksonnetAppStateManager allows to compare application using KSonnet CLI
type ksonnetAppStateManager struct {
	db            db.ArgoDB
	appclientset  appclientset.Interface
	repoClientset reposerver.Clientset
	namespace     string
}

// groupLiveObjects deduplicate list of kubernetes resources and choose correct version of resource: if resource has corresponding expected application resource then method pick
// kubernetes resource with matching version, otherwise chooses single kubernetes resource with any version
func groupLiveObjects(liveObjs []*unstructured.Unstructured, targetObjs []*unstructured.Unstructured) map[string]*unstructured.Unstructured {
	targetByFullName := make(map[string]*unstructured.Unstructured)
	for _, obj := range targetObjs {
		targetByFullName[getResourceFullName(obj)] = obj
	}

	liveListByFullName := make(map[string][]*unstructured.Unstructured)
	for _, obj := range liveObjs {
		list := liveListByFullName[getResourceFullName(obj)]
		if list == nil {
			list = make([]*unstructured.Unstructured, 0)
		}
		list = append(list, obj)
		liveListByFullName[getResourceFullName(obj)] = list
	}

	liveByFullName := make(map[string]*unstructured.Unstructured)

	for fullName, list := range liveListByFullName {
		targetObj := targetByFullName[fullName]
		var liveObj *unstructured.Unstructured
		if targetObj != nil {
			for i := range list {
				if list[i].GetAPIVersion() == targetObj.GetAPIVersion() {
					liveObj = list[i]
					break
				}
			}
		} else {
			liveObj = list[0]
		}
		if liveObj != nil {
			liveByFullName[getResourceFullName(liveObj)] = liveObj
		}
	}
	return liveByFullName
}

func (s *ksonnetAppStateManager) getTargetObjs(app *v1alpha1.Application, revision string, overrides []v1alpha1.ComponentParameter) ([]*unstructured.Unstructured, *repository.ManifestResponse, error) {
	repo := s.getRepo(app.Spec.Source.RepoURL)
	conn, repoClient, err := s.repoClientset.NewRepositoryClient()
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
		Environment:                 app.Spec.Source.Environment,
		Path:                        app.Spec.Source.Path,
		Revision:                    revision,
		ComponentParameterOverrides: mfReqOverrides,
		AppLabel:                    app.Name,
		ValueFiles:                  app.Spec.Source.ValuesFiles,
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
		if isHook(obj) {
			continue
		}
		targetObjs = append(targetObjs, obj)
	}
	return targetObjs, manifestInfo, nil
}

func (s *ksonnetAppStateManager) getLiveObjs(app *v1alpha1.Application, targetObjs []*unstructured.Unstructured) (
	[]*unstructured.Unstructured, map[string]*unstructured.Unstructured, error) {

	// Get the REST config for the cluster corresponding to the environment
	clst, err := s.db.GetCluster(context.Background(), app.Spec.Destination.Server)
	if err != nil {
		return nil, nil, err
	}
	restConfig := clst.RESTConfig()

	// Retrieve the live versions of the objects. exclude any hook objects
	labeledObjs, err := kubeutil.GetResourcesWithLabel(restConfig, app.Spec.Destination.Namespace, common.LabelApplicationName, app.Name)
	if err != nil {
		return nil, nil, err
	}
	liveObjs := make([]*unstructured.Unstructured, 0)
	for _, obj := range labeledObjs {
		if isHook(obj) {
			continue
		}
		liveObjs = append(liveObjs, obj)
	}

	liveObjByFullName := groupLiveObjects(liveObjs, targetObjs)

	controlledLiveObj := make([]*unstructured.Unstructured, len(targetObjs))

	// Move live resources which have corresponding target object to controlledLiveObj
	dynClientPool := dynamic.NewDynamicClientPool(restConfig)
	disco, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, nil, err
	}
	for i, targetObj := range targetObjs {
		fullName := getResourceFullName(targetObj)
		liveObj := liveObjByFullName[fullName]
		if liveObj == nil && targetObj.GetName() != "" {
			// If we get here, it indicates we did not find the live resource when querying using
			// our app label. However, it is possible that the resource was created/modified outside
			// of ArgoCD. In order to determine that it is truly missing, we fall back to perform a
			// direct lookup of the resource by name. See issue #141
			gvk := targetObj.GroupVersionKind()
			dclient, err := dynClientPool.ClientForGroupVersionKind(gvk)
			if err != nil {
				return nil, nil, err
			}
			apiResource, err := kubeutil.ServerResourceForGroupVersionKind(disco, gvk)
			if err != nil {
				return nil, nil, err
			}
			liveObj, err = kubeutil.GetLiveResource(dclient, targetObj, apiResource, app.Spec.Destination.Namespace)
			if err != nil {
				return nil, nil, err
			}
		}
		controlledLiveObj[i] = liveObj
		delete(liveObjByFullName, fullName)
	}

	return controlledLiveObj, liveObjByFullName, nil
}

// CompareAppState compares application git state to the live app state, using the specified
// revision and supplied overrides. If revision or overrides are empty, then compares against
// revision and overrides in the app spec.
func (s *ksonnetAppStateManager) CompareAppState(app *v1alpha1.Application, revision string, overrides []v1alpha1.ComponentParameter) (
	*v1alpha1.ComparisonResult, *repository.ManifestResponse, []v1alpha1.ApplicationCondition, error) {

	failedToLoadObjs := false
	conditions := make([]v1alpha1.ApplicationCondition, 0)
	targetObjs, manifestInfo, err := s.getTargetObjs(app, revision, overrides)
	if err != nil {
		targetObjs = make([]*unstructured.Unstructured, 0)
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error()})
		failedToLoadObjs = true
	}

	controlledLiveObj, liveObjByFullName, err := s.getLiveObjs(app, targetObjs)
	if err != nil {
		controlledLiveObj = make([]*unstructured.Unstructured, len(targetObjs))
		liveObjByFullName = make(map[string]*unstructured.Unstructured)
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionComparisonError, Message: err.Error()})
		failedToLoadObjs = true
	}

	for _, liveObj := range controlledLiveObj {
		if liveObj != nil && liveObj.GetLabels() != nil {
			if appLabelVal, ok := liveObj.GetLabels()[common.LabelApplicationName]; ok && appLabelVal != "" && appLabelVal != app.Name {
				conditions = append(conditions, v1alpha1.ApplicationCondition{
					Type:    v1alpha1.ApplicationConditionSharedResourceWarning,
					Message: fmt.Sprintf("Resource %s/%s is controller by applications '%s' and '%s'", liveObj.GetKind(), liveObj.GetName(), app.Name, appLabelVal),
				})
			}
		}
	}

	// Move root level live resources to controlledLiveObj and add nil to targetObjs to indicate that target object is missing
	for fullName := range liveObjByFullName {
		liveObj := liveObjByFullName[fullName]
		if !hasParent(liveObj) {
			targetObjs = append(targetObjs, nil)
			controlledLiveObj = append(controlledLiveObj, liveObj)
		}
	}

	log.Infof("Comparing app %s state in cluster %s (namespace: %s)", app.ObjectMeta.Name, app.Spec.Destination.Server, app.Spec.Destination.Namespace)

	// Do the actual comparison
	diffResults, err := diff.DiffArray(targetObjs, controlledLiveObj)
	if err != nil {
		return nil, nil, nil, err
	}

	comparisonStatus := v1alpha1.ComparisonStatusSynced

	resources := make([]v1alpha1.ResourceState, len(targetObjs))
	for i := 0; i < len(targetObjs); i++ {
		resState := v1alpha1.ResourceState{
			ChildLiveResources: make([]v1alpha1.ResourceNode, 0),
		}
		diffResult := diffResults.Diffs[i]
		if diffResult.Modified {
			// Set resource state to 'OutOfSync' since target and corresponding live resource are different
			resState.Status = v1alpha1.ComparisonStatusOutOfSync
			comparisonStatus = v1alpha1.ComparisonStatusOutOfSync
		} else {
			resState.Status = v1alpha1.ComparisonStatusSynced
		}

		if targetObjs[i] == nil {
			resState.TargetState = "null"
			// Set resource state to 'OutOfSync' since target resource is missing and live resource is unexpected
			resState.Status = v1alpha1.ComparisonStatusOutOfSync
			comparisonStatus = v1alpha1.ComparisonStatusOutOfSync
		} else {
			targetObjBytes, err := json.Marshal(targetObjs[i].Object)
			if err != nil {
				return nil, nil, nil, err
			}
			resState.TargetState = string(targetObjBytes)
		}

		if controlledLiveObj[i] == nil {
			resState.LiveState = "null"
			// Set resource state to 'OutOfSync' since target resource present but corresponding live resource is missing
			resState.Status = v1alpha1.ComparisonStatusOutOfSync
			comparisonStatus = v1alpha1.ComparisonStatusOutOfSync
		} else {
			liveObjBytes, err := json.Marshal(controlledLiveObj[i].Object)
			if err != nil {
				return nil, nil, nil, err
			}
			resState.LiveState = string(liveObjBytes)
		}

		resources[i] = resState
	}

	for i, resource := range resources {
		liveResource := controlledLiveObj[i]
		if liveResource != nil {
			childResources, err := getChildren(liveResource, liveObjByFullName)
			if err != nil {
				return nil, nil, nil, err
			}
			resource.ChildLiveResources = childResources
			resources[i] = resource
		}
	}
	if failedToLoadObjs {
		comparisonStatus = v1alpha1.ComparisonStatusUnknown
	}
	compResult := v1alpha1.ComparisonResult{
		ComparedTo: app.Spec.Source,
		ComparedAt: metav1.Time{Time: time.Now().UTC()},
		Resources:  resources,
		Status:     comparisonStatus,
	}
	return &compResult, manifestInfo, conditions, nil
}

func hasParent(obj *unstructured.Unstructured) bool {
	// TODO: remove special case after Service and Endpoint get explicit relationship ( https://github.com/kubernetes/kubernetes/issues/28483 )
	return obj.GetKind() == kubeutil.EndpointsKind || metav1.GetControllerOf(obj) != nil
}

func isControlledBy(obj *unstructured.Unstructured, parent *unstructured.Unstructured) bool {
	// TODO: remove special case after Service and Endpoint get explicit relationship ( https://github.com/kubernetes/kubernetes/issues/28483 )
	if obj.GetKind() == kubeutil.EndpointsKind && parent.GetKind() == kubeutil.ServiceKind {
		return obj.GetName() == parent.GetName()
	}
	return metav1.IsControlledBy(obj, parent)
}

func getChildren(parent *unstructured.Unstructured, liveObjByFullName map[string]*unstructured.Unstructured) ([]v1alpha1.ResourceNode, error) {
	children := make([]v1alpha1.ResourceNode, 0)
	for fullName, obj := range liveObjByFullName {
		if isControlledBy(obj, parent) {
			delete(liveObjByFullName, fullName)
			childResource := v1alpha1.ResourceNode{}
			json, err := json.Marshal(obj)
			if err != nil {
				return nil, err
			}
			childResource.State = string(json)
			childResourceChildren, err := getChildren(obj, liveObjByFullName)
			if err != nil {
				return nil, err
			}
			childResource.Children = childResourceChildren
			children = append(children, childResource)
		}
	}
	return children, nil
}

func getResourceFullName(obj *unstructured.Unstructured) string {
	return fmt.Sprintf("%s:%s", obj.GetKind(), obj.GetName())
}

func (s *ksonnetAppStateManager) getRepo(repoURL string) *v1alpha1.Repository {
	repo, err := s.db.GetRepository(context.Background(), repoURL)
	if err != nil {
		// If we couldn't retrieve from the repo service, assume public repositories
		repo = &v1alpha1.Repository{Repo: repoURL}
	}
	return repo
}

func (s *ksonnetAppStateManager) persistDeploymentInfo(
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
		Params:                      params,
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
	_, err = s.appclientset.ArgoprojV1alpha1().Applications(s.namespace).Patch(app.Name, types.MergePatchType, patch)
	return err
}

// NewAppStateManager creates new instance of Ksonnet app comparator
func NewAppStateManager(
	db db.ArgoDB,
	appclientset appclientset.Interface,
	repoClientset reposerver.Clientset,
	namespace string,
) AppStateManager {
	return &ksonnetAppStateManager{
		db:            db,
		appclientset:  appclientset,
		repoClientset: repoClientset,
		namespace:     namespace,
	}
}
