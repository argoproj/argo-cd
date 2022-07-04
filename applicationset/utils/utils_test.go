package utils

import (
	"testing"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	argoappsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argoappsetv1 "github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
)

func TestCheckInvalidGenerators(t *testing.T) {

	scheme := runtime.NewScheme()
	err := argoappsetv1.AddToScheme(scheme)
	assert.Nil(t, err)
	err = argoappsv1.AddToScheme(scheme)
	assert.Nil(t, err)

	for _, c := range []struct {
		testName    string
		appSet      argoappsetv1.ApplicationSet
		expectedMsg string
	}{
		{
			testName: "invalid generator, without annotation",
			appSet: argoappsetv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app-set",
					Namespace: "namespace",
				},
				Spec: argoappsetv1.ApplicationSetSpec{
					Generators: []argoappsetv1.ApplicationSetGenerator{
						{
							List:     &argoappsetv1.ListGenerator{},
							Clusters: nil,
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      &argoappsetv1.GitGenerator{},
						},
					},
				},
			},
			expectedMsg: "ApplicationSet test-app-set contains unrecognized generators",
		},
		{
			testName: "invalid generator, with annotation",
			appSet: argoappsetv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app-set",
					Namespace: "namespace",
					Annotations: map[string]string{
						"kubectl.kubernetes.io/last-applied-configuration": `{
							"spec":{
								"generators":[
									{"list":{}},
									{"bbb":{}},
									{"git":{}},
									{"aaa":{}}
								]
							}
						}`,
					},
				},
				Spec: argoappsetv1.ApplicationSetSpec{
					Generators: []argoappsetv1.ApplicationSetGenerator{
						{
							List:     &argoappsetv1.ListGenerator{},
							Clusters: nil,
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      &argoappsetv1.GitGenerator{},
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
					},
				},
			},
			expectedMsg: "ApplicationSet test-app-set contains unrecognized generators: aaa, bbb",
		},
	} {
		oldhooks := logrus.StandardLogger().ReplaceHooks(logrus.LevelHooks{})
		defer logrus.StandardLogger().ReplaceHooks(oldhooks)
		hook := logtest.NewGlobal()

		CheckInvalidGenerators(&c.appSet)
		assert.True(t, len(hook.Entries) >= 1, c.testName)
		assert.NotNil(t, hook.LastEntry(), c.testName)
		if hook.LastEntry() != nil {
			assert.Equal(t, logrus.WarnLevel, hook.LastEntry().Level, c.testName)
			assert.Equal(t, c.expectedMsg, hook.LastEntry().Message, c.testName)
		}
		hook.Reset()
	}
}

