package generators

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestGenerateListParams(t *testing.T) {
	testCases := []struct {
		elements []apiextensionsv1.JSON
		expected []map[string]any
	}{
		{
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url"}`)}},
			expected: []map[string]any{{"cluster": "cluster", "url": "url"}},
		}, {
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url","values":{"foo":"bar"}}`)}},
			expected: []map[string]any{{"cluster": "cluster", "url": "url", "values.foo": "bar"}},
		}, {
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"name": "test", "syncPrune": true, "number": 123, "values": {"innerBool": false}}`)}},
			expected: []map[string]any{
				{
					"name":             "test",
					"syncPrune":        "true",
					"number":           "123",
					"values.innerBool": "false",
				},
			},
		}, {
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"name": "test", "float": 1.5, "nullable": null}`)}},
			expected: []map[string]any{
				{
					"name":     "test",
					"float":    "1.5",
					"nullable": "",
				},
			},
		},
	}

	for _, testCase := range testCases {
		listGenerator := NewListGenerator()

		applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "set",
			},
			Spec: argoprojiov1alpha1.ApplicationSetSpec{},
		}

		got, err := listGenerator.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{
			List: &argoprojiov1alpha1.ListGenerator{
				Elements: testCase.elements,
			},
		}, &applicationSetInfo, nil)

		require.NoError(t, err)
		assert.ElementsMatch(t, testCase.expected, got)
	}
}

func TestGenerateListParamsError(t *testing.T) {
	testCases := []struct {
		name     string
		elements []apiextensionsv1.JSON
	}{
		{
			name:     "nested array",
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster", "tags": ["prod", "us-east"]}`)}},
		},
		{
			name:     "nested object",
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster", "nested": {"key": "value"}}`)}},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			listGenerator := NewListGenerator()

			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{},
			}

			_, err := listGenerator.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{
				List: &argoprojiov1alpha1.ListGenerator{
					Elements: testCase.elements,
				},
			}, &applicationSetInfo, nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "nested objects and arrays are not supported in non-GoTemplate mode")
		})
	}
}

func TestGenerateListParamsGoTemplate(t *testing.T) {
	testCases := []struct {
		elements []apiextensionsv1.JSON
		expected []map[string]any
	}{
		{
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url"}`)}},
			expected: []map[string]any{{"cluster": "cluster", "url": "url"}},
		}, {
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url","values":{"foo":"bar"}}`)}},
			expected: []map[string]any{{"cluster": "cluster", "url": "url", "values": map[string]any{"foo": "bar"}}},
		}, {
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"name": "test", "float": 1.5, "nullable": null}`)}},
			expected: []map[string]any{{"name": "test", "float": 1.5, "nullable": nil}},
		},
	}

	for _, testCase := range testCases {
		listGenerator := NewListGenerator()

		applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "set",
			},
			Spec: argoprojiov1alpha1.ApplicationSetSpec{
				GoTemplate: true,
			},
		}

		got, err := listGenerator.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{
			List: &argoprojiov1alpha1.ListGenerator{
				Elements: testCase.elements,
			},
		}, &applicationSetInfo, nil)

		require.NoError(t, err)
		assert.ElementsMatch(t, testCase.expected, got)
	}
}

func TestGenerateListParamsYaml(t *testing.T) {
	testCases := []struct {
		name         string
		elementsYaml string
		goTemplate   bool
		expected     []map[string]any
		expectedErr  string
	}{
		{
			name:         "primitives",
			elementsYaml: "- name: test\n  syncPrune: true\n  number: 123",
			expected: []map[string]any{
				{
					"name":      "test",
					"syncPrune": "true",
					"number":    "123",
				},
			},
		},
		{
			name:         "values",
			elementsYaml: "- name: test\n  values:\n    foo: bar",
			expected: []map[string]any{
				{
					"name":       "test",
					"values.foo": "bar",
				},
			},
		},
		{
			name:         "go template",
			elementsYaml: "- name: test\n  number: 123\n  values:\n    foo: bar",
			goTemplate:   true,
			expected: []map[string]any{
				{
					"name":   "test",
					"number": float64(123),
					"values": map[string]any{"foo": "bar"},
				},
			},
		},
		{
			name:         "nested error",
			elementsYaml: "- name: test\n  nested: {foo: bar}",
			expectedErr:  "not supported",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			listGenerator := NewListGenerator()

			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					GoTemplate: testCase.goTemplate,
				},
			}

			got, err := listGenerator.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{
				List: &argoprojiov1alpha1.ListGenerator{
					ElementsYaml: testCase.elementsYaml,
				},
			}, &applicationSetInfo, nil)

			if testCase.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), testCase.expectedErr)
			} else {
				require.NoError(t, err)
				assert.ElementsMatch(t, testCase.expected, got)
			}
		})
	}
}
