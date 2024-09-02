package generators

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj/argo-cd/v2/applicationset/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestMatrixGenerate(t *testing.T) {
	gitGenerator := &argoprojiov1alpha1.GitGenerator{
		RepoURL:     "RepoURL",
		Revision:    "Revision",
		Directories: []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
	}

	listGenerator := &argoprojiov1alpha1.ListGenerator{
		Elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "Cluster","url": "Url", "templated": "test-{{path.basenameNormalized}}"}`)}},
	}

	testCases := []struct {
		name           string
		baseGenerators []argoprojiov1alpha1.ApplicationSetNestedGenerator
		expectedErr    error
		expected       []map[string]interface{}
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
			expected: []map[string]interface{}{
				{"path": "app1", "path.basename": "app1", "path.basenameNormalized": "app1", "cluster": "Cluster", "url": "Url", "templated": "test-app1"},
				{"path": "app2", "path.basename": "app2", "path.basenameNormalized": "app2", "cluster": "Cluster", "url": "Url", "templated": "test-app2"},
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
			expected: []map[string]interface{}{
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
			genMock := &generatorMock{}
			appSet := &argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{},
			}

			for _, g := range testCaseCopy.baseGenerators {
				gitGeneratorSpec := argoprojiov1alpha1.ApplicationSetGenerator{
					Git:  g.Git,
					List: g.List,
				}
				genMock.On("GenerateParams", mock.AnythingOfType("*v1alpha1.ApplicationSetGenerator"), appSet, mock.Anything).Return([]map[string]interface{}{
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

				genMock.On("GetTemplate", &gitGeneratorSpec).
					Return(&argoprojiov1alpha1.ApplicationSetTemplate{})
			}

			matrixGenerator := NewMatrixGenerator(
				map[string]Generator{
					"Git":  genMock,
					"List": &ListGenerator{},
				},
			)

			got, err := matrixGenerator.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{
				Matrix: &argoprojiov1alpha1.MatrixGenerator{
					Generators: testCaseCopy.baseGenerators,
					Template:   argoprojiov1alpha1.ApplicationSetTemplate{},
				},
			}, appSet, nil)

			if testCaseCopy.expectedErr != nil {
				require.ErrorIs(t, err, testCaseCopy.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testCaseCopy.expected, got)
			}
		})
	}
}

func TestMatrixGenerateGoTemplate(t *testing.T) {
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
		expected       []map[string]interface{}
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
			expected: []map[string]interface{}{
				{
					"path": map[string]string{
						"path":               "app1",
						"basename":           "app1",
						"basenameNormalized": "app1",
					},
					"cluster": "Cluster",
					"url":     "Url",
				},
				{
					"path": map[string]string{
						"path":               "app2",
						"basename":           "app2",
						"basenameNormalized": "app2",
					},
					"cluster": "Cluster",
					"url":     "Url",
				},
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
			expected: []map[string]interface{}{
				{"a": "1", "b": "1"},
				{"a": "1", "b": "2"},
				{"a": "2", "b": "1"},
				{"a": "2", "b": "2"},
			},
		},
		{
			name: "parameter override: first list elements take precedence",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					List: &argoprojiov1alpha1.ListGenerator{
						Elements: []apiextensionsv1.JSON{
							{Raw: []byte(`{"booleanFalse": false, "booleanTrue": true, "stringFalse": "false", "stringTrue": "true"}`)},
						},
					},
				},
				{
					List: &argoprojiov1alpha1.ListGenerator{
						Elements: []apiextensionsv1.JSON{
							{Raw: []byte(`{"booleanFalse": true, "booleanTrue": false, "stringFalse": "true", "stringTrue": "false"}`)},
						},
					},
				},
			},
			expected: []map[string]interface{}{
				{"booleanFalse": false, "booleanTrue": true, "stringFalse": "false", "stringTrue": "true"},
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
			genMock := &generatorMock{}
			appSet := &argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					GoTemplate: true,
				},
			}

			for _, g := range testCaseCopy.baseGenerators {
				gitGeneratorSpec := argoprojiov1alpha1.ApplicationSetGenerator{
					Git:  g.Git,
					List: g.List,
				}
				genMock.On("GenerateParams", mock.AnythingOfType("*v1alpha1.ApplicationSetGenerator"), appSet, mock.Anything).Return([]map[string]interface{}{
					{
						"path": map[string]string{
							"path":               "app1",
							"basename":           "app1",
							"basenameNormalized": "app1",
						},
					},
					{
						"path": map[string]string{
							"path":               "app2",
							"basename":           "app2",
							"basenameNormalized": "app2",
						},
					},
				}, nil)

				genMock.On("GetTemplate", &gitGeneratorSpec).
					Return(&argoprojiov1alpha1.ApplicationSetTemplate{})
			}

			matrixGenerator := NewMatrixGenerator(
				map[string]Generator{
					"Git":  genMock,
					"List": &ListGenerator{},
				},
			)

			got, err := matrixGenerator.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{
				Matrix: &argoprojiov1alpha1.MatrixGenerator{
					Generators: testCaseCopy.baseGenerators,
					Template:   argoprojiov1alpha1.ApplicationSetTemplate{},
				},
			}, appSet, nil)

			if testCaseCopy.expectedErr != nil {
				require.ErrorIs(t, err, testCaseCopy.expectedErr)
			} else {
				require.NoError(t, err)
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

	pullRequestGenerator := &argoprojiov1alpha1.PullRequestGenerator{}

	scmGenerator := &argoprojiov1alpha1.SCMProviderGenerator{}

	duckTypeGenerator := &argoprojiov1alpha1.DuckTypeGenerator{}

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
		{
			name: "returns the minimal time for pull request",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					Git: gitGenerator,
				},
				{
					PullRequest: pullRequestGenerator,
				},
			},
			gitGetRequeueAfter: time.Duration(15 * time.Second),
			expected:           time.Duration(15 * time.Second),
		},
		{
			name: "returns the default time if no requeueAfterSeconds is provided",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					Git: gitGenerator,
				},
				{
					PullRequest: pullRequestGenerator,
				},
			},
			expected: time.Duration(30 * time.Minute),
		},
		{
			name: "returns the default time for duck type generator",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					Git: gitGenerator,
				},
				{
					ClusterDecisionResource: duckTypeGenerator,
				},
			},
			expected: time.Duration(3 * time.Minute),
		},
		{
			name: "returns the default time for scm generator",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					Git: gitGenerator,
				},
				{
					SCMProvider: scmGenerator,
				},
			},
			expected: time.Duration(30 * time.Minute),
		},
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase // Since tests may run in parallel

		t.Run(testCaseCopy.name, func(t *testing.T) {
			mock := &generatorMock{}

			for _, g := range testCaseCopy.baseGenerators {
				gitGeneratorSpec := argoprojiov1alpha1.ApplicationSetGenerator{
					Git:                     g.Git,
					List:                    g.List,
					PullRequest:             g.PullRequest,
					SCMProvider:             g.SCMProvider,
					ClusterDecisionResource: g.ClusterDecisionResource,
				}
				mock.On("GetRequeueAfter", &gitGeneratorSpec).Return(testCaseCopy.gitGetRequeueAfter, nil)
			}

			matrixGenerator := NewMatrixGenerator(
				map[string]Generator{
					"Git":                     mock,
					"List":                    &ListGenerator{},
					"PullRequest":             &PullRequestGenerator{},
					"SCMProvider":             &SCMProviderGenerator{},
					"ClusterDecisionResource": &DuckTypeGenerator{},
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

func TestInterpolatedMatrixGenerate(t *testing.T) {
	interpolatedGitGenerator := &argoprojiov1alpha1.GitGenerator{
		RepoURL:  "RepoURL",
		Revision: "Revision",
		Files: []argoprojiov1alpha1.GitFileGeneratorItem{
			{Path: "examples/git-generator-files-discovery/cluster-config/dev/config.json"},
			{Path: "examples/git-generator-files-discovery/cluster-config/prod/config.json"},
		},
	}

	interpolatedClusterGenerator := &argoprojiov1alpha1.ClusterGenerator{
		Selector: metav1.LabelSelector{
			MatchLabels:      map[string]string{"environment": "{{path.basename}}"},
			MatchExpressions: nil,
		},
	}
	testCases := []struct {
		name           string
		baseGenerators []argoprojiov1alpha1.ApplicationSetNestedGenerator
		expectedErr    error
		expected       []map[string]interface{}
		clientError    bool
	}{
		{
			name: "happy flow - generate interpolated params",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					Git: interpolatedGitGenerator,
				},
				{
					Clusters: interpolatedClusterGenerator,
				},
			},
			expected: []map[string]interface{}{
				{"path": "examples/git-generator-files-discovery/cluster-config/dev/config.json", "path.basename": "dev", "path.basenameNormalized": "dev", "name": "dev-01", "nameNormalized": "dev-01", "server": "https://dev-01.example.com", "metadata.labels.environment": "dev", "metadata.labels.argocd.argoproj.io/secret-type": "cluster"},
				{"path": "examples/git-generator-files-discovery/cluster-config/prod/config.json", "path.basename": "prod", "path.basenameNormalized": "prod", "name": "prod-01", "nameNormalized": "prod-01", "server": "https://prod-01.example.com", "metadata.labels.environment": "prod", "metadata.labels.argocd.argoproj.io/secret-type": "cluster"},
			},
			clientError: false,
		},
	}
	clusters := []client.Object{
		&corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dev-01",
				Namespace: "namespace",
				Labels: map[string]string{
					"argocd.argoproj.io/secret-type": "cluster",
					"environment":                    "dev",
				},
			},
			Data: map[string][]byte{
				"config": []byte("{}"),
				"name":   []byte("dev-01"),
				"server": []byte("https://dev-01.example.com"),
			},
			Type: corev1.SecretType("Opaque"),
		},
		&corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prod-01",
				Namespace: "namespace",
				Labels: map[string]string{
					"argocd.argoproj.io/secret-type": "cluster",
					"environment":                    "prod",
				},
			},
			Data: map[string][]byte{
				"config": []byte("{}"),
				"name":   []byte("prod-01"),
				"server": []byte("https://prod-01.example.com"),
			},
			Type: corev1.SecretType("Opaque"),
		},
	}
	// convert []client.Object to []runtime.Object, for use by kubefake package
	runtimeClusters := []runtime.Object{}
	for _, clientCluster := range clusters {
		runtimeClusters = append(runtimeClusters, clientCluster)
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase // Since tests may run in parallel

		t.Run(testCaseCopy.name, func(t *testing.T) {
			genMock := &generatorMock{}
			appSet := &argoprojiov1alpha1.ApplicationSet{}

			appClientset := kubefake.NewSimpleClientset(runtimeClusters...)
			fakeClient := fake.NewClientBuilder().WithObjects(clusters...).Build()
			cl := &possiblyErroringFakeCtrlRuntimeClient{
				fakeClient,
				testCase.clientError,
			}
			clusterGenerator := NewClusterGenerator(cl, context.Background(), appClientset, "namespace")

			for _, g := range testCaseCopy.baseGenerators {
				gitGeneratorSpec := argoprojiov1alpha1.ApplicationSetGenerator{
					Git:      g.Git,
					Clusters: g.Clusters,
				}
				genMock.On("GenerateParams", mock.AnythingOfType("*v1alpha1.ApplicationSetGenerator"), appSet).Return([]map[string]interface{}{
					{
						"path":                    "examples/git-generator-files-discovery/cluster-config/dev/config.json",
						"path.basename":           "dev",
						"path.basenameNormalized": "dev",
					},
					{
						"path":                    "examples/git-generator-files-discovery/cluster-config/prod/config.json",
						"path.basename":           "prod",
						"path.basenameNormalized": "prod",
					},
				}, nil)
				genMock.On("GetTemplate", &gitGeneratorSpec).
					Return(&argoprojiov1alpha1.ApplicationSetTemplate{})
			}
			matrixGenerator := NewMatrixGenerator(
				map[string]Generator{
					"Git":      genMock,
					"Clusters": clusterGenerator,
				},
			)

			got, err := matrixGenerator.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{
				Matrix: &argoprojiov1alpha1.MatrixGenerator{
					Generators: testCaseCopy.baseGenerators,
					Template:   argoprojiov1alpha1.ApplicationSetTemplate{},
				},
			}, appSet, nil)

			if testCaseCopy.expectedErr != nil {
				require.ErrorIs(t, err, testCaseCopy.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testCaseCopy.expected, got)
			}
		})
	}
}

func TestInterpolatedMatrixGenerateGoTemplate(t *testing.T) {
	interpolatedGitGenerator := &argoprojiov1alpha1.GitGenerator{
		RepoURL:  "RepoURL",
		Revision: "Revision",
		Files: []argoprojiov1alpha1.GitFileGeneratorItem{
			{Path: "examples/git-generator-files-discovery/cluster-config/dev/config.json"},
			{Path: "examples/git-generator-files-discovery/cluster-config/prod/config.json"},
		},
	}

	interpolatedClusterGenerator := &argoprojiov1alpha1.ClusterGenerator{
		Selector: metav1.LabelSelector{
			MatchLabels:      map[string]string{"environment": "{{.path.basename}}"},
			MatchExpressions: nil,
		},
	}
	testCases := []struct {
		name           string
		baseGenerators []argoprojiov1alpha1.ApplicationSetNestedGenerator
		expectedErr    error
		expected       []map[string]interface{}
		clientError    bool
	}{
		{
			name: "happy flow - generate interpolated params",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					Git: interpolatedGitGenerator,
				},
				{
					Clusters: interpolatedClusterGenerator,
				},
			},
			expected: []map[string]interface{}{
				{
					"path": map[string]string{
						"path":               "examples/git-generator-files-discovery/cluster-config/dev/config.json",
						"basename":           "dev",
						"basenameNormalized": "dev",
					},
					"name":           "dev-01",
					"nameNormalized": "dev-01",
					"server":         "https://dev-01.example.com",
					"metadata": map[string]interface{}{
						"labels": map[string]string{
							"environment":                    "dev",
							"argocd.argoproj.io/secret-type": "cluster",
						},
					},
				},
				{
					"path": map[string]string{
						"path":               "examples/git-generator-files-discovery/cluster-config/prod/config.json",
						"basename":           "prod",
						"basenameNormalized": "prod",
					},
					"name":           "prod-01",
					"nameNormalized": "prod-01",
					"server":         "https://prod-01.example.com",
					"metadata": map[string]interface{}{
						"labels": map[string]string{
							"environment":                    "prod",
							"argocd.argoproj.io/secret-type": "cluster",
						},
					},
				},
			},
			clientError: false,
		},
	}
	clusters := []client.Object{
		&corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dev-01",
				Namespace: "namespace",
				Labels: map[string]string{
					"argocd.argoproj.io/secret-type": "cluster",
					"environment":                    "dev",
				},
			},
			Data: map[string][]byte{
				"config": []byte("{}"),
				"name":   []byte("dev-01"),
				"server": []byte("https://dev-01.example.com"),
			},
			Type: corev1.SecretType("Opaque"),
		},
		&corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prod-01",
				Namespace: "namespace",
				Labels: map[string]string{
					"argocd.argoproj.io/secret-type": "cluster",
					"environment":                    "prod",
				},
			},
			Data: map[string][]byte{
				"config": []byte("{}"),
				"name":   []byte("prod-01"),
				"server": []byte("https://prod-01.example.com"),
			},
			Type: corev1.SecretType("Opaque"),
		},
	}
	// convert []client.Object to []runtime.Object, for use by kubefake package
	runtimeClusters := []runtime.Object{}
	for _, clientCluster := range clusters {
		runtimeClusters = append(runtimeClusters, clientCluster)
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase // Since tests may run in parallel

		t.Run(testCaseCopy.name, func(t *testing.T) {
			genMock := &generatorMock{}
			appSet := &argoprojiov1alpha1.ApplicationSet{
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					GoTemplate: true,
				},
			}

			appClientset := kubefake.NewSimpleClientset(runtimeClusters...)
			fakeClient := fake.NewClientBuilder().WithObjects(clusters...).Build()
			cl := &possiblyErroringFakeCtrlRuntimeClient{
				fakeClient,
				testCase.clientError,
			}
			clusterGenerator := NewClusterGenerator(cl, context.Background(), appClientset, "namespace")

			for _, g := range testCaseCopy.baseGenerators {
				gitGeneratorSpec := argoprojiov1alpha1.ApplicationSetGenerator{
					Git:      g.Git,
					Clusters: g.Clusters,
				}
				genMock.On("GenerateParams", mock.AnythingOfType("*v1alpha1.ApplicationSetGenerator"), appSet).Return([]map[string]interface{}{
					{
						"path": map[string]string{
							"path":               "examples/git-generator-files-discovery/cluster-config/dev/config.json",
							"basename":           "dev",
							"basenameNormalized": "dev",
						},
					},
					{
						"path": map[string]string{
							"path":               "examples/git-generator-files-discovery/cluster-config/prod/config.json",
							"basename":           "prod",
							"basenameNormalized": "prod",
						},
					},
				}, nil)
				genMock.On("GetTemplate", &gitGeneratorSpec).
					Return(&argoprojiov1alpha1.ApplicationSetTemplate{})
			}
			matrixGenerator := NewMatrixGenerator(
				map[string]Generator{
					"Git":      genMock,
					"Clusters": clusterGenerator,
				},
			)

			got, err := matrixGenerator.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{
				Matrix: &argoprojiov1alpha1.MatrixGenerator{
					Generators: testCaseCopy.baseGenerators,
					Template:   argoprojiov1alpha1.ApplicationSetTemplate{},
				},
			}, appSet, nil)

			if testCaseCopy.expectedErr != nil {
				require.ErrorIs(t, err, testCaseCopy.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testCaseCopy.expected, got)
			}
		})
	}
}

func TestMatrixGenerateListElementsYaml(t *testing.T) {
	gitGenerator := &argoprojiov1alpha1.GitGenerator{
		RepoURL:  "RepoURL",
		Revision: "Revision",
		Files: []argoprojiov1alpha1.GitFileGeneratorItem{
			{Path: "config.yaml"},
		},
	}

	listGenerator := &argoprojiov1alpha1.ListGenerator{
		Elements:     []apiextensionsv1.JSON{},
		ElementsYaml: "{{ .foo.bar | toJson }}",
	}

	testCases := []struct {
		name           string
		baseGenerators []argoprojiov1alpha1.ApplicationSetNestedGenerator
		expectedErr    error
		expected       []map[string]interface{}
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
			expected: []map[string]interface{}{
				{
					"chart":   "a",
					"version": "1",
					"foo": map[string]interface{}{
						"bar": []interface{}{
							map[string]interface{}{
								"chart":   "a",
								"version": "1",
							},
							map[string]interface{}{
								"chart":   "b",
								"version": "2",
							},
						},
					},
					"path": map[string]interface{}{
						"basename":           "dir",
						"basenameNormalized": "dir",
						"filename":           "file_name.yaml",
						"filenameNormalized": "file-name.yaml",
						"path":               "path/dir",
						"segments": []string{
							"path",
							"dir",
						},
					},
				},
				{
					"chart":   "b",
					"version": "2",
					"foo": map[string]interface{}{
						"bar": []interface{}{
							map[string]interface{}{
								"chart":   "a",
								"version": "1",
							},
							map[string]interface{}{
								"chart":   "b",
								"version": "2",
							},
						},
					},
					"path": map[string]interface{}{
						"basename":           "dir",
						"basenameNormalized": "dir",
						"filename":           "file_name.yaml",
						"filenameNormalized": "file-name.yaml",
						"path":               "path/dir",
						"segments": []string{
							"path",
							"dir",
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase // Since tests may run in parallel

		t.Run(testCaseCopy.name, func(t *testing.T) {
			genMock := &generatorMock{}
			appSet := &argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					GoTemplate: true,
				},
			}

			for _, g := range testCaseCopy.baseGenerators {
				gitGeneratorSpec := argoprojiov1alpha1.ApplicationSetGenerator{
					Git:  g.Git,
					List: g.List,
				}
				genMock.On("GenerateParams", mock.AnythingOfType("*v1alpha1.ApplicationSetGenerator"), appSet).Return([]map[string]any{{
					"foo": map[string]interface{}{
						"bar": []interface{}{
							map[string]interface{}{
								"chart":   "a",
								"version": "1",
							},
							map[string]interface{}{
								"chart":   "b",
								"version": "2",
							},
						},
					},
					"path": map[string]interface{}{
						"basename":           "dir",
						"basenameNormalized": "dir",
						"filename":           "file_name.yaml",
						"filenameNormalized": "file-name.yaml",
						"path":               "path/dir",
						"segments": []string{
							"path",
							"dir",
						},
					},
				}}, nil)
				genMock.On("GetTemplate", &gitGeneratorSpec).
					Return(&argoprojiov1alpha1.ApplicationSetTemplate{})
			}

			matrixGenerator := NewMatrixGenerator(
				map[string]Generator{
					"Git":  genMock,
					"List": &ListGenerator{},
				},
			)

			got, err := matrixGenerator.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{
				Matrix: &argoprojiov1alpha1.MatrixGenerator{
					Generators: testCaseCopy.baseGenerators,
					Template:   argoprojiov1alpha1.ApplicationSetTemplate{},
				},
			}, appSet, nil)

			if testCaseCopy.expectedErr != nil {
				require.ErrorIs(t, err, testCaseCopy.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testCaseCopy.expected, got)
			}
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

func (g *generatorMock) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, appSet *argoprojiov1alpha1.ApplicationSet, _ client.Client) ([]map[string]interface{}, error) {
	args := g.Called(appSetGenerator, appSet)

	return args.Get(0).([]map[string]interface{}), args.Error(1)
}

func (g *generatorMock) GetRequeueAfter(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) time.Duration {
	args := g.Called(appSetGenerator)

	return args.Get(0).(time.Duration)
}

func TestGitGenerator_GenerateParams_list_x_git_matrix_generator(t *testing.T) {
	// Given a matrix generator over a list generator and a git files generator, the nested git files generator should
	// be treated as a files generator, and it should produce parameters.

	// This tests for a specific bug where a nested git files generator was being treated as a directory generator. This
	// happened because, when the matrix generator was being processed, the nested git files generator was being
	// interpolated by the deeplyReplace function. That function cannot differentiate between a nil slice and an empty
	// slice. So it was replacing the `Directories` field with an empty slice, which the ApplicationSet controller
	// interpreted as meaning this was a directory generator, not a files generator.

	// Now instead of checking for nil, we check whether the field is a non-empty slice. This test prevents a regression
	// of that bug.

	listGeneratorMock := &generatorMock{}
	listGeneratorMock.On("GenerateParams", mock.AnythingOfType("*v1alpha1.ApplicationSetGenerator"), mock.AnythingOfType("*v1alpha1.ApplicationSet"), mock.Anything).Return([]map[string]interface{}{
		{"some": "value"},
	}, nil)
	listGeneratorMock.On("GetTemplate", mock.AnythingOfType("*v1alpha1.ApplicationSetGenerator")).Return(&argoprojiov1alpha1.ApplicationSetTemplate{})

	gitGeneratorSpec := &argoprojiov1alpha1.GitGenerator{
		RepoURL: "https://git.example.com",
		Files: []argoprojiov1alpha1.GitFileGeneratorItem{
			{Path: "some/path.json"},
		},
	}

	repoServiceMock := &mocks.Repos{}
	repoServiceMock.On("GetFiles", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(map[string][]byte{
		"some/path.json": []byte("test: content"),
	}, nil)
	gitGenerator := NewGitGenerator(repoServiceMock, "")

	matrixGenerator := NewMatrixGenerator(map[string]Generator{
		"List": listGeneratorMock,
		"Git":  gitGenerator,
	})

	matrixGeneratorSpec := &argoprojiov1alpha1.MatrixGenerator{
		Generators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
			{
				List: &argoprojiov1alpha1.ListGenerator{
					Elements: []apiextensionsv1.JSON{
						{
							Raw: []byte(`{"some": "value"}`),
						},
					},
				},
			},
			{
				Git: gitGeneratorSpec,
			},
		},
	}

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)
	appProject := argoprojiov1alpha1.AppProject{}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appProject).Build()

	params, err := matrixGenerator.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{
		Matrix: matrixGeneratorSpec,
	}, &argoprojiov1alpha1.ApplicationSet{}, client)
	require.NoError(t, err)
	assert.Equal(t, []map[string]interface{}{{
		"path":                    "some",
		"path.basename":           "some",
		"path.basenameNormalized": "some",
		"path.filename":           "path.json",
		"path.filenameNormalized": "path.json",
		"path[0]":                 "some",
		"some":                    "value",
		"test":                    "content",
	}}, params)
}
