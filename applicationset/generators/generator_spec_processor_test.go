package generators

import (
	"context"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/applicationset/services/mocks"

	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	crtclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMatchValues(t *testing.T) {
	testCases := []struct {
		name     string
		elements []apiextensionsv1.JSON
		selector *metav1.LabelSelector
		expected []map[string]interface{}
	}{
		{
			name:     "no filter",
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url"}`)}},
			selector: &metav1.LabelSelector{},
			expected: []map[string]interface{}{{"cluster": "cluster", "url": "url"}},
		},
		{
			name:     "nil",
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url"}`)}},
			selector: nil,
			expected: []map[string]interface{}{{"cluster": "cluster", "url": "url"}},
		},
		{
			name:     "values.foo should be foo but is ignore element",
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url","values":{"foo":"bar"}}`)}},
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"values.foo": "foo",
				},
			},
			expected: []map[string]interface{}{},
		},
		{
			name:     "values.foo should be bar",
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url","values":{"foo":"bar"}}`)}},
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"values.foo": "bar",
				},
			},
			expected: []map[string]interface{}{{"cluster": "cluster", "url": "url", "values.foo": "bar"}},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			listGenerator := NewListGenerator()
			data := map[string]Generator{
				"List": listGenerator,
			}

			applicationSetInfo := argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					GoTemplate: false,
				},
			}

			results, err := Transform(argov1alpha1.ApplicationSetGenerator{
				Selector: testCase.selector,
				List: &argov1alpha1.ListGenerator{
					Elements: testCase.elements,
					Template: emptyTemplate(),
				},
			},
				data,
				emptyTemplate(),
				&applicationSetInfo, nil, nil)

			require.NoError(t, err)
			assert.ElementsMatch(t, testCase.expected, results[0].Params)
		})
	}
}

func TestMatchValuesGoTemplate(t *testing.T) {
	testCases := []struct {
		name     string
		elements []apiextensionsv1.JSON
		selector *metav1.LabelSelector
		expected []map[string]interface{}
	}{
		{
			name:     "no filter",
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url"}`)}},
			selector: &metav1.LabelSelector{},
			expected: []map[string]interface{}{{"cluster": "cluster", "url": "url"}},
		},
		{
			name:     "nil",
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url"}`)}},
			selector: nil,
			expected: []map[string]interface{}{{"cluster": "cluster", "url": "url"}},
		},
		{
			name:     "values.foo should be foo but is ignore element",
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url","values":{"foo":"bar"}}`)}},
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"values.foo": "foo",
				},
			},
			expected: []map[string]interface{}{},
		},
		{
			name:     "values.foo should be bar",
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url","values":{"foo":"bar"}}`)}},
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"values.foo": "bar",
				},
			},
			expected: []map[string]interface{}{{"cluster": "cluster", "url": "url", "values": map[string]interface{}{"foo": "bar"}}},
		},
		{
			name:     "values.0 should be bar",
			elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url","values":["bar"]}`)}},
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"values.0": "bar",
				},
			},
			expected: []map[string]interface{}{{"cluster": "cluster", "url": "url", "values": []interface{}{"bar"}}},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			listGenerator := NewListGenerator()
			data := map[string]Generator{
				"List": listGenerator,
			}

			applicationSetInfo := argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argov1alpha1.ApplicationSetSpec{
					GoTemplate: true,
				},
			}

			results, err := Transform(argov1alpha1.ApplicationSetGenerator{
				Selector: testCase.selector,
				List: &argov1alpha1.ListGenerator{
					Elements: testCase.elements,
					Template: emptyTemplate(),
				},
			},
				data,
				emptyTemplate(),
				&applicationSetInfo, nil, nil)

			require.NoError(t, err)
			assert.ElementsMatch(t, testCase.expected, results[0].Params)
		})
	}
}

