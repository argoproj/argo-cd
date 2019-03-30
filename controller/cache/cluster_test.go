package cache

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"

	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/argoproj/argo-cd/errors"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/kube/kubetest"
	"github.com/argoproj/argo-cd/util/settings"
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
    name: helm-guestbook-pod
    namespace: default
    ownerReferences:
    - apiVersion: extensions/v1beta1
      kind: ReplicaSet
      name: helm-guestbook-rs
    resourceVersion: "123"`)

	testRS = strToUnstructured(`
  apiVersion: apps/v1
  kind: ReplicaSet
  metadata:
    name: helm-guestbook-rs
    namespace: default
    ownerReferences:
    - apiVersion: extensions/v1beta1
      kind: Deployment
      name: helm-guestbook
    resourceVersion: "123"`)

	testDeploy = strToUnstructured(`
  apiVersion: apps/v1
  kind: Deployment
  metadata:
    labels:
      app.kubernetes.io/instance: helm-guestbook
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
          path: /`)
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

	return newClusterExt(kubetest.MockKubectlCmd{APIResources: apiResources})
}

func newClusterExt(kubectl kube.Kubectl) *clusterInfo {
	return &clusterInfo{
		lock:         &sync.Mutex{},
		nodes:        make(map[kube.ResourceKey]*node),
		onAppUpdated: func(appName string, fullRefresh bool) {},
		kubectl:      kubectl,
		nsIndex:      make(map[string]map[kube.ResourceKey]*node),
		cluster:      &appv1.Cluster{},
		syncTime:     nil,
		syncLock:     &sync.Mutex{},
		apisMeta:     make(map[schema.GroupKind]*apiMeta),
		log:          log.WithField("cluster", "test"),
		settings:     &settings.ArgoCDSettings{},
	}
}

func getChildren(cluster *clusterInfo, un *unstructured.Unstructured) []appv1.ResourceNode {
	hierarchy := make([]appv1.ResourceNode, 0)
	cluster.iterateHierarchy(kube.GetResourceKey(un), func(child appv1.ResourceNode) {
		hierarchy = append(hierarchy, child)
	})
	return hierarchy[1:]
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
		},
		ParentRefs: []appv1.ResourceRef{{
			Group:     "apps",
			Version:   "",
			Kind:      "ReplicaSet",
			Namespace: "default",
			Name:      "helm-guestbook-rs",
		}},
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
		},
		ResourceVersion: "123",
		Info:            []appv1.InfoItem{},
		ParentRefs:      []appv1.ResourceRef{{Group: "apps", Version: "", Kind: "Deployment", Namespace: "default", Name: "helm-guestbook"}},
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
	}, []*unstructured.Unstructured{targetDeploy})
	assert.Nil(t, err)
	assert.Equal(t, managedObjs, map[kube.ResourceKey]*unstructured.Unstructured{
		kube.NewResourceKey("apps", "Deployment", "default", "helm-guestbook"): testDeploy,
	})
}

func TestChildDeletedEvent(t *testing.T) {
	cluster := newCluster(testPod, testRS, testDeploy)
	err := cluster.ensureSynced()
	assert.Nil(t, err)

	err = cluster.processEvent(watch.Deleted, testPod)
	assert.Nil(t, err)

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
    name: helm-guestbook-pod2
    namespace: default
    ownerReferences:
    - apiVersion: extensions/v1beta1
      kind: ReplicaSet
      name: helm-guestbook-rs
    resourceVersion: "123"`)

	err = cluster.processEvent(watch.Added, newPod)
	assert.Nil(t, err)

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
		},
		Info:           []appv1.InfoItem{{Name: "Containers", Value: "0/0"}},
		NetworkingInfo: &appv1.ResourceNetworkingInfo{Labels: testPod.GetLabels()},
		ParentRefs: []appv1.ResourceRef{{
			Group:     "apps",
			Version:   "",
			Kind:      "ReplicaSet",
			Namespace: "default",
			Name:      "helm-guestbook-rs",
		}},
		ResourceVersion: "123",
	}, {
		ResourceRef: appv1.ResourceRef{
			Kind:      "Pod",
			Namespace: "default",
			Name:      "helm-guestbook-pod2",
			Group:     "",
			Version:   "v1",
		},
		NetworkingInfo: &appv1.ResourceNetworkingInfo{Labels: testPod.GetLabels()},
		Info:           []appv1.InfoItem{{Name: "Containers", Value: "0/0"}},
		ParentRefs: []appv1.ResourceRef{{
			Group:     "apps",
			Version:   "",
			Kind:      "ReplicaSet",
			Namespace: "default",
			Name:      "helm-guestbook-rs",
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
	err = cluster.processEvent(watch.Modified, mustToUnstructured(pod))
	assert.Nil(t, err)

	podNode = cluster.nodes[kube.GetResourceKey(mustToUnstructured(pod))]

	assert.NotNil(t, podNode)
	assert.Equal(t, []appv1.InfoItem{{Name: "Status Reason", Value: "ExitCode:-1"}, {Name: "Containers", Value: "0/1"}}, podNode.info)
}

func TestUpdateAppResource(t *testing.T) {
	updatesReceived := make([]string, 0)
	cluster := newCluster(testPod, testRS, testDeploy)
	cluster.onAppUpdated = func(appName string, fullRefresh bool) {
		updatesReceived = append(updatesReceived, fmt.Sprintf("%s: %v", appName, fullRefresh))
	}

	err := cluster.ensureSynced()
	assert.Nil(t, err)

	err = cluster.processEvent(watch.Modified, mustToUnstructured(testPod))
	assert.Nil(t, err)

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
