package cache

import (
	"sort"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo/normalizers"
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

	testLinkAnnotatedService = strToUnstructured(`
  apiVersion: v1
  kind: Service
  metadata:
    name: helm-guestbook
    namespace: default
    resourceVersion: "123"
    uid: "4"
    annotations:
      link.argocd.argoproj.io/external-link: http://my-grafana.example.com/pre-generated-link
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
    - host: helm-guestbook.example.com
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
    - host: helm-guestbook.example.com
    secretName: my-tls-secret
  status:
    loadBalancer:
      ingress:
      - ip: 107.178.210.11`)

	testLinkAnnotatedIngress = strToUnstructured(`
  apiVersion: extensions/v1beta1
  kind: Ingress
  metadata:
    name: helm-guestbook
    namespace: default
    uid: "4"
    annotations:
      link.argocd.argoproj.io/external-link: http://my-grafana.example.com/ingress-link
  spec:
    backend:
      serviceName: not-found-service
      servicePort: 443
    rules:
    - host: helm-guestbook.example.com
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
    - host: helm-guestbook.example.com
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
    - host: helm-guestbook.example.com
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
    - host: helm-guestbook.example.com
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
    - host: helm-guestbook.example.com
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

	testIngressNetworkingV1 = strToUnstructured(`
  apiVersion: networking.k8s.io/v1
  kind: Ingress
  metadata:
    name: helm-guestbook
    namespace: default
    uid: "4"
  spec:
    backend:
      service:
        name: not-found-service
        port:
          number: 443
    rules:
    - host: helm-guestbook.example.com
      http:
        paths:
        - backend:
            service:
              name: helm-guestbook
              port:
                number: 443
          path: /
        - backend:
            service:
              name: helm-guestbook
              port:
                name: https
          path: /
    tls:
    - host: helm-guestbook.example.com
    secretName: my-tls-secret
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

	testIstioServiceEntry = strToUnstructured(`
apiVersion: networking.istio.io/v1beta1
kind: ServiceEntry
metadata:
  name: echo
spec:
  exportTo:
  - '*'
  hosts:
  - echo.internal
  location: MESH_INTERNAL
  ports:
  - name: http
    number: 80
    protocol: HTTP
    targetPort: 5678 
  resolution: DNS

  workloadSelector:
    labels:
      app.kubernetes.io/name: echo-2
`)
)

// These tests are equivalent to tests in ui/src/app/applications/components/utils.test.tsx. If you update tests here,
// please make sure to update the equivalent tests in the UI.
func TestGetPodInfo(t *testing.T) {
	t.Parallel()

	t.Run("TestGetPodInfo", func(t *testing.T) {
		t.Parallel()

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
		populateNodeInfo(pod, info, []string{})
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
	})

	t.Run("TestGetPodWithInitialContainerInfo", func(t *testing.T) {
		pod := strToUnstructured(`
  apiVersion: "v1"
  kind: "Pod"
  metadata: 
    labels: 
      app: "app-with-initial-container"
    name: "app-with-initial-container-5f46976fdb-vd6rv"
    namespace: "default"
    ownerReferences: 
    - apiVersion: "apps/v1"
      kind: "ReplicaSet"
      name: "app-with-initial-container-5f46976fdb"
  spec: 
    containers: 
    - image: "alpine:latest"
      imagePullPolicy: "Always"
      name: "app-with-initial-container"
    initContainers: 
    - image: "alpine:latest"
      imagePullPolicy: "Always"
      name: "app-with-initial-container-logshipper"
    nodeName: "minikube"
  status: 
    containerStatuses: 
    - image: "alpine:latest"
      name: "app-with-initial-container"
      ready: true
      restartCount: 0
      started: true
      state: 
        running: 
          startedAt: "2024-10-08T08:44:25Z"
    initContainerStatuses: 
    - image: "alpine:latest"
      name: "app-with-initial-container-logshipper"
      ready: true
      restartCount: 0
      started: false
      state: 
        terminated: 
          exitCode: 0
          reason: "Completed"
    phase: "Running"
`)

		info := &ResourceInfo{}
		populateNodeInfo(pod, info, []string{})
		assert.Equal(t, []v1alpha1.InfoItem{
			{Name: "Status Reason", Value: "Running"},
			{Name: "Node", Value: "minikube"},
			{Name: "Containers", Value: "1/1"},
		}, info.Info)
	})

	t.Run("TestGetPodInfoWithSidecar", func(t *testing.T) {
		t.Parallel()

		pod := strToUnstructured(`
  apiVersion: v1
  kind: Pod
  metadata:
    labels:
      app: app-with-sidecar
    name: app-with-sidecar-6664cc788c-lqlrp
    namespace: default
    ownerReferences:
      - apiVersion: apps/v1
        kind: ReplicaSet
        name: app-with-sidecar-6664cc788c
  spec:
    containers:
    - image: 'docker.m.daocloud.io/library/alpine:latest'
      imagePullPolicy: Always
      name: app-with-sidecar
    initContainers:
    - image: 'docker.m.daocloud.io/library/alpine:latest'
      imagePullPolicy: Always
      name: logshipper
      restartPolicy: Always
    nodeName: minikube
  status:
    containerStatuses:
    - image: 'docker.m.daocloud.io/library/alpine:latest'
      name: app-with-sidecar
      ready: true
      restartCount: 0
      started: true
      state:
        running:
          startedAt: '2024-10-08T08:39:43Z'
    initContainerStatuses:
    - image: 'docker.m.daocloud.io/library/alpine:latest'
      name: logshipper
      ready: true
      restartCount: 0
      started: true
      state:
        running:
          startedAt: '2024-10-08T08:39:40Z'
    phase: Running
`)

		info := &ResourceInfo{}
		populateNodeInfo(pod, info, []string{})
		assert.Equal(t, []v1alpha1.InfoItem{
			{Name: "Status Reason", Value: "Running"},
			{Name: "Node", Value: "minikube"},
			{Name: "Containers", Value: "2/2"},
		}, info.Info)
	})

	t.Run("TestGetPodInfoWithInitialContainer", func(t *testing.T) {
		t.Parallel()

		pod := strToUnstructured(`
  apiVersion: v1
  kind: Pod
  metadata:
    generateName: myapp-long-exist-56b7d8794d-
    labels:
      app: myapp-long-exist
    name: myapp-long-exist-56b7d8794d-pbgrd
    namespace: linghao
    ownerReferences:
      - apiVersion: apps/v1
        kind: ReplicaSet
        name: myapp-long-exist-56b7d8794d
  spec:
    containers:
      - image: alpine:latest
        imagePullPolicy: Always
        name: myapp-long-exist
    initContainers:
      - image: alpine:latest
        imagePullPolicy: Always
        name: myapp-long-exist-logshipper
    nodeName: minikube
  status:
    containerStatuses:
      - image: alpine:latest
        name: myapp-long-exist
        ready: false
        restartCount: 0
        started: false
        state:
          waiting:
            reason: PodInitializing
    initContainerStatuses:
      - image: alpine:latest
        name: myapp-long-exist-logshipper
        ready: false
        restartCount: 0
        started: true
        state:
          running:
            startedAt: '2024-10-09T08:03:45Z'
    phase: Pending
    startTime: '2024-10-09T08:02:39Z'
`)

		info := &ResourceInfo{}
		populateNodeInfo(pod, info, []string{})
		assert.Equal(t, []v1alpha1.InfoItem{
			{Name: "Status Reason", Value: "Init:0/1"},
			{Name: "Node", Value: "minikube"},
			{Name: "Containers", Value: "0/1"},
		}, info.Info)
	})

	// Test pod has 2 restartable init containers, the first one running but not started.
	t.Run("TestGetPodInfoWithRestartableInitContainer", func(t *testing.T) {
		t.Parallel()

		pod := strToUnstructured(`
  apiVersion: v1
  kind: Pod
  metadata:
    name: test1
  spec:
    initContainers:
      - name: restartable-init-1
        restartPolicy: Always
      - name: restartable-init-2
        restartPolicy: Always
    containers:
      - name: container
    nodeName: minikube
  status:
    phase: Pending
    initContainerStatuses:
      - name: restartable-init-1
        ready: false
        restartCount: 3
        state:
          running: {}
        started: false
        lastTerminationState:
          terminated:
            finishedAt: "2023-10-01T00:00:00Z" # Replace with actual time
      - name: restartable-init-2
        ready: false
        state:
          waiting: {}
        started: false
    containerStatuses:
      - ready: false
        restartCount: 0
        state:
          waiting: {}
    conditions:
      - type: ContainersReady
        status: "False"
      - type: Initialized
        status: "False"
`)

		info := &ResourceInfo{}
		populateNodeInfo(pod, info, []string{})
		assert.Equal(t, []v1alpha1.InfoItem{
			{Name: "Status Reason", Value: "Init:0/2"},
			{Name: "Node", Value: "minikube"},
			{Name: "Containers", Value: "0/3"},
			{Name: "Restart Count", Value: "3"},
		}, info.Info)
	})

	// Test pod has 2 restartable init containers, the first one started and the second one running but not started.
	t.Run("TestGetPodInfoWithPartiallyStartedInitContainers", func(t *testing.T) {
		t.Parallel()

		pod := strToUnstructured(`
  apiVersion: v1
  kind: Pod
  metadata:
    name: test1
  spec:
    initContainers:
      - name: restartable-init-1
        restartPolicy: Always
      - name: restartable-init-2
        restartPolicy: Always
    containers:
      - name: container
    nodeName: minikube
  status:
    phase: Pending
    initContainerStatuses:
      - name: restartable-init-1
        ready: false
        restartCount: 3
        state:
          running: {}
        started: true
        lastTerminationState:
          terminated:
            finishedAt: "2023-10-01T00:00:00Z" # Replace with actual time
      - name: restartable-init-2
        ready: false
        state:
          running: {}
        started: false
    containerStatuses:
      - ready: false
        restartCount: 0
        state:
          waiting: {}
    conditions:
      - type: ContainersReady
        status: "False"
      - type: Initialized
        status: "False"
`)

		info := &ResourceInfo{}
		populateNodeInfo(pod, info, []string{})
		assert.Equal(t, []v1alpha1.InfoItem{
			{Name: "Status Reason", Value: "Init:1/2"},
			{Name: "Node", Value: "minikube"},
			{Name: "Containers", Value: "0/3"},
			{Name: "Restart Count", Value: "3"},
		}, info.Info)
	})

	// Test pod has 2 restartable init containers started and 1 container running
	t.Run("TestGetPodInfoWithStartedInitContainers", func(t *testing.T) {
		t.Parallel()

		pod := strToUnstructured(`
  apiVersion: v1
  kind: Pod
  metadata:
    name: test2
  spec:
    initContainers:
      - name: restartable-init-1
        restartPolicy: Always
      - name: restartable-init-2
        restartPolicy: Always
    containers:
      - name: container
    nodeName: minikube
  status:
    phase: Running
    initContainerStatuses:
      - name: restartable-init-1
        ready: false
        restartCount: 3
        state:
          running: {}
        started: true
        lastTerminationState:
          terminated:
            finishedAt: "2023-10-01T00:00:00Z" # Replace with actual time
      - name: restartable-init-2
        ready: false
        state:
          running: {}
        started: true
    containerStatuses:
      - ready: true
        restartCount: 4
        state:
          running: {}
        lastTerminationState:
          terminated:
            finishedAt: "2023-10-01T00:00:00Z" # Replace with actual time
    conditions:
      - type: ContainersReady
        status: "False"
      - type: Initialized
        status: "True"
`)

		info := &ResourceInfo{}
		populateNodeInfo(pod, info, []string{})
		assert.Equal(t, []v1alpha1.InfoItem{
			{Name: "Status Reason", Value: "Running"},
			{Name: "Node", Value: "minikube"},
			{Name: "Containers", Value: "1/3"},
			{Name: "Restart Count", Value: "7"},
		}, info.Info)
	})

	// Test pod has 1 init container restarting and 1 container not running
	t.Run("TestGetPodInfoWithNormalInitContainer", func(t *testing.T) {
		t.Parallel()

		pod := strToUnstructured(`
  apiVersion: v1
  kind: Pod
  metadata:
    name: test7
  spec:
    initContainers:
      - name: init-container
    containers:
      - name: main-container
    nodeName: minikube
  status:
    phase: podPhase
    initContainerStatuses:
      - ready: false
        restartCount: 3
        state:
          running: {}
        lastTerminationState:
          terminated:
            finishedAt: "2023-10-01T00:00:00Z" # Replace with the actual time
    containerStatuses:
      - ready: false
        restartCount: 0
        state:
          waiting: {}
`)

		info := &ResourceInfo{}
		populateNodeInfo(pod, info, []string{})
		assert.Equal(t, []v1alpha1.InfoItem{
			{Name: "Status Reason", Value: "Init:0/1"},
			{Name: "Node", Value: "minikube"},
			{Name: "Containers", Value: "0/1"},
			{Name: "Restart Count", Value: "3"},
		}, info.Info)
	})

	// Test pod condition succeed
	t.Run("TestPodConditionSucceeded", func(t *testing.T) {
		t.Parallel()

		pod := strToUnstructured(`
  apiVersion: v1
  kind: Pod
  metadata:
    name: test8
  spec:
    nodeName: minikube
    containers:
      - name: container
  status:
    phase: Succeeded
    containerStatuses:
      - ready: false
        restartCount: 0
        state:
          terminated:
            reason: Completed
            exitCode: 0
`)
		info := &ResourceInfo{}
		populateNodeInfo(pod, info, []string{})
		assert.Equal(t, []v1alpha1.InfoItem{
			{Name: "Status Reason", Value: "Completed"},
			{Name: "Node", Value: "minikube"},
			{Name: "Containers", Value: "0/1"},
		}, info.Info)
	})

	// Test pod condition failed
	t.Run("TestPodConditionFailed", func(t *testing.T) {
		t.Parallel()

		pod := strToUnstructured(`
  apiVersion: v1
  kind: Pod
  metadata:
    name: test9
  spec:
    nodeName: minikube
    containers:
      - name: container
  status:
    phase: Failed
    containerStatuses:
      - ready: false
        restartCount: 0
        state:
          terminated:
            reason: Error
            exitCode: 1
`)
		info := &ResourceInfo{}
		populateNodeInfo(pod, info, []string{})
		assert.Equal(t, []v1alpha1.InfoItem{
			{Name: "Status Reason", Value: "Error"},
			{Name: "Node", Value: "minikube"},
			{Name: "Containers", Value: "0/1"},
		}, info.Info)
	})

	// Test pod condition succeed with deletion
	t.Run("TestPodConditionSucceededWithDeletion", func(t *testing.T) {
		t.Parallel()

		pod := strToUnstructured(`
  apiVersion: v1
  kind: Pod
  metadata:
    name: test10
    deletionTimestamp: "2023-10-01T00:00:00Z"
  spec:
    nodeName: minikube
    containers:
      - name: container
  status:
    phase: Succeeded
    containerStatuses:
      - ready: false
        restartCount: 0
        state:
          terminated:
            reason: Completed
            exitCode: 0
`)
		info := &ResourceInfo{}
		populateNodeInfo(pod, info, []string{})
		assert.Equal(t, []v1alpha1.InfoItem{
			{Name: "Status Reason", Value: "Completed"},
			{Name: "Node", Value: "minikube"},
			{Name: "Containers", Value: "0/1"},
		}, info.Info)
	})

	// Test pod condition running with deletion
	t.Run("TestPodConditionRunningWithDeletion", func(t *testing.T) {
		t.Parallel()

		pod := strToUnstructured(`
  apiVersion: v1
  kind: Pod
  metadata:
    name: test11
    deletionTimestamp: "2023-10-01T00:00:00Z"
  spec:
    nodeName: minikube
    containers:
      - name: container
  status:
    phase: Running
    containerStatuses:
      - ready: false
        restartCount: 0
        state:
          running: {}
`)
		info := &ResourceInfo{}
		populateNodeInfo(pod, info, []string{})
		assert.Equal(t, []v1alpha1.InfoItem{
			{Name: "Status Reason", Value: "Terminating"},
			{Name: "Node", Value: "minikube"},
			{Name: "Containers", Value: "0/1"},
		}, info.Info)
	})

	// Test pod condition pending with deletion
	t.Run("TestPodConditionPendingWithDeletion", func(t *testing.T) {
		t.Parallel()

		pod := strToUnstructured(`
  apiVersion: v1
  kind: Pod
  metadata:
    name: test12
    deletionTimestamp: "2023-10-01T00:00:00Z"
  spec:
    nodeName: minikube
    containers:
      - name: container
  status:
    phase: Pending
`)
		info := &ResourceInfo{}
		populateNodeInfo(pod, info, []string{})
		assert.Equal(t, []v1alpha1.InfoItem{
			{Name: "Status Reason", Value: "Terminating"},
			{Name: "Node", Value: "minikube"},
			{Name: "Containers", Value: "0/1"},
		}, info.Info)
	})

	// Test PodScheduled condition with reason SchedulingGated
	t.Run("TestPodScheduledWithSchedulingGated", func(t *testing.T) {
		t.Parallel()

		pod := strToUnstructured(`
  apiVersion: v1
  kind: Pod
  metadata:
    name: test13
  spec:
    nodeName: minikube
    containers:
      - name: container1
      - name: container2
  status:
    phase: podPhase
    conditions:
      - type: PodScheduled
        status: "False"
        reason: SchedulingGated
`)
		info := &ResourceInfo{}
		populateNodeInfo(pod, info, []string{})
		assert.Equal(t, []v1alpha1.InfoItem{
			{Name: "Status Reason", Value: "SchedulingGated"},
			{Name: "Node", Value: "minikube"},
			{Name: "Containers", Value: "0/2"},
		}, info.Info)
	})
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
	populateNodeInfo(node, info, []string{})
	assert.Equal(t, &NodeInfo{
		Name:       "minikube",
		Capacity:   v1.ResourceList{v1.ResourceMemory: resource.MustParse("6091320Ki"), v1.ResourceCPU: resource.MustParse("6")},
		SystemInfo: v1.NodeSystemInfo{Architecture: "amd64", OperatingSystem: "linux", OSImage: "Ubuntu 20.04 LTS"},
	}, info.NodeInfo)
}

func TestGetServiceInfo(t *testing.T) {
	info := &ResourceInfo{}
	populateNodeInfo(testService, info, []string{})
	assert.Empty(t, info.Info)
	assert.Equal(t, &v1alpha1.ResourceNetworkingInfo{
		TargetLabels: map[string]string{"app": "guestbook"},
		Ingress:      []v1.LoadBalancerIngress{{Hostname: "localhost"}},
	}, info.NetworkingInfo)
}

func TestGetLinkAnnotatedServiceInfo(t *testing.T) {
	info := &ResourceInfo{}
	populateNodeInfo(testLinkAnnotatedService, info, []string{})
	assert.Empty(t, info.Info)
	assert.Equal(t, &v1alpha1.ResourceNetworkingInfo{
		TargetLabels: map[string]string{"app": "guestbook"},
		Ingress:      []v1.LoadBalancerIngress{{Hostname: "localhost"}},
		ExternalURLs: []string{"http://my-grafana.example.com/pre-generated-link"},
	}, info.NetworkingInfo)
}

func TestGetIstioVirtualServiceInfo(t *testing.T) {
	info := &ResourceInfo{}
	populateNodeInfo(testIstioVirtualService, info, []string{})
	assert.Empty(t, info.Info)
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

func TestGetIstioServiceEntryInfo(t *testing.T) {
	info := &ResourceInfo{}
	populateNodeInfo(testIstioServiceEntry, info, []string{})
	assert.Empty(t, info.Info)
	require.NotNil(t, info.NetworkingInfo)
	require.NotNil(t, info.NetworkingInfo.TargetRefs)
	assert.Contains(t, info.NetworkingInfo.TargetRefs, v1alpha1.ResourceRef{
		Kind: kube.PodKind,
	})

	assert.Equal(t, map[string]string{
		"app.kubernetes.io/name": "echo-2",
	}, info.NetworkingInfo.TargetLabels)
}

func TestGetIngressInfo(t *testing.T) {
	tests := []struct {
		Ingress *unstructured.Unstructured
	}{
		{testIngress},
		{testIngressNetworkingV1},
	}
	for _, tc := range tests {
		info := &ResourceInfo{}
		populateNodeInfo(tc.Ingress, info, []string{})
		assert.Empty(t, info.Info)
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
			ExternalURLs: []string{"https://helm-guestbook.example.com/"},
		}, info.NetworkingInfo)
	}
}

