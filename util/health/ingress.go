package health

import (
	"fmt"

	extv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func getIngressHealth(obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	ingress := &extv1beta1.Ingress{}
	err := scheme.Scheme.Convert(obj, ingress, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, ingress, err)
	}
	health := appv1.HealthStatus{}
	if len(ingress.Status.LoadBalancer.Ingress) > 0 {
		health.Status = appv1.HealthStatusHealthy
	} else {
		health.Status = appv1.HealthStatusProgressing
	}
	return &health, nil
}
