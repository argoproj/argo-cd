package controller

import (
	"fmt"
	"sort"
	"testing"

	"github.com/argoproj/argo-cd/util/kube/kubetest"

	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	fakedisco "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/rest"
	testcore "k8s.io/client-go/testing"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/kube"
)

func newTestSyncCtx(resources ...*v1.APIResourceList) *syncContext {
	fakeDisco := &fakedisco.FakeDiscovery{Fake: &testcore.Fake{}}
	fakeDisco.Resources = append(resources, &v1.APIResourceList{
		APIResources: []v1.APIResource{
			{Kind: "pod", Namespaced: true},
			{Kind: "deployment", Namespaced: true},
			{Kind: "service", Namespaced: true},
		},
	})
	return &syncContext{
		comparison: &v1alpha1.ComparisonResult{},
		config:     &rest.Config{},
		namespace:  "test-namespace",
		server:     "https://test-server",
		syncRes:    &v1alpha1.SyncOperationResult{},
		syncOp: &v1alpha1.SyncOperation{
			Prune: true,
			SyncStrategy: &v1alpha1.SyncStrategy{
				Apply: &v1alpha1.SyncStrategyApply{},
			},
		},
		proj: &v1alpha1.AppProject{
			ObjectMeta: v1.ObjectMeta{
				Name: "test",
			},
			Spec: v1alpha1.AppProjectSpec{
				Destinations: []v1alpha1.ApplicationDestination{{
					Server:    "https://test-server",
					Namespace: "test-namespace",
				}},
				ClusterResourceWhitelist: []v1.GroupKind{
					{Group: "*", Kind: "*"},
				},
			},
		},
		opState: &v1alpha1.OperationState{},
		disco:   fakeDisco,
		log:     log.WithFields(log.Fields{"application": "fake-app"}),
	}
}

func TestSyncNotPermittedNamespace(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.kubectl = kubetest.MockKubectlCmd{}
	syncCtx.resources = []ManagedResource{{
		Live:   nil,
		Target: kube.MustToUnstructured(&apiv1.Pod{TypeMeta: v1.TypeMeta{Kind: "pod"}, ObjectMeta: v1.ObjectMeta{Namespace: "kube-system"}}),
	}, {
		Live:   nil,
		Target: kube.MustToUnstructured(&apiv1.Service{TypeMeta: v1.TypeMeta{Kind: "service"}}),
	}}
	syncCtx.sync()
	assert.Equal(t, v1alpha1.OperationFailed, syncCtx.opState.Phase)
	assert.Contains(t, syncCtx.syncRes.Resources[0].Message, "not permitted in project")
}

func TestSyncCreateInSortedOrder(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.kubectl = kubetest.MockKubectlCmd{}
	syncCtx.resources = []ManagedResource{{
		Live:   nil,
		Target: kube.MustToUnstructured(&apiv1.Pod{TypeMeta: v1.TypeMeta{Kind: "pod"}}),
	}, {
		Live:   nil,
		Target: kube.MustToUnstructured(&apiv1.Service{TypeMeta: v1.TypeMeta{Kind: "service"}}),
	}}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 2)
	for i := range syncCtx.syncRes.Resources {
		if syncCtx.syncRes.Resources[i].Kind == "pod" {
			assert.Equal(t, v1alpha1.ResourceDetailsSynced, syncCtx.syncRes.Resources[i].Status)
		} else if syncCtx.syncRes.Resources[i].Kind == "service" {
			assert.Equal(t, v1alpha1.ResourceDetailsSynced, syncCtx.syncRes.Resources[i].Status)
		} else {
			t.Error("Resource isn't a pod or a service")
		}
	}
	syncCtx.sync()
	assert.Equal(t, syncCtx.opState.Phase, v1alpha1.OperationSucceeded)
}

