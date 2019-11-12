package cache

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/argoproj/argo-cd/engine/util/settings"

	"github.com/argoproj/argo-cd/engine/util/lua"

	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic/fake"

	enginecommon "github.com/argoproj/argo-cd/engine/common"
	appv1 "github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/engine/util/errors"
	"github.com/argoproj/argo-cd/engine/util/kube"
	"github.com/argoproj/argo-cd/engine/util/kube/kubetest"
)

func strToUnstructured(jsonStr string) *unstructured.Unstructured {
	obj := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(jsonStr), &obj)
	errors.CheckError(err)
	return &unstructured.Unstructured{Object: obj}
}

func mustToUnstructured(obj interface{}) *unstructured.Unstructured {
	un, err := kube.ToUnstructured(obj)
	errors.CheckError(err)
	return un
}

var (
	testPod = strToUnstructured(`
  apiVersion: v1
  kind: Pod
  metadata:
    uid: "1"
    name: helm-guestbook-pod
    namespace: default
    ownerReferences:
    - apiVersion: apps/v1
      kind: ReplicaSet
      name: helm-guestbook-rs
      uid: "2"
    resourceVersion: "123"`)

	testRS = strToUnstructured(`
  apiVersion: apps/v1
  kind: ReplicaSet
  metadata:
    uid: "2"
    name: helm-guestbook-rs
    namespace: default
    annotations:
      deployment.kubernetes.io/revision: "2"
    ownerReferences:
    - apiVersion: apps/v1beta1
      kind: Deployment
      name: helm-guestbook
      uid: "3"
    resourceVersion: "123"`)

	testDeploy = strToUnstructured(`
  apiVersion: apps/v1
  kind: Deployment
  metadata:
    labels:
      app.kubernetes.io/instance: helm-guestbook
    uid: "3"
    name: helm-guestbook
    namespace: default
    resourceVersion: "123"`)

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

func newCluster(objs ...*unstructured.Unstructured) *clusterInfo {
	runtimeObjs := make([]runtime.Object, len(objs))
	for i := range objs {
		runtimeObjs[i] = objs[i]
	}
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme, runtimeObjs...)

	apiResources := []kube.APIResourceInfo{{
		GroupKind: schema.GroupKind{Group: "", Kind: "Pod"},
		Interface: client.Resource(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}),
		Meta:      metav1.APIResource{Namespaced: true},
	}, {
		GroupKind: schema.GroupKind{Group: "apps", Kind: "ReplicaSet"},
		Interface: client.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}),
		Meta:      metav1.APIResource{Namespaced: true},
	}, {
		GroupKind: schema.GroupKind{Group: "apps", Kind: "Deployment"},
		Interface: client.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}),
		Meta:      metav1.APIResource{Namespaced: true},
	}}

	return newClusterExt(&kubetest.MockKubectlCmd{APIResources: apiResources})
}

func newClusterExt(kubectl kube.Kubectl) *clusterInfo {
	return &clusterInfo{
		lock:            &sync.Mutex{},
		nodes:           make(map[kube.ResourceKey]*node),
		onObjectUpdated: func(managedByApp map[string]bool, reference corev1.ObjectReference) {},
		kubectl:         kubectl,
		nsIndex:         make(map[string]map[kube.ResourceKey]*node),
		cluster:         &appv1.Cluster{},
		syncTime:        nil,
		syncLock:        &sync.Mutex{},
		apisMeta:        make(map[schema.GroupKind]*apiMeta),
		log:             log.WithField("cluster", "test"),
		cacheSettingsSrc: func() *cacheSettings {
			return &cacheSettings{AppInstanceLabelKey: enginecommon.LabelKeyAppInstance}
		},
		luaVMFactory: func(overrides map[string]appv1.ResourceOverride) *lua.VM {
			return &lua.VM{ResourceOverrides: overrides}
		},
		callbacks: settings.NewNoOpCallbacks(),
	}
}