func TestInvalidGenerators(t *testing.T) {

	scheme := runtime.NewScheme()
	err := argoappsetv1.AddToScheme(scheme)
	assert.Nil(t, err)
	err = argoappsv1.AddToScheme(scheme)
	assert.Nil(t, err)

	for _, c := range []struct {
		testName        string
		appSet          argoappsetv1.ApplicationSet
		expectedInvalid bool
		expectedNames   map[string]bool
	}{
		{
			testName: "valid generators, with annotation",
			appSet: argoappsetv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
					Annotations: map[string]string{
						"kubectl.kubernetes.io/last-applied-configuration": `{
							"spec":{
								"generators":[
									{"list":{}},
									{"cluster":{}},
									{"git":{}}
								]
							}
						}`,
					},
				},
				Spec: argoappsetv1.ApplicationSetSpec{
					Generators: []argoappsetv1.ApplicationSetGenerator{
						{
							List:     &argoappsetv1.ListGenerator{},
							Clusters: nil,
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: &argoappsetv1.ClusterGenerator{},
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      &argoappsetv1.GitGenerator{},
						},
					},
				},
			},
			expectedInvalid: false,
			expectedNames:   map[string]bool{},
		},
		{
			testName: "invalid generators, no annotation",
			appSet: argoappsetv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: argoappsetv1.ApplicationSetSpec{
					Generators: []argoappsetv1.ApplicationSetGenerator{
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
					},
				},
			},
			expectedInvalid: true,
			expectedNames:   map[string]bool{},
		},
		{
			testName: "valid and invalid generators, no annotation",
			appSet: argoappsetv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: argoappsetv1.ApplicationSetSpec{
					Generators: []argoappsetv1.ApplicationSetGenerator{
						{
							List:     nil,
							Clusters: &argoappsetv1.ClusterGenerator{},
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      &argoappsetv1.GitGenerator{},
						},
					},
				},
			},
			expectedInvalid: true,
			expectedNames:   map[string]bool{},
		},
		{
			testName: "valid and invalid generators, with annotation",
			appSet: argoappsetv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
					Annotations: map[string]string{
						"kubectl.kubernetes.io/last-applied-configuration": `{
							"spec":{
								"generators":[
									{"cluster":{}},
									{"bbb":{}},
									{"git":{}},
									{"aaa":{}}
								]
							}
						}`,
					},
				},
				Spec: argoappsetv1.ApplicationSetSpec{
					Generators: []argoappsetv1.ApplicationSetGenerator{
						{
							List:     nil,
							Clusters: &argoappsetv1.ClusterGenerator{},
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      &argoappsetv1.GitGenerator{},
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
					},
				},
			},
			expectedInvalid: true,
			expectedNames: map[string]bool{
				"aaa": true,
				"bbb": true,
			},
		},
		{
			testName: "invalid generator, annotation with missing spec",
			appSet: argoappsetv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
					Annotations: map[string]string{
						"kubectl.kubernetes.io/last-applied-configuration": `{
						}`,
					},
				},
				Spec: argoappsetv1.ApplicationSetSpec{
					Generators: []argoappsetv1.ApplicationSetGenerator{
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
					},
				},
			},
			expectedInvalid: true,
			expectedNames:   map[string]bool{},
		},
		{
			testName: "invalid generator, annotation with missing generators array",
			appSet: argoappsetv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
					Annotations: map[string]string{
						"kubectl.kubernetes.io/last-applied-configuration": `{
							"spec":{
							}
						}`,
					},
				},
				Spec: argoappsetv1.ApplicationSetSpec{
					Generators: []argoappsetv1.ApplicationSetGenerator{
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
					},
				},
			},
			expectedInvalid: true,
			expectedNames:   map[string]bool{},
		},
		{
			testName: "invalid generator, annotation with empty generators array",
			appSet: argoappsetv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
					Annotations: map[string]string{
						"kubectl.kubernetes.io/last-applied-configuration": `{
							"spec":{
								"generators":[
								]
							}
						}`,
					},
				},
				Spec: argoappsetv1.ApplicationSetSpec{
					Generators: []argoappsetv1.ApplicationSetGenerator{
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
					},
				},
			},
			expectedInvalid: true,
			expectedNames:   map[string]bool{},
		},
		{
			testName: "invalid generator, annotation with empty generator",
			appSet: argoappsetv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
					Annotations: map[string]string{
						"kubectl.kubernetes.io/last-applied-configuration": `{
							"spec":{
								"generators":[
									{}
								]
							}
						}`,
					},
				},
				Spec: argoappsetv1.ApplicationSetSpec{
					Generators: []argoappsetv1.ApplicationSetGenerator{
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
					},
				},
			},
			expectedInvalid: true,
			expectedNames:   map[string]bool{},
		},
	} {
		hasInvalid, names := invalidGenerators(&c.appSet)
		assert.Equal(t, c.expectedInvalid, hasInvalid, c.testName)
		assert.Equal(t, c.expectedNames, names, c.testName)
	}
}

func TestNormalizeBitbucketBasePath(t *testing.T) {
	for _, c := range []struct {
		testName         string
		basePath         string
		expectedBasePath string
	}{
		{
			testName:         "default api url",
			basePath:         "https://company.bitbucket.com",
			expectedBasePath: "https://company.bitbucket.com/rest",
		},
		{
			testName:         "with /rest suffix",
			basePath:         "https://company.bitbucket.com/rest",
			expectedBasePath: "https://company.bitbucket.com/rest",
		},
		{
			testName:         "with /rest/ suffix",
			basePath:         "https://company.bitbucket.com/rest/",
			expectedBasePath: "https://company.bitbucket.com/rest",
		},
	} {
		result := NormalizeBitbucketBasePath(c.basePath)
		assert.Equal(t, c.expectedBasePath, result, c.testName)
	}
}
