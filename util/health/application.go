package health

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func getApplicationHealth(obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	application := &appv1.Application{}
	err := scheme.Scheme.Convert(obj, application, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, application, err)
	}

	return &application.Status.Health, nil
}
