package health

import (
	"fmt"

	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func getPVCHealth(obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	pvc := &coreV1.PersistentVolumeClaim{}
	err := scheme.Scheme.Convert(obj, pvc, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, pvc, err)
	}
	switch pvc.Status.Phase {
	case coreV1.ClaimLost:
		return &appv1.HealthStatus{Status: appv1.HealthStatusDegraded}, nil
	case coreV1.ClaimPending:
		return &appv1.HealthStatus{Status: appv1.HealthStatusProgressing}, nil
	case coreV1.ClaimBound:
		return &appv1.HealthStatus{Status: appv1.HealthStatusHealthy}, nil
	default:
		return &appv1.HealthStatus{Status: appv1.HealthStatusUnknown}, nil
	}
}
