package cache

import (
	"fmt"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8snode "k8s.io/kubernetes/pkg/util/node"

	"github.com/argoproj/argo-cd/util/kube"
)

func getNodeInfo(un *unstructured.Unstructured) []v1alpha1.InfoItem {
	gvk := un.GroupVersionKind()

	if gvk.Kind == kube.PodKind && gvk.Group == "" {
		return getPodInfo(un)
	}
	return []v1alpha1.InfoItem{}
}

func getPodInfo(un *unstructured.Unstructured) []v1alpha1.InfoItem {
	pod := v1.Pod{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(un.Object, &pod)
	if err != nil {
		return []v1alpha1.InfoItem{}
	}
	restarts := 0
	totalContainers := len(pod.Spec.Containers)
	readyContainers := 0

	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}

	initializing := false
	for i := range pod.Status.InitContainerStatuses {
		container := pod.Status.InitContainerStatuses[i]
		restarts += int(container.RestartCount)
		switch {
		case container.State.Terminated != nil && container.State.Terminated.ExitCode == 0:
			continue
		case container.State.Terminated != nil:
			// initialization is failed
			if len(container.State.Terminated.Reason) == 0 {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Init:Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("Init:ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else {
				reason = "Init:" + container.State.Terminated.Reason
			}
			initializing = true
		case container.State.Waiting != nil && len(container.State.Waiting.Reason) > 0 && container.State.Waiting.Reason != "PodInitializing":
			reason = "Init:" + container.State.Waiting.Reason
			initializing = true
		default:
			reason = fmt.Sprintf("Init:%d/%d", i, len(pod.Spec.InitContainers))
			initializing = true
		}
		break
	}
	if !initializing {
		restarts = 0
		hasRunning := false
		for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
			container := pod.Status.ContainerStatuses[i]

			restarts += int(container.RestartCount)
			if container.State.Waiting != nil && container.State.Waiting.Reason != "" {
				reason = container.State.Waiting.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason != "" {
				reason = container.State.Terminated.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason == "" {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else if container.Ready && container.State.Running != nil {
				hasRunning = true
				readyContainers++
			}
		}

		// change pod status back to "Running" if there is at least one container still reporting as "Running" status
		if reason == "Completed" && hasRunning {
			reason = "Running"
		}
	}

	if pod.DeletionTimestamp != nil && pod.Status.Reason == k8snode.NodeUnreachablePodReason {
		reason = "Unknown"
	} else if pod.DeletionTimestamp != nil {
		reason = "Terminating"
	}

	info := make([]v1alpha1.InfoItem, 0)
	if reason != "" {
		info = append(info, v1alpha1.InfoItem{Name: "Status Reason", Value: reason})
	}
	return append(info, v1alpha1.InfoItem{Name: "Containers", Value: fmt.Sprintf("%d/%d", readyContainers, totalContainers)})
}