func TestTransForm(t *testing.T) {
	testCases := []struct {
		name     string
		selector *metav1.LabelSelector
		expected []map[string]interface{}
	}{
		{
			name: "server filter",
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"server": "https://production-01.example.com"},
			},
			expected: []map[string]interface{}{{
				"metadata.annotations.foo.argoproj.io":           "production",
				"metadata.labels.argocd.argoproj.io/secret-type": "cluster",
				"metadata.labels.environment":                    "production",
				"metadata.labels.org":                            "bar",
				"name":                                           "production_01/west",
				"nameNormalized":                                 "production-01-west",
				"server":                                         "https://production-01.example.com",
			}},
		},
		{
			name: "server filter with long url",
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"server": "https://some-really-long-url-that-will-exceed-63-characters.com"},
			},
			expected: []map[string]interface{}{{
				"metadata.annotations.foo.argoproj.io":           "production",
				"metadata.labels.argocd.argoproj.io/secret-type": "cluster",
				"metadata.labels.environment":                    "production",
				"metadata.labels.org":                            "bar",
				"name":                                           "some-really-long-server-url",
				"nameNormalized":                                 "some-really-long-server-url",
				"server":                                         "https://some-really-long-url-that-will-exceed-63-characters.com",
			}},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testGenerators := map[string]Generator{
				"Clusters": getMockClusterGenerator(),
			}

			applicationSetInfo := argov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argov1alpha1.ApplicationSetSpec{},
			}

			results, err := Transform(
				argov1alpha1.ApplicationSetGenerator{
					Selector: testCase.selector,
					Clusters: &argov1alpha1.ClusterGenerator{
						Selector: metav1.LabelSelector{},
						Template: argov1alpha1.ApplicationSetTemplate{},
						Values:   nil,
					},
				},
				testGenerators,
				emptyTemplate(),
				&applicationSetInfo, nil, nil)

			require.NoError(t, err)
			assert.ElementsMatch(t, testCase.expected, results[0].Params)
		})
	}
}

func emptyTemplate() argov1alpha1.ApplicationSetTemplate {
	return argov1alpha1.ApplicationSetTemplate{
		Spec: argov1alpha1.ApplicationSpec{
			Project: "project",
		},
	}
}

func getMockClusterGenerator() Generator {
	clusters := []crtclient.Object{
		&corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "staging-01",
				Namespace: "namespace",
				Labels: map[string]string{
					"argocd.argoproj.io/secret-type": "cluster",
					"environment":                    "staging",
					"org":                            "foo",
				},
				Annotations: map[string]string{
					"foo.argoproj.io": "staging",
				},
			},
			Data: map[string][]byte{
				"config": []byte("{}"),
				"name":   []byte("staging-01"),
				"server": []byte("https://staging-01.example.com"),
			},
			Type: corev1.SecretType("Opaque"),
		},
		&corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "production-01",
				Namespace: "namespace",
				Labels: map[string]string{
					"argocd.argoproj.io/secret-type": "cluster",
					"environment":                    "production",
					"org":                            "bar",
				},
				Annotations: map[string]string{
					"foo.argoproj.io": "production",
				},
			},
			Data: map[string][]byte{
				"config": []byte("{}"),
				"name":   []byte("production_01/west"),
				"server": []byte("https://production-01.example.com"),
			},
			Type: corev1.SecretType("Opaque"),
		},
		&corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-really-long-server-url",
				Namespace: "namespace",
				Labels: map[string]string{
					"argocd.argoproj.io/secret-type": "cluster",
					"environment":                    "production",
					"org":                            "bar",
				},
				Annotations: map[string]string{
					"foo.argoproj.io": "production",
				},
			},
			Data: map[string][]byte{
				"config": []byte("{}"),
				"name":   []byte("some-really-long-server-url"),
				"server": []byte("https://some-really-long-url-that-will-exceed-63-characters.com"),
			},
			Type: corev1.SecretType("Opaque"),
		},
	}
	runtimeClusters := []runtime.Object{}
	for _, clientCluster := range clusters {
		runtimeClusters = append(runtimeClusters, clientCluster)
	}
	appClientset := kubefake.NewSimpleClientset(runtimeClusters...)

	fakeClient := fake.NewClientBuilder().WithObjects(clusters...).Build()
	return NewClusterGenerator(fakeClient, context.Background(), appClientset, "namespace")
}

func getMockGitGenerator() Generator {
	argoCDServiceMock := mocks.Repos{}
	argoCDServiceMock.On("GetDirectories", mock.Anything, mock.Anything, mock.Anything).Return([]string{"app1", "app2", "app_3", "p1/app4"}, nil)
	gitGenerator := NewGitGenerator(&argoCDServiceMock, "namespace")
	return gitGenerator
}

