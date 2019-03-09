package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test"
	"github.com/argoproj/argo-cd/util/kube/kubetest"
)

var clusterRoleHook = `
{
  "apiVersion": "rbac.authorization.k8s.io/v1",
  "kind": "ClusterRole",
  "metadata": {
    "name": "cluster-role-hook",
    "annotations": {
      "argocd.argoproj.io/hook": "PostSync"
	}
  }
}`

func TestSyncHookProjectPermissions(t *testing.T) {
	syncCtx := newTestSyncCtx(&v1.APIResourceList{
		GroupVersion: "v1",
		APIResources: []v1.APIResource{
			{Name: "pod", Namespaced: true, Kind: "Pod", Group: "v1"},
		},
	}, &v1.APIResourceList{
		GroupVersion: "rbac.authorization.k8s.io/v1",
		APIResources: []v1.APIResource{
			{Name: "clusterroles", Namespaced: false, Kind: "ClusterRole", Group: "rbac.authorization.k8s.io"},
		},
	})

	syncCtx.kubectl = kubetest.MockKubectlCmd{}
	crHook, _ := v1alpha1.UnmarshalToUnstructured(clusterRoleHook)
	syncCtx.compareResult = &comparisonResult{
		hooks: []*unstructured.Unstructured{
			crHook,
		},
		managedResources: []managedResource{{
			Target: test.NewPod(),
		}},
	}
	syncCtx.proj.Spec.ClusterResourceWhitelist = []v1.GroupKind{}

	syncCtx.syncOp.SyncStrategy = nil
	syncCtx.sync()
	assert.Equal(t, v1alpha1.OperationFailed, syncCtx.opState.Phase)
	assert.Len(t, syncCtx.syncRes.Resources, 0)
	assert.Contains(t, syncCtx.opState.Message, "not permitted in project")

	// Now add the resource to the whitelist and try again. Resource should be created
	syncCtx.proj.Spec.ClusterResourceWhitelist = []v1.GroupKind{
		{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"},
	}
	syncCtx.syncOp.SyncStrategy = nil
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 1)
	assert.Equal(t, v1alpha1.ResultCodeSynced, syncCtx.syncRes.Resources[0].Status)
}
