package health

import (
	"fmt"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func getReplicaSetHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	gvk := obj.GroupVersionKind()
	switch gvk {
	case appsv1.SchemeGroupVersion.WithKind(kube.ReplicaSetKind):
		var replicaSet appsv1.ReplicaSet
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &replicaSet)
		if err != nil {
			return nil, fmt.Errorf("failed to convert unstructured ReplicaSet to typed: %v", err)
		}
		return getAppsv1ReplicaSetHealth(&replicaSet)
	case appsv1beta2.SchemeGroupVersion.WithKind(kube.ReplicaSetKind):
		var replicaSet appsv1beta2.ReplicaSet
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &replicaSet)
		if err != nil {
			return nil, fmt.Errorf("failed to convert unstructured ReplicaSet to typed: %v", err)
		}
		return getAppsv1beta1ReplicaSetHealth(&replicaSet)
	case extv1beta1.SchemeGroupVersion.WithKind(kube.ReplicaSetKind):
		var replicaSet extv1beta1.ReplicaSet
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &replicaSet)
		if err != nil {
			return nil, fmt.Errorf("failed to convert unstructured ReplicaSet to typed: %v", err)
		}
		return getExtv1beta1ReplicaSetHealth(&replicaSet)
	default:
		return nil, fmt.Errorf("unsupported ReplicaSet GVK: %s", gvk)
	}
}

func getAppsv1ReplicaSetHealth(replicaSet *appsv1.ReplicaSet) (*HealthStatus, error) {
	if replicaSet.Generation <= replicaSet.Status.ObservedGeneration {
		cond := getAppsv1ReplicaSetCondition(replicaSet.Status, appsv1.ReplicaSetReplicaFailure)
		if cond != nil && cond.Status == corev1.ConditionTrue {
			return &HealthStatus{
				Status:  HealthStatusDegraded,
				Message: cond.Message,
			}, nil
		} else if replicaSet.Spec.Replicas != nil && replicaSet.Status.AvailableReplicas < *replicaSet.Spec.Replicas {
			return &HealthStatus{
				Status:  HealthStatusProgressing,
				Message: fmt.Sprintf("Waiting for rollout to finish: %d out of %d new replicas are available...", replicaSet.Status.AvailableReplicas, *replicaSet.Spec.Replicas),
			}, nil
		}
	} else {
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: "Waiting for rollout to finish: observed replica set generation less then desired generation",
		}, nil
	}

	return &HealthStatus{
		Status: HealthStatusHealthy,
	}, nil
}

func getAppsv1beta1ReplicaSetHealth(replicaSet *appsv1beta2.ReplicaSet) (*HealthStatus, error) {
	if replicaSet.Generation <= replicaSet.Status.ObservedGeneration {
		cond := getAppsv1beta2ReplicaSetCondition(replicaSet.Status, appsv1beta2.ReplicaSetReplicaFailure)
		if cond != nil && cond.Status == corev1.ConditionTrue {
			return &HealthStatus{
				Status:  HealthStatusDegraded,
				Message: cond.Message,
			}, nil
		} else if replicaSet.Spec.Replicas != nil && replicaSet.Status.AvailableReplicas < *replicaSet.Spec.Replicas {
			return &HealthStatus{
				Status:  HealthStatusProgressing,
				Message: fmt.Sprintf("Waiting for rollout to finish: %d out of %d new replicas are available...", replicaSet.Status.AvailableReplicas, *replicaSet.Spec.Replicas),
			}, nil
		}
	} else {
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: "Waiting for rollout to finish: observed replica set generation less then desired generation",
		}, nil
	}

	return &HealthStatus{
		Status: HealthStatusHealthy,
	}, nil
}

func getExtv1beta1ReplicaSetHealth(replicaSet *extv1beta1.ReplicaSet) (*HealthStatus, error) {
	if replicaSet.Generation <= replicaSet.Status.ObservedGeneration {
		cond := getExtv1beta1ReplicaSetCondition(replicaSet.Status, extv1beta1.ReplicaSetReplicaFailure)
		if cond != nil && cond.Status == corev1.ConditionTrue {
			return &HealthStatus{
				Status:  HealthStatusDegraded,
				Message: cond.Message,
			}, nil
		} else if replicaSet.Spec.Replicas != nil && replicaSet.Status.AvailableReplicas < *replicaSet.Spec.Replicas {
			return &HealthStatus{
				Status:  HealthStatusProgressing,
				Message: fmt.Sprintf("Waiting for rollout to finish: %d out of %d new replicas are available...", replicaSet.Status.AvailableReplicas, *replicaSet.Spec.Replicas),
			}, nil
		}
	} else {
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: "Waiting for rollout to finish: observed replica set generation less then desired generation",
		}, nil
	}

	return &HealthStatus{
		Status: HealthStatusHealthy,
	}, nil
}

func getAppsv1ReplicaSetCondition(status appsv1.ReplicaSetStatus, condType appsv1.ReplicaSetConditionType) *appsv1.ReplicaSetCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

func getAppsv1beta2ReplicaSetCondition(status appsv1beta2.ReplicaSetStatus, condType appsv1beta2.ReplicaSetConditionType) *appsv1beta2.ReplicaSetCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

func getExtv1beta1ReplicaSetCondition(status extv1beta1.ReplicaSetStatus, condType extv1beta1.ReplicaSetConditionType) *extv1beta1.ReplicaSetCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}
