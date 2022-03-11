package health

import (
	"fmt"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func getDaemonSetHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	gvk := obj.GroupVersionKind()
	switch gvk {
	case appsv1.SchemeGroupVersion.WithKind(kube.DaemonSetKind):
		var daemon appsv1.DaemonSet
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &daemon)
		if err != nil {
			return nil, fmt.Errorf("failed to convert unstructured DaemonSet to typed: %v", err)
		}
		return getAppsv1DaemonSetHealth(&daemon)
	default:
		return nil, fmt.Errorf("unsupported DaemonSet GVK: %s", gvk)
	}
}

func getAppsv1DaemonSetHealth(daemon *appsv1.DaemonSet) (*HealthStatus, error) {
	// Borrowed at kubernetes/kubectl/rollout_status.go https://github.com/kubernetes/kubernetes/blob/5232ad4a00ec93942d0b2c6359ee6cd1201b46bc/pkg/kubectl/rollout_status.go#L110
	if daemon.Generation <= daemon.Status.ObservedGeneration {
		if daemon.Spec.UpdateStrategy.Type == appsv1.OnDeleteDaemonSetStrategyType {
			return &HealthStatus{
				Status:  HealthStatusHealthy,
				Message: fmt.Sprintf("daemon set %d out of %d new pods have been updated", daemon.Status.UpdatedNumberScheduled, daemon.Status.DesiredNumberScheduled),
			}, nil
		}
		if daemon.Status.UpdatedNumberScheduled < daemon.Status.DesiredNumberScheduled {
			return &HealthStatus{
				Status:  HealthStatusProgressing,
				Message: fmt.Sprintf("Waiting for daemon set %q rollout to finish: %d out of %d new pods have been updated...", daemon.Name, daemon.Status.UpdatedNumberScheduled, daemon.Status.DesiredNumberScheduled),
			}, nil
		}
		if daemon.Status.NumberAvailable < daemon.Status.DesiredNumberScheduled {
			return &HealthStatus{
				Status:  HealthStatusProgressing,
				Message: fmt.Sprintf("Waiting for daemon set %q rollout to finish: %d of %d updated pods are available...", daemon.Name, daemon.Status.NumberAvailable, daemon.Status.DesiredNumberScheduled),
			}, nil
		}
	} else {
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: "Waiting for rollout to finish: observed daemon set generation less then desired generation",
		}, nil
	}
	return &HealthStatus{
		Status: HealthStatusHealthy,
	}, nil
}
