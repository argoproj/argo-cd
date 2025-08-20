package health

import (
	"fmt"
	"strings"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubectl/pkg/util/podutils"
)

func getPodHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	gvk := obj.GroupVersionKind()
	switch gvk {
	case corev1.SchemeGroupVersion.WithKind(kube.PodKind):
		var pod corev1.Pod
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &pod)
		if err != nil {
			return nil, fmt.Errorf("failed to convert unstructured Pod to typed: %v", err)
		}
		return getCorev1PodHealth(&pod)
	default:
		return nil, fmt.Errorf("unsupported Pod GVK: %s", gvk)
	}
}

func getCorev1PodHealth(pod *corev1.Pod) (*HealthStatus, error) {
	// This logic cannot be applied when the pod.Spec.RestartPolicy is: corev1.RestartPolicyOnFailure,
	// corev1.RestartPolicyNever, otherwise it breaks the resource hook logic.
	// The issue is, if we mark a pod with ImagePullBackOff as Degraded, and the pod is used as a resource hook,
	// then we will prematurely fail the PreSync/PostSync hook. Meanwhile, when that error condition is resolved
	// (e.g. the image is available), the resource hook pod will unexpectedly be executed even though the sync has
	// completed.
	if pod.Spec.RestartPolicy == corev1.RestartPolicyAlways {
		var status HealthStatusCode
		var messages []string

		for _, containerStatus := range pod.Status.ContainerStatuses {
			waiting := containerStatus.State.Waiting
			// Article listing common container errors: https://medium.com/kokster/debugging-crashloopbackoffs-with-init-containers-26f79e9fb5bf
			if waiting != nil && (strings.HasPrefix(waiting.Reason, "Err") || strings.HasSuffix(waiting.Reason, "Error") || strings.HasSuffix(waiting.Reason, "BackOff")) {
				status = HealthStatusDegraded
				messages = append(messages, waiting.Message)
			}
		}

		if status != "" {
			return &HealthStatus{
				Status:  status,
				Message: strings.Join(messages, ", "),
			}, nil
		}
	}

	getFailMessage := func(ctr *corev1.ContainerStatus) string {
		if ctr.State.Terminated != nil {
			if ctr.State.Terminated.Message != "" {
				return ctr.State.Terminated.Message
			}
			if ctr.State.Terminated.Reason == "OOMKilled" {
				return ctr.State.Terminated.Reason
			}
			if ctr.State.Terminated.ExitCode != 0 {
				return fmt.Sprintf("container %q failed with exit code %d", ctr.Name, ctr.State.Terminated.ExitCode)
			}
		}
		return ""
	}

	switch pod.Status.Phase {
	case corev1.PodPending:
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: pod.Status.Message,
		}, nil
	case corev1.PodSucceeded:
		return &HealthStatus{
			Status:  HealthStatusHealthy,
			Message: pod.Status.Message,
		}, nil
	case corev1.PodFailed:
		if pod.Status.Message != "" {
			// Pod has a nice error message. Use that.
			return &HealthStatus{Status: HealthStatusDegraded, Message: pod.Status.Message}, nil
		}
		for _, ctr := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
			if msg := getFailMessage(&ctr); msg != "" {
				return &HealthStatus{Status: HealthStatusDegraded, Message: msg}, nil
			}
		}

		return &HealthStatus{Status: HealthStatusDegraded, Message: ""}, nil
	case corev1.PodRunning:
		switch pod.Spec.RestartPolicy {
		case corev1.RestartPolicyAlways:
			// if pod is ready, it is automatically healthy
			if podutils.IsPodReady(pod) {
				return &HealthStatus{
					Status:  HealthStatusHealthy,
					Message: pod.Status.Message,
				}, nil
			}
			// if it's not ready, check to see if any container terminated, if so, it's degraded
			for _, ctrStatus := range pod.Status.ContainerStatuses {
				if ctrStatus.LastTerminationState.Terminated != nil {
					return &HealthStatus{
						Status:  HealthStatusDegraded,
						Message: pod.Status.Message,
					}, nil
				}
			}
			// otherwise we are progressing towards a ready state
			return &HealthStatus{
				Status:  HealthStatusProgressing,
				Message: pod.Status.Message,
			}, nil
		case corev1.RestartPolicyOnFailure, corev1.RestartPolicyNever:
			// pods set with a restart policy of OnFailure or Never, have a finite life.
			// These pods are typically resource hooks. Thus, we consider these as Progressing
			// instead of healthy.
			return &HealthStatus{
				Status:  HealthStatusProgressing,
				Message: pod.Status.Message,
			}, nil
		}
	}
	return &HealthStatus{
		Status:  HealthStatusUnknown,
		Message: pod.Status.Message,
	}, nil
}
