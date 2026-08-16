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
	var status HealthStatusCode
	var msg string
	switch pvc.Status.Phase {
	case corev1.ClaimLost:
		status = HealthStatusDegraded
	case corev1.ClaimPending:
		// PVCs with a storage-provisioner annotation are actively being provisioned.
		if pvc.Annotations != nil {
			if _, ok := pvc.Annotations["volume.beta.kubernetes.io/storage-provisioner"]; ok {
				status = HealthStatusProgressing
				msg = "Provisioning volume"
			} else {
				// PVC is pending but not actively provisioning — it may be
				// waiting for a consumer (WaitForFirstConsumer) or similar.
				status = HealthStatusHealthy
				msg = "Waiting for consumer"
			}
		} else {
			status = HealthStatusHealthy
			msg = "Waiting for consumer"
		}
	case corev1.ClaimBound:
		status = HealthStatusHealthy
	default:
		status = HealthStatusUnknown
	}
	return &HealthStatus{Status: status, Message: msg}, nil
}
