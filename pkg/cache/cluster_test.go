package cache

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
	testcore "k8s.io/client-go/testing"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/kube/kubetest"
)

func strToUnstructured(jsonStr string) *unstructured.Unstructured {
	obj := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(jsonStr), &obj)
	if err != nil {
		panic(err)
	}
	return &unstructured.Unstructured{Object: obj}
}

var (
	testCreationTime, _ = time.Parse(time.RFC3339, "2018-09-20T06:47:27Z")
	testPod             = strToUnstructured(fmt.Sprintf(`
  apiVersion: v1
  kind: Pod
  metadata:
    uid: "1"
    creationTimestamp: "%s"
    name: helm-guestbook-pod
    namespace: default
    ownerReferences:
    - apiVersion: apps/v1
      kind: ReplicaSet
      name: helm-guestbook-rs
      uid: "2"
    resourceVersion: "123"`, testCreationTime.UTC().Format(time.RFC3339)))

	testRS = strToUnstructured(fmt.Sprintf(`
  apiVersion: apps/v1
  kind: ReplicaSet
  metadata:
    uid: "2"
    creationTimestamp: "%s"
    name: helm-guestbook-rs
    namespace: default
    annotations:
      deployment.kubernetes.io/revision: "2"
    ownerReferences:
    - apiVersion: apps/v1beta1
      kind: Deployment
      name: helm-guestbook
      uid: "3"
    resourceVersion: "123"`, testCreationTime.UTC().Format(time.RFC3339)))

	testDeploy = strToUnstructured(fmt.Sprintf(`
  apiVersion: apps/v1
  kind: Deployment
  metadata:
    labels:
      app.kubernetes.io/instance: helm-guestbook
    uid: "3"
    creationTimestamp: "%s"
    name: helm-guestbook
    namespace: default
    resourceVersion: "123"`, testCreationTime.UTC().Format(time.RFC3339)))

	testService = strToUnstructured(fmt.Sprintf(`
  apiVersion: v1
  kind: Service
  metadata:
    name: helm-guestbook
    namespace: default
    resourceVersion: "123"
    uid: "4"
    creationTimestamp: "%s"
  spec:
    selector:
      app: guestbook
    type: LoadBalancer
  status:
    loadBalancer:
      ingress:
      - hostname: localhost`, testCreationTime.UTC().Format(time.RFC3339)))
)

func newCluster(objs ...*unstructured.Unstructured) *clusterCache {
	runtimeObjs := make([]runtime.Object, len(objs))
	for i := range objs {
		runtimeObjs[i] = objs[i]
	}
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme, runtimeObjs...)
	reactor := client.ReactionChain[0]
	client.PrependReactor("list", "*", func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
		handled, ret, err = reactor.React(action)
		// make sure list response have resource version
		if list, ok := ret.(*unstructured.UnstructuredList); ok {
			list.SetResourceVersion("123")
		}
		return
	})

	apiResources := []kube.APIResourceInfo{{
		GroupKind:            schema.GroupKind{Group: "", Kind: "Pod"},
		GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		Meta:                 metav1.APIResource{Namespaced: true},
	}, {
		GroupKind:            schema.GroupKind{Group: "apps", Kind: "ReplicaSet"},
		GroupVersionResource: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"},
		Meta:                 metav1.APIResource{Namespaced: true},
	}, {
		GroupKind:            schema.GroupKind{Group: "apps", Kind: "Deployment"},
		GroupVersionResource: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		Meta:                 metav1.APIResource{Namespaced: true},
	}}

	return NewClusterCache(
		&rest.Config{Host: "https://test"}, SetKubectl(&kubetest.MockKubectlCmd{APIResources: apiResources, DynamicClient: client}))
}

func getChildren(cluster *clusterCache, un *unstructured.Unstructured) []*Resource {
	hierarchy := make([]*Resource, 0)
	cluster.IterateHierarchy(kube.GetResourceKey(un), func(child *Resource, _ map[kube.ResourceKey]*Resource) {
		hierarchy = append(hierarchy, child)
	})
	return hierarchy[1:]
}

func TestEnsureSynced(t *testing.T) {
	obj1 := strToUnstructured(`
  apiVersion: apps/v1
  kind: Deployment
  metadata: {"name": "helm-guestbook1", "namespace": "default1"}
`)
	obj2 := strToUnstructured(`
  apiVersion: apps/v1
  kind: Deployment
  metadata: {"name": "helm-guestbook2", "namespace": "default2"}
`)

	cluster := newCluster(obj1, obj2)
	err := cluster.EnsureSynced()
	assert.Nil(t, err)

	assert.Len(t, cluster.resources, 2)
	var names []string
	for k := range cluster.resources {
		names = append(names, k.Name)
	}
	assert.ElementsMatch(t, []string{"helm-guestbook1", "helm-guestbook2"}, names)
}