func TestGetRelevantGenerators(t *testing.T) {
	testGenerators := map[string]Generator{
		"Clusters": getMockClusterGenerator(),
		"Git":      getMockGitGenerator(),
	}

	testGenerators["Matrix"] = NewMatrixGenerator(testGenerators)
	testGenerators["Merge"] = NewMergeGenerator(testGenerators)
	testGenerators["List"] = NewListGenerator()

	requestedGenerator := &argov1alpha1.ApplicationSetGenerator{
		List: &argov1alpha1.ListGenerator{
			Elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url","values":{"foo":"bar"}}`)}},
		},
	}

	relevantGenerators := GetRelevantGenerators(requestedGenerator, testGenerators)
	assert.Len(t, relevantGenerators, 1)
	assert.IsType(t, &ListGenerator{}, relevantGenerators[0])

	requestedGenerator = &argov1alpha1.ApplicationSetGenerator{
		Clusters: &argov1alpha1.ClusterGenerator{
			Selector: metav1.LabelSelector{},
			Template: argov1alpha1.ApplicationSetTemplate{},
			Values:   nil,
		},
	}

	relevantGenerators = GetRelevantGenerators(requestedGenerator, testGenerators)
	assert.Len(t, relevantGenerators, 1)
	assert.IsType(t, &ClusterGenerator{}, relevantGenerators[0])

	requestedGenerator = &argov1alpha1.ApplicationSetGenerator{
		Git: &argov1alpha1.GitGenerator{
			RepoURL:             "",
			Directories:         nil,
			Files:               nil,
			Revision:            "",
			RequeueAfterSeconds: nil,
			Template:            argov1alpha1.ApplicationSetTemplate{},
		},
	}

	relevantGenerators = GetRelevantGenerators(requestedGenerator, testGenerators)
	assert.Len(t, relevantGenerators, 1)
	assert.IsType(t, &GitGenerator{}, relevantGenerators[0])
}

func TestInterpolateGenerator(t *testing.T) {
	requestedGenerator := &argov1alpha1.ApplicationSetGenerator{
		Clusters: &argov1alpha1.ClusterGenerator{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"argocd.argoproj.io/secret-type": "cluster",
					"path-basename":                  "{{path.basename}}",
					"path-zero":                      "{{path[0]}}",
					"path-full":                      "{{path}}",
				},
			},
		},
	}
	gitGeneratorParams := map[string]interface{}{
		"path":                    "p1/p2/app3",
		"path.basename":           "app3",
		"path[0]":                 "p1",
		"path[1]":                 "p2",
		"path.basenameNormalized": "app3",
	}
	interpolatedGenerator, err := InterpolateGenerator(requestedGenerator, gitGeneratorParams, false, nil)
	if err != nil {
		log.WithError(err).WithField("requestedGenerator", requestedGenerator).Error("error interpolating Generator")
		return
	}
	assert.Equal(t, "app3", interpolatedGenerator.Clusters.Selector.MatchLabels["path-basename"])
	assert.Equal(t, "p1", interpolatedGenerator.Clusters.Selector.MatchLabels["path-zero"])
	assert.Equal(t, "p1/p2/app3", interpolatedGenerator.Clusters.Selector.MatchLabels["path-full"])

	fileNamePath := argov1alpha1.GitFileGeneratorItem{
		Path: "{{name}}",
	}
	fileServerPath := argov1alpha1.GitFileGeneratorItem{
		Path: "{{server}}",
	}

	requestedGenerator = &argov1alpha1.ApplicationSetGenerator{
		Git: &argov1alpha1.GitGenerator{
			Files:    append([]argov1alpha1.GitFileGeneratorItem{}, fileNamePath, fileServerPath),
			Template: argov1alpha1.ApplicationSetTemplate{},
		},
	}
	clusterGeneratorParams := map[string]interface{}{
		"name": "production_01/west", "server": "https://production-01.example.com",
	}
	interpolatedGenerator, err = InterpolateGenerator(requestedGenerator, clusterGeneratorParams, false, nil)
	if err != nil {
		log.WithError(err).WithField("requestedGenerator", requestedGenerator).Error("error interpolating Generator")
		return
	}
	assert.Equal(t, "production_01/west", interpolatedGenerator.Git.Files[0].Path)
	assert.Equal(t, "https://production-01.example.com", interpolatedGenerator.Git.Files[1].Path)
}

