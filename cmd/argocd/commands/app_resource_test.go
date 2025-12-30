package commands

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"text/tabwriter"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestPrintTreeViewAppResources(t *testing.T) {
	nodes := []*v1alpha1.ResourceNode{
		{
			ResourceRef: v1alpha1.ResourceRef{
				Group:     "",
				Version:   "v1",
				Kind:      "Pod",
				Namespace: "sandbox-rollout-numalogic-demo",
				Name:      "numalogic-rollout-demo-5dcd5457d5-6trpt",
				UID:       "92c3a5fe-d13e-4ae2-b8ec-c10dd3543b28",
			},
			ParentRefs: []v1alpha1.ResourceRef{
				{
					Group:     "apps",
					Version:   "v1",
					Kind:      "ReplicaSet",
					Namespace: "sandbox-rollout-numalogic-demo",
					Name:      "numalogic-rollout-demo-5dcd5457d5",
					UID:       "75c30dce-1b66-414f-a86c-573a74be0f40",
				},
			},
		},
		{
			ResourceRef: v1alpha1.ResourceRef{
				Group:     "apps",
				Version:   "v1",
				Kind:      "ReplicaSet",
				Namespace: "sandbox-rollout-numalogic-demo",
				Name:      "numalogic-rollout-demo-5dcd5457d5",
				UID:       "75c30dce-1b66-414f-a86c-573a74be0f40",
			},
			ParentRefs: []v1alpha1.ResourceRef{
				{
					Group:     "argoproj.io",
					Version:   "",
					Kind:      "Rollout",
					Namespace: "sandbox-rollout-numalogic-demo",
					Name:      "numalogic-rollout-demo",
					UID:       "87f3aab0-f634-4b2c-959a-7ddd30675ed0",
				},
			},
		},
		{
			ResourceRef: v1alpha1.ResourceRef{
				Group:     "argoproj.io",
				Version:   "",
				Kind:      "Rollout",
				Namespace: "sandbox-rollout-numalogic-demo",
				Name:      "numalogic-rollout-demo",
				UID:       "87f3aab0-f634-4b2c-959a-7ddd30675ed0",
			},
		},
	}
	nodeMapping := make(map[string]*v1alpha1.ResourceNode)
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
	nodes := []*v1alpha1.ResourceNode{
		{
			ResourceRef: v1alpha1.ResourceRef{
				Group:     "",
				Version:   "v1",
				Kind:      "Pod",
				Namespace: "sandbox-rollout-numalogic-demo",
				Name:      "numalogic-rollout-demo-5dcd5457d5-6trpt",
				UID:       "92c3a5fe-d13e-4ae2-b8ec-c10dd3543b28",
			},
			ParentRefs: []v1alpha1.ResourceRef{
				{
					Group:     "apps",
					Version:   "v1",
					Kind:      "ReplicaSet",
					Namespace: "sandbox-rollout-numalogic-demo",
					Name:      "numalogic-rollout-demo-5dcd5457d5",
					UID:       "75c30dce-1b66-414f-a86c-573a74be0f40",
				},
			},
		},
		{
			ResourceRef: v1alpha1.ResourceRef{
				Group:     "apps",
				Version:   "v1",
				Kind:      "ReplicaSet",
				Namespace: "sandbox-rollout-numalogic-demo",
				Name:      "numalogic-rollout-demo-5dcd5457d5",
				UID:       "75c30dce-1b66-414f-a86c-573a74be0f40",
			},
			ParentRefs: []v1alpha1.ResourceRef{
				{
					Group:     "argoproj.io",
					Version:   "",
					Kind:      "Rollout",
					Namespace: "sandbox-rollout-numalogic-demo",
					Name:      "numalogic-rollout-demo",
					UID:       "87f3aab0-f634-4b2c-959a-7ddd30675ed0",
				},
			},
		},
		{
			ResourceRef: v1alpha1.ResourceRef{
				Group:     "argoproj.io",
				Version:   "",
				Kind:      "Rollout",
				Namespace: "sandbox-rollout-numalogic-demo",
				Name:      "numalogic-rollout-demo",
				UID:       "87f3aab0-f634-4b2c-959a-7ddd30675ed0",
			},
			Health: &v1alpha1.HealthStatus{
				Status:  "Degraded",
				Message: "Readiness Gate failed",
			},
		},
	}

	nodeMapping := make(map[string]*v1alpha1.ResourceNode)
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

func TestExtractNestedItem(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]any
		fields   []string
		depth    int
		expected map[string]any
	}{
		{
			name: "extract simple nested item",
			obj: map[string]any{
				"listofitems": []any{
					map[string]any{
						"extract":     "123",
						"dontextract": "abc",
					},
					map[string]any{
						"extract":     "456",
						"dontextract": "def",
					},
					map[string]any{
						"extract":     "789",
						"dontextract": "ghi",
					},
				},
			},
			fields: []string{"listofitems", "extract"},
			depth:  0,
			expected: map[string]any{
				"listofitems": []any{
					map[string]any{
						"extract": "123",
					},
					map[string]any{
						"extract": "456",
					},
					map[string]any{
						"extract": "789",
					},
				},
			},
		},
		{
			name: "double nested list of objects",
			obj: map[string]any{
				"listofitems": []any{
					map[string]any{
						"doublenested": []any{
							map[string]any{
								"extract": "123",
							},
						},
						"dontextract": "abc",
					},
					map[string]any{
						"doublenested": []any{
							map[string]any{
								"extract": "456",
							},
						},
						"dontextract": "def",
					},
					map[string]any{
						"doublenested": []any{
							map[string]any{
								"extract": "789",
							},
						},
						"dontextract": "ghi",
					},
				},
			},
			fields: []string{"listofitems", "doublenested", "extract"},
			depth:  0,
			expected: map[string]any{
				"listofitems": []any{
					map[string]any{
						"doublenested": []any{
							map[string]any{
								"extract": "123",
							},
						},
					},
					map[string]any{
						"doublenested": []any{
							map[string]any{
								"extract": "456",
							},
						},
					},
					map[string]any{
						"doublenested": []any{
							map[string]any{
								"extract": "789",
							},
						},
					},
				},
			},
		},
		{
			name:     "depth is greater then list of field size",
			obj:      map[string]any{"test1": "1234567890"},
			fields:   []string{"test1"},
			depth:    4,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filteredObj := extractNestedItem(tt.obj, tt.fields, tt.depth)
			assert.Equal(t, tt.expected, filteredObj, "Did not get the correct filtered obj")
		})
	}
}

