package health

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func getIngressHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	ingresses, _, _ := unstructured.NestedSlice(obj.Object, "status", "loadBalancer", "ingress")
	health := HealthStatus{}
	if len(ingresses) > 0 {
		health.Status = HealthStatusHealthy
	} else {
		health.Status = HealthStatusProgressing
	}
	return &health, nil
}
