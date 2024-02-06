package health

import (
	"fmt"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func getStatefulSetHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	gvk := obj.GroupVersionKind()
	switch gvk {
	case appsv1.SchemeGroupVersion.WithKind(kube.StatefulSetKind):
		var sts appsv1.StatefulSet
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &sts)
		if err != nil {
			return nil, fmt.Errorf("failed to convert unstructured StatefulSet to typed: %v", err)
		}
		return getAppsv1StatefulSetHealth(&sts)
	default:
		return nil, fmt.Errorf("unsupported StatefulSet GVK: %s", gvk)
	}
}

func getAppsv1StatefulSetHealth(sts *appsv1.StatefulSet) (*HealthStatus, error) {
	// Borrowed at kubernetes/kubectl/rollout_status.go https://github.com/kubernetes/kubernetes/blob/5232ad4a00ec93942d0b2c6359ee6cd1201b46bc/pkg/kubectl/rollout_status.go#L131
	if sts.Status.ObservedGeneration == 0 || sts.Generation > sts.Status.ObservedGeneration {
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: "Waiting for statefulset spec update to be observed...",
		}, nil
	}
	if sts.Spec.Replicas != nil && sts.Status.ReadyReplicas < *sts.Spec.Replicas {
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: fmt.Sprintf("Waiting for %d pods to be ready...", *sts.Spec.Replicas-sts.Status.ReadyReplicas),
		}, nil
	}
	if sts.Spec.UpdateStrategy.Type == appsv1.RollingUpdateStatefulSetStrategyType && sts.Spec.UpdateStrategy.RollingUpdate != nil {
		if sts.Spec.Replicas != nil && sts.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
			if sts.Status.UpdatedReplicas < (*sts.Spec.Replicas - *sts.Spec.UpdateStrategy.RollingUpdate.Partition) {
				return &HealthStatus{
					Status: HealthStatusProgressing,
					Message: fmt.Sprintf("Waiting for partitioned roll out to finish: %d out of %d new pods have been updated...",
						sts.Status.UpdatedReplicas, (*sts.Spec.Replicas - *sts.Spec.UpdateStrategy.RollingUpdate.Partition)),
				}, nil
			}
		}
		return &HealthStatus{
			Status:  HealthStatusHealthy,
			Message: fmt.Sprintf("partitioned roll out complete: %d new pods have been updated...", sts.Status.UpdatedReplicas),
		}, nil
	}
	if sts.Spec.UpdateStrategy.Type == appsv1.OnDeleteStatefulSetStrategyType {
		return &HealthStatus{
			Status:  HealthStatusHealthy,
			Message: fmt.Sprintf("statefulset has %d ready pods", sts.Status.ReadyReplicas),
		}, nil
	}
	if sts.Status.UpdateRevision != sts.Status.CurrentRevision {
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: fmt.Sprintf("waiting for statefulset rolling update to complete %d pods at revision %s...", sts.Status.UpdatedReplicas, sts.Status.UpdateRevision),
		}, nil
	}
	return &HealthStatus{
		Status:  HealthStatusHealthy,
		Message: fmt.Sprintf("statefulset rolling update complete %d pods at revision %s...", sts.Status.CurrentReplicas, sts.Status.CurrentRevision),
	}, nil
}
