package generators

import (
	"context"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	crtclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
)

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
	}
	runtimeClusters := []runtime.Object{}
	appClientset := kubefake.NewSimpleClientset(runtimeClusters...)

	fakeClient := fake.NewClientBuilder().WithObjects(clusters...).Build()
	return NewClusterGenerator(fakeClient, context.Background(), appClientset, "namespace")
}

func getMockGitGenerator() Generator {
	argoCDServiceMock := argoCDServiceMock{mock: &mock.Mock{}}
	argoCDServiceMock.mock.On("GetDirectories", mock.Anything, mock.Anything, mock.Anything).Return([]string{"app1", "app2", "app_3", "p1/app4"}, nil)
	var gitGenerator = NewGitGenerator(argoCDServiceMock)
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

	requestedGenerator := &argoprojiov1alpha1.ApplicationSetGenerator{
		List: &argoprojiov1alpha1.ListGenerator{
			Elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url","values":{"foo":"bar"}}`)}},
		}}

	relevantGenerators := GetRelevantGenerators(requestedGenerator, testGenerators)
	assert.Len(t, relevantGenerators, 1)
	assert.IsType(t, &ListGenerator{}, relevantGenerators[0])

	requestedGenerator = &argoprojiov1alpha1.ApplicationSetGenerator{
		Clusters: &argoprojiov1alpha1.ClusterGenerator{
			Selector: metav1.LabelSelector{},
			Template: argoprojiov1alpha1.ApplicationSetTemplate{},
			Values:   nil,
		},
	}

	relevantGenerators = GetRelevantGenerators(requestedGenerator, testGenerators)
	assert.Len(t, relevantGenerators, 1)
	assert.IsType(t, &ClusterGenerator{}, relevantGenerators[0])

	requestedGenerator = &argoprojiov1alpha1.ApplicationSetGenerator{
		Git: &argoprojiov1alpha1.GitGenerator{
			RepoURL:             "",
			Directories:         nil,
			Files:               nil,
			Revision:            "",
			RequeueAfterSeconds: nil,
			Template:            argoprojiov1alpha1.ApplicationSetTemplate{},
		},
	}

	relevantGenerators = GetRelevantGenerators(requestedGenerator, testGenerators)
	assert.Len(t, relevantGenerators, 1)
	assert.IsType(t, &GitGenerator{}, relevantGenerators[0])
}

func TestInterpolateGenerator(t *testing.T) {
	requestedGenerator := &argoprojiov1alpha1.ApplicationSetGenerator{
		Clusters: &argoprojiov1alpha1.ClusterGenerator{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"argocd.argoproj.io/secret-type": "cluster",
					"path-basename":                  "{{path.basename}}",
					"path-zero":                      "{{path[0]}}",
					"path-full":                      "{{path}}",
				}},
		},
	}
	gitGeneratorParams := map[string]string{
		"path": "p1/p2/app3", "path.basename": "app3", "path[0]": "p1", "path[1]": "p2", "path.basenameNormalized": "app3",
	}
	interpolatedGenerator, err := interpolateGenerator(requestedGenerator, gitGeneratorParams)
	if err != nil {
		log.WithError(err).WithField("requestedGenerator", requestedGenerator).Error("error interpolating Generator")
		return
	}
	assert.Equal(t, "app3", interpolatedGenerator.Clusters.Selector.MatchLabels["path-basename"])
	assert.Equal(t, "p1", interpolatedGenerator.Clusters.Selector.MatchLabels["path-zero"])
	assert.Equal(t, "p1/p2/app3", interpolatedGenerator.Clusters.Selector.MatchLabels["path-full"])

	fileNamePath := argoprojiov1alpha1.GitFileGeneratorItem{
		Path: "{{name}}",
	}
	fileServerPath := argoprojiov1alpha1.GitFileGeneratorItem{
		Path: "{{server}}",
	}

	requestedGenerator = &argoprojiov1alpha1.ApplicationSetGenerator{
		Git: &argoprojiov1alpha1.GitGenerator{
			Files:    append([]argoprojiov1alpha1.GitFileGeneratorItem{}, fileNamePath, fileServerPath),
			Template: argoprojiov1alpha1.ApplicationSetTemplate{},
		},
	}
	clusterGeneratorParams := map[string]string{
		"name": "production_01/west", "server": "https://production-01.example.com",
	}
	interpolatedGenerator, err = interpolateGenerator(requestedGenerator, clusterGeneratorParams)
	if err != nil {
		log.WithError(err).WithField("requestedGenerator", requestedGenerator).Error("error interpolating Generator")
		return
	}
	assert.Equal(t, "production_01/west", interpolatedGenerator.Git.Files[0].Path)
	assert.Equal(t, "https://production-01.example.com", interpolatedGenerator.Git.Files[1].Path)
}
