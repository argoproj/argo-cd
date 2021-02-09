package cache

import (
	"sort"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/pkg/errors"
	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
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
    tls:
    - host: helm-guestbook.com
    secretName: my-tls-secret
  status:
    loadBalancer:
      ingress:
      - ip: 107.178.210.11`)

	testIngressWildCardPath = strToUnstructured(`
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
          path: /*
        - backend:
            serviceName: helm-guestbook
            servicePort: https
          path: /*
    tls:
    - host: helm-guestbook.com
    secretName: my-tls-secret
  status:
    loadBalancer:
      ingress:
      - ip: 107.178.210.11`)

	testIngressWithoutTls = strToUnstructured(`
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

	testIstioVirtualService = strToUnstructured(`
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: hello-world
  namespace: demo
spec:
  http:
    - match:
        - uri:
            prefix: "/1"
      route:
        - destination:
            host: service_full.demo.svc.cluster.local
        - destination:
            host: service_namespace.namespace
    - match:
        - uri:
            prefix: "/2"
      route:
        - destination:
            host: service
`)
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
    nodeName: minikube
    containers:
    - image: bar
      resources:
        requests:
          memory: 128Mi
`)

	info := &ResourceInfo{}
	populateNodeInfo(pod, info)
	assert.Equal(t, []v1alpha1.InfoItem{
		{Name: "Node", Value: "minikube"},
		{Name: "Containers", Value: "0/1"},
	}, info.Info)
	assert.Equal(t, []string{"bar"}, info.Images)
	assert.Equal(t, &PodInfo{
		NodeName:         "minikube",
		ResourceRequests: v1.ResourceList{v1.ResourceMemory: resource.MustParse("128Mi")},
	}, info.PodInfo)
	assert.Equal(t, &v1alpha1.ResourceNetworkingInfo{Labels: map[string]string{"app": "guestbook"}}, info.NetworkingInfo)
}

func TestGetNodeInfo(t *testing.T) {
	node := strToUnstructured(`
apiVersion: v1
kind: Node
metadata:
  name: minikube
spec: {}
status:
  capacity:
    cpu: "6"
    memory: 6091320Ki
  nodeInfo:
    architecture: amd64
    operatingSystem: linux
    osImage: Ubuntu 20.04 LTS
`)

	info := &ResourceInfo{}
	populateNodeInfo(node, info)
	assert.Equal(t, &NodeInfo{
		Name:       "minikube",
		Capacity:   v1.ResourceList{v1.ResourceMemory: resource.MustParse("6091320Ki"), v1.ResourceCPU: resource.MustParse("6")},
		SystemInfo: v1.NodeSystemInfo{Architecture: "amd64", OperatingSystem: "linux", OSImage: "Ubuntu 20.04 LTS"},
	}, info.NodeInfo)
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

func TestGetIstioVirtualServiceInfo(t *testing.T) {
	info := &ResourceInfo{}
	populateNodeInfo(testIstioVirtualService, info)
	assert.Equal(t, 0, len(info.Info))
	require.NotNil(t, info.NetworkingInfo)
	require.NotNil(t, info.NetworkingInfo.TargetRefs)
	assert.Contains(t, info.NetworkingInfo.TargetRefs, v1alpha1.ResourceRef{
		Kind:      kube.ServiceKind,
		Name:      "service_full",
		Namespace: "demo",
	})
	assert.Contains(t, info.NetworkingInfo.TargetRefs, v1alpha1.ResourceRef{
		Kind:      kube.ServiceKind,
		Name:      "service_namespace",
		Namespace: "namespace",
	})
	assert.Contains(t, info.NetworkingInfo.TargetRefs, v1alpha1.ResourceRef{
		Kind:      kube.ServiceKind,
		Name:      "service",
		Namespace: "demo",
	})
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

func TestGetIngressInfoWildCardPath(t *testing.T) {
	info := &ResourceInfo{}
	populateNodeInfo(testIngressWildCardPath, info)
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

func TestGetIngressInfoWithoutTls(t *testing.T) {
	info := &ResourceInfo{}
	populateNodeInfo(testIngressWithoutTls, info)
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
		ExternalURLs: []string{"http://helm-guestbook.com/"},
	}, info.NetworkingInfo)
}

func TestGetIngressInfoWithHost(t *testing.T) {
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
    tls:
    - secretName: my-tls
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
    tls:
    - secretName: my-tls
      `)

	info := &ResourceInfo{}
	populateNodeInfo(ingress, info)

	assert.Equal(t, &v1alpha1.ResourceNetworkingInfo{
		TargetRefs: []v1alpha1.ResourceRef{{
			Namespace: "default",
			Group:     "",
			Kind:      kube.ServiceKind,
			Name:      "helm-guestbook",
		}},
	}, info.NetworkingInfo)
	assert.Equal(t, len(info.NetworkingInfo.ExternalURLs), 0)
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
    tls:
    - secretName: my-tls
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
    tls:
    - secretName: my-tls
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
    tls:
    - secretName: my-tls
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
    tls:
    - secretName: my-tls
  status:
    loadBalancer:
      ingress:
      - ip: 107.178.210.11`)

	info := &ResourceInfo{}
	populateNodeInfo(ingress, info)

	expectedExternalUrls := []string{"https://107.178.210.11"}
	assert.Equal(t, expectedExternalUrls, info.NetworkingInfo.ExternalURLs)
}
