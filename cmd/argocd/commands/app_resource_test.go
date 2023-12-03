package commands

import (
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

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
