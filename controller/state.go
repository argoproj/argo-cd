package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/diff"
	"github.com/argoproj/argo-cd/util/kube"
	kubeutil "github.com/argoproj/argo-cd/util/kube"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// AppStateManager defines methods which allow to compare application spec and actual application state.
type AppStateManager interface {
	CompareAppState(app *v1alpha1.Application) (*v1alpha1.ComparisonResult, *repository.ManifestResponse, error)
	SyncAppState(app *v1alpha1.Application, revision string, overrides *[]v1alpha1.ComponentParameter, dryRun bool, prune bool) *v1alpha1.OperationState
}

// KsonnetAppStateManager allows to compare application using KSonnet CLI
type KsonnetAppStateManager struct {
	db            db.ArgoDB
	appclientset  appclientset.Interface
	repoClientset reposerver.Clientset
	namespace     string
}

// groupLiveObjects deduplicate list of kubernetes resources and choose correct version of resource: if resource has corresponding expected application resource then method pick
// kubernetes resource with matching version, otherwise chooses single kubernetes resource with any version
func (ks *KsonnetAppStateManager) groupLiveObjects(liveObjs []*unstructured.Unstructured, targetObjs []*unstructured.Unstructured) map[string]*unstructured.Unstructured {
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

// GetManifest retrieves a manifest from the given repo and app version.
func GetManifest(repoClientset reposerver.Clientset, repo *v1alpha1.Repository, app *v1alpha1.Application) (*repository.ManifestResponse, error) {
	conn, repoClient, err := repoClientset.NewRepositoryClient()
	if err != nil {
		return nil, err
	}
	defer util.Close(conn)
	overrides := make([]*v1alpha1.ComponentParameter, len(app.Spec.Source.ComponentParameterOverrides))
	if app.Spec.Source.ComponentParameterOverrides != nil {
		for i := range app.Spec.Source.ComponentParameterOverrides {
			item := app.Spec.Source.ComponentParameterOverrides[i]
			overrides[i] = &item
		}
	}

	return repoClient.GenerateManifest(context.Background(), &repository.ManifestRequest{
		Repo:                        repo,
		Environment:                 app.Spec.Source.Environment,
		Path:                        app.Spec.Source.Path,
		Revision:                    app.Spec.Source.TargetRevision,
		ComponentParameterOverrides: overrides,
		AppLabel:                    app.Name,
	})
}

// CompareAppState compares application spec and real app state using KSonnet
func (ks *KsonnetAppStateManager) CompareAppState(app *v1alpha1.Application) (*v1alpha1.ComparisonResult, *repository.ManifestResponse, error) {
	repo := ks.getRepo(app.Spec.Source.RepoURL)
	manifestInfo, err := GetManifest(ks.repoClientset, repo, app)
	if err != nil {
		return nil, nil, err
	}

	targetObjs := make([]*unstructured.Unstructured, len(manifestInfo.Manifests))
	for i, manifest := range manifestInfo.Manifests {
		obj, err := v1alpha1.UnmarshalToUnstructured(manifest)
		if err != nil {
			return nil, nil, err
		}
		targetObjs[i] = obj
	}

	server, namespace := app.Spec.Destination.Server, app.Spec.Destination.Namespace

	log.Infof("Comparing app %s state in cluster %s (namespace: %s)", app.ObjectMeta.Name, server, namespace)
	// Get the REST config for the cluster corresponding to the environment
	clst, err := ks.db.GetCluster(context.Background(), server)
	if err != nil {
		return nil, nil, err
	}
	restConfig := clst.RESTConfig()

	// Retrieve the live versions of the objects
	liveObjs, err := kubeutil.GetResourcesWithLabel(restConfig, namespace, common.LabelApplicationName, app.Name)
	if err != nil {
		return nil, nil, err
	}

	liveObjByFullName := ks.groupLiveObjects(liveObjs, targetObjs)

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
		if liveObj == nil {
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
			liveObj, err = kubeutil.GetLiveResource(dclient, targetObj, apiResource, namespace)
			if err != nil {
				return nil, nil, err
			}
		}
		controlledLiveObj[i] = liveObj
		delete(liveObjByFullName, fullName)
	}

	// Move root level live resources to controlledLiveObj and add nil to targetObjs to indicate that target object is missing
	for fullName := range liveObjByFullName {
		liveObj := liveObjByFullName[fullName]
		if !hasParent(liveObj) {
			targetObjs = append(targetObjs, nil)
			controlledLiveObj = append(controlledLiveObj, liveObj)
		}
	}

	// Do the actual comparison
	diffResults, err := diff.DiffArray(targetObjs, controlledLiveObj)
	if err != nil {
		return nil, nil, err
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
				return nil, nil, err
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
				return nil, nil, err
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
				return nil, nil, err
			}
			resource.ChildLiveResources = childResources
			resources[i] = resource
		}
	}
	compResult := v1alpha1.ComparisonResult{
		ComparedTo: app.Spec.Source,
		ComparedAt: metav1.Time{Time: time.Now().UTC()},
		Resources:  resources,
		Status:     comparisonStatus,
	}
	return &compResult, manifestInfo, nil
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