func TestInterpolateGenerator_go(t *testing.T) {
	requestedGenerator := &argov1alpha1.ApplicationSetGenerator{
		Clusters: &argov1alpha1.ClusterGenerator{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"argocd.argoproj.io/secret-type": "cluster",
					"path-basename":                  "{{base .path.path}}",
					"path-zero":                      "{{index .path.segments 0}}",
					"path-full":                      "{{.path.path}}",
					"kubernetes.io/environment":      `{{default "foo" .my_label}}`,
				},
			},
		},
	}
	gitGeneratorParams := map[string]interface{}{
		"path": map[string]interface{}{
			"path":     "p1/p2/app3",
			"segments": []string{"p1", "p2", "app3"},
		},
	}
	interpolatedGenerator, err := InterpolateGenerator(requestedGenerator, gitGeneratorParams, true, nil)
	require.NoError(t, err)
	if err != nil {
		log.WithError(err).WithField("requestedGenerator", requestedGenerator).Error("error interpolating Generator")
		return
	}
	assert.Equal(t, "app3", interpolatedGenerator.Clusters.Selector.MatchLabels["path-basename"])
	assert.Equal(t, "p1", interpolatedGenerator.Clusters.Selector.MatchLabels["path-zero"])
	assert.Equal(t, "p1/p2/app3", interpolatedGenerator.Clusters.Selector.MatchLabels["path-full"])

	fileNamePath := argov1alpha1.GitFileGeneratorItem{
		Path: "{{.name}}",
	}
	fileServerPath := argov1alpha1.GitFileGeneratorItem{
		Path: "{{.server}}",
	}

	requestedGenerator = &argov1alpha1.ApplicationSetGenerator{
		Git: &argov1alpha1.GitGenerator{
			Files:    append([]argov1alpha1.GitFileGeneratorItem{}, fileNamePath, fileServerPath),
			Template: argov1alpha1.ApplicationSetTemplate{},
		},
	}
	clusterGeneratorParams := map[string]interface{}{
		"name": "production_01/west", "server": "https://production-01.example.com",
	}
	interpolatedGenerator, err = InterpolateGenerator(requestedGenerator, clusterGeneratorParams, true, nil)
	if err != nil {
		log.WithError(err).WithField("requestedGenerator", requestedGenerator).Error("error interpolating Generator")
		return
	}
	assert.Equal(t, "production_01/west", interpolatedGenerator.Git.Files[0].Path)
	assert.Equal(t, "https://production-01.example.com", interpolatedGenerator.Git.Files[1].Path)
}

func TestInterpolateGeneratorError(t *testing.T) {
	type args struct {
		requestedGenerator *argov1alpha1.ApplicationSetGenerator
		params             map[string]interface{}
		useGoTemplate      bool
		goTemplateOptions  []string
	}
	tests := []struct {
		name           string
		args           args
		want           argov1alpha1.ApplicationSetGenerator
		expectedErrStr string
	}{
		{name: "Empty Gen", args: args{
			requestedGenerator: nil,
			params:             nil,
			useGoTemplate:      false,
			goTemplateOptions:  nil,
		}, want: argov1alpha1.ApplicationSetGenerator{}, expectedErrStr: "generator is empty"},
		{name: "No Params", args: args{
			requestedGenerator: &argov1alpha1.ApplicationSetGenerator{},
			params:             map[string]interface{}{},
			useGoTemplate:      false,
			goTemplateOptions:  nil,
		}, want: argov1alpha1.ApplicationSetGenerator{}, expectedErrStr: ""},
		{name: "Error templating", args: args{
			requestedGenerator: &argov1alpha1.ApplicationSetGenerator{Git: &argov1alpha1.GitGenerator{
				RepoURL:  "foo",
				Files:    []argov1alpha1.GitFileGeneratorItem{{Path: "bar/"}},
				Revision: "main",
				Values: map[string]string{
					"git_test":  "{{ toPrettyJson . }}",
					"selection": "{{ default .override .test }}",
					"resolved":  "{{ index .rmap (default .override .test) }}",
				},
			}},
			params: map[string]interface{}{
				"name":     "in-cluster",
				"override": "foo",
			},
			useGoTemplate:     true,
			goTemplateOptions: []string{},
		}, want: argov1alpha1.ApplicationSetGenerator{}, expectedErrStr: "failed to replace parameters in generator: failed to execute go template {{ index .rmap (default .override .test) }}: template: :1:3: executing \"\" at <index .rmap (default .override .test)>: error calling index: index of untyped nil"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := InterpolateGenerator(tt.args.requestedGenerator, tt.args.params, tt.args.useGoTemplate, tt.args.goTemplateOptions)
			if tt.expectedErrStr != "" {
				require.EqualError(t, err, tt.expectedErrStr)
			} else {
				require.NoError(t, err)
			}
			assert.Equalf(t, tt.want, got, "InterpolateGenerator(%v, %v, %v, %v)", tt.args.requestedGenerator, tt.args.params, tt.args.useGoTemplate, tt.args.goTemplateOptions)
		})
	}
}
