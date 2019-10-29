package health

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func getDaemonSetHealth(obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	daemon := &appsv1.DaemonSet{}
	err := scheme.Scheme.Convert(obj, daemon, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, daemon, err)
	}
	// Borrowed at kubernetes/kubectl/rollout_status.go https://github.com/kubernetes/kubernetes/blob/5232ad4a00ec93942d0b2c6359ee6cd1201b46bc/pkg/kubectl/rollout_status.go#L110
	if daemon.Generation <= daemon.Status.ObservedGeneration {
		if daemon.Status.UpdatedNumberScheduled < daemon.Status.DesiredNumberScheduled {
			return &appv1.HealthStatus{
				Status:  appv1.HealthStatusProgressing,
				Message: fmt.Sprintf("Waiting for daemon set %q rollout to finish: %d out of %d new pods have been updated...", daemon.Name, daemon.Status.UpdatedNumberScheduled, daemon.Status.DesiredNumberScheduled),
			}, nil
		}
		if daemon.Status.NumberAvailable < daemon.Status.DesiredNumberScheduled {
			return &appv1.HealthStatus{
				Status:  appv1.HealthStatusProgressing,
				Message: fmt.Sprintf("Waiting for daemon set %q rollout to finish: %d of %d updated pods are available...", daemon.Name, daemon.Status.NumberAvailable, daemon.Status.DesiredNumberScheduled),
			}, nil
		}

	} else {
		return &appv1.HealthStatus{
			Status:  appv1.HealthStatusProgressing,
			Message: "Waiting for rollout to finish: observed daemon set generation less then desired generation",
		}, nil
	}
	return &appv1.HealthStatus{
		Status: appv1.HealthStatusHealthy,
	}, nil
}