func (s *KsonnetAppStateManager) SyncAppState(
	app *v1alpha1.Application, revision string, overrides *[]v1alpha1.ComponentParameter, dryRun bool, prune bool) *v1alpha1.OperationState {

	if revision != "" {
		app.Spec.Source.TargetRevision = revision
	}

	if overrides != nil {
		app.Spec.Source.ComponentParameterOverrides = *overrides
	}

	opRes, manifest := s.syncAppResources(app, dryRun, prune)
	if !dryRun && opRes.Phase.Successful() {
		err := s.persistDeploymentInfo(app, manifest.Revision, manifest.Params, nil)
		if err != nil {
			opRes.Phase = v1alpha1.OperationError
			opRes.Message = fmt.Sprintf("failed to record sync to history: %v", err)
		}
	}
	return opRes
}

func (s *KsonnetAppStateManager) getRepo(repoURL string) *v1alpha1.Repository {
	repo, err := s.db.GetRepository(context.Background(), repoURL)
	if err != nil {
		// If we couldn't retrieve from the repo service, assume public repositories
		repo = &v1alpha1.Repository{Repo: repoURL}
	}
	return repo
}

func (s *KsonnetAppStateManager) persistDeploymentInfo(
	app *v1alpha1.Application, revision string, envParams []*v1alpha1.ComponentParameter, overrides *[]v1alpha1.ComponentParameter) error {

	params := make([]v1alpha1.ComponentParameter, len(envParams))
	for i := range envParams {
		param := *envParams[i]
		params[i] = param
	}
	var nextId int64 = 0
	if len(app.Status.History) > 0 {
		nextId = app.Status.History[len(app.Status.History)-1].ID + 1
	}
	history := append(app.Status.History, v1alpha1.DeploymentInfo{
		ComponentParameterOverrides: app.Spec.Source.ComponentParameterOverrides,
		Revision:                    revision,
		Params:                      params,
		DeployedAt:                  metav1.NewTime(time.Now()),
		ID:                          nextId,
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

func (s *KsonnetAppStateManager) syncAppResources(
	app *v1alpha1.Application,
	dryRun bool,
	prune bool) (*v1alpha1.OperationState, *repository.ManifestResponse) {

	opRes := v1alpha1.OperationState{
		SyncResult: &v1alpha1.SyncOperationResult{},
	}

	comparison, manifestInfo, err := s.CompareAppState(app)
	if err != nil {
		opRes.Phase = v1alpha1.OperationError
		opRes.Message = err.Error()
		return &opRes, manifestInfo
	}

	clst, err := s.db.GetCluster(context.Background(), app.Spec.Destination.Server)
	if err != nil {
		opRes.Phase = v1alpha1.OperationError
		opRes.Message = err.Error()
		return &opRes, manifestInfo
	}
	config := clst.RESTConfig()

	opRes.SyncResult.Resources = make([]*v1alpha1.ResourceDetails, len(comparison.Resources))
	liveObjs := make([]*unstructured.Unstructured, len(comparison.Resources))
	targetObjs := make([]*unstructured.Unstructured, len(comparison.Resources))

	// First perform a `kubectl apply --dry-run` against all the manifests. This will detect most
	// (but not all) validation issues with the users' manifests (e.g. will detect syntax issues,
	// but will not not detect if they are mutating immutable fields). If anything fails, we will
	// refuse to perform the sync.
	dryRunSuccessful := true
	for i, resourceState := range comparison.Resources {
		liveObj, err := resourceState.LiveObject()
		if err != nil {
			opRes.Phase = v1alpha1.OperationError
			opRes.Message = fmt.Sprintf("Failed to unmarshal live object: %v", err)
			return &opRes, manifestInfo
		}
		targetObj, err := resourceState.TargetObject()
		if err != nil {
			opRes.Phase = v1alpha1.OperationError
			opRes.Message = fmt.Sprintf("Failed to unmarshal target object: %v", err)
			return &opRes, manifestInfo
		}
		liveObjs[i] = liveObj
		targetObjs[i] = targetObj
		resDetails, successful := syncObject(config, app.Spec.Destination.Namespace, targetObj, liveObj, prune, true)
		if !successful {
			dryRunSuccessful = false
		}
		opRes.SyncResult.Resources[i] = &resDetails
	}
	if !dryRunSuccessful {
		opRes.Phase = v1alpha1.OperationFailed
		opRes.Message = "one or more objects failed to apply (dry run)"
		return &opRes, manifestInfo
	}
	if dryRun {
		opRes.Phase = v1alpha1.OperationSucceeded
		opRes.Message = "successfully synced (dry run)"
		return &opRes, manifestInfo
	}
	// If we get here, all objects passed their dry-run, so we are now ready to actually perform the
	// `kubectl apply`. Loop through the resources again, this time without dry-run.
	syncSuccessful := true
	for i := range comparison.Resources {
		resDetails, successful := syncObject(config, app.Spec.Destination.Namespace, targetObjs[i], liveObjs[i], prune, false)
		if !successful {
			syncSuccessful = false
		}
		opRes.SyncResult.Resources[i] = &resDetails
	}
	if !syncSuccessful {
		opRes.Message = "one or more objects failed to apply"
		opRes.Phase = v1alpha1.OperationFailed
	} else {
		opRes.Message = "successfully synced"
		opRes.Phase = v1alpha1.OperationSucceeded
	}
	return &opRes, manifestInfo
}

// syncObject performs a sync of a single resource
func syncObject(config *rest.Config, namespace string, targetObj, liveObj *unstructured.Unstructured, prune, dryRun bool) (v1alpha1.ResourceDetails, bool) {
	obj := targetObj
	if obj == nil {
		obj = liveObj
	}
	resDetails := v1alpha1.ResourceDetails{
		Name:      obj.GetName(),
		Kind:      obj.GetKind(),
		Namespace: namespace,
	}
	needsDelete := targetObj == nil
	successful := true
	if needsDelete {
		if prune {
			if dryRun {
				resDetails.Message = "pruned (dry run)"
			} else {
				err := kubeutil.DeleteResource(config, liveObj, namespace)
				if err != nil {
					resDetails.Message = err.Error()
					successful = false
				} else {
					resDetails.Message = "pruned"
				}
			}
		} else {
			resDetails.Message = "ignored (requires pruning)"
		}
	} else {
		message, err := kube.ApplyResource(config, targetObj, namespace, dryRun)
		if err != nil {
			resDetails.Message = err.Error()
			successful = false
		} else {
			resDetails.Message = message
		}
	}
	return resDetails, successful
}

// NewAppStateManager creates new instance of Ksonnet app comparator
func NewAppStateManager(
	db db.ArgoDB,
	appclientset appclientset.Interface,
	repoClientset reposerver.Clientset,
	namespace string,
) AppStateManager {
	return &KsonnetAppStateManager{
		db:            db,
		appclientset:  appclientset,
		repoClientset: repoClientset,
		namespace:     namespace,
	}
}
