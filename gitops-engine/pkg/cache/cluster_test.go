package cache

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sync/semaphore"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	testcore "k8s.io/client-go/testing"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/kube/kubetest"
)

func mustToUnstructured(obj any) *unstructured.Unstructured {
	un, err := kube.ToUnstructured(obj)
	if err != nil {
		panic(err)
	}
	return un
}

func strToUnstructured(jsonStr string) *unstructured.Unstructured {
	obj := make(map[string]any)
	err := yaml.Unmarshal([]byte(jsonStr), &obj)
	if err != nil {
		panic(err)
	}
	return &unstructured.Unstructured{Object: obj}
}

var (
	testCreationTime, _ = time.Parse(time.RFC3339, "2018-09-20T06:47:27Z")

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

func newCluster(tb testing.TB, objs ...runtime.Object) *clusterCache {
	tb.Helper()
	cache := newClusterWithOptions(tb, []UpdateSettingsFunc{}, objs...)

	tb.Cleanup(func() {
		cache.Invalidate()
	})

	return cache
}

func newClusterWithOptions(_ testing.TB, opts []UpdateSettingsFunc, objs ...runtime.Object) *clusterCache {
	client := fake.NewSimpleDynamicClient(scheme.Scheme, objs...)
	reactor := client.ReactionChain[0]
	client.PrependReactor("list", "*", func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
		handled, ret, err = reactor.React(action)
		if err != nil || !handled {
			return handled, ret, fmt.Errorf("reactor failed: %w", err)
		}
		// make sure list response have resource version
		ret.(metav1.ListInterface).SetResourceVersion("123")
		return handled, ret, nil
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
	}, {
		GroupKind:            schema.GroupKind{Group: "apps", Kind: "StatefulSet"},
		GroupVersionResource: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"},
		Meta:                 metav1.APIResource{Namespaced: true},
	}, {
		GroupKind:            schema.GroupKind{Group: "extensions", Kind: "ReplicaSet"},
		GroupVersionResource: schema.GroupVersionResource{Group: "extensions", Version: "v1beta1", Resource: "replicasets"},
		Meta:                 metav1.APIResource{Namespaced: true},
	}}

	opts = append([]UpdateSettingsFunc{
		SetKubectl(&kubetest.MockKubectlCmd{APIResources: apiResources, DynamicClient: client}),
	}, opts...)

	cache := NewClusterCache(
		&rest.Config{Host: "https://test"},
		opts...,
	)
	return cache
}

func (c *clusterCache) WithAPIResources(newApiResources []kube.APIResourceInfo) *clusterCache {
	apiResources := c.kubectl.(*kubetest.MockKubectlCmd).APIResources
	apiResources = append(apiResources, newApiResources...)
	c.kubectl.(*kubetest.MockKubectlCmd).APIResources = apiResources
	return c
}

func getChildren(cluster *clusterCache, un *unstructured.Unstructured) []*Resource {
	hierarchy := make([]*Resource, 0)
	cluster.IterateHierarchyV2([]kube.ResourceKey{kube.GetResourceKey(un)}, func(child *Resource, _ map[kube.ResourceKey]*Resource) bool {
		hierarchy = append(hierarchy, child)
		return true
	}, "")
	return hierarchy[1:]
}

// Benchmark_sync is meant to simulate cluster initialization when populateResourceInfoHandler does nontrivial work.
func Benchmark_sync(t *testing.B) {
	resources := []runtime.Object{}
	for i := 0; i < 100; i++ {
		resources = append(resources, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("pod-%d", i),
				Namespace: "default",
			},
		}, &appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("rs-%d", i),
				Namespace: "default",
			},
		}, &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("deploy-%d", i),
				Namespace: "default",
			},
		}, &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("sts-%d", i),
				Namespace: "default",
			},
		})
	}

	c := newCluster(t, resources...)

	c.populateResourceInfoHandler = func(_ *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {
		time.Sleep(10 * time.Microsecond)
		return nil, false
	}

	t.ResetTimer()

	for n := 0; n < t.N; n++ {
		err := c.sync()
		require.NoError(t, err)
	}
}

func TestEnsureSynced(t *testing.T) {
	obj1 := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "helm-guestbook1",
			Namespace: "default1",
		},
	}
	obj2 := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "helm-guestbook2",
			Namespace: "default2",
		},
	}

	cluster := newCluster(t, obj1, obj2)
	err := cluster.EnsureSynced()
	require.NoError(t, err)

	cluster.lock.Lock()
	defer cluster.lock.Unlock()

	assert.Len(t, cluster.resources, 2)
	var names []string
	for k := range cluster.resources {
		names = append(names, k.Name)
	}
	assert.ElementsMatch(t, []string{"helm-guestbook1", "helm-guestbook2"}, names)
}

func TestStatefulSetOwnershipInferred(t *testing.T) {
	var opts []UpdateSettingsFunc
	opts = append(opts, func(c *clusterCache) {
		c.batchEventsProcessing = true
		c.eventProcessingInterval = 1 * time.Millisecond
	})

	sts := &appsv1.StatefulSet{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: kube.StatefulSetKind},
		ObjectMeta: metav1.ObjectMeta{UID: "123", Name: "web", Namespace: "default"},
		Spec: appsv1.StatefulSetSpec{
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "www",
				},
			}},
		},
	}

	tests := []struct {
		name          string
		cluster       *clusterCache
		pvc           *corev1.PersistentVolumeClaim
		expectedRefs  []metav1.OwnerReference
		expectNoOwner bool
	}{
		{
			name:    "STSTemplateNameNotMatching",
			cluster: newCluster(t, sts),
			pvc: &corev1.PersistentVolumeClaim{
				TypeMeta:   metav1.TypeMeta{Kind: kube.PersistentVolumeClaimKind},
				ObjectMeta: metav1.ObjectMeta{Name: "www1-web-0", Namespace: "default"},
			},
			expectNoOwner: true,
		},
		{
			name:    "MatchingSTSExists",
			cluster: newCluster(t, sts),
			pvc: &corev1.PersistentVolumeClaim{
				TypeMeta:   metav1.TypeMeta{Kind: kube.PersistentVolumeClaimKind},
				ObjectMeta: metav1.ObjectMeta{Name: "www-web-0", Namespace: "default"},
			},
			expectedRefs: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: kube.StatefulSetKind, Name: "web", UID: "123"}},
		},
		{
			name:    "STSTemplateNameNotMatchingWithBatchProcessing",
			cluster: newClusterWithOptions(t, opts, sts),
			pvc: &corev1.PersistentVolumeClaim{
				TypeMeta:   metav1.TypeMeta{Kind: kube.PersistentVolumeClaimKind},
				ObjectMeta: metav1.ObjectMeta{Name: "www1-web-0", Namespace: "default"},
			},
			expectNoOwner: true,
		},
		{
			name:    "MatchingSTSExistsWithBatchProcessing",
			cluster: newClusterWithOptions(t, opts, sts),
			pvc: &corev1.PersistentVolumeClaim{
				TypeMeta:   metav1.TypeMeta{Kind: kube.PersistentVolumeClaimKind},
				ObjectMeta: metav1.ObjectMeta{Name: "www-web-0", Namespace: "default"},
			},
			expectedRefs: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: kube.StatefulSetKind, Name: "web", UID: "123"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cluster.EnsureSynced()
			require.NoError(t, err)

			pvc := mustToUnstructured(tc.pvc)
			tc.cluster.recordEvent(watch.Added, pvc)

			require.Eventually(t, func() bool {
				tc.cluster.lock.Lock()
				defer tc.cluster.lock.Unlock()

				resource := tc.cluster.resources[kube.GetResourceKey(pvc)]
				if resource == nil {
					return false // Resource not ready yet, keep retrying
				}
				refs := resource.OwnerRefs
				if tc.expectNoOwner {
					return len(refs) == 0
				}
				return assert.ElementsMatch(t, refs, tc.expectedRefs)
			}, 5*time.Second, 20*time.Millisecond, "Expected PVC to have correct owner reference")
		})
	}
}