func getChildren(cluster *clusterInfo, un *unstructured.Unstructured) []appv1.ResourceNode {
	hierarchy := make([]appv1.ResourceNode, 0)
	cluster.iterateHierarchy(kube.GetResourceKey(un), func(child appv1.ResourceNode, app string) {
		hierarchy = append(hierarchy, child)
	})
	return hierarchy[1:]
}

func TestGetNamespaceResources(t *testing.T) {
	defaultNamespaceTopLevel1 := strToUnstructured(`
  apiVersion: apps/v1
  kind: Deployment
  metadata: {"name": "helm-guestbook1", "namespace": "default"}
`)
	defaultNamespaceTopLevel2 := strToUnstructured(`
  apiVersion: apps/v1
  kind: Deployment
  metadata: {"name": "helm-guestbook2", "namespace": "default"}
`)
	kubesystemNamespaceTopLevel2 := strToUnstructured(`
  apiVersion: apps/v1
  kind: Deployment
  metadata: {"name": "helm-guestbook3", "namespace": "kube-system"}
`)

	cluster := newCluster(defaultNamespaceTopLevel1, defaultNamespaceTopLevel2, kubesystemNamespaceTopLevel2)
	err := cluster.ensureSynced()
	assert.Nil(t, err)

	resources := cluster.getNamespaceTopLevelResources("default")
	assert.Len(t, resources, 2)
	assert.Equal(t, resources[kube.GetResourceKey(defaultNamespaceTopLevel1)].Name, "helm-guestbook1")
	assert.Equal(t, resources[kube.GetResourceKey(defaultNamespaceTopLevel2)].Name, "helm-guestbook2")

	resources = cluster.getNamespaceTopLevelResources("kube-system")
	assert.Len(t, resources, 1)
	assert.Equal(t, resources[kube.GetResourceKey(kubesystemNamespaceTopLevel2)].Name, "helm-guestbook3")
}

func TestGetChildren(t *testing.T) {
	cluster := newCluster(testPod, testRS, testDeploy)
	err := cluster.ensureSynced()
	assert.Nil(t, err)

	rsChildren := getChildren(cluster, testRS)
	assert.Equal(t, []appv1.ResourceNode{{
		ResourceRef: appv1.ResourceRef{
			Kind:      "Pod",
			Namespace: "default",
			Name:      "helm-guestbook-pod",
			Group:     "",
			Version:   "v1",
			UID:       "1",
		},
		ParentRefs: []appv1.ResourceRef{{
			Group:     "apps",
			Version:   "",
			Kind:      "ReplicaSet",
			Namespace: "default",
			Name:      "helm-guestbook-rs",
			UID:       "2",
		}},
		Health:          &appv1.HealthStatus{Status: appv1.HealthStatusUnknown},
		NetworkingInfo:  &appv1.ResourceNetworkingInfo{Labels: testPod.GetLabels()},
		ResourceVersion: "123",
		Info:            []appv1.InfoItem{{Name: "Containers", Value: "0/0"}},
	}}, rsChildren)
	deployChildren := getChildren(cluster, testDeploy)

	assert.Equal(t, append([]appv1.ResourceNode{{
		ResourceRef: appv1.ResourceRef{
			Kind:      "ReplicaSet",
			Namespace: "default",
			Name:      "helm-guestbook-rs",
			Group:     "apps",
			Version:   "v1",
			UID:       "2",
		},
		ResourceVersion: "123",
		Health:          &appv1.HealthStatus{Status: appv1.HealthStatusHealthy},
		Info:            []appv1.InfoItem{{Name: "Revision", Value: "Rev:2"}},
		ParentRefs:      []appv1.ResourceRef{{Group: "apps", Version: "", Kind: "Deployment", Namespace: "default", Name: "helm-guestbook", UID: "3"}},
	}}, rsChildren...), deployChildren)
}

