package cache

import (
	"fmt"
	"strings"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/text"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	resourcehelper "k8s.io/kubectl/pkg/util/resource"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/resource"
)

func populateNodeInfo(un *unstructured.Unstructured, res *ResourceInfo) {
	gvk := un.GroupVersionKind()
	revision := resource.GetRevision(un)
	if revision > 0 {
		res.Info = append(res.Info, v1alpha1.InfoItem{Name: "Revision", Value: fmt.Sprintf("Rev:%v", revision)})
	}
	switch gvk.Group {
	case "":
		switch gvk.Kind {
		case kube.PodKind:
			populatePodInfo(un, res)
			return
		case kube.ServiceKind:
			populateServiceInfo(un, res)
			return
		case "Node":
			populateHostNodeInfo(un, res)
			return
		}
	case "extensions", "networking.k8s.io":
		switch gvk.Kind {
		case kube.IngressKind:
			populateIngressInfo(un, res)
			return
		}
	case "networking.istio.io":
		switch gvk.Kind {
		case "VirtualService":
			populateIstioVirtualServiceInfo(un, res)
			return
		}
	}

	for k, v := range un.GetAnnotations() {
		if strings.HasPrefix(k, common.AnnotationKeyLinkPrefix) {
			if res.NetworkingInfo == nil {
				res.NetworkingInfo = &v1alpha1.ResourceNetworkingInfo{}
			}
			res.NetworkingInfo.ExternalURLs = append(res.NetworkingInfo.ExternalURLs, v)
		}
	}
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

func populateServiceInfo(un *unstructured.Unstructured, res *ResourceInfo) {
	targetLabels, _, _ := unstructured.NestedStringMap(un.Object, "spec", "selector")
	ingress := make([]v1.LoadBalancerIngress, 0)
	if serviceType, ok, err := unstructured.NestedString(un.Object, "spec", "type"); ok && err == nil && serviceType == string(v1.ServiceTypeLoadBalancer) {
		ingress = getIngress(un)
	}
	res.NetworkingInfo = &v1alpha1.ResourceNetworkingInfo{TargetLabels: targetLabels, Ingress: ingress}
}

func populateIngressInfo(un *unstructured.Unstructured, res *ResourceInfo) {
	ingress := getIngress(un)
	targetsMap := make(map[v1alpha1.ResourceRef]bool)
	if backend, ok, err := unstructured.NestedMap(un.Object, "spec", "backend"); ok && err == nil {
		targetsMap[v1alpha1.ResourceRef{
			Group:     "",
			Kind:      kube.ServiceKind,
			Namespace: un.GetNamespace(),
			Name:      fmt.Sprintf("%s", backend["serviceName"]),
		}] = true
	}
	urlsSet := make(map[string]bool)
	if rules, ok, err := unstructured.NestedSlice(un.Object, "spec", "rules"); ok && err == nil {
		for i := range rules {
			rule, ok := rules[i].(map[string]interface{})
			if !ok {
				continue
			}
			host := rule["host"]
			if host == nil || host == "" {
				for i := range ingress {
					host = text.FirstNonEmpty(ingress[i].Hostname, ingress[i].IP)
					if host != "" {
						break
					}
				}
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
					targetsMap[v1alpha1.ResourceRef{
						Group:     "",
						Kind:      kube.ServiceKind,
						Namespace: un.GetNamespace(),
						Name:      serviceName,
					}] = true
				}

				if host == nil || host == "" {
					continue
				}
				stringPort := "http"
				if tls, ok, err := unstructured.NestedSlice(un.Object, "spec", "tls"); ok && err == nil {
					for i := range tls {
						tlsline, ok := tls[i].(map[string]interface{})
						secretName := tlsline["secretName"]
						if ok && secretName != nil {
							stringPort = "https"
						}
						tlshost := tlsline["host"]
						if tlshost == host {
							stringPort = "https"
						}
					}
				}

				externalURL := fmt.Sprintf("%s://%s", stringPort, host)

				subPath := ""
				if nestedPath, ok, err := unstructured.NestedString(path, "path"); ok && err == nil {
					subPath = strings.TrimSuffix(nestedPath, "*")
				}
				externalURL += subPath
				urlsSet[externalURL] = true
			}
		}
	}
	targets := make([]v1alpha1.ResourceRef, 0)
	for target := range targetsMap {
		targets = append(targets, target)
	}

	var urls []string
	if res.NetworkingInfo != nil {
		urls = res.NetworkingInfo.ExternalURLs
	}
	for url := range urlsSet {
		urls = append(urls, url)
	}
	res.NetworkingInfo = &v1alpha1.ResourceNetworkingInfo{TargetRefs: targets, Ingress: ingress, ExternalURLs: urls}
}

