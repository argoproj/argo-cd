package commands

import (
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_logsIsSelectedPod(t *testing.T) {
	deployment := argoappv1.ResourceRef{Group: "", Version: "v1", Kind: "Deployment", Name: "deployment", UID: "1"}
	rs := argoappv1.ResourceRef{Group: "", Version: "v1", Kind: "ReplicaSet", Name: "rs", UID: "2"}
	podRS := argoappv1.ResourceRef{Group: "", Version: "v1", Kind: "Pod", Name: "podrs", UID: "3"}
	pod := argoappv1.ResourceRef{Group: "", Version: "v1", Kind: "Pod", Name: "pod", UID: "4"}
	treeNodes := []argoappv1.ResourceNode{
		{ResourceRef: deployment, ParentRefs: nil},
		{ResourceRef: rs, ParentRefs: []argoappv1.ResourceRef{deployment}},
		{ResourceRef: podRS, ParentRefs: []argoappv1.ResourceRef{rs}},
		{ResourceRef: pod, ParentRefs: nil},
	}

	t.Run("GetAllPods", func(t *testing.T) {
		var selectedResources []argoappv1.SyncOperationResource
		pods := getSelectedPods(treeNodes, selectedResources)
		assert.Equal(t, 2, len(pods))
	})

	t.Run("GetRSPods", func(t *testing.T) {
		selectedResources := []argoappv1.SyncOperationResource{
			{Group: "", Kind: "ReplicaSet", Name: "rs"},
		}
		pods := getSelectedPods(treeNodes, selectedResources)
		assert.Equal(t, 1, len(pods))
	})

	t.Run("GetDeploymentPods", func(t *testing.T) {
		selectedResources := []argoappv1.SyncOperationResource{
			{Group: "", Kind: "Deployment", Name: "deployment"},
		}
		pods := getSelectedPods(treeNodes, selectedResources)
		assert.Equal(t, 1, len(pods))
	})

	t.Run("NoMatchingPods", func(t *testing.T) {
		selectedResources := []argoappv1.SyncOperationResource{
			{Group: "", Kind: "Service", Name: "service"},
		}
		pods := getSelectedPods(treeNodes, selectedResources)
		assert.Equal(t, 0, len(pods))
	})
}