func TestEnsureSyncedSingleNamespace(t *testing.T) {
	obj1 := strToUnstructured(`
  apiVersion: apps/v1
  kind: Deployment
  metadata: {"name": "helm-guestbook1", "namespace": "default1"}
`)
	obj2 := strToUnstructured(`
  apiVersion: apps/v1
  kind: Deployment
  metadata: {"name": "helm-guestbook2", "namespace": "default2"}
`)

	cluster := newCluster(obj1, obj2)
	cluster.namespaces = []string{"default1"}
	err := cluster.EnsureSynced()
	assert.Nil(t, err)

	assert.Len(t, cluster.resources, 1)
	var names []string
	for k := range cluster.resources {
		names = append(names, k.Name)
	}
	assert.ElementsMatch(t, []string{"helm-guestbook1"}, names)
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
	err := cluster.EnsureSynced()
	assert.Nil(t, err)

	resources := cluster.GetNamespaceTopLevelResources("default")
	assert.Len(t, resources, 2)
	assert.Equal(t, resources[kube.GetResourceKey(defaultNamespaceTopLevel1)].Ref.Name, "helm-guestbook1")
	assert.Equal(t, resources[kube.GetResourceKey(defaultNamespaceTopLevel2)].Ref.Name, "helm-guestbook2")

	resources = cluster.GetNamespaceTopLevelResources("kube-system")
	assert.Len(t, resources, 1)
	assert.Equal(t, resources[kube.GetResourceKey(kubesystemNamespaceTopLevel2)].Ref.Name, "helm-guestbook3")
}

func TestGetChildren(t *testing.T) {
	cluster := newCluster(testPod, testRS, testDeploy)
	err := cluster.EnsureSynced()
	assert.Nil(t, err)

	rsChildren := getChildren(cluster, testRS)
	assert.Equal(t, []*Resource{{
		Ref: corev1.ObjectReference{
			Kind:       "Pod",
			Namespace:  "default",
			Name:       "helm-guestbook-pod",
			APIVersion: "v1",
			UID:        "1",
		},
		OwnerRefs: []metav1.OwnerReference{{
			APIVersion: "apps/v1",
			Kind:       "ReplicaSet",
			Name:       "helm-guestbook-rs",
			UID:        "2",
		}},
		ResourceVersion: "123",
		CreationTimestamp: &metav1.Time{
			Time: testCreationTime.Local(),
		},
	}}, rsChildren)
	deployChildren := getChildren(cluster, testDeploy)

	assert.Equal(t, append([]*Resource{{
		Ref: corev1.ObjectReference{
			Kind:       "ReplicaSet",
			Namespace:  "default",
			Name:       "helm-guestbook-rs",
			APIVersion: "apps/v1",
			UID:        "2",
		},
		ResourceVersion: "123",
		OwnerRefs:       []metav1.OwnerReference{{APIVersion: "apps/v1beta1", Kind: "Deployment", Name: "helm-guestbook", UID: "3"}},
		CreationTimestamp: &metav1.Time{
			Time: testCreationTime.Local(),
		},
	}}, rsChildren...), deployChildren)
}

