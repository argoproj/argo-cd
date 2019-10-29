package health

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func getStatefulSetHealth(obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	sts := &appsv1.StatefulSet{}
	err := scheme.Scheme.Convert(obj, sts, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, sts, err)
	}
	// Borrowed at kubernetes/kubectl/rollout_status.go https://github.com/kubernetes/kubernetes/blob/5232ad4a00ec93942d0b2c6359ee6cd1201b46bc/pkg/kubectl/rollout_status.go#L131
	if sts.Status.ObservedGeneration == 0 || sts.Generation > sts.Status.ObservedGeneration {
		return &appv1.HealthStatus{
			Status:  appv1.HealthStatusProgressing,
			Message: "Waiting for statefulset spec update to be observed...",
		}, nil
	}
	if sts.Spec.Replicas != nil && sts.Status.ReadyReplicas < *sts.Spec.Replicas {
		return &appv1.HealthStatus{
			Status:  appv1.HealthStatusProgressing,
			Message: fmt.Sprintf("Waiting for %d pods to be ready...", *sts.Spec.Replicas-sts.Status.ReadyReplicas),
		}, nil
	}
	if sts.Spec.UpdateStrategy.Type == apps.RollingUpdateStatefulSetStrategyType && sts.Spec.UpdateStrategy.RollingUpdate != nil {
		if sts.Spec.Replicas != nil && sts.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
			if sts.Status.UpdatedReplicas < (*sts.Spec.Replicas - *sts.Spec.UpdateStrategy.RollingUpdate.Partition) {
				return &appv1.HealthStatus{
					Status: appv1.HealthStatusProgressing,
					Message: fmt.Sprintf("Waiting for partitioned roll out to finish: %d out of %d new pods have been updated...",
						sts.Status.UpdatedReplicas, (*sts.Spec.Replicas - *sts.Spec.UpdateStrategy.RollingUpdate.Partition)),
				}, nil
			}
		}
		return &appv1.HealthStatus{
			Status:  appv1.HealthStatusHealthy,
			Message: fmt.Sprintf("partitioned roll out complete: %d new pods have been updated...", sts.Status.UpdatedReplicas),
		}, nil
	}
	if sts.Status.UpdateRevision != sts.Status.CurrentRevision {
		return &appv1.HealthStatus{
			Status:  appv1.HealthStatusProgressing,
			Message: fmt.Sprintf("waiting for statefulset rolling update to complete %d pods at revision %s...", sts.Status.UpdatedReplicas, sts.Status.UpdateRevision),
		}, nil
	}
	return &appv1.HealthStatus{
		Status:  appv1.HealthStatusHealthy,
		Message: fmt.Sprintf("statefulset rolling update complete %d pods at revision %s...", sts.Status.CurrentReplicas, sts.Status.CurrentRevision),
	}, nil
}
