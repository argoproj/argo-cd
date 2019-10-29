package health

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func getReplicaSetHealth(obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	replicaSet := &appsv1.ReplicaSet{}
	err := scheme.Scheme.Convert(obj, replicaSet, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, replicaSet, err)
	}
	if replicaSet.Generation <= replicaSet.Status.ObservedGeneration {
		cond := getReplicaSetCondition(replicaSet.Status, v1.ReplicaSetReplicaFailure)
		if cond != nil && cond.Status == coreV1.ConditionTrue {
			return &appv1.HealthStatus{
				Status:  appv1.HealthStatusDegraded,
				Message: cond.Message,
			}, nil
		} else if replicaSet.Spec.Replicas != nil && replicaSet.Status.AvailableReplicas < *replicaSet.Spec.Replicas {
			return &appv1.HealthStatus{
				Status:  appv1.HealthStatusProgressing,
				Message: fmt.Sprintf("Waiting for rollout to finish: %d out of %d new replicas are available...", replicaSet.Status.AvailableReplicas, *replicaSet.Spec.Replicas),
			}, nil
		}
	} else {
		return &appv1.HealthStatus{
			Status:  appv1.HealthStatusProgressing,
			Message: "Waiting for rollout to finish: observed replica set generation less then desired generation",
		}, nil
	}

	return &appv1.HealthStatus{
		Status: appv1.HealthStatusHealthy,
	}, nil
}

func getReplicaSetCondition(status v1.ReplicaSetStatus, condType v1.ReplicaSetConditionType) *v1.ReplicaSetCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}