func TestGetManagedLiveObjs(t *testing.T) {
	cluster := newCluster(testPod, testRS, testDeploy)
	cluster.Invalidate(SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, isRoot bool) (info interface{}, cacheManifest bool) {
		return nil, true
	}))

	err := cluster.EnsureSynced()
	assert.Nil(t, err)

	targetDeploy := strToUnstructured(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: helm-guestbook
  labels:
    app: helm-guestbook`)

	managedObjs, err := cluster.GetManagedLiveObjs([]*unstructured.Unstructured{targetDeploy}, func(r *Resource) bool {
		return len(r.OwnerRefs) == 0
	})
	assert.Nil(t, err)
	assert.Equal(t, managedObjs, map[kube.ResourceKey]*unstructured.Unstructured{
		kube.NewResourceKey("apps", "Deployment", "default", "helm-guestbook"): testDeploy,
	})
}

func TestChildDeletedEvent(t *testing.T) {
	cluster := newCluster(testPod, testRS, testDeploy)
	err := cluster.EnsureSynced()
	assert.Nil(t, err)

	cluster.processEvent(watch.Deleted, testPod)

	rsChildren := getChildren(cluster, testRS)
	assert.Equal(t, []*Resource{}, rsChildren)
}

func TestProcessNewChildEvent(t *testing.T) {
	cluster := newCluster(testPod, testRS, testDeploy)
	err := cluster.EnsureSynced()
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
		return strings.Compare(rsChildren[i].Ref.Name, rsChildren[j].Ref.Name) < 0
	})
	assert.Equal(t, []*Resource{{
		Ref: corev1.ObjectReference{
			Kind:       "Pod",
			Namespace:  "default",
			Name:       "helm-guestbook-pod",
			APIVersion: "v1",
			UID:        "1",
		},
		OwnerRefs: []metav1.OwnerReference{{
			APIVersion: "apps/v1",
			Kind:       "ReplicaSet",
			Name:       "helm-guestbook-rs",
			UID:        "2",
		}},
		ResourceVersion: "123",
		CreationTimestamp: &metav1.Time{
			Time: testCreationTime.Local(),
		},
	}, {
		Ref: corev1.ObjectReference{
			Kind:       "Pod",
			Namespace:  "default",
			Name:       "helm-guestbook-pod2",
			APIVersion: "v1",
			UID:        "4",
		},
		OwnerRefs: []metav1.OwnerReference{{
			APIVersion: "apps/v1",
			Kind:       "ReplicaSet",
			Name:       "helm-guestbook-rs",
			UID:        "2",
		}},
		ResourceVersion: "123",
	}}, rsChildren)
}

func TestWatchCacheUpdated(t *testing.T) {
	removed := testPod.DeepCopy()
	removed.SetName(testPod.GetName() + "-removed-pod")

	updated := testPod.DeepCopy()
	updated.SetName(testPod.GetName() + "-updated-pod")
	updated.SetResourceVersion("updated-pod-version")

	cluster := newCluster(removed, updated)
	err := cluster.EnsureSynced()

	assert.Nil(t, err)

	added := testPod.DeepCopy()
	added.SetName(testPod.GetName() + "-new-pod")

	podGroupKind := testPod.GroupVersionKind().GroupKind()

	cluster.lock.Lock()
	cluster.replaceResourceCache(podGroupKind, []unstructured.Unstructured{*updated, *added}, "")

	_, ok := cluster.resources[kube.GetResourceKey(removed)]
	assert.False(t, ok)
}

func TestNamespaceModeReplace(t *testing.T) {
	ns1Pod := testPod.DeepCopy()
	ns1Pod.SetNamespace("ns1")
	ns1Pod.SetName("pod1")

	ns2Pod := testPod.DeepCopy()
	ns2Pod.SetNamespace("ns2")
	podGroupKind := testPod.GroupVersionKind().GroupKind()

	cluster := newCluster(ns1Pod, ns2Pod)
	err := cluster.EnsureSynced()
	assert.Nil(t, err)

	cluster.replaceResourceCache(podGroupKind, nil, "ns1")

	_, ok := cluster.resources[kube.GetResourceKey(ns1Pod)]
	assert.False(t, ok)

	_, ok = cluster.resources[kube.GetResourceKey(ns2Pod)]
	assert.True(t, ok)
}

func TestGetDuplicatedChildren(t *testing.T) {
	extensionsRS := testRS.DeepCopy()
	extensionsRS.SetGroupVersionKind(schema.GroupVersionKind{Group: "extensions", Kind: kube.ReplicaSetKind, Version: "v1beta1"})
	cluster := newCluster(testDeploy, testRS, extensionsRS)
	err := cluster.EnsureSynced()

	assert.Nil(t, err)

	// Get children multiple times to make sure the right child is picked up every time.
	for i := 0; i < 5; i++ {
		children := getChildren(cluster, testDeploy)
		assert.Len(t, children, 1)
		assert.Equal(t, "apps/v1", children[0].Ref.APIVersion)
		assert.Equal(t, kube.ReplicaSetKind, children[0].Ref.Kind)
		assert.Equal(t, testRS.GetName(), children[0].Ref.Name)
	}
}

func ExampleNewClusterCache_inspectNamespaceResources() {
	// kubernetes cluster config here
	config := &rest.Config{}

	clusterCache := NewClusterCache(config,
		// cache default namespace only
		SetNamespaces([]string{"default", "kube-system"}),
		// configure custom logic to cache resources manifest and additional metadata
		SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, isRoot bool) (info interface{}, cacheManifest bool) {
			// if resource belongs to 'extensions' group then mark if with 'deprecated' label
			if un.GroupVersionKind().Group == "extensions" {
				info = []string{"deprecated"}
			}
			_, ok := un.GetLabels()["acme.io/my-label"]
			// cache whole manifest if resource has label
			cacheManifest = ok
			return
		}),
	)
	// Ensure cluster is synced before using it
	if err := clusterCache.EnsureSynced(); err != nil {
		panic(err)
	}
	// Iterate default namespace resources tree
	for _, root := range clusterCache.GetNamespaceTopLevelResources("default") {
		clusterCache.IterateHierarchy(root.ResourceKey(), func(resource *Resource, _ map[kube.ResourceKey]*Resource) {
			println(fmt.Sprintf("resource: %s, info: %v", resource.Ref.String(), resource.Info))
		})
	}
}

func ExampleNewClusterCache_resourceUpdatedEvents() {
	// kubernetes cluster config here
	config := &rest.Config{}

	clusterCache := NewClusterCache(config)
	// Ensure cluster is synced before using it
	if err := clusterCache.EnsureSynced(); err != nil {
		panic(err)
	}
	unsubscribe := clusterCache.OnResourceUpdated(func(newRes *Resource, oldRes *Resource, _ map[kube.ResourceKey]*Resource) {
		if newRes == nil {
			println(fmt.Sprintf("%s deleted", oldRes.Ref.String()))
		} else if oldRes == nil {
			println(fmt.Sprintf("%s created", newRes.Ref.String()))
		} else {
			println(fmt.Sprintf("%s updated", newRes.Ref.String()))
		}
	})
	defer unsubscribe()
	// observe resource modifications for 1 minute
	time.Sleep(time.Minute)
}
