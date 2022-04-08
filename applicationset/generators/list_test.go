package generators

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
)

func TestGenerateListParams(t *testing.T) {
	testCases := []struct {
		elements []apiextensionsv1.JSON
		expected []map[string]string
	}{
		{
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url"}`)}},
			expected: []map[string]string{{"cluster": "cluster", "url": "url"}},
		}, {
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url","values":{"foo":"bar"}}`)}},
			expected: []map[string]string{{"cluster": "cluster", "url": "url", "values.foo": "bar"}},
		},
	}

	for _, testCase := range testCases {

		var listGenerator = NewListGenerator()

		got, err := listGenerator.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{
			List: &argoprojiov1alpha1.ListGenerator{
				Elements: testCase.elements,
			}}, nil)

		assert.NoError(t, err)
		assert.ElementsMatch(t, testCase.expected, got)

	}
}
