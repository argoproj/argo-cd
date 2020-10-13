package health

import (
	"fmt"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func getIngressHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	gvk := obj.GroupVersionKind()
	switch gvk {
	case networkingv1.SchemeGroupVersion.WithKind(kube.IngressKind):
		var ingress networkingv1.Ingress
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &ingress)
		if err != nil {
			return nil, fmt.Errorf("failed to convert unstructured Ingress to typed: %v", err)
		}
		return getNetworkingv1IngressHealth(&ingress)
	case networkingv1beta1.SchemeGroupVersion.WithKind(kube.IngressKind):
		var ingress networkingv1beta1.Ingress
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &ingress)
		if err != nil {
			return nil, fmt.Errorf("failed to convert unstructured Ingress to typed: %v", err)
		}
		return getNetworkingv1beta1IngressHealth(&ingress)
	case extv1beta1.SchemeGroupVersion.WithKind(kube.IngressKind):
		var ingress extv1beta1.Ingress
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &ingress)
		if err != nil {
			return nil, fmt.Errorf("failed to convert unstructured Ingress to typed: %v", err)
		}
		return getExtv1beta1IngressHealth(&ingress)
	default:
		return nil, fmt.Errorf("unsupported Ingress GVK: %s", gvk)
	}
}

func getNetworkingv1IngressHealth(ingress *networkingv1.Ingress) (*HealthStatus, error) {
	health := HealthStatus{}
	if len(ingress.Status.LoadBalancer.Ingress) > 0 {
		health.Status = HealthStatusHealthy
	} else {
		health.Status = HealthStatusProgressing
	}
	return &health, nil
}

func getNetworkingv1beta1IngressHealth(ingress *networkingv1beta1.Ingress) (*HealthStatus, error) {
	health := HealthStatus{}
	if len(ingress.Status.LoadBalancer.Ingress) > 0 {
		health.Status = HealthStatusHealthy
	} else {
		health.Status = HealthStatusProgressing
	}
	return &health, nil
}

func getExtv1beta1IngressHealth(ingress *extv1beta1.Ingress) (*HealthStatus, error) {
	health := HealthStatus{}
	if len(ingress.Status.LoadBalancer.Ingress) > 0 {
		health.Status = HealthStatusHealthy
	} else {
		health.Status = HealthStatusProgressing
	}
	return &health, nil
}