func TestGetManagedLiveObjs(t *testing.T) {
	cluster := newCluster(testPod, testRS, testDeploy)
	err := cluster.ensureSynced()
	assert.Nil(t, err)

	targetDeploy := strToUnstructured(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: helm-guestbook
  labels:
    app: helm-guestbook`)

	managedObjs, err := cluster.getManagedLiveObjs(&appv1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: "helm-guestbook"},
		Spec: appv1.ApplicationSpec{
			Destination: appv1.ApplicationDestination{
				Namespace: "default",
			},
		},
	}, []*unstructured.Unstructured{targetDeploy}, nil)
	assert.Nil(t, err)
	assert.Equal(t, managedObjs, map[kube.ResourceKey]*unstructured.Unstructured{
		kube.NewResourceKey("apps", "Deployment", "default", "helm-guestbook"): testDeploy,
	})
}

func TestChildDeletedEvent(t *testing.T) {
	cluster := newCluster(testPod, testRS, testDeploy)
	err := cluster.ensureSynced()
	assert.Nil(t, err)

	cluster.processEvent(watch.Deleted, testPod)

	rsChildren := getChildren(cluster, testRS)
	assert.Equal(t, []appv1.ResourceNode{}, rsChildren)
}

func TestProcessNewChildEvent(t *testing.T) {
	cluster := newCluster(testPod, testRS, testDeploy)
	err := cluster.ensureSynced()
	assert.Nil(t, err)

	newPod := strToUnstructured(`
  apiVersion: v1
  kind: Pod
  metadata:
    uid: "4"
    name: helm-guestbook-pod2
    namespace: default
    ownerReferences:
    - apiVersion: apps/v1
      kind: ReplicaSet
      name: helm-guestbook-rs
      uid: "2"
    resourceVersion: "123"`)

	cluster.processEvent(watch.Added, newPod)

	rsChildren := getChildren(cluster, testRS)
	sort.Slice(rsChildren, func(i, j int) bool {
		return strings.Compare(rsChildren[i].Name, rsChildren[j].Name) < 0
	})
	assert.Equal(t, []appv1.ResourceNode{{
		ResourceRef: appv1.ResourceRef{
			Kind:      "Pod",
			Namespace: "default",
			Name:      "helm-guestbook-pod",
			Group:     "",
			Version:   "v1",
			UID:       "1",
		},
		Info:           []appv1.InfoItem{{Name: "Containers", Value: "0/0"}},
		Health:         &appv1.HealthStatus{Status: appv1.HealthStatusUnknown},
		NetworkingInfo: &appv1.ResourceNetworkingInfo{Labels: testPod.GetLabels()},
		ParentRefs: []appv1.ResourceRef{{
			Group:     "apps",
			Version:   "",
			Kind:      "ReplicaSet",
			Namespace: "default",
			Name:      "helm-guestbook-rs",
			UID:       "2",
		}},
		ResourceVersion: "123",
	}, {
		ResourceRef: appv1.ResourceRef{
			Kind:      "Pod",
			Namespace: "default",
			Name:      "helm-guestbook-pod2",
			Group:     "",
			Version:   "v1",
			UID:       "4",
		},
		NetworkingInfo: &appv1.ResourceNetworkingInfo{Labels: testPod.GetLabels()},
		Info:           []appv1.InfoItem{{Name: "Containers", Value: "0/0"}},
		Health:         &appv1.HealthStatus{Status: appv1.HealthStatusUnknown},
		ParentRefs: []appv1.ResourceRef{{
			Group:     "apps",
			Version:   "",
			Kind:      "ReplicaSet",
			Namespace: "default",
			Name:      "helm-guestbook-rs",
			UID:       "2",
		}},
		ResourceVersion: "123",
	}}, rsChildren)
}

func TestUpdateResourceTags(t *testing.T) {
	pod := &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "testPod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test",
				Image: "test",
			}},
		},
	}
	cluster := newCluster(mustToUnstructured(pod))

	err := cluster.ensureSynced()
	assert.Nil(t, err)

	podNode := cluster.nodes[kube.GetResourceKey(mustToUnstructured(pod))]

	assert.NotNil(t, podNode)
	assert.Equal(t, []appv1.InfoItem{{Name: "Containers", Value: "0/1"}}, podNode.info)

	pod.Status = corev1.PodStatus{
		ContainerStatuses: []corev1.ContainerStatus{{
			State: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode: -1,
				},
			},
		}},
	}
	cluster.processEvent(watch.Modified, mustToUnstructured(pod))

	podNode = cluster.nodes[kube.GetResourceKey(mustToUnstructured(pod))]

	assert.NotNil(t, podNode)
	assert.Equal(t, []appv1.InfoItem{{Name: "Status Reason", Value: "ExitCode:-1"}, {Name: "Containers", Value: "0/1"}}, podNode.info)
}

func TestUpdateAppResource(t *testing.T) {
	updatesReceived := make([]string, 0)
	cluster := newCluster(testPod, testRS, testDeploy)
	cluster.onObjectUpdated = func(managedByApp map[string]bool, _ corev1.ObjectReference) {
		for appName, fullRefresh := range managedByApp {
			updatesReceived = append(updatesReceived, fmt.Sprintf("%s: %v", appName, fullRefresh))
		}
	}

	err := cluster.ensureSynced()
	assert.Nil(t, err)

	cluster.processEvent(watch.Modified, mustToUnstructured(testPod))

	assert.Contains(t, updatesReceived, "helm-guestbook: false")
}

func TestCircularReference(t *testing.T) {
	dep := testDeploy.DeepCopy()
	dep.SetOwnerReferences([]metav1.OwnerReference{{
		Name:       testPod.GetName(),
		Kind:       testPod.GetKind(),
		APIVersion: testPod.GetAPIVersion(),
	}})
	cluster := newCluster(testPod, testRS, dep)
	err := cluster.ensureSynced()

	assert.Nil(t, err)

	children := getChildren(cluster, dep)
	assert.Len(t, children, 2)

	node := cluster.nodes[kube.GetResourceKey(dep)]
	assert.NotNil(t, node)
	app := node.getApp(cluster.nodes)
	assert.Equal(t, "", app)
}

func TestWatchCacheUpdated(t *testing.T) {
	removed := testPod.DeepCopy()
	removed.SetName(testPod.GetName() + "-removed-pod")

	updated := testPod.DeepCopy()
	updated.SetName(testPod.GetName() + "-updated-pod")
	updated.SetResourceVersion("updated-pod-version")

	cluster := newCluster(removed, updated)
	err := cluster.ensureSynced()

	assert.Nil(t, err)

	added := testPod.DeepCopy()
	added.SetName(testPod.GetName() + "-new-pod")

	podGroupKind := testPod.GroupVersionKind().GroupKind()

	cluster.replaceResourceCache(podGroupKind, "updated-list-version", []unstructured.Unstructured{*updated, *added})

	_, ok := cluster.nodes[kube.GetResourceKey(removed)]
	assert.False(t, ok)

	updatedNode, ok := cluster.nodes[kube.GetResourceKey(updated)]
	assert.True(t, ok)
	assert.Equal(t, updatedNode.resourceVersion, "updated-pod-version")

	_, ok = cluster.nodes[kube.GetResourceKey(added)]
	assert.True(t, ok)
}

func TestGetDuplicatedChildren(t *testing.T) {
	extensionsRS := testRS.DeepCopy()
	extensionsRS.SetGroupVersionKind(schema.GroupVersionKind{Group: "extensions", Kind: kube.ReplicaSetKind, Version: "v1beta1"})
	cluster := newCluster(testDeploy, testRS, extensionsRS)
	err := cluster.ensureSynced()

	assert.Nil(t, err)

	// Get children multiple times to make sure the right child is picked up every time.
	for i := 0; i < 5; i++ {
		children := getChildren(cluster, testDeploy)
		assert.Len(t, children, 1)
		assert.Equal(t, "apps", children[0].Group)
		assert.Equal(t, kube.ReplicaSetKind, children[0].Kind)
		assert.Equal(t, testRS.GetName(), children[0].Name)
	}
}
