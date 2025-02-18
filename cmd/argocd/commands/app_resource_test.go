package commands

import (
	"bytes"
	"testing"
	"text/tabwriter"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestPrintTreeViewAppResources(t *testing.T) {
	var nodes [3]v1alpha1.ResourceNode
	nodes[0].ResourceRef = v1alpha1.ResourceRef{Group: "", Version: "v1", Kind: "Pod", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo-5dcd5457d5-6trpt", UID: "92c3a5fe-d13e-4ae2-b8ec-c10dd3543b28"}
	nodes[0].ParentRefs = []v1alpha1.ResourceRef{{Group: "apps", Version: "v1", Kind: "ReplicaSet", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo-5dcd5457d5", UID: "75c30dce-1b66-414f-a86c-573a74be0f40"}}
	nodes[1].ResourceRef = v1alpha1.ResourceRef{Group: "apps", Version: "v1", Kind: "ReplicaSet", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo-5dcd5457d5", UID: "75c30dce-1b66-414f-a86c-573a74be0f40"}
	nodes[1].ParentRefs = []v1alpha1.ResourceRef{{Group: "argoproj.io", Version: "", Kind: "Rollout", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo", UID: "87f3aab0-f634-4b2c-959a-7ddd30675ed0"}}
	nodes[2].ResourceRef = v1alpha1.ResourceRef{Group: "argoproj.io", Version: "", Kind: "Rollout", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo", UID: "87f3aab0-f634-4b2c-959a-7ddd30675ed0"}
	nodeMapping := make(map[string]v1alpha1.ResourceNode)
	mapParentToChild := make(map[string][]string)
	parentNode := make(map[string]struct{})
	for _, node := range nodes {
		nodeMapping[node.UID] = node
		if len(node.ParentRefs) > 0 {
			_, ok := mapParentToChild[node.ParentRefs[0].UID]
			if !ok {
				var temp []string
				mapParentToChild[node.ParentRefs[0].UID] = temp
			}
			mapParentToChild[node.ParentRefs[0].UID] = append(mapParentToChild[node.ParentRefs[0].UID], node.UID)
		} else {
			parentNode[node.UID] = struct{}{}
		}
	}
	buf := &bytes.Buffer{}
	w := tabwriter.NewWriter(buf, 0, 0, 2, ' ', 0)

	printTreeViewAppResourcesNotOrphaned(nodeMapping, mapParentToChild, parentNode, w)
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	assert.Contains(t, output, "Rollout")
	assert.Contains(t, output, "argoproj.io")
}

func TestPrintTreeViewDetailedAppResources(t *testing.T) {
	var nodes [3]v1alpha1.ResourceNode
	nodes[0].ResourceRef = v1alpha1.ResourceRef{Group: "", Version: "v1", Kind: "Pod", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo-5dcd5457d5-6trpt", UID: "92c3a5fe-d13e-4ae2-b8ec-c10dd3543b28"}
	nodes[0].ParentRefs = []v1alpha1.ResourceRef{{Group: "apps", Version: "v1", Kind: "ReplicaSet", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo-5dcd5457d5", UID: "75c30dce-1b66-414f-a86c-573a74be0f40"}}
	nodes[1].ResourceRef = v1alpha1.ResourceRef{Group: "apps", Version: "v1", Kind: "ReplicaSet", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo-5dcd5457d5", UID: "75c30dce-1b66-414f-a86c-573a74be0f40"}
	nodes[1].ParentRefs = []v1alpha1.ResourceRef{{Group: "argoproj.io", Version: "", Kind: "Rollout", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo", UID: "87f3aab0-f634-4b2c-959a-7ddd30675ed0"}}
	nodes[2].ResourceRef = v1alpha1.ResourceRef{Group: "argoproj.io", Version: "", Kind: "Rollout", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo", UID: "87f3aab0-f634-4b2c-959a-7ddd30675ed0"}
	nodes[2].Health = &v1alpha1.HealthStatus{
		Status:  "Degraded",
		Message: "Readiness Gate failed",
	}

	nodeMapping := make(map[string]v1alpha1.ResourceNode)
	mapParentToChild := make(map[string][]string)
	parentNode := make(map[string]struct{})
	for _, node := range nodes {
		nodeMapping[node.UID] = node
		if len(node.ParentRefs) > 0 {
			_, ok := mapParentToChild[node.ParentRefs[0].UID]
			if !ok {
				var temp []string
				mapParentToChild[node.ParentRefs[0].UID] = temp
			}
			mapParentToChild[node.ParentRefs[0].UID] = append(mapParentToChild[node.ParentRefs[0].UID], node.UID)
		} else {
			parentNode[node.UID] = struct{}{}
		}
	}
	buf := &bytes.Buffer{}
	w := tabwriter.NewWriter(buf, 0, 0, 2, ' ', 0)

	printDetailedTreeViewAppResourcesNotOrphaned(nodeMapping, mapParentToChild, parentNode, w)
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	assert.Contains(t, output, "Rollout")
	assert.Contains(t, output, "Degraded")
	assert.Contains(t, output, "Readiness Gate failed")
}

func TestPrintResourcesTree(t *testing.T) {
	tree := v1alpha1.ApplicationTree{
		Nodes: []v1alpha1.ResourceNode{
			{
				ResourceRef: v1alpha1.ResourceRef{
					Group:     "group",
					Kind:      "kind",
					Namespace: "ns",
					Name:      "rs1",
				},
			},
		},
		OrphanedNodes: []v1alpha1.ResourceNode{
			{
				ResourceRef: v1alpha1.ResourceRef{
					Group:     "group2",
					Kind:      "kind2",
					Namespace: "ns2",
					Name:      "rs2",
				},
			},
		},
	}
	output, _ := captureOutput(func() error {
		printResources(true, false, &tree, "")
		return nil
	})

	expectation := "GROUP   KIND   NAMESPACE  NAME  ORPHANED\ngroup   kind   ns         rs1   No\ngroup2  kind2  ns2        rs2   Yes\n"

	assert.Equal(t, expectation, output)
}