func TestExtractItemsFromList(t *testing.T) {
	tests := []struct {
		name     string
		list     []any
		fields   []string
		expected []any
	}{
		{
			name: "test simple field",
			list: []any{
				map[string]any{"extract": "value1", "dontextract": "valueA"},
				map[string]any{"extract": "value2", "dontextract": "valueB"},
				map[string]any{"extract": "value3", "dontextract": "valueC"},
			},
			fields: []string{"extract"},
			expected: []any{
				map[string]any{"extract": "value1"},
				map[string]any{"extract": "value2"},
				map[string]any{"extract": "value3"},
			},
		},
		{
			name: "test simple field with some depth",
			list: []any{
				map[string]any{
					"test1": map[string]any{
						"test2": map[string]any{
							"extract":     "123",
							"dontextract": "abc",
						},
					},
				},
				map[string]any{
					"test1": map[string]any{
						"test2": map[string]any{
							"extract":     "456",
							"dontextract": "def",
						},
					},
				},
				map[string]any{
					"test1": map[string]any{
						"test2": map[string]any{
							"extract":     "789",
							"dontextract": "ghi",
						},
					},
				},
			},
			fields: []string{"test1", "test2", "extract"},
			expected: []any{
				map[string]any{
					"test1": map[string]any{
						"test2": map[string]any{
							"extract": "123",
						},
					},
				},
				map[string]any{
					"test1": map[string]any{
						"test2": map[string]any{
							"extract": "456",
						},
					},
				},
				map[string]any{
					"test1": map[string]any{
						"test2": map[string]any{
							"extract": "789",
						},
					},
				},
			},
		},
		{
			name: "test a missing field",
			list: []any{
				map[string]any{"test1": "123"},
				map[string]any{"test1": "456"},
				map[string]any{"test1": "789"},
			},
			fields:   []string{"test2"},
			expected: nil,
		},
		{
			name: "test getting an object",
			list: []any{
				map[string]any{
					"extract": map[string]any{
						"keyA": "valueA",
						"keyB": "valueB",
						"keyC": "valueC",
					},
					"dontextract": map[string]any{
						"key1": "value1",
						"key2": "value2",
						"key3": "value3",
					},
				},
				map[string]any{
					"extract": map[string]any{
						"keyD": "valueD",
						"keyE": "valueE",
						"keyF": "valueF",
					},
					"dontextract": map[string]any{
						"key4": "value4",
						"key5": "value5",
						"key6": "value6",
					},
				},
			},
			fields: []string{"extract"},
			expected: []any{
				map[string]any{
					"extract": map[string]any{
						"keyA": "valueA",
						"keyB": "valueB",
						"keyC": "valueC",
					},
				},
				map[string]any{
					"extract": map[string]any{
						"keyD": "valueD",
						"keyE": "valueE",
						"keyF": "valueF",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractedList := extractItemsFromList(tt.list, tt.fields)
			assert.Equal(t, tt.expected, extractedList, "Lists were not equal")
		})
	}
}

func TestReconstructObject(t *testing.T) {
	tests := []struct {
		name      string
		extracted []any
		fields    []string
		depth     int
		expected  map[string]any
	}{
		{
			name:      "simple single field at depth 0",
			extracted: []any{"value1", "value2"},
			fields:    []string{"items"},
			depth:     0,
			expected: map[string]any{
				"items": []any{"value1", "value2"},
			},
		},
		{
			name:      "object nested at depth 1",
			extracted: []any{map[string]any{"key": "value"}},
			fields:    []string{"test1", "test2"},
			depth:     1,
			expected: map[string]any{
				"test1": map[string]any{
					"test2": []any{map[string]any{"key": "value"}},
				},
			},
		},
		{
			name:      "empty list of extracted items",
			extracted: []any{},
			fields:    []string{"test1"},
			depth:     0,
			expected: map[string]any{
				"test1": []any{},
			},
		},
		{
			name: "complex object nesteed at depth 2",
			extracted: []any{map[string]any{
				"obj1": map[string]any{
					"key1": "value1",
					"key2": "value2",
				},
				"obj2": map[string]any{
					"keyA": "valueA",
					"keyB": "valueB",
				},
			}},
			fields: []string{"test1", "test2", "test3"},
			depth:  2,
			expected: map[string]any{
				"test1": map[string]any{
					"test2": map[string]any{
						"test3": []any{
							map[string]any{
								"obj1": map[string]any{
									"key1": "value1",
									"key2": "value2",
								},
								"obj2": map[string]any{
									"keyA": "valueA",
									"keyB": "valueB",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filteredObj := reconstructObject(tt.extracted, tt.fields, tt.depth)
			assert.Equal(t, tt.expected, filteredObj, "objects were not equal")
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
	assert.Contains(t, output, "FIELD           RESOURCE NAME  VALUE", "Missing or incorrect header line for table print with showing names.")
	assert.Contains(t, output, "apiVersion      unit-test      vX", "Missing or incorrect row in table related to apiVersion with showing names.")
	assert.Contains(t, output, "kind            unit-test      test", "Missing or incorrect line in the table related to kind with showing names.")
	assert.Contains(t, output, "spec.testfield  unit-test      testvalue", "Missing or incorrect line in the table related to spec.testfield with showing names.")
	assert.NotContains(t, output, "metadata.name   unit-test      testvalue", "Missing or incorrect line in the table related to metadata.name with showing names.")

	output, _ = captureOutput(func() error {
		printManifests(&[]unstructured.Unstructured{obj}, true, false, "wide")
		return nil
	})
	assert.Contains(t, output, "FIELD           VALUE", "Missing or incorrect header line for table print with not showing names.")
	assert.Contains(t, output, "apiVersion      vX", "Missing or incorrect row in table related to apiVersion with not showing names.")
	assert.Contains(t, output, "kind            test", "Missing or incorrect row in the table related to kind with not showing names.")
	assert.Contains(t, output, "spec.testfield  testvalue", "Missing or incorrect row in the table related to spec.testefield with not showing names.")
	assert.NotContains(t, output, "metadata.name   testvalue", "Missing or incorrect row in the tbale related to metadata.name with not showing names.")
}

func TestPrintManifests_FilterNestedListObject_Wide(t *testing.T) {
	obj := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "vX",
			"kind":       "kind",
			"metadata": map[string]any{
				"name": "unit-test",
			},
			"status": map[string]any{
				"podIPs": []map[string]any{
					{
						"IP": "127.0.0.1",
					},
					{
						"IP": "127.0.0.2",
					},
				},
			},
		},
	}

	output, _ := captureOutput(func() error {
		v, err := json.Marshal(&obj)
		if err != nil {
			return nil
		}

		var obj2 *unstructured.Unstructured
		err = json.Unmarshal([]byte(v), &obj2)
		if err != nil {
			return nil
		}
		printManifests(&[]unstructured.Unstructured{*obj2}, false, true, "wide")
		return nil
	})

	// Verify table header
	assert.Contains(t, output, "FIELD                RESOURCE NAME  VALUE", "Missing a line in the table")
	assert.Contains(t, output, "apiVersion           unit-test      vX", "Test for apiVersion field failed for wide output")
	assert.Contains(t, output, "kind                 unit-test      kind", "Test for kind field failed for wide output")
	assert.Contains(t, output, "status.podIPs[0].IP  unit-test      127.0.0.1", "Test for podIP array index 0 field failed for wide output")
	assert.Contains(t, output, "status.podIPs[1].IP  unit-test      127.0.0.2", "Test for podIP array index 1 field failed for wide output")
}
