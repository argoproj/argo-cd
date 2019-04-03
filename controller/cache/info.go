package cache

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8snode "k8s.io/kubernetes/pkg/util/node"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/kube"
)

func populateNodeInfo(un *unstructured.Unstructured, node *node) {
	gvk := un.GroupVersionKind()

	switch gvk.Group {
	case "":
		switch gvk.Kind {
		case kube.PodKind:
			populatePodInfo(un, node)
			return
		case kube.ServiceKind:
			populateServiceInfo(un, node)
			return
		}
	case "extensions":
		switch gvk.Kind {
		case kube.IngressKind:
			populateIngressInfo(un, node)
			return
		}
	}
	node.info = []v1alpha1.InfoItem{}
}

func getIngress(un *unstructured.Unstructured) []v1.LoadBalancerIngress {
	ingress, ok, err := unstructured.NestedSlice(un.Object, "status", "loadBalancer", "ingress")
	if !ok || err != nil {
		return nil
	}
	res := make([]v1.LoadBalancerIngress, 0)
	for _, item := range ingress {
		if lbIngress, ok := item.(map[string]interface{}); ok {
			if hostname := lbIngress["hostname"]; hostname != nil {
				res = append(res, v1.LoadBalancerIngress{Hostname: fmt.Sprintf("%s", hostname)})
			} else if ip := lbIngress["ip"]; ip != nil {
				res = append(res, v1.LoadBalancerIngress{IP: fmt.Sprintf("%s", ip)})
			}
		}
	}
	return res
}

func populateServiceInfo(un *unstructured.Unstructured, node *node) {
	targetLabels, _, _ := unstructured.NestedStringMap(un.Object, "spec", "selector")
	ingress := make([]v1.LoadBalancerIngress, 0)
	if serviceType, ok, err := unstructured.NestedString(un.Object, "spec", "type"); ok && err == nil && serviceType == string(v1.ServiceTypeLoadBalancer) {
		ingress = getIngress(un)
	}
	node.networkingInfo = &v1alpha1.ResourceNetworkingInfo{TargetLabels: targetLabels, Ingress: ingress}
}

func populateIngressInfo(un *unstructured.Unstructured, node *node) {
	targets := make([]v1alpha1.ResourceRef, 0)
	if backend, ok, err := unstructured.NestedMap(un.Object, "spec", "backend"); ok && err == nil {
		targets = append(targets, v1alpha1.ResourceRef{
			Group:     "",
			Kind:      kube.ServiceKind,
			Namespace: un.GetNamespace(),
			Name:      fmt.Sprintf("%s", backend["serviceName"]),
		})
	}
	if rules, ok, err := unstructured.NestedSlice(un.Object, "spec", "rules"); ok && err == nil {
		for i := range rules {
			rule, ok := rules[i].(map[string]interface{})
			if !ok {
				continue
			}
			paths, ok, err := unstructured.NestedSlice(rule, "http", "paths")
			if !ok || err != nil {
				continue
			}
			for i := range paths {
				path, ok := paths[i].(map[string]interface{})
				if !ok {
					continue
				}
				if serviceName, ok, err := unstructured.NestedString(path, "backend", "serviceName"); ok && err == nil {
					targets = append(targets, v1alpha1.ResourceRef{
						Group:     "",
						Kind:      kube.ServiceKind,
						Namespace: un.GetNamespace(),
						Name:      serviceName,
					})
				}
			}
		}
	}
	node.networkingInfo = &v1alpha1.ResourceNetworkingInfo{TargetRefs: targets, Ingress: getIngress(un)}
}

func populatePodInfo(un *unstructured.Unstructured, node *node) {
	pod := v1.Pod{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(un.Object, &pod)
	if err != nil {
		node.info = []v1alpha1.InfoItem{}
		return
	}
	restarts := 0
	totalContainers := len(pod.Spec.Containers)
	readyContainers := 0

	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}

	initializing := false

	// note that I ignore initContainers
	for _, container := range pod.Spec.Containers {
		node.images = append(node.images, container.Image)
	}

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

	node.info = make([]v1alpha1.InfoItem, 0)
	if reason != "" {
		node.info = append(node.info, v1alpha1.InfoItem{Name: "Status Reason", Value: reason})
	}
	node.health = createHealth(pod.Status, reason)
	node.info = append(node.info, v1alpha1.InfoItem{Name: "Containers", Value: fmt.Sprintf("%d/%d", readyContainers, totalContainers)})
	node.networkingInfo = &v1alpha1.ResourceNetworkingInfo{Labels: un.GetLabels()}
}

func createHealth(status v1.PodStatus, reason string) *v1alpha1.HealthStatus {

	healthStatus := &v1alpha1.HealthStatus{}
	switch reason {
	case "CrashLoopBackOff", "RunContainerError", "ErrImagePull", "ImagePullBackOff":
		healthStatus.Status = v1alpha1.HealthStatusDegraded
		healthStatus.Message = reason
	default:
		healthStatus.Status = map[v1.PodPhase]v1alpha1.HealthStatusCode{
			v1.PodPending:             v1alpha1.HealthStatusProgressing,
			v1.PodRunning:             v1alpha1.HealthStatusHealthy,
			v1.PodSucceeded:           v1alpha1.HealthStatusHealthy,
			v1.PodFailed:              v1alpha1.HealthStatusDegraded,
			v1.PodUnknown:             v1alpha1.HealthStatusUnknown,
			v1.PodReasonUnschedulable: v1alpha1.HealthStatusMissing,
		}[status.Phase]
		healthStatus.Message = status.Message
	}

	return healthStatus
}
