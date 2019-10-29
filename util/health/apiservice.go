package health

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func getAPIServiceHealth(obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	apiservice := &apiregistrationv1.APIService{}
	err := scheme.Scheme.Convert(obj, apiservice, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, apiservice, err)
	}

	for _, c := range apiservice.Status.Conditions {
		switch c.Type {
		case apiregistrationv1.Available:
			if c.Status == apiregistrationv1.ConditionTrue {
				return &appv1.HealthStatus{
					Status:  appv1.HealthStatusHealthy,
					Message: fmt.Sprintf("%s: %s", c.Reason, c.Message),
				}, nil
			} else {
				return &appv1.HealthStatus{
					Status:  appv1.HealthStatusProgressing,
					Message: fmt.Sprintf("%s: %s", c.Reason, c.Message),
				}, nil
			}
		}
	}
	return &appv1.HealthStatus{
		Status:  appv1.HealthStatusProgressing,
		Message: "Waiting to be processed",
	}, nil
}