func populateIstioVirtualServiceInfo(un *unstructured.Unstructured, res *ResourceInfo) {
	targetsMap := make(map[v1alpha1.ResourceRef]bool)

	if rules, ok, err := unstructured.NestedSlice(un.Object, "spec", "http"); ok && err == nil {
		for i := range rules {
			rule, ok := rules[i].(map[string]interface{})
			if !ok {
				continue
			}
			routes, ok, err := unstructured.NestedSlice(rule, "route")
			if !ok || err != nil {
				continue
			}
			for i := range routes {
				route, ok := routes[i].(map[string]interface{})
				if !ok {
					continue
				}

				if hostName, ok, err := unstructured.NestedString(route, "destination", "host"); ok && err == nil {
					hostSplits := strings.Split(hostName, ".")
					serviceName := hostSplits[0]

					var namespace string
					if len(hostSplits) >= 2 {
						namespace = hostSplits[1]
					} else {
						namespace = un.GetNamespace()
					}

					targetsMap[v1alpha1.ResourceRef{
						Kind:      kube.ServiceKind,
						Name:      serviceName,
						Namespace: namespace,
					}] = true
				}
			}
		}
	}
	targets := make([]v1alpha1.ResourceRef, 0)
	for target := range targetsMap {
		targets = append(targets, target)
	}

	res.NetworkingInfo = &v1alpha1.ResourceNetworkingInfo{TargetRefs: targets}
}

func populatePodInfo(un *unstructured.Unstructured, res *ResourceInfo) {
	pod := v1.Pod{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(un.Object, &pod)
	if err != nil {
		return
	}
	restarts := 0
	totalContainers := len(pod.Spec.Containers)
	readyContainers := 0

	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}

	imagesSet := make(map[string]bool)
	for _, container := range pod.Spec.InitContainers {
		imagesSet[container.Image] = true
	}
	for _, container := range pod.Spec.Containers {
		imagesSet[container.Image] = true
	}

	res.Images = nil
	for image := range imagesSet {
		res.Images = append(res.Images, image)
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

	// "NodeLost" = https://github.com/kubernetes/kubernetes/blob/cb8ad64243d48d9a3c26b11b2e0945c098457282/pkg/util/node/node.go#L46
	// But depending on the k8s.io/kubernetes package just for a constant
	// is not worth it.
	// See https://github.com/argoproj/argo-cd/issues/5173
	// and https://github.com/kubernetes/kubernetes/issues/90358#issuecomment-617859364
	if pod.DeletionTimestamp != nil && pod.Status.Reason == "NodeLost" {
		reason = "Unknown"
	} else if pod.DeletionTimestamp != nil {
		reason = "Terminating"
	}

	if reason != "" {
		res.Info = append(res.Info, v1alpha1.InfoItem{Name: "Status Reason", Value: reason})
	}

	req, _ := resourcehelper.PodRequestsAndLimits(&pod)
	res.PodInfo = &PodInfo{NodeName: pod.Spec.NodeName, ResourceRequests: req}

	res.Info = append(res.Info, v1alpha1.InfoItem{Name: "Node", Value: pod.Spec.NodeName})
	res.Info = append(res.Info, v1alpha1.InfoItem{Name: "Containers", Value: fmt.Sprintf("%d/%d", readyContainers, totalContainers)})
	if restarts > 0 {
		res.Info = append(res.Info, v1alpha1.InfoItem{Name: "Restart Count", Value: fmt.Sprintf("%d", restarts)})
	}
	res.NetworkingInfo = &v1alpha1.ResourceNetworkingInfo{Labels: un.GetLabels()}
}

func populateHostNodeInfo(un *unstructured.Unstructured, res *ResourceInfo) {
	node := v1.Node{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(un.Object, &node)
	if err != nil {
		return
	}
	res.NodeInfo = &NodeInfo{
		Name:       node.Name,
		Capacity:   node.Status.Capacity,
		SystemInfo: node.Status.NodeInfo,
	}
}
