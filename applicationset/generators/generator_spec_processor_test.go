package generators

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
)

func TestMatchValues(t *testing.T) {
	testCases := []struct {
		name     string
		elements []apiextensionsv1.JSON
		selector *metav1.LabelSelector
		expected []map[string]string
	}{
		{
			name:     "no filter",
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url"}`)}},
			selector: &metav1.LabelSelector{},
			expected: []map[string]string{{"cluster": "cluster", "url": "url"}},
		},
		{
			name:     "nil",
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url"}`)}},
			selector: nil,
			expected: []map[string]string{{"cluster": "cluster", "url": "url"}},
		},
		{
			name:     "values.foo should be foo but is ignore element",
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url","values":{"foo":"bar"}}`)}},
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"values.foo": "foo",
				},
			},
			expected: []map[string]string{},
		},
		{
			name:     "values.foo should be bar",
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url","values":{"foo":"bar"}}`)}},
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"values.foo": "bar",
				},
			},
			expected: []map[string]string{{"cluster": "cluster", "url": "url", "values.foo": "bar"}},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var listGenerator = NewListGenerator()
			var data = map[string]Generator{
				"List": listGenerator,
			}

			results, err := Transform(v1alpha1.ApplicationSetGenerator{
				Selector: testCase.selector,
				List: &v1alpha1.ListGenerator{
					Elements: testCase.elements,
					Template: emptyTemplate(),
				}},
				data,
				emptyTemplate(),
				nil)

			assert.NoError(t, err)
			assert.ElementsMatch(t, testCase.expected, results[0].Params)
		})
	}
}

func emptyTemplate() v1alpha1.ApplicationSetTemplate {
	return v1alpha1.ApplicationSetTemplate{
		Spec: argov1alpha1.ApplicationSpec{
			Project: "project",
		},
	}
}
