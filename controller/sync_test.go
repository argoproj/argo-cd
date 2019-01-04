package controller

import (
	"fmt"
	"sort"
	"testing"

	"github.com/argoproj/argo-cd/reposerver/repository"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	fakedisco "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/rest"
	testcore "k8s.io/client-go/testing"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/kube/kubetest"
)

func newTestSyncCtx(resources ...*v1.APIResourceList) *syncContext {
	fakeDisco := &fakedisco.FakeDiscovery{Fake: &testcore.Fake{}}
	fakeDisco.Resources = append(resources,
		&v1.APIResourceList{
			GroupVersion: "v1",
			APIResources: []v1.APIResource{
				{Kind: "Pod", Group: "", Version: "v1", Namespaced: true},
				{Kind: "Service", Group: "", Version: "v1", Namespaced: true},
			},
		},
		&v1.APIResourceList{
			GroupVersion: "apps/v1",
			APIResources: []v1.APIResource{
				{Kind: "Deployment", Group: "apps", Version: "v1", Namespaced: true},
			},
		})
	sc := syncContext{
		config:    &rest.Config{},
		namespace: test.FakeArgoCDNamespace,
		server:    test.FakeClusterURL,
		syncRes:   &v1alpha1.SyncOperationResult{},
		syncOp: &v1alpha1.SyncOperation{
			Prune: true,
			SyncStrategy: &v1alpha1.SyncStrategy{
				Apply: &v1alpha1.SyncStrategyApply{},
			},
		},
		proj: &v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: v1alpha1.AppProjectSpec{
				Destinations: []v1alpha1.ApplicationDestination{{
					Server:    test.FakeClusterURL,
					Namespace: test.FakeArgoCDNamespace,
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
	sc.kubectl = kubetest.MockKubectlCmd{}
	return &sc
}

func TestSyncNotPermittedNamespace(t *testing.T) {
	syncCtx := newTestSyncCtx()
	targetPod := test.NewPod()
	targetPod.SetNamespace("kube-system")
	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{
			Live:   nil,
			Target: targetPod,
		}, {
			Live:   nil,
			Target: test.NewService(),
		}},
	}
	syncCtx.sync()
	assert.Equal(t, v1alpha1.OperationFailed, syncCtx.opState.Phase)
	assert.Contains(t, syncCtx.syncRes.Resources[0].Message, "not permitted in project")
}

func TestSyncCreateInSortedOrder(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{
			Live:   nil,
			Target: test.NewPod(),
		}, {
			Live:   nil,
			Target: test.NewService(),
		}},
	}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 2)
	for i := range syncCtx.syncRes.Resources {
		if syncCtx.syncRes.Resources[i].Kind == "Pod" {
			assert.Equal(t, v1alpha1.ResultCodeSynced, syncCtx.syncRes.Resources[i].Status)
		} else if syncCtx.syncRes.Resources[i].Kind == "Service" {
			assert.Equal(t, v1alpha1.ResultCodeSynced, syncCtx.syncRes.Resources[i].Status)
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
	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{
			Live: nil,
			Target: kube.MustToUnstructured(&rbacv1.ClusterRole{
				TypeMeta:   metav1.TypeMeta{Kind: "ClusterRole", APIVersion: "rbac.authorization.k8s.io/v1"},
				ObjectMeta: metav1.ObjectMeta{Name: "argo-ui-cluster-role"}}),
		}},
	}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 1)
	assert.Equal(t, v1alpha1.ResultCodeSyncFailed, syncCtx.syncRes.Resources[0].Status)
	assert.Contains(t, syncCtx.syncRes.Resources[0].Message, "not permitted in project")
}

func TestSyncBlacklistedNamespacedResources(t *testing.T) {
	syncCtx := newTestSyncCtx()

	syncCtx.proj.Spec.NamespaceResourceBlacklist = []v1.GroupKind{
		{Group: "*", Kind: "Deployment"},
	}

	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{
			Live:   nil,
			Target: test.NewDeployment(),
		}},
	}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 1)
	assert.Equal(t, v1alpha1.ResultCodeSyncFailed, syncCtx.syncRes.Resources[0].Status)
	assert.Contains(t, syncCtx.syncRes.Resources[0].Message, "not permitted in project")
}

