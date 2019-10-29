package health

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func getDeploymentHealth(obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	deployment := &appsv1.Deployment{}
	err := scheme.Scheme.Convert(obj, deployment, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, deployment, err)
	}
	if deployment.Spec.Paused {
		return &appv1.HealthStatus{
			Status:  appv1.HealthStatusSuspended,
			Message: "Deployment is paused",
		}, nil
	}
	// Borrowed at kubernetes/kubectl/rollout_status.go https://github.com/kubernetes/kubernetes/blob/5232ad4a00ec93942d0b2c6359ee6cd1201b46bc/pkg/kubectl/rollout_status.go#L80
	if deployment.Generation <= deployment.Status.ObservedGeneration {
		cond := getDeploymentCondition(deployment.Status, v1.DeploymentProgressing)
		if cond != nil && cond.Reason == "ProgressDeadlineExceeded" {
			return &appv1.HealthStatus{
				Status:  appv1.HealthStatusDegraded,
				Message: fmt.Sprintf("Deployment %q exceeded its progress deadline", obj.GetName()),
			}, nil
		} else if deployment.Spec.Replicas != nil && deployment.Status.UpdatedReplicas < *deployment.Spec.Replicas {
			return &appv1.HealthStatus{
				Status:  appv1.HealthStatusProgressing,
				Message: fmt.Sprintf("Waiting for rollout to finish: %d out of %d new replicas have been updated...", deployment.Status.UpdatedReplicas, *deployment.Spec.Replicas),
			}, nil
		} else if deployment.Status.Replicas > deployment.Status.UpdatedReplicas {
			return &appv1.HealthStatus{
				Status:  appv1.HealthStatusProgressing,
				Message: fmt.Sprintf("Waiting for rollout to finish: %d old replicas are pending termination...", deployment.Status.Replicas-deployment.Status.UpdatedReplicas),
			}, nil
		} else if deployment.Status.AvailableReplicas < deployment.Status.UpdatedReplicas {
			return &appv1.HealthStatus{
				Status:  appv1.HealthStatusProgressing,
				Message: fmt.Sprintf("Waiting for rollout to finish: %d of %d updated replicas are available...", deployment.Status.AvailableReplicas, deployment.Status.UpdatedReplicas),
			}, nil
		}
	} else {
		return &appv1.HealthStatus{
			Status:  appv1.HealthStatusProgressing,
			Message: "Waiting for rollout to finish: observed deployment generation less then desired generation",
		}, nil
	}

	return &appv1.HealthStatus{
		Status: appv1.HealthStatusHealthy,
	}, nil
}

func getDeploymentCondition(status v1.DeploymentStatus, condType v1.DeploymentConditionType) *v1.DeploymentCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}
