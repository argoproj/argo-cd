package health

import (
	"fmt"

	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func getServiceHealth(obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	service := &coreV1.Service{}
	err := scheme.Scheme.Convert(obj, service, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, service, err)
	}
	health := appv1.HealthStatus{Status: appv1.HealthStatusHealthy}
	if service.Spec.Type == coreV1.ServiceTypeLoadBalancer {
		if len(service.Status.LoadBalancer.Ingress) > 0 {
			health.Status = appv1.HealthStatusHealthy
		} else {
			health.Status = appv1.HealthStatusProgressing
		}
	}
	return &health, nil
}
