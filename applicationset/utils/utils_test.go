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

func TestRenderTemplateParams(t *testing.T) {

	// Believe it or not, this is actually less complex than the equivalent solution using reflection
	fieldMap := map[string]func(app *argoappsv1.Application) *string{}
	fieldMap["Path"] = func(app *argoappsv1.Application) *string { return &app.Spec.Source.Path }
	fieldMap["RepoURL"] = func(app *argoappsv1.Application) *string { return &app.Spec.Source.RepoURL }
	fieldMap["TargetRevision"] = func(app *argoappsv1.Application) *string { return &app.Spec.Source.TargetRevision }
	fieldMap["Chart"] = func(app *argoappsv1.Application) *string { return &app.Spec.Source.Chart }

	fieldMap["Server"] = func(app *argoappsv1.Application) *string { return &app.Spec.Destination.Server }
	fieldMap["Namespace"] = func(app *argoappsv1.Application) *string { return &app.Spec.Destination.Namespace }
	fieldMap["Name"] = func(app *argoappsv1.Application) *string { return &app.Spec.Destination.Name }

	fieldMap["Project"] = func(app *argoappsv1.Application) *string { return &app.Spec.Project }

	emptyApplication := &argoappsv1.Application{
		Spec: argoappsv1.ApplicationSpec{
			Source: argoappsv1.ApplicationSource{
				Path:           "",
				RepoURL:        "",
				TargetRevision: "",
				Chart:          "",
			},
			Destination: argoappsv1.ApplicationDestination{
				Server:    "",
				Namespace: "",
				Name:      "",
			},
			Project: "",
		},
	}

	tests := []struct {
		name        string
		fieldVal    string
		params      map[string]string
		expectedVal string
	}{
		{
			name:        "simple substitution",
			fieldVal:    "{{one}}",
			expectedVal: "two",
			params: map[string]string{
				"one": "two",
			},
		},
		{
			name:        "simple substitution with whitespace",
			fieldVal:    "{{ one }}",
			expectedVal: "two",
			params: map[string]string{
				"one": "two",
			},
		},

		{
			name:        "template characters but not in a template",
			fieldVal:    "}} {{",
			expectedVal: "}} {{",
			params: map[string]string{
				"one": "two",
			},
		},

		{
			name:        "nested template",
			fieldVal:    "{{ }}",
			expectedVal: "{{ }}",
			params: map[string]string{
				"one": "{{ }}",
			},
		},
		{
			name:        "field with whitespace",
			fieldVal:    "{{ }}",
			expectedVal: "{{ }}",
			params: map[string]string{
				" ": "two",
				"":  "three",
			},
		},

		{
			name:        "template contains itself, containing itself",
			fieldVal:    "{{one}}",
			expectedVal: "{{one}}",
			params: map[string]string{
				"{{one}}": "{{one}}",
			},
		},

		{
			name:        "template contains itself, containing something else",
			fieldVal:    "{{one}}",
			expectedVal: "{{one}}",
			params: map[string]string{
				"{{one}}": "{{two}}",
			},
		},

		{
			name:        "templates are case sensitive",
			fieldVal:    "{{ONE}}",
			expectedVal: "{{ONE}}",
			params: map[string]string{
				"{{one}}": "two",
			},
		},
		{
			name:        "multiple on a line",
			fieldVal:    "{{one}}{{one}}",
			expectedVal: "twotwo",
			params: map[string]string{
				"one": "two",
			},
		},
		{
			name:        "multiple different on a line",
			fieldVal:    "{{one}}{{three}}",
			expectedVal: "twofour",
			params: map[string]string{
				"one":   "two",
				"three": "four",
			},
		},
	}

	for _, test := range tests {

		t.Run(test.name, func(t *testing.T) {

			for fieldName, getPtrFunc := range fieldMap {

				// Clone the template application
				application := emptyApplication.DeepCopy()

				// Set the value of the target field, to the test value
				*getPtrFunc(application) = test.fieldVal

				// Render the cloned application, into a new application
				render := Render{}
				newApplication, err := render.RenderTemplateParams(application, nil, test.params)

				// Retrieve the value of the target field from the newApplication, then verify that
				// the target field has been templated into the expected value
				actualValue := *getPtrFunc(newApplication)
				assert.Equal(t, test.expectedVal, actualValue, "Field '%s' had an unexpected value. expected: '%s' value: '%s'", fieldName, test.expectedVal, actualValue)
				assert.NoError(t, err)

			}
		})
	}

}

func TestRenderTemplateParamsFinalizers(t *testing.T) {

	emptyApplication := &argoappsv1.Application{
		Spec: argoappsv1.ApplicationSpec{
			Source: argoappsv1.ApplicationSource{
				Path:           "",
				RepoURL:        "",
				TargetRevision: "",
				Chart:          "",
			},
			Destination: argoappsv1.ApplicationDestination{
				Server:    "",
				Namespace: "",
				Name:      "",
			},
			Project: "",
		},
	}

	for _, c := range []struct {
		testName           string
		syncPolicy         *argoappsetv1.ApplicationSetSyncPolicy
		existingFinalizers []string
		expectedFinalizers []string
	}{
		{
			testName:           "existing finalizer should be preserved",
			existingFinalizers: []string{"existing-finalizer"},
			syncPolicy:         nil,
			expectedFinalizers: []string{"existing-finalizer"},
		},
		{
			testName:           "background finalizer should be preserved",
			existingFinalizers: []string{"resources-finalizer.argocd.argoproj.io/background"},
			syncPolicy:         nil,
			expectedFinalizers: []string{"resources-finalizer.argocd.argoproj.io/background"},
		},

		{
			testName:           "empty finalizer and empty sync should use standard finalizer",
			existingFinalizers: nil,
			syncPolicy:         nil,
			expectedFinalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},

		{
			testName:           "standard finalizer should be preserved",
			existingFinalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			syncPolicy:         nil,
			expectedFinalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		{
			testName:           "empty array finalizers should use standard finalizer",
			existingFinalizers: []string{},
			syncPolicy:         nil,
			expectedFinalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		{
			testName:           "non-nil sync policy should use standard finalizer",
			existingFinalizers: nil,
			syncPolicy:         &argoappsetv1.ApplicationSetSyncPolicy{},
			expectedFinalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		{
			testName:           "preserveResourcesOnDeletion should not have a finalizer",
			existingFinalizers: nil,
			syncPolicy: &argoappsetv1.ApplicationSetSyncPolicy{
				PreserveResourcesOnDeletion: true,
			},
			expectedFinalizers: nil,
		},
		{
			testName:           "user-specified finalizer should overwrite preserveResourcesOnDeletion",
			existingFinalizers: []string{"resources-finalizer.argocd.argoproj.io/background"},
			syncPolicy: &argoappsetv1.ApplicationSetSyncPolicy{
				PreserveResourcesOnDeletion: true,
			},
			expectedFinalizers: []string{"resources-finalizer.argocd.argoproj.io/background"},
		},
	} {

		t.Run(c.testName, func(t *testing.T) {

			// Clone the template application
			application := emptyApplication.DeepCopy()
			application.Finalizers = c.existingFinalizers

			params := map[string]string{
				"one": "two",
			}

			// Render the cloned application, into a new application
			render := Render{}

			res, err := render.RenderTemplateParams(application, c.syncPolicy, params)
			assert.Nil(t, err)

			assert.ElementsMatch(t, res.Finalizers, c.expectedFinalizers)

		})

	}

}

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
