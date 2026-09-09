package health

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
)

func getPVCHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	gvk := obj.GroupVersionKind()
	switch gvk {
	case corev1.SchemeGroupVersion.WithKind(kube.PersistentVolumeClaimKind):
		var pvc corev1.PersistentVolumeClaim
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &pvc)
		if err != nil {
			return nil, fmt.Errorf("failed to convert unstructured PersistentVolumeClaim to typed: %w", err)
		}
		return getCorev1PVCHealth(&pvc)
	default:
		return nil, fmt.Errorf("unsupported PersistentVolumeClaim GVK: %s", gvk)
	}
}

func getCorev1PVCHealth(pvc *corev1.PersistentVolumeClaim) (*HealthStatus, error) {
	switch pvc.Status.Phase {
	case corev1.ClaimLost:
		return &HealthStatus{Status: HealthStatusDegraded}, nil
	case corev1.ClaimPending:
		// A PVC with WaitForFirstConsumer binding mode stays Pending until a pod
		// is scheduled that consumes it. This is an expected steady state, not
		// progress toward a goal, so report Healthy instead of Progressing.
		for _, condition := range pvc.Status.Conditions {
			if condition.Reason == "WaitForFirstConsumer" {
				return &HealthStatus{
					Status:  HealthStatusHealthy,
					Message: "Waiting for first consumer",
				}, nil
			}
		}
		return &HealthStatus{Status: HealthStatusProgressing}, nil
	case corev1.ClaimBound:
		return &HealthStatus{Status: HealthStatusHealthy}, nil
	default:
		return &HealthStatus{Status: HealthStatusUnknown}, nil
	}
}