func TestSyncSuccessfully(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{
			Live:   nil,
			Target: test.NewService(),
		}, {
			Live:   test.NewPod(),
			Target: nil,
		}},
	}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 2)
	for i := range syncCtx.syncRes.Resources {
		if syncCtx.syncRes.Resources[i].Kind == "Pod" {
			assert.Equal(t, v1alpha1.ResultCodePruned, syncCtx.syncRes.Resources[i].Status)
		} else if syncCtx.syncRes.Resources[i].Kind == "Service" {
			assert.Equal(t, v1alpha1.ResultCodeSynced, syncCtx.syncRes.Resources[i].Status)
		} else {
			t.Error("Resource isn't a pod or a service")
		}
	}
	syncCtx.sync()
	assert.Equal(t, syncCtx.opState.Phase, v1alpha1.OperationSucceeded)
}

func TestSyncDeleteSuccessfully(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{
			Live:   test.NewService(),
			Target: nil,
		}, {
			Live:   test.NewPod(),
			Target: nil,
		}},
	}
	syncCtx.sync()
	for i := range syncCtx.syncRes.Resources {
		if syncCtx.syncRes.Resources[i].Kind == "Pod" {
			assert.Equal(t, v1alpha1.ResultCodePruned, syncCtx.syncRes.Resources[i].Status)
		} else if syncCtx.syncRes.Resources[i].Kind == "Service" {
			assert.Equal(t, v1alpha1.ResultCodePruned, syncCtx.syncRes.Resources[i].Status)
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
	testSvc := test.NewService()
	testSvc.SetAPIVersion("")
	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{
			Live:   nil,
			Target: testSvc,
		}},
	}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 1)
	assert.Equal(t, v1alpha1.ResultCodeSyncFailed, syncCtx.syncRes.Resources[0].Status)
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
	testSvc := test.NewService()
	testSvc.SetName("test-service")
	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{
			Live:   testSvc,
			Target: nil,
		}},
	}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 1)
	assert.Equal(t, v1alpha1.ResultCodeSyncFailed, syncCtx.syncRes.Resources[0].Status)
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

func TestDontSyncOrPruneHooks(t *testing.T) {
	syncCtx := newTestSyncCtx()
	targetPod := test.NewPod()
	targetPod.SetName("dont-create-me")
	targetPod.SetAnnotations(map[string]string{common.AnnotationKeyHook: "PreSync"})
	liveSvc := test.NewService()
	liveSvc.SetName("dont-prune-me")
	liveSvc.SetAnnotations(map[string]string{common.AnnotationKeyHook: "PreSync"})

	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{
			Live:   nil,
			Target: targetPod,
			Hook:   true,
		}, {
			Live:   liveSvc,
			Target: nil,
			Hook:   true,
		}},
	}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 0)
	syncCtx.sync()
	assert.Equal(t, syncCtx.opState.Phase, v1alpha1.OperationSucceeded)
}

func TestPersistRevisionHistory(t *testing.T) {
	app := newFakeApp()
	defaultProject := &v1alpha1.AppProject{
		ObjectMeta: v1.ObjectMeta{
			Namespace: test.FakeArgoCDNamespace,
			Name:      "default",
		},
	}
	data := fakeData{
		apps: []runtime.Object{app, defaultProject},
		manifestResponse: &repository.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data)

	ctrl.appStateManager.SyncAppState(app, &v1alpha1.OperationState{Operation: v1alpha1.Operation{
		Sync: &v1alpha1.SyncOperation{},
	}})

	updatedApp, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Get(app.Name, v1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(updatedApp.Status.History))
	assert.Equal(t, 0, len(updatedApp.Status.History[0].ComponentParameterOverrides))
	assert.Equal(t, "abc123", updatedApp.Status.History[0].Revision)

	overrides := []v1alpha1.ComponentParameter{{Name: "test", Value: "123"}}
	ctrl.appStateManager.SyncAppState(app, &v1alpha1.OperationState{Operation: v1alpha1.Operation{
		Sync: &v1alpha1.SyncOperation{
			ParameterOverrides: overrides,
		},
	}})

	updatedApp, err = ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Get(app.Name, v1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(updatedApp.Status.History))
	assert.ElementsMatch(t, overrides, updatedApp.Status.History[0].ComponentParameterOverrides)
	assert.Equal(t, "abc123", updatedApp.Status.History[0].Revision)
}