func TestEnsureSyncedSingleNamespace(t *testing.T) {
	obj1 := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "helm-guestbook1",
			Namespace: "default1",
		},
	}
	obj2 := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "helm-guestbook2",
			Namespace: "default2",
		},
	}

	cluster := newCluster(t, obj1, obj2)
	cluster.namespaces = []string{"default1"}
	err := cluster.EnsureSynced()
	require.NoError(t, err)

	cluster.lock.Lock()
	defer cluster.lock.Unlock()

	assert.Len(t, cluster.resources, 1)
	var names []string
	for k := range cluster.resources {
		names = append(names, k.Name)
	}
	assert.ElementsMatch(t, []string{"helm-guestbook1"}, names)
}

func TestGetChildren(t *testing.T) {
	cluster := newCluster(t, testPod1(), testRS(), testDeploy())
	err := cluster.EnsureSynced()
	require.NoError(t, err)

	rsChildren := getChildren(cluster, mustToUnstructured(testRS()))
	assert.Equal(t, []*Resource{{
		Ref: corev1.ObjectReference{
			Kind:       "Pod",
			Namespace:  "default",
			Name:       "helm-guestbook-pod-1",
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
	deployChildren := getChildren(cluster, mustToUnstructured(testDeploy()))

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
	cluster := newCluster(t, testPod1(), testRS(), testDeploy())
	cluster.Invalidate(SetPopulateResourceInfoHandler(func(_ *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {
		return nil, true
	}))

	err := cluster.EnsureSynced()
	require.NoError(t, err)

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
	require.NoError(t, err)
	assert.Equal(t, map[kube.ResourceKey]*unstructured.Unstructured{
		kube.NewResourceKey("apps", "Deployment", "default", "helm-guestbook"): mustToUnstructured(testDeploy()),
	}, managedObjs)
}

func TestGetManagedLiveObjsNamespacedModeClusterLevelResource(t *testing.T) {
	cluster := newCluster(t, testPod1(), testRS(), testDeploy())
	cluster.Invalidate(SetPopulateResourceInfoHandler(func(_ *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {
		return nil, true
	}))
	cluster.namespaces = []string{"default", "production"}

	err := cluster.EnsureSynced()
	require.NoError(t, err)

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
	assert.Nil(t, managedObjs)
	assert.EqualError(t, err, "cluster level Deployment \"helm-guestbook\" can not be managed when in namespaced mode")
}

func TestGetManagedLiveObjsNamespacedModeClusterLevelResource_ClusterResourceEnabled(t *testing.T) {
	cluster := newCluster(t, testPod1(), testRS(), testDeploy())
	cluster.Invalidate(SetPopulateResourceInfoHandler(func(_ *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {
		return nil, true
	}))
	cluster.namespaces = []string{"default", "production"}
	cluster.clusterResources = true

	err := cluster.EnsureSynced()
	require.NoError(t, err)

	clusterLevelRes := strToUnstructured(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: helm-guestbook
  labels:
    app: helm-guestbook`)

	cluster.clusterResources = true
	_, err = cluster.GetManagedLiveObjs([]*unstructured.Unstructured{clusterLevelRes}, func(r *Resource) bool {
		return len(r.OwnerRefs) == 0
	})
	require.NoError(t, err)

	otherNamespaceRes := strToUnstructured(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: helm-guestbook
  namespace: some-other-namespace
  labels:
    app: helm-guestbook`)

	cluster.clusterResources = true
	_, err = cluster.GetManagedLiveObjs([]*unstructured.Unstructured{otherNamespaceRes}, func(r *Resource) bool {
		return len(r.OwnerRefs) == 0
	})
	assert.EqualError(t, err, "namespace \"some-other-namespace\" for Deployment \"helm-guestbook\" is not managed")
}

func TestGetManagedLiveObjsAllNamespaces(t *testing.T) {
	cluster := newCluster(t, testPod1(), testRS(), testDeploy())
	cluster.Invalidate(SetPopulateResourceInfoHandler(func(_ *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {
		return nil, true
	}))
	cluster.namespaces = nil

	err := cluster.EnsureSynced()
	require.NoError(t, err)

	targetDeploy := strToUnstructured(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: helm-guestbook
  namespace: production
  labels:
    app: helm-guestbook`)

	managedObjs, err := cluster.GetManagedLiveObjs([]*unstructured.Unstructured{targetDeploy}, func(r *Resource) bool {
		return len(r.OwnerRefs) == 0
	})
	require.NoError(t, err)
	assert.Equal(t, map[kube.ResourceKey]*unstructured.Unstructured{
		kube.NewResourceKey("apps", "Deployment", "default", "helm-guestbook"): mustToUnstructured(testDeploy()),
	}, managedObjs)
}

func TestGetManagedLiveObjsValidNamespace(t *testing.T) {
	cluster := newCluster(t, testPod1(), testRS(), testDeploy())
	cluster.Invalidate(SetPopulateResourceInfoHandler(func(_ *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {
		return nil, true
	}))
	cluster.namespaces = []string{"default", "production"}

	err := cluster.EnsureSynced()
	require.NoError(t, err)

	targetDeploy := strToUnstructured(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: helm-guestbook
  namespace: production
  labels:
    app: helm-guestbook`)

	managedObjs, err := cluster.GetManagedLiveObjs([]*unstructured.Unstructured{targetDeploy}, func(r *Resource) bool {
		return len(r.OwnerRefs) == 0
	})
	require.NoError(t, err)
	assert.Equal(t, map[kube.ResourceKey]*unstructured.Unstructured{
		kube.NewResourceKey("apps", "Deployment", "default", "helm-guestbook"): mustToUnstructured(testDeploy()),
	}, managedObjs)
}

func TestGetManagedLiveObjsInvalidNamespace(t *testing.T) {
	cluster := newCluster(t, testPod1(), testRS(), testDeploy())
	cluster.Invalidate(SetPopulateResourceInfoHandler(func(_ *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {
		return nil, true
	}))
	cluster.namespaces = []string{"default", "develop"}

	err := cluster.EnsureSynced()
	require.NoError(t, err)

	targetDeploy := strToUnstructured(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: helm-guestbook
  namespace: production
  labels:
    app: helm-guestbook`)

	managedObjs, err := cluster.GetManagedLiveObjs([]*unstructured.Unstructured{targetDeploy}, func(r *Resource) bool {
		return len(r.OwnerRefs) == 0
	})
	assert.Nil(t, managedObjs)
	assert.EqualError(t, err, "namespace \"production\" for Deployment \"helm-guestbook\" is not managed")
}

func TestGetManagedLiveObjsFailedConversion(t *testing.T) {
	cronTabGroup := "stable.example.com"

	testCases := []struct {
		name                         string
		localConvertFails            bool
		expectConvertToVersionCalled bool
		expectGetResourceCalled      bool
	}{
		{
			name:                         "local convert fails, so GetResource is called",
			localConvertFails:            true,
			expectConvertToVersionCalled: true,
			expectGetResourceCalled:      true,
		},
		{
			name:                         "local convert succeeds, so GetResource is not called",
			localConvertFails:            false,
			expectConvertToVersionCalled: true,
			expectGetResourceCalled:      false,
		},
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase
		t.Run(testCaseCopy.name, func(t *testing.T) {
			err := apiextensions.AddToScheme(scheme.Scheme)
			require.NoError(t, err)
			cluster := newCluster(t, testCRD(), testCronTab()).
				WithAPIResources([]kube.APIResourceInfo{
					{
						GroupKind:            schema.GroupKind{Group: cronTabGroup, Kind: "CronTab"},
						GroupVersionResource: schema.GroupVersionResource{Group: cronTabGroup, Version: "v1", Resource: "crontabs"},
						Meta:                 metav1.APIResource{Namespaced: true},
					},
				})
			cluster.Invalidate(SetPopulateResourceInfoHandler(func(_ *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {
				return nil, true
			}))
			cluster.namespaces = []string{"default"}

			err = cluster.EnsureSynced()
			require.NoError(t, err)

			targetDeploy := strToUnstructured(`
apiVersion: stable.example.com/v1
kind: CronTab
metadata:
  name: test-crontab
  namespace: default`)

			convertToVersionWasCalled := false
			getResourceWasCalled := false
			cluster.kubectl.(*kubetest.MockKubectlCmd).
				WithConvertToVersionFunc(func(obj *unstructured.Unstructured, _ string, _ string) (*unstructured.Unstructured, error) {
					convertToVersionWasCalled = true

					if testCaseCopy.localConvertFails {
						return nil, errors.New("failed to convert resource client-side")
					}

					return obj, nil
				}).
				WithGetResourceFunc(func(_ context.Context, _ *rest.Config, _ schema.GroupVersionKind, _ string, _ string) (*unstructured.Unstructured, error) {
					getResourceWasCalled = true
					return testCronTab(), nil
				})

			managedObjs, err := cluster.GetManagedLiveObjs([]*unstructured.Unstructured{targetDeploy}, func(_ *Resource) bool {
				return true
			})
			require.NoError(t, err)
			assert.Equal(t, testCaseCopy.expectConvertToVersionCalled, convertToVersionWasCalled)
			assert.Equal(t, testCaseCopy.expectGetResourceCalled, getResourceWasCalled)
			assert.Equal(t, map[kube.ResourceKey]*unstructured.Unstructured{
				kube.NewResourceKey(cronTabGroup, "CronTab", "default", "test-crontab"): mustToUnstructured(testCronTab()),
			}, managedObjs)
		})
	}
}

func TestChildDeletedEvent(t *testing.T) {
	cluster := newCluster(t, testPod1(), testRS(), testDeploy())
	err := cluster.EnsureSynced()
	require.NoError(t, err)

	cluster.recordEvent(watch.Deleted, mustToUnstructured(testPod1()))

	rsChildren := getChildren(cluster, mustToUnstructured(testRS()))
	assert.Equal(t, []*Resource{}, rsChildren)
}

func TestProcessNewChildEvent(t *testing.T) {
	cluster := newCluster(t, testPod1(), testRS(), testDeploy())
	err := cluster.EnsureSynced()
	require.NoError(t, err)
	newPod := strToUnstructured(`
  apiVersion: v1
  kind: Pod
  metadata:
    uid: "5"
    name: helm-guestbook-pod-1-new
    namespace: default
    ownerReferences:
    - apiVersion: apps/v1
      kind: ReplicaSet
      name: helm-guestbook-rs
      uid: "2"
    resourceVersion: "123"`)

	cluster.recordEvent(watch.Added, newPod)

	rsChildren := getChildren(cluster, mustToUnstructured(testRS()))
	sort.Slice(rsChildren, func(i, j int) bool {
		return strings.Compare(rsChildren[i].Ref.Name, rsChildren[j].Ref.Name) < 0
	})
	assert.Equal(t, []*Resource{{
		Ref: corev1.ObjectReference{
			Kind:       "Pod",
			Namespace:  "default",
			Name:       "helm-guestbook-pod-1",
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
			Name:       "helm-guestbook-pod-1-new",
			APIVersion: "v1",
			UID:        "5",
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
	removed := testPod1()
	removed.SetName(removed.GetName() + "-removed-pod")

	updated := testPod1()
	updated.SetName(updated.GetName() + "-updated-pod")
	updated.SetResourceVersion("updated-pod-version")

	cluster := newCluster(t, removed, updated)
	err := cluster.EnsureSynced()

	require.NoError(t, err)

	added := testPod1()
	added.SetName(added.GetName() + "-new-pod")

	podGroupKind := testPod1().GroupVersionKind().GroupKind()

	cluster.lock.Lock()
	defer cluster.lock.Unlock()
	cluster.replaceResourceCache(podGroupKind, []*Resource{cluster.newResource(mustToUnstructured(updated)), cluster.newResource(mustToUnstructured(added))}, "")

	_, ok := cluster.resources[getResourceKey(t, removed)]
	assert.False(t, ok)
}

func TestNamespaceModeReplace(t *testing.T) {
	ns1Pod := testPod1()
	ns1Pod.SetNamespace("ns1")
	ns1Pod.SetName("pod1")

	ns2Pod := testPod1()
	ns2Pod.SetNamespace("ns2")
	podGroupKind := testPod1().GroupVersionKind().GroupKind()

	cluster := newCluster(t, ns1Pod, ns2Pod)
	err := cluster.EnsureSynced()
	require.NoError(t, err)

	cluster.lock.Lock()
	defer cluster.lock.Unlock()

	cluster.replaceResourceCache(podGroupKind, nil, "ns1")

	_, ok := cluster.resources[getResourceKey(t, ns1Pod)]
	assert.False(t, ok)

	_, ok = cluster.resources[getResourceKey(t, ns2Pod)]
	assert.True(t, ok)
}

func TestGetDuplicatedChildren(t *testing.T) {
	extensionsRS := testExtensionsRS()
	cluster := newCluster(t, testDeploy(), testRS(), extensionsRS)
	err := cluster.EnsureSynced()

	require.NoError(t, err)

	// Get children multiple times to make sure the right child is picked up every time.
	for i := 0; i < 5; i++ {
		children := getChildren(cluster, mustToUnstructured(testDeploy()))
		assert.Len(t, children, 1)
		assert.Equal(t, "apps/v1", children[0].Ref.APIVersion)
		assert.Equal(t, kube.ReplicaSetKind, children[0].Ref.Kind)
		assert.Equal(t, testRS().GetName(), children[0].Ref.Name)
	}
}

func TestGetClusterInfo(t *testing.T) {
	cluster := newCluster(t)
	cluster.apiResources = []kube.APIResourceInfo{{GroupKind: schema.GroupKind{Group: "test", Kind: "test kind"}}}
	cluster.serverVersion = "v1.16"
	info := cluster.GetClusterInfo()
	assert.Equal(t, ClusterInfo{
		Server:       cluster.config.Host,
		APIResources: cluster.apiResources,
		K8SVersion:   cluster.serverVersion,
	}, info)
}

func TestDeleteAPIResource(t *testing.T) {
	cluster := newCluster(t)
	cluster.apiResources = []kube.APIResourceInfo{{
		GroupKind:            schema.GroupKind{Group: "test", Kind: "test kind"},
		GroupVersionResource: schema.GroupVersionResource{Version: "v1"},
	}}

	cluster.deleteAPIResource(kube.APIResourceInfo{GroupKind: schema.GroupKind{Group: "wrong group", Kind: "wrong kind"}})
	assert.Len(t, cluster.apiResources, 1)
	cluster.deleteAPIResource(kube.APIResourceInfo{
		GroupKind:            schema.GroupKind{Group: "test", Kind: "test kind"},
		GroupVersionResource: schema.GroupVersionResource{Version: "wrong version"},
	})
	assert.Len(t, cluster.apiResources, 1)

	cluster.deleteAPIResource(kube.APIResourceInfo{
		GroupKind:            schema.GroupKind{Group: "test", Kind: "test kind"},
		GroupVersionResource: schema.GroupVersionResource{Version: "v1"},
	})
	assert.Empty(t, cluster.apiResources)
}

func TestAppendAPIResource(t *testing.T) {
	cluster := newCluster(t)

	resourceInfo := kube.APIResourceInfo{
		GroupKind:            schema.GroupKind{Group: "test", Kind: "test kind"},
		GroupVersionResource: schema.GroupVersionResource{Version: "v1"},
	}

	cluster.appendAPIResource(resourceInfo)
	assert.ElementsMatch(t, []kube.APIResourceInfo{resourceInfo}, cluster.apiResources)

	// make sure same group, kind version is not added twice
	cluster.appendAPIResource(resourceInfo)
	assert.ElementsMatch(t, []kube.APIResourceInfo{resourceInfo}, cluster.apiResources)
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
		switch {
		case newRes == nil:
			fmt.Printf("%s deleted\n", oldRes.Ref.String())
		case oldRes == nil:
			fmt.Printf("%s created\n", newRes.Ref.String())
		default:
			fmt.Printf("%s updated\n", newRes.Ref.String())
		}
	})
	defer unsubscribe()
	// observe resource modifications for 1 minute
	time.Sleep(time.Minute)
}

func getResourceKey(t *testing.T, obj runtime.Object) kube.ResourceKey {
	t.Helper()
	gvk := obj.GetObjectKind().GroupVersionKind()
	m, err := meta.Accessor(obj)
	require.NoError(t, err)
	return kube.NewResourceKey(gvk.Group, gvk.Kind, m.GetNamespace(), m.GetName())
}

func testPod1() *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "helm-guestbook-pod-1",
			Namespace:         "default",
			UID:               "1",
			ResourceVersion:   "123",
			CreationTimestamp: metav1.NewTime(testCreationTime),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "helm-guestbook-rs",
					UID:        "2",
				},
			},
		},
	}
}

// Similar to pod1, but owner reference lacks uid
func testPod2() *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "helm-guestbook-pod-2",
			Namespace:         "default",
			UID:               "4",
			ResourceVersion:   "123",
			CreationTimestamp: metav1.NewTime(testCreationTime),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "helm-guestbook-rs",
				},
			},
		},
	}
}

func testCRD() *apiextensions.CustomResourceDefinition {
	return &apiextensions.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "crontabs.stable.example.com",
		},
		Spec: apiextensions.CustomResourceDefinitionSpec{
			Group: "stable.example.com",
			Versions: []apiextensions.CustomResourceDefinitionVersion{
				{
					Name:    "v1",
					Served:  true,
					Storage: true,
					Schema: &apiextensions.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensions.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensions.JSONSchemaProps{
								"cronSpec": {Type: "string"},
								"image":    {Type: "string"},
								"replicas": {Type: "integer"},
							},
						},
					},
				},
			},
			Scope: "Namespaced",
			Names: apiextensions.CustomResourceDefinitionNames{
				Plural:     "crontabs",
				Singular:   "crontab",
				ShortNames: []string{"ct"},
				Kind:       "CronTab",
			},
		},
	}
}

func testCronTab() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "stable.example.com/v1",
		"kind":       "CronTab",
		"metadata": map[string]any{
			"name":      "test-crontab",
			"namespace": "default",
		},
		"spec": map[string]any{
			"cronSpec": "* * * * */5",
			"image":    "my-awesome-cron-image",
		},
	}}
}

func testExtensionsRS() *extensionsv1beta1.ReplicaSet {
	return &extensionsv1beta1.ReplicaSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions/v1beta1",
			Kind:       "ReplicaSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "helm-guestbook-rs",
			Namespace:         "default",
			UID:               "2",
			ResourceVersion:   "123",
			CreationTimestamp: metav1.NewTime(testCreationTime),
			Annotations: map[string]string{
				"deployment.kubernetes.io/revision": "2",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1beta1",
					Kind:       "Deployment",
					Name:       "helm-guestbook",
					UID:        "3",
				},
			},
		},
	}
}

func testRS() *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "ReplicaSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "helm-guestbook-rs",
			Namespace:         "default",
			UID:               "2",
			ResourceVersion:   "123",
			CreationTimestamp: metav1.NewTime(testCreationTime),
			Annotations: map[string]string{
				"deployment.kubernetes.io/revision": "2",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1beta1",
					Kind:       "Deployment",
					Name:       "helm-guestbook",
					UID:        "3",
				},
			},
		},
		Spec:   appsv1.ReplicaSetSpec{},
		Status: appsv1.ReplicaSetStatus{},
	}
}

func testDeploy() *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "helm-guestbook",
			Namespace:         "default",
			UID:               "3",
			ResourceVersion:   "123",
			CreationTimestamp: metav1.NewTime(testCreationTime),
			Labels: map[string]string{
				"app.kubernetes.io/instance": "helm-guestbook",
			},
		},
	}
}

func TestIterateHierarchyV2(t *testing.T) {
	cluster := newCluster(t, testPod1(), testPod2(), testRS(), testExtensionsRS(), testDeploy())
	err := cluster.EnsureSynced()
	require.NoError(t, err)

	t.Run("IterateAll", func(t *testing.T) {
		startKeys := []kube.ResourceKey{kube.GetResourceKey(mustToUnstructured(testDeploy()))}
		keys := []kube.ResourceKey{}
		cluster.IterateHierarchyV2(startKeys, func(child *Resource, _ map[kube.ResourceKey]*Resource) bool {
			keys = append(keys, child.ResourceKey())
			return true
		}, "")

		assert.ElementsMatch(t,
			[]kube.ResourceKey{
				kube.GetResourceKey(mustToUnstructured(testPod1())),
				kube.GetResourceKey(mustToUnstructured(testPod2())),
				kube.GetResourceKey(mustToUnstructured(testRS())),
				kube.GetResourceKey(mustToUnstructured(testDeploy())),
			},
			keys)
	})

	t.Run("ExitAtRoot", func(t *testing.T) {
		startKeys := []kube.ResourceKey{kube.GetResourceKey(mustToUnstructured(testDeploy()))}
		keys := []kube.ResourceKey{}
		cluster.IterateHierarchyV2(startKeys, func(child *Resource, _ map[kube.ResourceKey]*Resource) bool {
			keys = append(keys, child.ResourceKey())
			return false
		}, "")

		assert.ElementsMatch(t,
			[]kube.ResourceKey{
				kube.GetResourceKey(mustToUnstructured(testDeploy())),
			},
			keys)
	})

	t.Run("ExitAtSecondLevelChild", func(t *testing.T) {
		startKeys := []kube.ResourceKey{kube.GetResourceKey(mustToUnstructured(testDeploy()))}
		keys := []kube.ResourceKey{}
		cluster.IterateHierarchyV2(startKeys, func(child *Resource, _ map[kube.ResourceKey]*Resource) bool {
			keys = append(keys, child.ResourceKey())
			return child.ResourceKey().Kind != kube.ReplicaSetKind
		}, "")

		assert.ElementsMatch(t,
			[]kube.ResourceKey{
				kube.GetResourceKey(mustToUnstructured(testDeploy())),
				kube.GetResourceKey(mustToUnstructured(testRS())),
			},
			keys)
	})

	t.Run("ExitAtThirdLevelChild", func(t *testing.T) {
		startKeys := []kube.ResourceKey{kube.GetResourceKey(mustToUnstructured(testDeploy()))}
		keys := []kube.ResourceKey{}
		cluster.IterateHierarchyV2(startKeys, func(child *Resource, _ map[kube.ResourceKey]*Resource) bool {
			keys = append(keys, child.ResourceKey())
			return child.ResourceKey().Kind != kube.PodKind
		}, "")

		assert.ElementsMatch(t,
			[]kube.ResourceKey{
				kube.GetResourceKey(mustToUnstructured(testDeploy())),
				kube.GetResourceKey(mustToUnstructured(testRS())),
				kube.GetResourceKey(mustToUnstructured(testPod1())),
				kube.GetResourceKey(mustToUnstructured(testPod2())),
			},
			keys)
	})

	t.Run("IterateAllStartFromMultiple", func(t *testing.T) {
		startKeys := []kube.ResourceKey{
			kube.GetResourceKey(mustToUnstructured(testRS())),
			kube.GetResourceKey(mustToUnstructured(testDeploy())),
		}
		keys := []kube.ResourceKey{}
		cluster.IterateHierarchyV2(startKeys, func(child *Resource, _ map[kube.ResourceKey]*Resource) bool {
			keys = append(keys, child.ResourceKey())
			return true
		}, "")

		assert.ElementsMatch(t,
			[]kube.ResourceKey{
				kube.GetResourceKey(mustToUnstructured(testPod1())),
				kube.GetResourceKey(mustToUnstructured(testPod2())),
				kube.GetResourceKey(mustToUnstructured(testRS())),
				kube.GetResourceKey(mustToUnstructured(testDeploy())),
			},
			keys)
	})

	// After uid is backfilled for owner of pod2, it should appear in results here as well.
	t.Run("IterateStartFromExtensionsRS", func(t *testing.T) {
		startKeys := []kube.ResourceKey{kube.GetResourceKey(mustToUnstructured(testExtensionsRS()))}
		keys := []kube.ResourceKey{}
		cluster.IterateHierarchyV2(startKeys, func(child *Resource, _ map[kube.ResourceKey]*Resource) bool {
			keys = append(keys, child.ResourceKey())
			return true
		}, "")

		assert.ElementsMatch(t,
			[]kube.ResourceKey{
				kube.GetResourceKey(mustToUnstructured(testPod1())),
				kube.GetResourceKey(mustToUnstructured(testPod2())),
				kube.GetResourceKey(mustToUnstructured(testExtensionsRS())),
			},
			keys)
	})
}

func testClusterParent() *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-cluster-parent",
			UID:             "cluster-parent-123",
			ResourceVersion: "123",
		},
	}
}

func testNamespacedChild() *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "namespaced-child",
			Namespace:       "test-namespace",
			UID:             "namespaced-child-456",
			ResourceVersion: "123",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "Namespace",
				Name:       "test-cluster-parent",
				UID:        "cluster-parent-123",
			}},
		},
	}
}

func testClusterChild() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "cluster-child",
			UID:             "cluster-child-789",
			ResourceVersion: "123",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "Namespace",
				Name:       "test-cluster-parent",
				UID:        "cluster-parent-123",
			}},
		},
	}
}


func TestIterateHierarchyV2_ClusterScopedParentOnly_NoNamespaceScanning(t *testing.T) {
	// Test that without namespace scanning, only cluster-scoped children are found
	cluster := newCluster(t, testClusterParent(), testNamespacedChild(), testClusterChild()).WithAPIResources([]kube.APIResourceInfo{{
		GroupKind:            schema.GroupKind{Group: "", Kind: "Namespace"},
		GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
		Meta:                 metav1.APIResource{Namespaced: false},
	}, {
		GroupKind:            schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"},
		GroupVersionResource: schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
		Meta:                 metav1.APIResource{Namespaced: false},
	}})
	err := cluster.EnsureSynced()
	require.NoError(t, err)

	keys := []kube.ResourceKey{}
	// Only pass the cluster-scoped parent - without namespace scanning, should only find cluster-scoped children
	cluster.IterateHierarchyV2(
		[]kube.ResourceKey{kube.GetResourceKey(mustToUnstructured(testClusterParent()))},
		func(resource *Resource, _ map[kube.ResourceKey]*Resource) bool {
			t.Logf("Visiting resource: %v", resource.ResourceKey())
			keys = append(keys, resource.ResourceKey())
			return true
		},
		"", // No namespace scanning
	)

	// Without namespace scanning, should only find the parent and its cluster-scoped child
	expected := []kube.ResourceKey{
		kube.GetResourceKey(mustToUnstructured(testClusterParent())),
		kube.GetResourceKey(mustToUnstructured(testClusterChild())),
	}
	t.Logf("Expected: %v", expected)
	t.Logf("Actual: %v", keys)
	assert.ElementsMatch(t, expected, keys)
}

func TestIterateHierarchyV2_ClusterScopedParentOnly_WithNamespaceScanning(t *testing.T) {
	// Test that with namespace scanning, both cluster-scoped and namespaced children are found
	cluster := newCluster(t, testClusterParent(), testNamespacedChild(), testClusterChild()).WithAPIResources([]kube.APIResourceInfo{{
		GroupKind:            schema.GroupKind{Group: "", Kind: "Namespace"},
		GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
		Meta:                 metav1.APIResource{Namespaced: false},
	}, {
		GroupKind:            schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"},
		GroupVersionResource: schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
		Meta:                 metav1.APIResource{Namespaced: false},
	}})
	err := cluster.EnsureSynced()
	require.NoError(t, err)

	// Check what's in the namespace index
	t.Logf("Namespaces in index: %v", len(cluster.nsIndex))
	for ns, resources := range cluster.nsIndex {
		t.Logf("Namespace %q has %d resources", ns, len(resources))
		for key, res := range resources {
			t.Logf("  - %v with %d owner refs", key, len(res.OwnerRefs))
			for _, ref := range res.OwnerRefs {
				t.Logf("    owner: %s/%s name=%s uid=%s", ref.APIVersion, ref.Kind, ref.Name, ref.UID)
			}
		}
	}

	keys := []kube.ResourceKey{}
	// Only pass the cluster-scoped parent with namespace scanning enabled
	cluster.IterateHierarchyV2(
		[]kube.ResourceKey{kube.GetResourceKey(mustToUnstructured(testClusterParent()))},
		func(resource *Resource, _ map[kube.ResourceKey]*Resource) bool {
			t.Logf("Visiting resource: %v", resource.ResourceKey())
			keys = append(keys, resource.ResourceKey())
			return true
		},
		"test-namespace", // Scan for orphaned resources in test-namespace
	)

	// With namespace scanning, should find the parent, the namespaced child, and cluster child
	assert.ElementsMatch(t, []kube.ResourceKey{
		kube.GetResourceKey(mustToUnstructured(testClusterParent())),
		kube.GetResourceKey(mustToUnstructured(testNamespacedChild())), // in test-namespace
		kube.GetResourceKey(mustToUnstructured(testClusterChild())),
	}, keys)
}

func TestIterateHierarchyV2_ClusterScopedParentOnly_InferredUID(t *testing.T) {
	// Test that passing only a cluster-scoped parent finds children even with inferred UIDs.
	// This should never happen but we coded defensively for this case, and at worst it would link a child
	// to the wrong parent if there were multiple parents with the same name (i.e. deleted and recreated).
	namespacedChildNoUID := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "namespaced-child-no-uid",
			Namespace:       "test-namespace",
			UID:             "namespaced-child-789",
			ResourceVersion: "123",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "Namespace",
				Name:       "test-cluster-parent",
				// Note: No UID here - will need to be inferred
			}},
		},
	}

	cluster := newCluster(t, testClusterParent(), namespacedChildNoUID, testClusterChild()).WithAPIResources([]kube.APIResourceInfo{{
		GroupKind:            schema.GroupKind{Group: "", Kind: "Namespace"},
		GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
		Meta:                 metav1.APIResource{Namespaced: false},
	}, {
		GroupKind:            schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"},
		GroupVersionResource: schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
		Meta:                 metav1.APIResource{Namespaced: false},
	}})
	err := cluster.EnsureSynced()
	require.NoError(t, err)

	keys := []kube.ResourceKey{}
	// Test with all namespaces - need to pass both cluster parent and namespaced children
	// as explicit keys to find them all
	cluster.IterateHierarchyV2(
		[]kube.ResourceKey{
			kube.GetResourceKey(mustToUnstructured(testClusterParent())),
			kube.GetResourceKey(mustToUnstructured(namespacedChildNoUID)),
		},
		func(resource *Resource, _ map[kube.ResourceKey]*Resource) bool {
			keys = append(keys, resource.ResourceKey())
			return true
		},
		"", // No orphaned resource namespace
	)

	// Should find the parent and all its children, even with inferred UID
	assert.ElementsMatch(t, []kube.ResourceKey{
		kube.GetResourceKey(mustToUnstructured(testClusterParent())),
		kube.GetResourceKey(mustToUnstructured(namespacedChildNoUID)),
		kube.GetResourceKey(mustToUnstructured(testClusterChild())),
	}, keys)
}

func TestIterateHierarchyV2_DisabledClusterScopedParents(t *testing.T) {
	t.Setenv("GITOPS_ENGINE_DISABLE_CLUSTER_SCOPED_PARENT_REFS", "1")

	cluster := newCluster(t, testClusterParent(), testNamespacedChild()).WithAPIResources([]kube.APIResourceInfo{{
		GroupKind:            schema.GroupKind{Group: "", Kind: "Namespace"},
		GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
		Meta:                 metav1.APIResource{Namespaced: false},
	}})
	err := cluster.EnsureSynced()
	require.NoError(t, err)

	keys := []kube.ResourceKey{}
	cluster.IterateHierarchyV2(
		[]kube.ResourceKey{kube.GetResourceKey(mustToUnstructured(testClusterParent()))},
		func(resource *Resource, _ map[kube.ResourceKey]*Resource) bool {
			keys = append(keys, resource.ResourceKey())
			return true
		},
		"", // No orphaned resource namespace
	)

	// When disabled, should only visit the parent
	assert.Equal(t, []kube.ResourceKey{kube.GetResourceKey(mustToUnstructured(testClusterParent()))}, keys)
}

// Test_watchEvents_Deadlock validates that starting watches will not create a deadlock
// caused by using improper locking in various callback methods when there is a high load on the
// system.
func Test_watchEvents_Deadlock(t *testing.T) {
	// deadlock lock is used to simulate a user function calling the cluster cache while holding a lock
	// and using this lock in callbacks such as OnPopulateResourceInfoHandler.
	deadlock := sync.RWMutex{}

	hasDeadlock := false
	res1 := testPod1()
	res2 := testRS()

	cluster := newClusterWithOptions(t, []UpdateSettingsFunc{
		// Set low blocking semaphore
		SetListSemaphore(semaphore.NewWeighted(1)),
		// Resync watches often to use the semaphore and trigger the rate limiting behavior
		SetResyncTimeout(500 * time.Millisecond),
		// Use new resource handler to run code in the list callbacks
		SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {
			if un.GroupVersionKind().GroupKind() == res1.GroupVersionKind().GroupKind() ||
				un.GroupVersionKind().GroupKind() == res2.GroupVersionKind().GroupKind() {
				// Create a bottleneck for resources holding the semaphore
				time.Sleep(2 * time.Second)
			}

			//// Uncommenting the following code will simulate a different deadlock on purpose caused by
			//// client code holding a lock and trying to acquire the same lock in the event callback.
			//// It provides an easy way to validate if the test detect deadlocks as expected.
			//// If the test fails with this code commented, a deadlock do exist in the codebase.
			// deadlock.RLock()
			// defer deadlock.RUnlock()

			return
		}),
	}, res1, res2, testDeploy())
	defer func() {
		// Invalidate() is a blocking method and cannot be called safely in case of deadlock
		if !hasDeadlock {
			cluster.Invalidate()
		}
	}()

	err := cluster.EnsureSynced()
	require.NoError(t, err)

	for i := 0; i < 2; i++ {
		done := make(chan bool, 1)
		go func() {
			// Stop the watches, so startMissingWatches will restart them
			cluster.stopWatching(res1.GroupVersionKind().GroupKind(), res1.Namespace)
			cluster.stopWatching(res2.GroupVersionKind().GroupKind(), res2.Namespace)

			// calling startMissingWatches to simulate that a CRD event was received
			// TODO: how to simulate real watch events and test the full watchEvents function?
			err = runSynced(&cluster.lock, func() error {
				deadlock.Lock()
				defer deadlock.Unlock()
				return cluster.startMissingWatches()
			})
			require.NoError(t, err)
			done <- true
		}()
		select {
		case v := <-done:
			require.True(t, v)
		case <-time.After(10 * time.Second):
			hasDeadlock = true
			t.Errorf("timeout reached on attempt %d. It is possible that a deadlock occurred", i)
			// Tip: to debug the deadlock, increase the timer to a value higher than X in "go test -timeout X"
			// This will make the test panic with the goroutines information
			t.FailNow()
		}
	}
}

func buildTestResourceMap() map[kube.ResourceKey]*Resource {
	ns := make(map[kube.ResourceKey]*Resource)
	for i := 0; i < 100000; i++ {
		name := fmt.Sprintf("test-%d", i)
		ownerName := fmt.Sprintf("test-%d", i/10)
		uid := uuid.New().String()
		key := kube.ResourceKey{
			Namespace: "default",
			Name:      name,
			Kind:      "Pod",
		}
		resourceYaml := fmt.Sprintf(`
apiVersion: v1
kind: Pod
metadata:
  namespace: default
  name: %s
  uid: %s`, name, uid)
		if i/10 != 0 {
			owner := ns[kube.ResourceKey{
				Namespace: "default",
				Name:      ownerName,
				Kind:      "Pod",
			}]
			ownerUid := owner.Ref.UID
			resourceYaml += fmt.Sprintf(`
  ownerReferences:
  - apiVersion: v1
    kind: Pod
    name: %s
    uid: %s`, ownerName, ownerUid)
		}
		ns[key] = cacheTest.newResource(strToUnstructured(resourceYaml))
	}
	return ns
}

func BenchmarkBuildGraph(b *testing.B) {
	testResources := buildTestResourceMap()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		buildGraph(testResources)
	}
}

func BenchmarkIterateHierarchyV2(b *testing.B) {
	cluster := newCluster(b)
	testResources := buildTestResourceMap()
	for _, resource := range testResources {
		cluster.setNode(resource)
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		cluster.IterateHierarchyV2([]kube.ResourceKey{
			{Namespace: "default", Name: "test-1", Kind: "Pod"},
		}, func(_ *Resource, _ map[kube.ResourceKey]*Resource) bool {
			return true
		}, "")
	}
}

func buildParameterizedCrossNamespaceTestResourceMap(clusterParents, regularPods, crossNamespacePods int, includeUIDs bool) map[kube.ResourceKey]*Resource {
	resources := make(map[kube.ResourceKey]*Resource)
	clusterParentUIDs := make(map[string]string) // Map cluster role names to UIDs (when includeUIDs is true)

	// Create cluster-scoped parents (ClusterRoles)
	for i := 0; i < clusterParents; i++ {
		clusterRoleName := fmt.Sprintf("cluster-role-%d", i)
		uid := uuid.New().String()
		if includeUIDs {
			clusterParentUIDs[clusterRoleName] = uid // Store UID for later reference
		}

		key := kube.ResourceKey{
			Group:     "rbac.authorization.k8s.io",
			Kind:      "ClusterRole",
			Namespace: "", // cluster-scoped
			Name:      clusterRoleName,
		}

		resourceYaml := fmt.Sprintf(`
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: %s
  uid: %s
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]`, clusterRoleName, uid)

		resources[key] = cacheTest.newResource(strToUnstructured(resourceYaml))
	}

	// Create regular namespaced resources (Pods)
	namespaces := []string{"default", "kube-system", "test-ns-1", "test-ns-2", "test-ns-3"}
	for i := 0; i < regularPods; i++ {
		name := fmt.Sprintf("pod-%d", i)
		ownerName := fmt.Sprintf("pod-%d", i/10)
		namespace := namespaces[i%len(namespaces)]
		uid := uuid.New().String()

		key := kube.ResourceKey{
			Namespace: namespace,
			Name:      name,
			Kind:      "Pod",
		}

		resourceYaml := fmt.Sprintf(`
apiVersion: v1
kind: Pod
metadata:
  namespace: %s
  name: %s
  uid: %s`, namespace, name, uid)

		// Add owner references for hierarchical structure (similar to regular benchmark)
		if i/10 != 0 {
			ownerKey := kube.ResourceKey{
				Namespace: namespace,
				Name:      ownerName,
				Kind:      "Pod",
			}
			if owner, exists := resources[ownerKey]; exists {
				ownerUid := owner.Ref.UID
				resourceYaml += fmt.Sprintf(`
  ownerReferences:
  - apiVersion: v1
    kind: Pod
    name: %s
    uid: %s`, ownerName, ownerUid)
			}
		}

		resources[key] = cacheTest.newResource(strToUnstructured(resourceYaml))
	}

	// Create cross-namespace children (Pods) that reference cluster-scoped parents (ClusterRoles)
	for i := 0; i < crossNamespacePods; i++ {
		podName := fmt.Sprintf("cross-ns-pod-%d", i)
		namespace := namespaces[i%len(namespaces)]
		clusterRoleIndex := i % clusterParents // Reference one of the cluster roles
		clusterRoleName := fmt.Sprintf("cluster-role-%d", clusterRoleIndex)
		uid := uuid.New().String()

		key := kube.ResourceKey{
			Namespace: namespace,
			Name:      podName,
			Kind:      "Pod",
		}

		var resourceYaml string
		if includeUIDs {
			parentUID := clusterParentUIDs[clusterRoleName] // Get the parent's UID
			resourceYaml = fmt.Sprintf(`
apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
  uid: %s
  ownerReferences:
  - apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    name: %s
    uid: %s`, podName, namespace, uid, clusterRoleName, parentUID)
		} else {
			resourceYaml = fmt.Sprintf(`
apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
  uid: %s
  ownerReferences:
  - apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    name: %s`, podName, namespace, uid, clusterRoleName)
		}

		resources[key] = cacheTest.newResource(strToUnstructured(resourceYaml))
	}

	return resources
}

// Benchmark variations testing different cross-namespace percentages

func BenchmarkIterateHierarchyV2CrossNamespace_Percentage(b *testing.B) {
	testCases := []struct {
		name                  string
		crossNamespacePercent float64
		withNamespaceScan     bool
		scanNamespace         string
	}{
		{"0Percent_NoScan", 0.00, false, ""},
		{"1Percent_NoScan", 0.01, false, ""},
		{"5Percent_NoScan", 0.05, false, ""},
		{"10Percent_NoScan", 0.10, false, ""},
		{"25Percent_NoScan", 0.25, false, ""},
		{"25Percent_WithScan", 0.25, true, "default"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			cluster := newCluster(b).WithAPIResources([]kube.APIResourceInfo{{
				GroupKind:            schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"},
				GroupVersionResource: schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
				Meta:                 metav1.APIResource{Namespaced: false},
			}})

			// Calculate resource distribution
			totalResources := 10000
			crossNamespacePods := int(float64(totalResources) * tc.crossNamespacePercent)
			regularPods := totalResources - crossNamespacePods
			clusterParents := 100 // Fixed number of cluster-scoped resources

			testResources := buildParameterizedCrossNamespaceTestResourceMap(
				clusterParents,
				regularPods,
				crossNamespacePods,
				tc.withNamespaceScan, // Use UIDs when namespace scanning is enabled
			)

			for _, resource := range testResources {
				cluster.setNode(resource)
			}

			// Start key depends on whether we have cross-namespace resources
			var startKey kube.ResourceKey
			if crossNamespacePods > 0 {
				startKey = kube.ResourceKey{Namespace: "default", Name: "cross-ns-pod-1", Kind: "Pod"}
			} else {
				startKey = kube.ResourceKey{Namespace: "default", Name: "pod-1", Kind: "Pod"}
			}

			// For cluster-scoped start when testing namespace scanning
			if tc.withNamespaceScan {
				startKey = kube.ResourceKey{
					Group:     "rbac.authorization.k8s.io",
					Kind:      "ClusterRole",
					Namespace: "",
					Name:      "cluster-role-0",
				}
			}

			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				cluster.IterateHierarchyV2([]kube.ResourceKey{startKey}, func(_ *Resource, _ map[kube.ResourceKey]*Resource) bool {
					return true
				}, tc.scanNamespace)
			}
		})
	}
}

// BenchmarkIterateHierarchyV2_NamespaceScaling tests how namespace scanning scales with namespace size
func BenchmarkIterateHierarchyV2_NamespaceScaling(b *testing.B) {
	testCases := []struct {
		name                   string
		namespaceResourceCount int
		crossNamespacePercent  float64
	}{
		{"100_Resources_10pct", 100, 0.10},
		{"1000_Resources_10pct", 1000, 0.10},
		{"5000_Resources_10pct", 5000, 0.10},
		{"10000_Resources_10pct", 10000, 0.10},
		{"20000_Resources_10pct", 20000, 0.10},
		// Also test with different cross-namespace percentages
		{"5000_Resources_0pct", 5000, 0.00},
		{"5000_Resources_5pct", 5000, 0.05},
		{"5000_Resources_25pct", 5000, 0.25},
		{"5000_Resources_50pct", 5000, 0.50},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			cluster := newCluster(b).WithAPIResources([]kube.APIResourceInfo{{
				GroupKind:            schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"},
				GroupVersionResource: schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
				Meta:                 metav1.APIResource{Namespaced: false},
			}})

			// Calculate resource distribution
			clusterParents := 100 // Fixed number of cluster-scoped resources
			crossNamespacePods := int(float64(tc.namespaceResourceCount) * tc.crossNamespacePercent)
			regularPods := tc.namespaceResourceCount - crossNamespacePods

			testResources := buildParameterizedCrossNamespaceTestResourceMap(
				clusterParents,
				regularPods,
				crossNamespacePods,
				true,
			)

			for _, resource := range testResources {
				cluster.setNode(resource)
			}

			// Start from a cluster-scoped resource
			startKey := kube.ResourceKey{
				Group:     "rbac.authorization.k8s.io",
				Kind:      "ClusterRole",
				Namespace: "",
				Name:      "cluster-role-0",
			}

			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				// Test scanning the "default" namespace which has the most resources
				cluster.IterateHierarchyV2([]kube.ResourceKey{startKey}, func(_ *Resource, _ map[kube.ResourceKey]*Resource) bool {
					return true
				}, "default")
			}
		})
	}
}

// BenchmarkIterateHierarchyV2_NamespaceScaling_NoScan provides baseline for comparison
func BenchmarkIterateHierarchyV2_NamespaceScaling_NoScan(b *testing.B) {
	testCases := []struct {
		name                   string
		namespaceResourceCount int
	}{
		{"100_Resources", 100},
		{"1000_Resources", 1000},
		{"5000_Resources", 5000},
		{"10000_Resources", 10000},
		{"20000_Resources", 20000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			cluster := newCluster(b).WithAPIResources([]kube.APIResourceInfo{{
				GroupKind:            schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"},
				GroupVersionResource: schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
				Meta:                 metav1.APIResource{Namespaced: false},
			}})

			// All resources have cluster-scoped parents but we won't scan for them
			testResources := buildParameterizedCrossNamespaceTestResourceMap(
				100,
				0,
				tc.namespaceResourceCount,
				true,
			)

			for _, resource := range testResources {
				cluster.setNode(resource)
			}

			// Start from a cluster-scoped resource
			startKey := kube.ResourceKey{
				Group:     "rbac.authorization.k8s.io",
				Kind:      "ClusterRole",
				Namespace: "",
				Name:      "cluster-role-0",
			}

			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				// No namespace scanning - just traverse cluster-scoped children
				cluster.IterateHierarchyV2([]kube.ResourceKey{startKey}, func(_ *Resource, _ map[kube.ResourceKey]*Resource) bool {
					return true
				}, "")
			}
		})
	}
}

func TestIterateHierarchyV2_NoDuplicatesInSameNamespace(t *testing.T) {
	// Create a parent-child relationship in the same namespace
	parent := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "parent", Namespace: "default", UID: "parent-uid",
		},
	}
	child := &appsv1.ReplicaSet{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "ReplicaSet"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "child", Namespace: "default", UID: "child-uid",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "apps/v1", Kind: "Deployment", Name: "parent", UID: "parent-uid",
			}},
		},
	}

	cluster := newCluster(t, parent, child)
	err := cluster.EnsureSynced()
	require.NoError(t, err)

	visitCount := make(map[string]int)
	cluster.IterateHierarchyV2(
		[]kube.ResourceKey{
			kube.GetResourceKey(mustToUnstructured(parent)),
			kube.GetResourceKey(mustToUnstructured(child)),
		},
		func(resource *Resource, _ map[kube.ResourceKey]*Resource) bool {
			visitCount[resource.Ref.Name]++
			return true
		},
		"", // No orphaned resource namespace
	)

	// Each resource should be visited exactly once
	assert.Equal(t, 1, visitCount["parent"], "parent should be visited once")
	assert.Equal(t, 1, visitCount["child"], "child should be visited once")
}

func TestIterateHierarchyV2_NoDuplicatesCrossNamespace(t *testing.T) {
	// Test that cross-namespace parent-child relationships don't cause duplicates
	visitCount := make(map[string]int)

	cluster := newCluster(t, testClusterParent(), testNamespacedChild(), testClusterChild()).WithAPIResources([]kube.APIResourceInfo{{
		GroupKind:            schema.GroupKind{Group: "", Kind: "Namespace"},
		GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
		Meta:                 metav1.APIResource{Namespaced: false},
	}, {
		GroupKind:            schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"},
		GroupVersionResource: schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
		Meta:                 metav1.APIResource{Namespaced: false},
	}})
	err := cluster.EnsureSynced()
	require.NoError(t, err)

	cluster.IterateHierarchyV2(
		[]kube.ResourceKey{
			kube.GetResourceKey(mustToUnstructured(testClusterParent())),
			kube.GetResourceKey(mustToUnstructured(testNamespacedChild())),
			kube.GetResourceKey(mustToUnstructured(testClusterChild())),
		},
		func(resource *Resource, _ map[kube.ResourceKey]*Resource) bool {
			visitCount[resource.Ref.Name]++
			return true
		},
		"", // No orphaned resource namespace
	)

	// Each resource should be visited exactly once, even with cross-namespace relationships
	assert.Equal(t, 1, visitCount["test-cluster-parent"], "cluster parent should be visited once")
	assert.Equal(t, 1, visitCount["namespaced-child"], "namespaced child should be visited once")
	assert.Equal(t, 1, visitCount["cluster-child"], "cluster child should be visited once")
}