func TestGetLinkAnnotatedIngressInfo(t *testing.T) {
	info := &ResourceInfo{}
	populateNodeInfo(testLinkAnnotatedIngress, info, []string{})
	assert.Empty(t, info.Info)
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
		ExternalURLs: []string{"http://my-grafana.example.com/ingress-link", "https://helm-guestbook.example.com/"},
	}, info.NetworkingInfo)
}

func TestGetIngressInfoWildCardPath(t *testing.T) {
	info := &ResourceInfo{}
	populateNodeInfo(testIngressWildCardPath, info, []string{})
	assert.Empty(t, info.Info)
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
		ExternalURLs: []string{"https://helm-guestbook.example.com/"},
	}, info.NetworkingInfo)
}

func TestGetIngressInfoWithoutTls(t *testing.T) {
	info := &ResourceInfo{}
	populateNodeInfo(testIngressWithoutTls, info, []string{})
	assert.Empty(t, info.Info)
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
		ExternalURLs: []string{"http://helm-guestbook.example.com/"},
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
	populateNodeInfo(ingress, info, []string{})

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
	populateNodeInfo(ingress, info, []string{})

	assert.Equal(t, &v1alpha1.ResourceNetworkingInfo{
		TargetRefs: []v1alpha1.ResourceRef{{
			Namespace: "default",
			Group:     "",
			Kind:      kube.ServiceKind,
			Name:      "helm-guestbook",
		}},
	}, info.NetworkingInfo)
	assert.Empty(t, info.NetworkingInfo.ExternalURLs)
}

