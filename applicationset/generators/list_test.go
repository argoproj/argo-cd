package generators

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestGenerateListParams(t *testing.T) {
	testCases := []struct {
		elements []apiextensionsv1.JSON
		expected []map[string]interface{}
	}{
		{
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url"}`)}},
			expected: []map[string]interface{}{{"cluster": "cluster", "url": "url"}},
		}, {
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url","values":{"foo":"bar"}}`)}},
			expected: []map[string]interface{}{{"cluster": "cluster", "url": "url", "values.foo": "bar"}},
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

func TestGenerateListParamsGoTemplate(t *testing.T) {
	testCases := []struct {
		elements []apiextensionsv1.JSON
		expected []map[string]interface{}
	}{
		{
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url"}`)}},
			expected: []map[string]interface{}{{"cluster": "cluster", "url": "url"}},
		}, {
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url","values":{"foo":"bar"}}`)}},
			expected: []map[string]interface{}{{"cluster": "cluster", "url": "url", "values": map[string]interface{}{"foo": "bar"}}},
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
