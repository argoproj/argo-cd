package commands

import (
	"bytes"
	"strings"
	"testing"
	"text/tabwriter"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
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
	require.NoError(t, w.Flush())
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
	require.NoError(t, w.Flush())
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

func TestFilterFieldsFromObject(t *testing.T) {
	tests := []struct {
		name             string
		obj              unstructured.Unstructured
		filteredFields   []string
		expectedFields   []string
		unexpectedFields []string
	}{
		{
			name: "filter nested field",
			obj: unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "vX",
					"kind":       "kind",
					"metadata": map[string]any{
						"name": "test",
					},
					"spec": map[string]any{
						"testfield": map[string]any{
							"nestedtest": "test",
						},
						"testfield2": "test",
					},
				},
			},
			filteredFields:   []string{"spec.testfield.nestedtest"},
			expectedFields:   []string{"spec.testfield.nestedtest"},
			unexpectedFields: []string{"spec.testfield2"},
		},
		{
			name: "filter multiple fields",
			obj: unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "vX",
					"kind":       "kind",
					"metadata": map[string]any{
						"name": "test",
					},
					"spec": map[string]any{
						"testfield": map[string]any{
							"nestedtest": "test",
						},
						"testfield2": "test",
						"testfield3": "deleteme",
					},
				},
			},
			filteredFields:   []string{"spec.testfield.nestedtest", "spec.testfield3"},
			expectedFields:   []string{"spec.testfield.nestedtest"},
			unexpectedFields: []string{"spec.testfield2"},
		},
		{
			name: "filter nested list object",
			obj: unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "vX",
					"kind":       "kind",
					"metadata": map[string]any{
						"name": "test",
					},
					"spec": map[string]any{
						"testfield": map[string]any{
							"nestedtest": "test",
						},
						"testfield2": "test",
					},
				},
			},
			filteredFields:   []string{"spec.testfield.nestedtest"},
			expectedFields:   []string{"spec.testfield.nestedtest"},
			unexpectedFields: []string{"spec.testfield2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.obj.SetName("test-object")

			filtered := filterFieldsFromObject(&tt.obj, tt.filteredFields)

			for _, field := range tt.expectedFields {
				fieldPath := strings.Split(field, ".")
				_, exists, err := unstructured.NestedFieldCopy(filtered.Object, fieldPath...)
				require.NoError(t, err)
				assert.True(t, exists, "Expected field %s to exist", field)
			}

			for _, field := range tt.unexpectedFields {
				fieldPath := strings.Split(field, ".")
				_, exists, err := unstructured.NestedFieldCopy(filtered.Object, fieldPath...)
				require.NoError(t, err)
				assert.False(t, exists, "Expected field %s to not exist", field)
			}

			assert.Equal(t, tt.obj.GetName(), filtered.GetName())
		})
	}
}

func TestPrintManifests(t *testing.T) {
	obj := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "vX",
			"kind":       "test",
			"metadata": map[string]any{
				"name": "unit-test",
			},
			"spec": map[string]any{
				"testfield": "testvalue",
			},
		},
	}

	expectedYAML := `apiVersion: vX
kind: test
metadata:
    name: unit-test
spec:
    testfield: testvalue
`

	output, _ := captureOutput(func() error {
		printManifests(&[]unstructured.Unstructured{obj}, false, true, "yaml")
		return nil
	})
	assert.Equal(t, expectedYAML+"\n", output, "Incorrect yaml output for printManifests")

	output, _ = captureOutput(func() error {
		printManifests(&[]unstructured.Unstructured{obj, obj}, false, true, "yaml")
		return nil
	})
	assert.Equal(t, expectedYAML+"\n---\n"+expectedYAML+"\n", output, "Incorrect yaml output with multiple objs.")

	expectedJSON := `{
 "apiVersion": "vX",
 "kind": "test",
 "metadata": {
  "name": "unit-test"
 },
 "spec": {
  "testfield": "testvalue"
 }
}`

	output, _ = captureOutput(func() error {
		printManifests(&[]unstructured.Unstructured{obj}, false, true, "json")
		return nil
	})
	assert.Equal(t, expectedJSON+"\n", output, "Incorrect json output.")

	output, _ = captureOutput(func() error {
		printManifests(&[]unstructured.Unstructured{obj, obj}, false, true, "json")
		return nil
	})
	assert.Equal(t, expectedJSON+"\n---\n"+expectedJSON+"\n", output, "Incorrect json output with multiple objs.")

	output, _ = captureOutput(func() error {
		printManifests(&[]unstructured.Unstructured{obj}, true, true, "wide")
		return nil
	})
	assert.Contains(t, output, "FIELD           RESOURCE NAME  VALUE", "Missing a line in the table")
	assert.Contains(t, output, "apiVersion      unit-test      vX", "Missing a line in the table")
	assert.Contains(t, output, "kind            unit-test      test", "Missing a line in the table")
	assert.Contains(t, output, "spec.testfield  unit-test      testvalue", "Missing a line in the table")
	assert.NotContains(t, output, "metadata.name   unit-test      testvalue")

	output, _ = captureOutput(func() error {
		printManifests(&[]unstructured.Unstructured{obj}, true, false, "wide")
		return nil
	})
	assert.Contains(t, output, "FIELD           VALUE", "Missing a line in the table")
	assert.Contains(t, output, "apiVersion      vX", "Missing a line in the table")
	assert.Contains(t, output, "kind            test", "Missing a line in the table")
	assert.Contains(t, output, "spec.testfield  testvalue", "Missing a line in the table")
	assert.NotContains(t, output, "metadata.name   testvalue")
}
