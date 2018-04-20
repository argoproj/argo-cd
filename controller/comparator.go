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

	// Retrieve the live versions of the objects
	liveObjs, err := kubeutil.GetResourcesWithLabel(clst.RESTConfig(), namespace, common.LabelApplicationName, app.Name)

	if err != nil {
		return nil, err
	}
	objByFullName := ks.groupLiveObjects(liveObjs, targetObjs)

	controlledLiveObj := make([]*unstructured.Unstructured, len(targetObjs))

	for i, targetObj := range targetObjs {
		controlledLiveObj[i] = objByFullName[getResourceFullName(targetObj)]
	}

	// Do the actual comparison
	diffResults, err := diff.DiffArray(targetObjs, controlledLiveObj)
	if err != nil {
		return nil, err
	}

	resources := make([]v1alpha1.ResourceState, len(targetObjs))
	for i := 0; i < len(targetObjs); i++ {
		resState := v1alpha1.ResourceState{
			ChildLiveResources: make([]v1alpha1.ResourceNode, 0),
		}
		targetObjBytes, err := json.Marshal(targetObjs[i].Object)
		if err != nil {
			return nil, err
		}
		resState.TargetState = string(targetObjBytes)
		if controlledLiveObj[i] == nil {
			resState.LiveState = "null"
		} else {
			liveObjBytes, err := json.Marshal(controlledLiveObj[i].Object)
			if err != nil {
				return nil, err
			}
			resState.LiveState = string(liveObjBytes)
		}
		diffResult := diffResults.Diffs[i]
		if diffResult.Modified {
			resState.Status = v1alpha1.ComparisonStatusOutOfSync
		} else {
			resState.Status = v1alpha1.ComparisonStatusSynced
		}
		resources[i] = resState
	}

	for i, resource := range resources {
		liveResource := controlledLiveObj[i]
		if liveResource != nil {
			childResources, err := getChildren(liveResource, objByFullName)
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
	}
	if diffResults.Modified {
		compResult.Status = v1alpha1.ComparisonStatusOutOfSync
	} else {
		compResult.Status = v1alpha1.ComparisonStatusSynced
	}
	return &compResult, nil
}

func getChildren(parent *unstructured.Unstructured, objByFullName map[string]*unstructured.Unstructured) ([]v1alpha1.ResourceNode, error) {
	children := make([]v1alpha1.ResourceNode, 0)
	for _, obj := range objByFullName {
		if metav1.IsControlledBy(obj, parent) {
			childResource := v1alpha1.ResourceNode{}
			json, err := json.Marshal(obj)
			if err != nil {
				return nil, err
			}
			childResource.State = string(json)
			childResourceChildren, err := getChildren(obj, objByFullName)
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
