package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/server/cluster"
	"github.com/argoproj/argo-cd/util/diff"
	kubeutil "github.com/argoproj/argo-cd/util/kube"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

// AppComparator defines methods which allow to compare application spec and actual application state.
type AppComparator interface {
	CompareAppState(server string, namespace string, targetObjs []*unstructured.Unstructured, app *v1alpha1.Application) (*v1alpha1.ComparisonResult, error)
}

// KsonnetAppComparator allows to compare application using KSonnet CLI
type KsonnetAppComparator struct {
	clusterService cluster.ClusterServiceServer
}

// groupLiveObjects deduplicate list of kubernetes resources and choose correct version of resource: if resource has corresponding expected application resource then method pick
// kubernetes resource with matching version, otherwise chooses single kubernetes resource with any version
func (ks *KsonnetAppComparator) groupLiveObjects(liveObjs []*unstructured.Unstructured, targetObjs []*unstructured.Unstructured) map[string]*unstructured.Unstructured {
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

// CompareAppState compares application spec and real app state using KSonnet
func (ks *KsonnetAppComparator) CompareAppState(
	server string,
	namespace string,
	targetObjs []*unstructured.Unstructured,
	app *v1alpha1.Application) (*v1alpha1.ComparisonResult, error) {

	log.Infof("Comparing app %s state in cluster %s (namespace: %s)", app.ObjectMeta.Name, server, namespace)
	// Get the REST config for the cluster corresponding to the environment
	clst, err := ks.clusterService.Get(context.Background(), &cluster.ClusterQuery{Server: server})
	if err != nil {
		return nil, err
	}
	restConfig := clst.RESTConfig()

	// Retrieve the live versions of the objects
	liveObjs, err := kubeutil.GetResourcesWithLabel(restConfig, namespace, common.LabelApplicationName, app.Name)
	if err != nil {
		return nil, err
	}

	liveObjByFullName := ks.groupLiveObjects(liveObjs, targetObjs)

	controlledLiveObj := make([]*unstructured.Unstructured, len(targetObjs))

	// Move live resources which have corresponding target object to controlledLiveObj
	dynClientPool := dynamic.NewDynamicClientPool(restConfig)
	disco, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, err
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
				return nil, err
			}
			apiResource, err := kubeutil.ServerResourceForGroupVersionKind(disco, gvk)
			if err != nil {
				return nil, err
			}
			liveObj, err = kubeutil.GetLiveResource(dclient, targetObj, apiResource, namespace)
			if err != nil {
				return nil, err
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
		return nil, err
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
				return nil, err
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
				return nil, err
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
				return nil, err
			}
			resource.ChildLiveResources = childResources
			resources[i] = resource
		}
	}
	compResult := v1alpha1.ComparisonResult{
		ComparedTo: app.Spec.Source,
		ComparedAt: metav1.Time{Time: time.Now().UTC()},
		Server:     clst.Server,
		Namespace:  namespace,
		Resources:  resources,
		Status:     comparisonStatus,
	}
	return &compResult, nil
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

// NewKsonnetAppComparator creates new instance of Ksonnet app comparator
func NewKsonnetAppComparator(clusterService cluster.ClusterServiceServer) AppComparator {
	return &KsonnetAppComparator{
		clusterService: clusterService,
	}
}
