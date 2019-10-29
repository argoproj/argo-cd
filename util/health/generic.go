package health

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func getGenericHealth(obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	r := struct {
		Status struct {
			Status  string
			Message string
		}
	}{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &r)
	if err != nil {
		return nil, err
	}
	if r.Status.Status != "" {
		code, ok := map[string]appv1.HealthStatusCode{
			"Pending":      appv1.HealthStatusProgressing,
			"Running":      appv1.HealthStatusProgressing,
			"Successful":   appv1.HealthStatusHealthy,
			"Failed":       appv1.HealthStatusDegraded,
			"Error":        appv1.HealthStatusDegraded,
			"Inconclusive": appv1.HealthStatusSuspended,
		}[r.Status.Status]
		if ok {
			return &appv1.HealthStatus{Status: code, Message: r.Status.Message}, nil
		}
	}

	return nil, nil
}