func TestSyncCreateNotWhitelistedClusterResources(t *testing.T) {
	syncCtx := newTestSyncCtx(&v1.APIResourceList{
		GroupVersion: v1alpha1.SchemeGroupVersion.String(),
		APIResources: []v1.APIResource{
			{Name: "workflows", Namespaced: false, Kind: "Workflow", Group: "argoproj.io"},
			{Name: "application", Namespaced: false, Kind: "Application", Group: "argoproj.io"},
		},
	}, &v1.APIResourceList{
		GroupVersion: "rbac.authorization.k8s.io/v1",
		APIResources: []v1.APIResource{
			{Name: "clusterroles", Namespaced: false, Kind: "ClusterRole", Group: "rbac.authorization.k8s.io"},
		},
	})

	syncCtx.proj.Spec.ClusterResourceWhitelist = []v1.GroupKind{
		{Group: "argoproj.io", Kind: "*"},
	}

	syncCtx.kubectl = kubetest.MockKubectlCmd{}
	syncCtx.resources = []ManagedResource{{
		Live: nil,
		Target: kube.MustToUnstructured(&rbacv1.ClusterRole{
			TypeMeta:   v1.TypeMeta{Kind: "ClusterRole", APIVersion: "rbac.authorization.k8s.io/v1"},
			ObjectMeta: v1.ObjectMeta{Name: "argo-ui-cluster-role"}}),
	}}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 1)
	assert.Equal(t, v1alpha1.ResourceDetailsSyncFailed, syncCtx.syncRes.Resources[0].Status)
	assert.Contains(t, syncCtx.syncRes.Resources[0].Message, "not permitted in project")
}

func TestSyncBlacklistedNamespacedResources(t *testing.T) {
	syncCtx := newTestSyncCtx()

	syncCtx.proj.Spec.NamespaceResourceBlacklist = []v1.GroupKind{
		{Group: "*", Kind: "deployment"},
	}

	syncCtx.kubectl = kubetest.MockKubectlCmd{}
	syncCtx.resources = []ManagedResource{{
		Live:   nil,
		Target: kube.MustToUnstructured(&appsv1.Deployment{TypeMeta: v1.TypeMeta{Kind: "deployment"}}),
	}}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 1)
	assert.Equal(t, v1alpha1.ResourceDetailsSyncFailed, syncCtx.syncRes.Resources[0].Status)
	assert.Contains(t, syncCtx.syncRes.Resources[0].Message, "not permitted in project")
}

func TestSyncSuccessfully(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.kubectl = kubetest.MockKubectlCmd{}
	syncCtx.resources = []ManagedResource{{
		Live:   nil,
		Target: kube.MustToUnstructured(&apiv1.Service{TypeMeta: v1.TypeMeta{Kind: "service"}}),
	}, {
		Live:   kube.MustToUnstructured(&apiv1.Pod{TypeMeta: v1.TypeMeta{Kind: "pod"}}),
		Target: nil,
	}}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 2)
	for i := range syncCtx.syncRes.Resources {
		if syncCtx.syncRes.Resources[i].Kind == "pod" {
			assert.Equal(t, v1alpha1.ResourceDetailsSyncedAndPruned, syncCtx.syncRes.Resources[i].Status)
		} else if syncCtx.syncRes.Resources[i].Kind == "service" {
			assert.Equal(t, v1alpha1.ResourceDetailsSynced, syncCtx.syncRes.Resources[i].Status)
		} else {
			t.Error("Resource isn't a pod or a service")
		}
	}
	syncCtx.sync()
	assert.Equal(t, syncCtx.opState.Phase, v1alpha1.OperationSucceeded)
}

func TestSyncDeleteSuccessfully(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.kubectl = kubetest.MockKubectlCmd{}
	syncCtx.resources = []ManagedResource{{
		Live:   kube.MustToUnstructured(&apiv1.Service{TypeMeta: v1.TypeMeta{Kind: "service"}}),
		Target: nil,
	}, {
		Live:   kube.MustToUnstructured(&apiv1.Pod{TypeMeta: v1.TypeMeta{Kind: "pod"}}),
		Target: nil,
	}}
	syncCtx.sync()
	for i := range syncCtx.syncRes.Resources {
		if syncCtx.syncRes.Resources[i].Kind == "pod" {
			assert.Equal(t, v1alpha1.ResourceDetailsSyncedAndPruned, syncCtx.syncRes.Resources[i].Status)
		} else if syncCtx.syncRes.Resources[i].Kind == "service" {
			assert.Equal(t, v1alpha1.ResourceDetailsSyncedAndPruned, syncCtx.syncRes.Resources[i].Status)
		} else {
			t.Error("Resource isn't a pod or a service")
		}
	}
	syncCtx.sync()
	assert.Equal(t, syncCtx.opState.Phase, v1alpha1.OperationSucceeded)
}

