package cache

import (
	"sort"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/engine/util/kube"

	"github.com/stretchr/testify/assert"
)

func TestGetPodInfo(t *testing.T) {
	pod := strToUnstructured(`
  apiVersion: v1
  kind: Pod
  metadata:
    name: helm-guestbook-pod
    namespace: default
    ownerReferences:
    - apiVersion: extensions/v1beta1
      kind: ReplicaSet
      name: helm-guestbook-rs
    resourceVersion: "123"
    labels:
      app: guestbook
  spec:
    containers:
    - image: bar`)

	node := &node{}
	populateNodeInfo(pod, node)
	assert.Equal(t, []v1alpha1.InfoItem{{Name: "Containers", Value: "0/1"}}, node.info)
	assert.Equal(t, []string{"bar"}, node.images)
	assert.Equal(t, &v1alpha1.ResourceNetworkingInfo{Labels: map[string]string{"app": "guestbook"}}, node.networkingInfo)
}

func TestGetServiceInfo(t *testing.T) {
	node := &node{}
	populateNodeInfo(testService, node)
	assert.Equal(t, 0, len(node.info))
	assert.Equal(t, &v1alpha1.ResourceNetworkingInfo{
		TargetLabels: map[string]string{"app": "guestbook"},
		Ingress:      []v1.LoadBalancerIngress{{Hostname: "localhost"}},
	}, node.networkingInfo)
}

func TestGetIngressInfo(t *testing.T) {
	node := &node{}
	populateNodeInfo(testIngress, node)
	assert.Equal(t, 0, len(node.info))
	sort.Slice(node.networkingInfo.TargetRefs, func(i, j int) bool {
		return strings.Compare(node.networkingInfo.TargetRefs[j].Name, node.networkingInfo.TargetRefs[i].Name) < 0
	})
	assert.Equal(t, &v1alpha1.ResourceNetworkingInfo{
		Ingress: []v1.LoadBalancerIngress{{IP: "107.178.210.11"}},
		TargetRefs: []v1alpha1.ResourceRef{{
			Namespace: "default",
			Group:     "",
			Kind:      kube.ServiceKind,
			Name:      "not-found-service",
		}, {
			Namespace: "default",
			Group:     "",
			Kind:      kube.ServiceKind,
			Name:      "helm-guestbook",
		}},
		ExternalURLs: []string{"https://helm-guestbook.com/"},
	}, node.networkingInfo)
}

func TestGetIngressInfoNoHost(t *testing.T) {
	ingress := strToUnstructured(`
  apiVersion: extensions/v1beta1
  kind: Ingress
  metadata:
    name: helm-guestbook
    namespace: default
  spec:
    rules:
    - http:
        paths:
        - backend:
            serviceName: helm-guestbook
            servicePort: 443
          path: /
  status:
    loadBalancer:
      ingress:
      - ip: 107.178.210.11`)

	node := &node{}
	populateNodeInfo(ingress, node)

	assert.Equal(t, &v1alpha1.ResourceNetworkingInfo{
		Ingress: []v1.LoadBalancerIngress{{IP: "107.178.210.11"}},
		TargetRefs: []v1alpha1.ResourceRef{{
			Namespace: "default",
			Group:     "",
			Kind:      kube.ServiceKind,
			Name:      "helm-guestbook",
		}},
		ExternalURLs: []string{"https://107.178.210.11/"},
	}, node.networkingInfo)
}
func TestExternalUrlWithSubPath(t *testing.T) {
	ingress := strToUnstructured(`
  apiVersion: extensions/v1beta1
  kind: Ingress
  metadata:
    name: helm-guestbook
    namespace: default
  spec:
    rules:
    - http:
        paths:
        - backend:
            serviceName: helm-guestbook
            servicePort: 443
          path: /my/sub/path/
  status:
    loadBalancer:
      ingress:
      - ip: 107.178.210.11`)

	node := &node{}
	populateNodeInfo(ingress, node)

	expectedExternalUrls := []string{"https://107.178.210.11/my/sub/path/"}
	assert.Equal(t, expectedExternalUrls, node.networkingInfo.ExternalURLs)
}
func TestExternalUrlWithMultipleSubPaths(t *testing.T) {
	ingress := strToUnstructured(`
  apiVersion: extensions/v1beta1
  kind: Ingress
  metadata:
    name: helm-guestbook
    namespace: default
  spec:
    rules:
    - host: helm-guestbook.com
      http:
        paths:
        - backend:
            serviceName: helm-guestbook
            servicePort: 443
          path: /my/sub/path/
        - backend:
            serviceName: helm-guestbook-2
            servicePort: 443
          path: /my/sub/path/2
        - backend:
            serviceName: helm-guestbook-3
            servicePort: 443
  status:
    loadBalancer:
      ingress:
      - ip: 107.178.210.11`)

	node := &node{}
	populateNodeInfo(ingress, node)

	expectedExternalUrls := []string{"https://helm-guestbook.com/my/sub/path/", "https://helm-guestbook.com/my/sub/path/2", "https://helm-guestbook.com"}
	actualURLs := node.networkingInfo.ExternalURLs
	sort.Strings(expectedExternalUrls)
	sort.Strings(actualURLs)
	assert.Equal(t, expectedExternalUrls, actualURLs)
}
func TestExternalUrlWithNoSubPath(t *testing.T) {
	ingress := strToUnstructured(`
  apiVersion: extensions/v1beta1
  kind: Ingress
  metadata:
    name: helm-guestbook
    namespace: default
  spec:
    rules:
    - http:
        paths:
        - backend:
            serviceName: helm-guestbook
            servicePort: 443
  status:
    loadBalancer:
      ingress:
      - ip: 107.178.210.11`)

	node := &node{}
	populateNodeInfo(ingress, node)

	expectedExternalUrls := []string{"https://107.178.210.11"}
	assert.Equal(t, expectedExternalUrls, node.networkingInfo.ExternalURLs)
}

func TestExternalUrlWithNetworkingApi(t *testing.T) {
	ingress := strToUnstructured(`
  apiVersion: networking.k8s.io/v1beta1
  kind: Ingress
  metadata:
    name: helm-guestbook
    namespace: default
  spec:
    rules:
    - http:
        paths:
        - backend:
            serviceName: helm-guestbook
            servicePort: 443
  status:
    loadBalancer:
      ingress:
      - ip: 107.178.210.11`)

	node := &node{}
	populateNodeInfo(ingress, node)

	expectedExternalUrls := []string{"https://107.178.210.11"}
	assert.Equal(t, expectedExternalUrls, node.networkingInfo.ExternalURLs)
}