func TestExternalUrlWithSubPath(t *testing.T) {
	ingress := strToUnstructured(`
  apiVersion: networking.k8s.io/v1
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
	populateNodeInfo(ingress, info, []string{})

	expectedExternalUrls := []string{"https://107.178.210.11/my/sub/path/"}
	assert.Equal(t, expectedExternalUrls, info.NetworkingInfo.ExternalURLs)
}

func TestExternalUrlWithMultipleSubPaths(t *testing.T) {
	ingress := strToUnstructured(`
  apiVersion: networking.k8s.io/v1
  kind: Ingress
  metadata:
    name: helm-guestbook
    namespace: default
  spec:
    rules:
    - host: helm-guestbook.example.com
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
	populateNodeInfo(ingress, info, []string{})

	expectedExternalUrls := []string{"https://helm-guestbook.example.com/my/sub/path/", "https://helm-guestbook.example.com/my/sub/path/2", "https://helm-guestbook.example.com"}
	actualURLs := info.NetworkingInfo.ExternalURLs
	sort.Strings(expectedExternalUrls)
	sort.Strings(actualURLs)
	assert.Equal(t, expectedExternalUrls, actualURLs)
}

func TestExternalUrlWithNoSubPath(t *testing.T) {
	ingress := strToUnstructured(`
  apiVersion: networking.k8s.io/v1
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
	populateNodeInfo(ingress, info, []string{})

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
	populateNodeInfo(ingress, info, []string{})

	expectedExternalUrls := []string{"https://107.178.210.11"}
	assert.Equal(t, expectedExternalUrls, info.NetworkingInfo.ExternalURLs)
}

func TestCustomLabel(t *testing.T) {
	configmap := strToUnstructured(`
  apiVersion: v1
  kind: ConfigMap
  metadata:
    name: cm`)

	info := &ResourceInfo{}
	populateNodeInfo(configmap, info, []string{"my-label"})

	assert.Empty(t, info.Info)

	configmap = strToUnstructured(`
  apiVersion: v1
  kind: ConfigMap
  metadata:
    name: cm
    labels:
      my-label: value`)

	info = &ResourceInfo{}
	populateNodeInfo(configmap, info, []string{"my-label", "other-label"})

	assert.Len(t, info.Info, 1)
	assert.Equal(t, "my-label", info.Info[0].Name)
	assert.Equal(t, "value", info.Info[0].Value)

	configmap = strToUnstructured(`
  apiVersion: v1
  kind: ConfigMap
  metadata:
    name: cm
    labels:
      my-label: value
      other-label: value2`)

	info = &ResourceInfo{}
	populateNodeInfo(configmap, info, []string{"my-label", "other-label"})

	assert.Len(t, info.Info, 2)
	assert.Equal(t, "my-label", info.Info[0].Name)
	assert.Equal(t, "value", info.Info[0].Value)
	assert.Equal(t, "other-label", info.Info[1].Name)
	assert.Equal(t, "value2", info.Info[1].Value)
}

func TestManifestHash(t *testing.T) {
	manifest := strToUnstructured(`
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

	ignores := []v1alpha1.ResourceIgnoreDifferences{
		{
			Group:        "*",
			Kind:         "*",
			JSONPointers: []string{"/metadata/resourceVersion"},
		},
	}

	data, _ := strToUnstructured(`
  apiVersion: v1
  kind: Pod
  metadata:
    name: helm-guestbook-pod
    namespace: default
    ownerReferences:
    - apiVersion: extensions/v1beta1
      kind: ReplicaSet
      name: helm-guestbook-rs
    labels:
      app: guestbook
  spec:
    nodeName: minikube
    containers:
    - image: bar
      resources:
        requests:
          memory: 128Mi
`).MarshalJSON()

	expected := hash(data)

	hash, err := generateManifestHash(manifest, ignores, nil, normalizers.IgnoreNormalizerOpts{})
	assert.Equal(t, expected, hash)
	assert.NoError(t, err)
}