func TestSyncCreateFailure(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.kubectl = kubetest.MockKubectlCmd{
		Commands: map[string]kubetest.KubectlOutput{
			"test-service": {
				Output: "",
				Err:    fmt.Errorf("error: error validating \"test.yaml\": error validating data: apiVersion not set; if you choose to ignore these errors, turn validation off with --validate=false"),
			},
		},
	}
	syncCtx.resources = []ManagedResource{{
		Live:   nil,
		Target: kube.MustToUnstructured(&apiv1.Service{ObjectMeta: v1.ObjectMeta{Name: "test-service"}}),
	}}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 1)
	assert.Equal(t, v1alpha1.ResourceDetailsSyncFailed, syncCtx.syncRes.Resources[0].Status)
}

func TestSyncPruneFailure(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.kubectl = kubetest.MockKubectlCmd{
		Commands: map[string]kubetest.KubectlOutput{
			"test-service": {
				Output: "",
				Err:    fmt.Errorf(" error: timed out waiting for \"test-service\" to be synced"),
			},
		},
	}
	syncCtx.resources = []ManagedResource{{
		Live:   kube.MustToUnstructured(&apiv1.Service{ObjectMeta: v1.ObjectMeta{Name: "test-service"}}),
		Target: nil,
	}}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 1)
	assert.Equal(t, v1alpha1.ResourceDetailsSyncFailed, syncCtx.syncRes.Resources[0].Status)
}

func TestRunWorkflows(t *testing.T) {
	// syncCtx := newTestSyncCtx()
	// syncCtx.doWorkflowSync(nil, nil)

}

func unsortedManifest() []syncTask {
	return []syncTask{
		{
			targetObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"GroupVersion": apiv1.SchemeGroupVersion.String(),
					"kind":         "Pod",
				},
			},
		},
		{
			targetObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"GroupVersion": apiv1.SchemeGroupVersion.String(),
					"kind":         "Service",
				},
			},
		},
		{
			targetObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"GroupVersion": apiv1.SchemeGroupVersion.String(),
					"kind":         "PersistentVolume",
				},
			},
		},
		{
			targetObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"GroupVersion": apiv1.SchemeGroupVersion.String(),
				},
			},
		},
		{
			targetObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"GroupVersion": apiv1.SchemeGroupVersion.String(),
					"kind":         "ConfigMap",
				},
			},
		},
	}
}

func sortedManifest() []syncTask {
	return []syncTask{
		{
			targetObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"GroupVersion": apiv1.SchemeGroupVersion.String(),
					"kind":         "ConfigMap",
				},
			},
		},
		{
			targetObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"GroupVersion": apiv1.SchemeGroupVersion.String(),
					"kind":         "PersistentVolume",
				},
			},
		},
		{
			targetObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"GroupVersion": apiv1.SchemeGroupVersion.String(),
					"kind":         "Service",
				},
			},
		},
		{
			targetObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"GroupVersion": apiv1.SchemeGroupVersion.String(),
					"kind":         "Pod",
				},
			},
		},
		{
			targetObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"GroupVersion": apiv1.SchemeGroupVersion.String(),
				},
			},
		},
	}
}

func TestSortKubernetesResourcesSuccessfully(t *testing.T) {
	unsorted := unsortedManifest()
	ks := newKindSorter(unsorted, resourceOrder)
	sort.Sort(ks)

	expectedOrder := sortedManifest()
	assert.Equal(t, len(unsorted), len(expectedOrder))
	for i, sorted := range unsorted {
		assert.Equal(t, expectedOrder[i], sorted)
	}

}

func TestSortManifestHandleNil(t *testing.T) {
	task := syncTask{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "Service",
			},
		},
	}
	manifest := []syncTask{
		{},
		task,
	}
	ks := newKindSorter(manifest, resourceOrder)
	sort.Sort(ks)
	assert.Equal(t, task, manifest[0])
	assert.Nil(t, manifest[1].targetObj)

}
