package health

import (
	"fmt"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func getServiceHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	gvk := obj.GroupVersionKind()
	switch gvk {
	case corev1.SchemeGroupVersion.WithKind(kube.ServiceKind):
		var service corev1.Service
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &service)
		if err != nil {
			return nil, fmt.Errorf("failed to convert unstructured Service to typed: %v", err)
		}
		return getCorev1ServiceHealth(&service)
	default:
		return nil, fmt.Errorf("unsupported Service GVK: %s", gvk)
	}
}

func getCorev1ServiceHealth(service *corev1.Service) (*HealthStatus, error) {
	health := HealthStatus{Status: HealthStatusHealthy}
	if service.Spec.Type == corev1.ServiceTypeLoadBalancer {
		if len(service.Status.LoadBalancer.Ingress) > 0 {
			health.Status = HealthStatusHealthy
		} else {
			health.Status = HealthStatusProgressing
		}
	}
	return &health, nil
}
