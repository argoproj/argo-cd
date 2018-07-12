package health

import (
	"fmt"

	"k8s.io/api/apps/v1"
	appsv1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/kube"
)

func GetAppHealth(obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	switch obj.GetKind() {
	case kube.DeploymentKind:
		return getDeploymentHealth(obj)
	case kube.ServiceKind:
		return getServiceHealth(obj)
	case kube.IngressKind:
		return getIngressHealth(obj)
	default:
		return &appv1.HealthStatus{Status: appv1.HealthStatusHealthy}, nil
	}
}

// healthOrder is a list of health codes in order of most healthy to least healthy
var healthOrder = []appv1.HealthStatusCode{
	appv1.HealthStatusHealthy,
	appv1.HealthStatusProgressing,
	appv1.HealthStatusDegraded,
	appv1.HealthStatusMissing,
	appv1.HealthStatusUnknown,
}

// IsWorse returns whether or not the new health status code is a worser condition than the current
func IsWorse(current, new appv1.HealthStatusCode) bool {
	currentIndex := 0
	newIndex := 0
	for i, code := range healthOrder {
		if current == code {
			currentIndex = i
		}
		if new == code {
			newIndex = i
		}
	}
	return newIndex > currentIndex
}

func getIngressHealth(obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	obj, err := kube.ConvertToVersion(obj, "", "v1")
	if err != nil {
		return nil, err
	}
	var ingress v1beta1.Ingress
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &ingress)
	if err != nil {
		return nil, err
	}

	health := appv1.HealthStatus{Status: appv1.HealthStatusProgressing}
	for _, ingress := range ingress.Status.LoadBalancer.Ingress {
		if ingress.Hostname != "" || ingress.IP != "" {
			health.Status = appv1.HealthStatusHealthy
			break
		}
	}
	return &health, nil
}

func getServiceHealth(obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	obj, err := kube.ConvertToVersion(obj, "", "v1")
	if err != nil {
		return nil, err
	}
	var service coreV1.Service
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &service)
	if err != nil {
		return nil, err
	}
	health := appv1.HealthStatus{Status: appv1.HealthStatusHealthy}
	if service.Spec.Type == coreV1.ServiceTypeLoadBalancer {
		health.Status = appv1.HealthStatusProgressing
		for _, ingress := range service.Status.LoadBalancer.Ingress {
			if ingress.Hostname != "" || ingress.IP != "" {
				health.Status = appv1.HealthStatusHealthy
				break
			}
		}
	}
	return &health, nil
}

func getDeploymentHealth(obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	obj, err := kube.ConvertToVersion(obj, "apps", "v1")
	if err != nil {
		return nil, err
	}
	var deployment appsv1.Deployment
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &deployment)
	if err != nil {
		return nil, err
	}
	if deployment.Generation <= deployment.Status.ObservedGeneration {
		cond := getDeploymentCondition(deployment.Status, v1.DeploymentProgressing)
		if cond != nil && cond.Reason == "ProgressDeadlineExceeded" {
			return &appv1.HealthStatus{
				Status:        appv1.HealthStatusDegraded,
				StatusDetails: fmt.Sprintf("Deployment %q exceeded its progress deadline", obj.GetName()),
			}, nil
		} else if deployment.Spec.Replicas != nil && deployment.Status.UpdatedReplicas < *deployment.Spec.Replicas {
			return &appv1.HealthStatus{
				Status:        appv1.HealthStatusProgressing,
				StatusDetails: fmt.Sprintf("Waiting for rollout to finish: %d out of %d new replicas have been updated...\n", deployment.Status.UpdatedReplicas, *deployment.Spec.Replicas),
			}, nil
		} else if deployment.Status.Replicas > deployment.Status.UpdatedReplicas {
			return &appv1.HealthStatus{
				Status:        appv1.HealthStatusProgressing,
				StatusDetails: fmt.Sprintf("Waiting for rollout to finish: %d old replicas are pending termination...\n", deployment.Status.Replicas-deployment.Status.UpdatedReplicas),
			}, nil
		} else if deployment.Status.AvailableReplicas < deployment.Status.UpdatedReplicas {
			return &appv1.HealthStatus{
				Status:        appv1.HealthStatusProgressing,
				StatusDetails: fmt.Sprintf("Waiting for rollout to finish: %d of %d updated replicas are available...\n", deployment.Status.AvailableReplicas, deployment.Status.UpdatedReplicas),
			}, nil
		}
	} else {
		return &appv1.HealthStatus{
			Status:        appv1.HealthStatusProgressing,
			StatusDetails: "Waiting for rollout to finish: observed deployment generation less then desired generation",
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
