package controller

import (
	"context"
	"encoding/json"
	"time"

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

// CompareAppState compares application spec and real app state using KSonnet
func (ks *KsonnetAppComparator) CompareAppState(
	server string,
	namespace string,
	targetObjs []*unstructured.Unstructured,
	app *v1alpha1.Application) (*v1alpha1.ComparisonResult, error) {

	log.Infof("Comparing app %s state", app.ObjectMeta.Name)
	// Get the REST config for the cluster corresponding to the environment
	clst, err := ks.clusterService.Get(context.Background(), &cluster.ClusterQuery{Server: server})
	if err != nil {
		return nil, err
	}

	// Retrieve the live versions of the objects
	liveObjs, err := kubeutil.GetLiveResources(clst.RESTConfig(), targetObjs, namespace)
	if err != nil {
		return nil, err
	}

	// Do the actual comparison
	diffResults, err := diff.DiffArray(targetObjs, liveObjs)
	if err != nil {
		return nil, err
	}

	resources := make([]v1alpha1.ResourceState, len(targetObjs))
	for i := 0; i < len(targetObjs); i++ {
		resState := v1alpha1.ResourceState{}
		targetObjBytes, err := json.Marshal(targetObjs[i].Object)
		if err != nil {
			return nil, err
		}
		resState.TargetState = string(targetObjBytes)
		if liveObjs[i] == nil {
			resState.LiveState = "null"
		} else {
			liveObjBytes, err := json.Marshal(liveObjs[i].Object)
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

// NewKsonnetAppComparator creates new instance of Ksonnet app comparator
func NewKsonnetAppComparator(clusterService cluster.ClusterServiceServer) AppComparator {
	return &KsonnetAppComparator{
		clusterService: clusterService,
	}
}
