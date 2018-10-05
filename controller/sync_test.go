package controller

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/kube"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	fakedisco "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/rest"
	testcore "k8s.io/client-go/testing"
)

type kubectlOutput struct {
	output string
	err    error
}

type mockKubectlCmd struct {
	commands map[string]kubectlOutput
	events   chan watch.Event
}

func (k mockKubectlCmd) WatchResources(
	ctx context.Context, config *rest.Config, namespace string, selector func(kind schema.GroupVersionKind) v1.ListOptions) (chan watch.Event, error) {

	return k.events, nil
}

func (k mockKubectlCmd) DeleteResource(config *rest.Config, obj *unstructured.Unstructured, namespace string) error {
	command, ok := k.commands[obj.GetName()]
	if !ok {
		return nil
	}
	return command.err
}

func (k mockKubectlCmd) ApplyResource(config *rest.Config, obj *unstructured.Unstructured, namespace string, dryRun, force bool) (string, error) {
	command, ok := k.commands[obj.GetName()]
	if !ok {
		return "", nil
	}
	return command.output, command.err
}

// ConvertToVersion converts an unstructured object into the specified group/version
func (k mockKubectlCmd) ConvertToVersion(obj *unstructured.Unstructured, group, version string) (*unstructured.Unstructured, error) {
	return obj, nil
}

func newTestSyncCtx(resources ...*v1.APIResourceList) *syncContext {
	fakeDisco := &fakedisco.FakeDiscovery{Fake: &testcore.Fake{}}
	fakeDisco.Resources = append(resources, &v1.APIResourceList{
		APIResources: []v1.APIResource{
			{Kind: "pod", Namespaced: true},
			{Kind: "deployment", Namespaced: true},
			{Kind: "service", Namespaced: true},
		},
	})
	kube.FlushServerResourcesCache()
	return &syncContext{
		comparison: &v1alpha1.ComparisonResult{},
		config:     &rest.Config{},
		namespace:  "test-namespace",
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

func TestSyncCreateInSortedOrder(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.kubectl = mockKubectlCmd{}
	syncCtx.comparison = &v1alpha1.ComparisonResult{
		Resources: []v1alpha1.ResourceState{{
			LiveState:   "",
			TargetState: "{\"kind\":\"pod\"}",
		}, {
			LiveState:   "",
			TargetState: "{\"kind\":\"service\"}",
		},
		},
	}
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

	syncCtx.kubectl = mockKubectlCmd{}
	syncCtx.comparison = &v1alpha1.ComparisonResult{
		Resources: []v1alpha1.ResourceState{{
			LiveState:   "",
			TargetState: `{"apiVersion": "rbac.authorization.k8s.io/v1", "kind": "ClusterRole", "metadata": {"name": "argo-ui-cluster-role" }}`,
		}},
	}
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

	syncCtx.kubectl = mockKubectlCmd{}
	syncCtx.comparison = &v1alpha1.ComparisonResult{
		Resources: []v1alpha1.ResourceState{{
			LiveState:   "",
			TargetState: "{\"kind\":\"deployment\"}",
		}},
	}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 1)
	assert.Equal(t, v1alpha1.ResourceDetailsSyncFailed, syncCtx.syncRes.Resources[0].Status)
	assert.Contains(t, syncCtx.syncRes.Resources[0].Message, "not permitted in project")
}

func TestSyncSuccessfully(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.kubectl = mockKubectlCmd{}
	syncCtx.comparison = &v1alpha1.ComparisonResult{
		Resources: []v1alpha1.ResourceState{{
			LiveState:   "",
			TargetState: "{\"kind\":\"service\"}",
		}, {
			LiveState:   "{\"kind\":\"pod\"}",
			TargetState: "",
		},
		},
	}
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
	syncCtx.kubectl = mockKubectlCmd{}
	syncCtx.comparison = &v1alpha1.ComparisonResult{
		Resources: []v1alpha1.ResourceState{{
			LiveState:   "{\"kind\":\"service\"}",
			TargetState: "",
		}, {
			LiveState:   "{\"kind\":\"pod\"}",
			TargetState: "",
		},
		},
	}
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
	syncCtx.kubectl = mockKubectlCmd{
		commands: map[string]kubectlOutput{
			"test-service": {
				output: "",
				err:    fmt.Errorf("error: error validating \"test.yaml\": error validating data: apiVersion not set; if you choose to ignore these errors, turn validation off with --validate=false"),
			},
		},
	}
	syncCtx.comparison = &v1alpha1.ComparisonResult{
		Resources: []v1alpha1.ResourceState{{
			LiveState:   "",
			TargetState: "{\"kind\":\"service\", \"metadata\":{\"name\":\"test-service\"}}",
		},
		},
	}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 1)
	assert.Equal(t, v1alpha1.ResourceDetailsSyncFailed, syncCtx.syncRes.Resources[0].Status)
}

func TestSyncPruneFailure(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.kubectl = mockKubectlCmd{
		commands: map[string]kubectlOutput{
			"test-service": {
				output: "",
				err:    fmt.Errorf(" error: timed out waiting for \"test-service\" to be synced"),
			},
		},
	}
	syncCtx.comparison = &v1alpha1.ComparisonResult{
		Resources: []v1alpha1.ResourceState{{
			LiveState:   "{\"kind\":\"service\", \"metadata\":{\"name\":\"test-service\"}}",
			TargetState: "",
		},
		},
	}
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
