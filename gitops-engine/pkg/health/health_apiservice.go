package health

import (
	"fmt"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
)

func getAPIServiceHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	gvk := obj.GroupVersionKind()
	switch gvk {
	case apiregistrationv1.SchemeGroupVersion.WithKind(kube.APIServiceKind):
		var apiService apiregistrationv1.APIService
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &apiService)
		if err != nil {
			return nil, fmt.Errorf("failed to convert unstructured APIService to typed: %v", err)
		}
		return getApiregistrationv1APIServiceHealth(&apiService)
	case apiregistrationv1beta1.SchemeGroupVersion.WithKind(kube.APIServiceKind):
		var apiService apiregistrationv1beta1.APIService
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &apiService)
		if err != nil {
			return nil, fmt.Errorf("failed to convert unstructured APIService to typed: %v", err)
		}
		return getApiregistrationv1beta1APIServiceHealth(&apiService)
	default:
		return nil, fmt.Errorf("unsupported APIService GVK: %s", gvk)
	}
}

func getApiregistrationv1APIServiceHealth(apiservice *apiregistrationv1.APIService) (*HealthStatus, error) {
	for _, c := range apiservice.Status.Conditions {
		switch c.Type {
		case apiregistrationv1.Available:
			if c.Status == apiregistrationv1.ConditionTrue {
				return &HealthStatus{
					Status:  HealthStatusHealthy,
					Message: fmt.Sprintf("%s: %s", c.Reason, c.Message),
				}, nil
			} else {
				return &HealthStatus{
					Status:  HealthStatusProgressing,
					Message: fmt.Sprintf("%s: %s", c.Reason, c.Message),
				}, nil
			}
		}
	}
	return &HealthStatus{
		Status:  HealthStatusProgressing,
		Message: "Waiting to be processed",
	}, nil
}

func getApiregistrationv1beta1APIServiceHealth(apiservice *apiregistrationv1beta1.APIService) (*HealthStatus, error) {
	for _, c := range apiservice.Status.Conditions {
		switch c.Type {
		case apiregistrationv1beta1.Available:
			if c.Status == apiregistrationv1beta1.ConditionTrue {
				return &HealthStatus{
					Status:  HealthStatusHealthy,
					Message: fmt.Sprintf("%s: %s", c.Reason, c.Message),
				}, nil
			} else {
				return &HealthStatus{
					Status:  HealthStatusProgressing,
					Message: fmt.Sprintf("%s: %s", c.Reason, c.Message),
				}, nil
			}
		}
	}
	return &HealthStatus{
		Status:  HealthStatusProgressing,
		Message: "Waiting to be processed",
	}, nil
}
