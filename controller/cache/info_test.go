package cache

import (
	"sort"
	"strings"
	"testing"

	"github.com/argoproj/pkg/errors"
	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "k8s.io/api/core/v1"

	"github.com/argoproj/argo-cd/engine/pkg/utils/kube"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"

	"github.com/stretchr/testify/assert"
)

func strToUnstructured(jsonStr string) *unstructured.Unstructured {
	obj := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(jsonStr), &obj)
	errors.CheckError(err)
	return &unstructured.Unstructured{Object: obj}
}

var (
	testService = strToUnstructured(`
  apiVersion: v1
  kind: Service
  metadata:
    name: helm-guestbook
    namespace: default
    resourceVersion: "123"
    uid: "4"
  spec:
    selector:
      app: guestbook
    type: LoadBalancer
  status:
    loadBalancer:
      ingress:
      - hostname: localhost`)

	testIngress = strToUnstructured(`
  apiVersion: extensions/v1beta1
  kind: Ingress
  metadata:
    name: helm-guestbook
    namespace: default
    uid: "4"
  spec:
    backend:
      serviceName: not-found-service
      servicePort: 443
    rules:
    - host: helm-guestbook.com
      http:
        paths:
        - backend:
            serviceName: helm-guestbook
            servicePort: 443
          path: /
        - backend:
            serviceName: helm-guestbook
            servicePort: https
          path: /
  status:
    loadBalancer:
      ingress:
      - ip: 107.178.210.11`)
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

	info := &ResourceInfo{}
	populateNodeInfo(pod, info)
	assert.Equal(t, []v1alpha1.InfoItem{{Name: "Containers", Value: "0/1"}}, info.Info)
	assert.Equal(t, []string{"bar"}, info.Images)
	assert.Equal(t, &v1alpha1.ResourceNetworkingInfo{Labels: map[string]string{"app": "guestbook"}}, info.NetworkingInfo)
}

func TestGetServiceInfo(t *testing.T) {
	info := &ResourceInfo{}
	populateNodeInfo(testService, info)
	assert.Equal(t, 0, len(info.Info))
	assert.Equal(t, &v1alpha1.ResourceNetworkingInfo{
		TargetLabels: map[string]string{"app": "guestbook"},
		Ingress:      []v1.LoadBalancerIngress{{Hostname: "localhost"}},
	}, info.NetworkingInfo)
}

func TestGetIngressInfo(t *testing.T) {
	info := &ResourceInfo{}
	populateNodeInfo(testIngress, info)
	assert.Equal(t, 0, len(info.Info))
	sort.Slice(info.NetworkingInfo.TargetRefs, func(i, j int) bool {
		return strings.Compare(info.NetworkingInfo.TargetRefs[j].Name, info.NetworkingInfo.TargetRefs[i].Name) < 0
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
	}, info.NetworkingInfo)
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

	info := &ResourceInfo{}
	populateNodeInfo(ingress, info)

	assert.Equal(t, &v1alpha1.ResourceNetworkingInfo{
		Ingress: []v1.LoadBalancerIngress{{IP: "107.178.210.11"}},
		TargetRefs: []v1alpha1.ResourceRef{{
			Namespace: "default",
			Group:     "",
			Kind:      kube.ServiceKind,
			Name:      "helm-guestbook",
		}},
		ExternalURLs: []string{"https://107.178.210.11/"},
	}, info.NetworkingInfo)
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

	info := &ResourceInfo{}
	populateNodeInfo(ingress, info)

	expectedExternalUrls := []string{"https://107.178.210.11/my/sub/path/"}
	assert.Equal(t, expectedExternalUrls, info.NetworkingInfo.ExternalURLs)
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

	info := &ResourceInfo{}
	populateNodeInfo(ingress, info)

	expectedExternalUrls := []string{"https://helm-guestbook.com/my/sub/path/", "https://helm-guestbook.com/my/sub/path/2", "https://helm-guestbook.com"}
	actualURLs := info.NetworkingInfo.ExternalURLs
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

	info := &ResourceInfo{}
	populateNodeInfo(ingress, info)

	expectedExternalUrls := []string{"https://107.178.210.11"}
	assert.Equal(t, expectedExternalUrls, info.NetworkingInfo.ExternalURLs)
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

	info := &ResourceInfo{}
	populateNodeInfo(ingress, info)

	expectedExternalUrls := []string{"https://107.178.210.11"}
	assert.Equal(t, expectedExternalUrls, info.NetworkingInfo.ExternalURLs)
}
