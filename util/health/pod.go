package health

import (
	"fmt"
	"strings"

	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func getPodHealth(obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	pod := &coreV1.Pod{}
	err := scheme.Scheme.Convert(obj, pod, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, pod, err)
	}

	// This logic cannot be applied when the pod.Spec.RestartPolicy is: coreV1.RestartPolicyOnFailure,
	// coreV1.RestartPolicyNever, otherwise it breaks the resource hook logic.
	// The issue is, if we mark a pod with ImagePullBackOff as Degraded, and the pod is used as a resource hook,
	// then we will prematurely fail the PreSync/PostSync hook. Meanwhile, when that error condition is resolved
	// (e.g. the image is available), the resource hook pod will unexpectedly be executed even though the sync has
	// completed.
	if pod.Spec.RestartPolicy == coreV1.RestartPolicyAlways {
		var status appv1.HealthStatusCode
		var messages []string

		for _, containerStatus := range pod.Status.ContainerStatuses {
			waiting := containerStatus.State.Waiting
			// Article listing common container errors: https://medium.com/kokster/debugging-crashloopbackoffs-with-init-containers-26f79e9fb5bf
			if waiting != nil && (strings.HasPrefix(waiting.Reason, "Err") || strings.HasSuffix(waiting.Reason, "Error") || strings.HasSuffix(waiting.Reason, "BackOff")) {
				status = appv1.HealthStatusDegraded
				messages = append(messages, waiting.Message)
			}
		}

		if status != "" {
			return &appv1.HealthStatus{
				Status:  status,
				Message: strings.Join(messages, ", "),
			}, nil
		}
	}

	switch pod.Status.Phase {
	case coreV1.PodPending:
		return &appv1.HealthStatus{
			Status:  appv1.HealthStatusProgressing,
			Message: pod.Status.Message,
		}, nil
	case coreV1.PodSucceeded:
		return &appv1.HealthStatus{
			Status:  appv1.HealthStatusHealthy,
			Message: pod.Status.Message,
		}, nil
	case coreV1.PodFailed:
		return &appv1.HealthStatus{
			Status:  appv1.HealthStatusDegraded,
			Message: pod.Status.Message,
		}, nil
	case coreV1.PodRunning:
		switch pod.Spec.RestartPolicy {
		case coreV1.RestartPolicyAlways:
			// if pod is ready, it is automatically healthy
			if podutil.IsPodReady(pod) {
				return &appv1.HealthStatus{
					Status:  appv1.HealthStatusHealthy,
					Message: pod.Status.Message,
				}, nil
			}
			// if it's not ready, check to see if any container terminated, if so, it's degraded
			for _, ctrStatus := range pod.Status.ContainerStatuses {
				if ctrStatus.LastTerminationState.Terminated != nil {
					return &appv1.HealthStatus{
						Status:  appv1.HealthStatusDegraded,
						Message: pod.Status.Message,
					}, nil
				}
			}
			// otherwise we are progressing towards a ready state
			return &appv1.HealthStatus{
				Status:  appv1.HealthStatusProgressing,
				Message: pod.Status.Message,
			}, nil
		case coreV1.RestartPolicyOnFailure, coreV1.RestartPolicyNever:
			// pods set with a restart policy of OnFailure or Never, have a finite life.
			// These pods are typically resource hooks. Thus, we consider these as Progressing
			// instead of healthy.
			return &appv1.HealthStatus{
				Status:  appv1.HealthStatusProgressing,
				Message: pod.Status.Message,
			}, nil
		}
	}
	return &appv1.HealthStatus{
		Status:  appv1.HealthStatusUnknown,
		Message: pod.Status.Message,
	}, nil
}
