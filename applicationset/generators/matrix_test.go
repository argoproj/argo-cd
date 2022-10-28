package generators

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
)

func TestMatrixGenerate(t *testing.T) {

	gitGenerator := &argoprojiov1alpha1.GitGenerator{
		RepoURL:     "RepoURL",
		Revision:    "Revision",
		Directories: []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
	}

	listGenerator := &argoprojiov1alpha1.ListGenerator{
		Elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "Cluster","url": "Url"}`)}},
	}

	testCases := []struct {
		name           string
		baseGenerators []argoprojiov1alpha1.ApplicationSetNestedGenerator
		expectedErr    error
		expected       []map[string]string
	}{
		{
			name: "happy flow - generate params",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					Git: gitGenerator,
				},
				{
					List: listGenerator,
				},
			},
			expected: []map[string]string{
				{"path": "app1", "path.basename": "app1", "path.basenameNormalized": "app1", "cluster": "Cluster", "url": "Url"},
				{"path": "app2", "path.basename": "app2", "path.basenameNormalized": "app2", "cluster": "Cluster", "url": "Url"},
			},
		},
		{
			name: "happy flow - generate params from two lists",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					List: &argoprojiov1alpha1.ListGenerator{
						Elements: []apiextensionsv1.JSON{
							{Raw: []byte(`{"a": "1"}`)},
							{Raw: []byte(`{"a": "2"}`)},
						},
					},
				},
				{
					List: &argoprojiov1alpha1.ListGenerator{
						Elements: []apiextensionsv1.JSON{
							{Raw: []byte(`{"b": "1"}`)},
							{Raw: []byte(`{"b": "2"}`)},
						},
					},
				},
			},
			expected: []map[string]string{
				{"a": "1", "b": "1"},
				{"a": "1", "b": "2"},
				{"a": "2", "b": "1"},
				{"a": "2", "b": "2"},
			},
		},
		{
			name: "returns error if there is less than two base generators",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					Git: gitGenerator,
				},
			},
			expectedErr: ErrLessThanTwoGenerators,
		},
		{
			name: "returns error if there is more than two base generators",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					List: listGenerator,
				},
				{
					List: listGenerator,
				},
				{
					List: listGenerator,
				},
			},
			expectedErr: ErrMoreThanTwoGenerators,
		},
		{
			name: "returns error if there is more than one inner generator in the first base generator",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					Git:  gitGenerator,
					List: listGenerator,
				},
				{
					Git: gitGenerator,
				},
			},
			expectedErr: ErrMoreThenOneInnerGenerators,
		},
		{
			name: "returns error if there is more than one inner generator in the second base generator",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					List: listGenerator,
				},
				{
					Git:  gitGenerator,
					List: listGenerator,
				},
			},
			expectedErr: ErrMoreThenOneInnerGenerators,
		},
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase // Since tests may run in parallel

		t.Run(testCaseCopy.name, func(t *testing.T) {
			mock := &generatorMock{}
			appSet := &argoprojiov1alpha1.ApplicationSet{}

			for _, g := range testCaseCopy.baseGenerators {

				gitGeneratorSpec := argoprojiov1alpha1.ApplicationSetGenerator{
					Git:  g.Git,
					List: g.List,
				}
				mock.On("GenerateParams", &gitGeneratorSpec, appSet).Return([]map[string]string{
					{
						"path":                    "app1",
						"path.basename":           "app1",
						"path.basenameNormalized": "app1",
					},
					{
						"path":                    "app2",
						"path.basename":           "app2",
						"path.basenameNormalized": "app2",
					},
				}, nil)

				mock.On("GetTemplate", &gitGeneratorSpec).
					Return(&argoprojiov1alpha1.ApplicationSetTemplate{})
			}

			var matrixGenerator = NewMatrixGenerator(
				map[string]Generator{
					"Git":  mock,
					"List": &ListGenerator{},
				},
			)

			got, err := matrixGenerator.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{
				Matrix: &argoprojiov1alpha1.MatrixGenerator{
					Generators: testCaseCopy.baseGenerators,
					Template:   argoprojiov1alpha1.ApplicationSetTemplate{},
				},
			}, appSet)

			if testCaseCopy.expectedErr != nil {
				assert.EqualError(t, err, testCaseCopy.expectedErr.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCaseCopy.expected, got)
			}

		})

	}
}

func TestMatrixGetRequeueAfter(t *testing.T) {

	gitGenerator := &argoprojiov1alpha1.GitGenerator{
		RepoURL:     "RepoURL",
		Revision:    "Revision",
		Directories: []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
	}

	listGenerator := &argoprojiov1alpha1.ListGenerator{
		Elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "Cluster","url": "Url"}`)}},
	}

	testCases := []struct {
		name               string
		baseGenerators     []argoprojiov1alpha1.ApplicationSetNestedGenerator
		gitGetRequeueAfter time.Duration
		expected           time.Duration
	}{
		{
			name: "return NoRequeueAfter if all the inner baseGenerators returns it",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					Git: gitGenerator,
				},
				{
					List: listGenerator,
				},
			},
			gitGetRequeueAfter: NoRequeueAfter,
			expected:           NoRequeueAfter,
		},
		{
			name: "returns the minimal time",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					Git: gitGenerator,
				},
				{
					List: listGenerator,
				},
			},
			gitGetRequeueAfter: time.Duration(1),
			expected:           time.Duration(1),
		},
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase // Since tests may run in parallel

		t.Run(testCaseCopy.name, func(t *testing.T) {
			mock := &generatorMock{}

			for _, g := range testCaseCopy.baseGenerators {
				gitGeneratorSpec := argoprojiov1alpha1.ApplicationSetGenerator{
					Git:  g.Git,
					List: g.List,
				}
				mock.On("GetRequeueAfter", &gitGeneratorSpec).Return(testCaseCopy.gitGetRequeueAfter, nil)
			}

			var matrixGenerator = NewMatrixGenerator(
				map[string]Generator{
					"Git":  mock,
					"List": &ListGenerator{},
				},
			)

			got := matrixGenerator.GetRequeueAfter(&argoprojiov1alpha1.ApplicationSetGenerator{
				Matrix: &argoprojiov1alpha1.MatrixGenerator{
					Generators: testCaseCopy.baseGenerators,
					Template:   argoprojiov1alpha1.ApplicationSetTemplate{},
				},
			})

			assert.Equal(t, testCaseCopy.expected, got)

		})

	}
}

type generatorMock struct {
	mock.Mock
}

func (g *generatorMock) GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate {
	args := g.Called(appSetGenerator)

	return args.Get(0).(*argoprojiov1alpha1.ApplicationSetTemplate)
}

func (g *generatorMock) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, appSet *argoprojiov1alpha1.ApplicationSet) ([]map[string]string, error) {
	args := g.Called(appSetGenerator, appSet)

	return args.Get(0).([]map[string]string), args.Error(1)
}

func (g *generatorMock) GetRequeueAfter(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) time.Duration {
	args := g.Called(appSetGenerator)

	return args.Get(0).(time.Duration)

}
