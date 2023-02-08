package utils

import (
	"testing"

	testutils "github.com/argoproj/argo-cd/v2/test"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	argoappsetv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argoappsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func emptyApplication() map[string]interface{} {
	// Clone the template application
	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{"annotation-key": "annotation-value", "annotation-key2": "annotation-value2"},
			"labels":      map[string]string{"label-key": "label-value", "label-key2": "label-value2"},
			"name":        "application-one",
			"namespace":   "default",
		},
		"spec": map[string]interface{}{
			"source": map[string]interface{}{
				"path":           "",
				"repoURL":        "",
				"targetRevision": "",
				"chart":          "",
			},
			"destination": map[string]interface{}{
				"server":    "",
				"namespace": "",
				"name":      "",
			},
		},
	}
}

func TestRenderTemplateParams(t *testing.T) {

	pointers := []string{
		"/spec/source/path",
		"/spec/source/repoURL",
		"/spec/source/targetRevision",
		"/spec/source/chart",
		"/spec/destination/server",
		"/spec/destination/namespace",
		"/spec/destination/name",
		"/spec/project",
	}

	tests := []struct {
		name        string
		fieldVal    string
		params      map[string]interface{}
		expectedVal string
	}{
		{
			name:        "simple substitution",
			fieldVal:    "{{one}}",
			expectedVal: "two",
			params: map[string]interface{}{
				"one": "two",
			},
		},
		{
			name:        "simple substitution with whitespace",
			fieldVal:    "{{ one }}",
			expectedVal: "two",
			params: map[string]interface{}{
				"one": "two",
			},
		},

		{
			name:        "template characters but not in a template",
			fieldVal:    "}} {{",
			expectedVal: "}} {{",
			params: map[string]interface{}{
				"one": "two",
			},
		},

		{
			name:        "nested template",
			fieldVal:    "{{ }}",
			expectedVal: "{{ }}",
			params: map[string]interface{}{
				"one": "{{ }}",
			},
		},
		{
			name:        "field with whitespace",
			fieldVal:    "{{ }}",
			expectedVal: "{{ }}",
			params: map[string]interface{}{
				" ": "two",
				"":  "three",
			},
		},

		{
			name:        "template contains itself, containing itself",
			fieldVal:    "{{one}}",
			expectedVal: "{{one}}",
			params: map[string]interface{}{
				"{{one}}": "{{one}}",
			},
		},

		{
			name:        "template contains itself, containing something else",
			fieldVal:    "{{one}}",
			expectedVal: "{{one}}",
			params: map[string]interface{}{
				"{{one}}": "{{two}}",
			},
		},

		{
			name:        "templates are case sensitive",
			fieldVal:    "{{ONE}}",
			expectedVal: "{{ONE}}",
			params: map[string]interface{}{
				"{{one}}": "two",
			},
		},
		{
			name:        "multiple on a line",
			fieldVal:    "{{one}}{{one}}",
			expectedVal: "twotwo",
			params: map[string]interface{}{
				"one": "two",
			},
		},
		{
			name:        "multiple different on a line",
			fieldVal:    "{{one}}{{three}}",
			expectedVal: "twofour",
			params: map[string]interface{}{
				"one":   "two",
				"three": "four",
			},
		},
		{
			name:        "multiple different on a line with quote",
			fieldVal:    "{{one}} {{three}}",
			expectedVal: "\"hello\" world four",
			params: map[string]interface{}{
				"one":   "\"hello\" world",
				"three": "four",
			},
		},
	}

	for _, test := range tests {

		t.Run(test.name, func(t *testing.T) {

			for _, pointer := range pointers {

				application := testutils.UpdateData(emptyApplication(), pointer, test.fieldVal)

				// Render the cloned application, into a new application
				render := Render{}
				newApplication, err := render.RenderTemplateParams(application.(map[string]interface{}), nil, test.params, false)

				// Retrieve the value of the target field from the newApplication, then verify that
				// the target field has been templated into the expected value
				actualValue := testutils.GetData(newApplication, pointer)
				assert.Equal(t, test.expectedVal, actualValue, "Field '%s' had an unexpected value. expected: '%s' value: '%s'", pointer, test.expectedVal, actualValue)
				assert.Equal(t, newApplication.ObjectMeta.Annotations["annotation-key"], "annotation-value")
				assert.Equal(t, newApplication.ObjectMeta.Annotations["annotation-key2"], "annotation-value2")
				assert.Equal(t, newApplication.ObjectMeta.Labels["label-key"], "label-value")
				assert.Equal(t, newApplication.ObjectMeta.Labels["label-key2"], "label-value2")
				assert.Equal(t, newApplication.ObjectMeta.Name, "application-one")
				assert.Equal(t, newApplication.ObjectMeta.Namespace, "default")
				assert.NoError(t, err)
			}
		})
	}

}

func TestRenderTemplateParamsGoTemplate(t *testing.T) {

	// Believe it or not, this is actually less complex than the equivalent solution using reflection
	pointers := []string{
		"/spec/source/path",
		"/spec/source/repoURL",
		"/spec/source/targetRevision",
		"/spec/source/chart",
		"/spec/destination/server",
		"/spec/destination/namespace",
		"/spec/destination/name",
		"/spec/project",
	}

	tests := []struct {
		name         string
		fieldVal     string
		params       map[string]interface{}
		expectedVal  string
		errorMessage string
	}{
		{
			name:        "simple substitution",
			fieldVal:    "{{ .one }}",
			expectedVal: "two",
			params: map[string]interface{}{
				"one": "two",
			},
		},
		{
			name:        "simple substitution with whitespace",
			fieldVal:    "{{ .one }}",
			expectedVal: "two",
			params: map[string]interface{}{
				"one": "two",
			},
		},
		{
			name:        "template contains itself, containing itself",
			fieldVal:    "{{ .one }}",
			expectedVal: "{{one}}",
			params: map[string]interface{}{
				"one": "{{one}}",
			},
		},

		{
			name:        "template contains itself, containing something else",
			fieldVal:    "{{ .one }}",
			expectedVal: "{{two}}",
			params: map[string]interface{}{
				"one": "{{two}}",
			},
		},
		{
			name:        "multiple on a line",
			fieldVal:    "{{.one}}{{.one}}",
			expectedVal: "twotwo",
			params: map[string]interface{}{
				"one": "two",
			},
		},
		{
			name:        "multiple different on a line",
			fieldVal:    "{{.one}}{{.three}}",
			expectedVal: "twofour",
			params: map[string]interface{}{
				"one":   "two",
				"three": "four",
			},
		},
		{
			name:        "multiple different on a line with quote",
			fieldVal:    "{{.one}} {{.three}}",
			expectedVal: "\"hello\" world four",
			params: map[string]interface{}{
				"one":   "\"hello\" world",
				"three": "four",
			},
		},
		{
			name:        "depth",
			fieldVal:    "{{ .image.version }}",
			expectedVal: "latest",
			params: map[string]interface{}{
				"replicas": 3,
				"image": map[string]interface{}{
					"name":    "busybox",
					"version": "latest",
				},
			},
		},
		{
			name:        "multiple depth",
			fieldVal:    "{{ .image.name }}:{{ .image.version }}",
			expectedVal: "busybox:latest",
			params: map[string]interface{}{
				"replicas": 3,
				"image": map[string]interface{}{
					"name":    "busybox",
					"version": "latest",
				},
			},
		},
		{
			name:        "if ok",
			fieldVal:    "{{ if .hpa.enabled }}{{ .hpa.maxReplicas }}{{ else }}{{ .replicas }}{{ end }}",
			expectedVal: "5",
			params: map[string]interface{}{
				"replicas": 3,
				"hpa": map[string]interface{}{
					"enabled":     true,
					"minReplicas": 1,
					"maxReplicas": 5,
				},
			},
		},
		{
			name:        "if not ok",
			fieldVal:    "{{ if .hpa.enabled }}{{ .hpa.maxReplicas }}{{ else }}{{ .replicas }}{{ end }}",
			expectedVal: "3",
			params: map[string]interface{}{
				"replicas": 3,
				"hpa": map[string]interface{}{
					"enabled":     false,
					"minReplicas": 1,
					"maxReplicas": 5,
				},
			},
		},
		{
			name:        "loop",
			fieldVal:    "{{ range .volumes }}[{{ .name }}]{{ end }}",
			expectedVal: "[volume-one][volume-two]",
			params: map[string]interface{}{
				"replicas": 3,
				"volumes": []map[string]interface{}{
					{
						"name":     "volume-one",
						"emptyDir": map[string]interface{}{},
					},
					{
						"name":     "volume-two",
						"emptyDir": map[string]interface{}{},
					},
				},
			},
		},
		{
			name:        "Index",
			fieldVal:    `{{ index .admin "admin-ca" }}, {{ index .admin "admin-jks" }}`,
			expectedVal: "value admin ca, value admin jks",
			params: map[string]interface{}{
				"admin": map[string]interface{}{
					"admin-ca":  "value admin ca",
					"admin-jks": "value admin jks",
				},
			},
		},
		{
			name:        "Index",
			fieldVal:    `{{ index .admin "admin-ca" }}, \\ "Hello world", {{ index .admin "admin-jks" }}`,
			expectedVal: `value "admin" ca with \, \\ "Hello world", value admin jks`,
			params: map[string]interface{}{
				"admin": map[string]interface{}{
					"admin-ca":  `value "admin" ca with \`,
					"admin-jks": "value admin jks",
				},
			},
		},
		{
			name:        "quote",
			fieldVal:    `{{.quote}}`,
			expectedVal: `"`,
			params: map[string]interface{}{
				"quote": `"`,
			},
		},
		{
			name:        "Test No Data",
			fieldVal:    `{{.data}}`,
			expectedVal: "{{.data}}",
			params:      map[string]interface{}{},
		},
		{
			name:        "Test Parse Error",
			fieldVal:    `{{functiondoesnotexist}}`,
			expectedVal: "",
			params: map[string]interface{}{
				"data": `a data string`,
			},
			errorMessage: `failed to parse template {{functiondoesnotexist}}: template: :1: function "functiondoesnotexist" not defined`,
		},
		{
			name:        "Test template error",
			fieldVal:    `{{.data.test}}`,
			expectedVal: "",
			params: map[string]interface{}{
				"data": `a data string`,
			},
			errorMessage: `failed to execute go template {{.data.test}}: template: :1:7: executing "" at <.data.test>: can't evaluate field test in type interface {}`,
		},
	}

	for _, test := range tests {

		t.Run(test.name, func(t *testing.T) {

			for _, pointer := range pointers {

				application := testutils.UpdateData(emptyApplication(), pointer, test.fieldVal)

				// Render the cloned application, into a new application
				render := Render{}
				newApplication, err := render.RenderTemplateParams(application.(map[string]interface{}), nil, test.params, true)

				// Retrieve the value of the target field from the newApplication, then verify that
				// the target field has been templated into the expected value
				if test.errorMessage != "" {
					assert.Error(t, err)
					assert.Equal(t, test.errorMessage, err.Error())
				} else {
					assert.NoError(t, err)
					// Retrieve the value of the target field from the newApplication, then verify that
					actualValue := testutils.GetData(newApplication, pointer)
					assert.Equal(t, test.expectedVal, actualValue, "Field '%s' had an unexpected value. expected: '%s' value: '%s'", pointer, test.expectedVal, actualValue)
					assert.Equal(t, newApplication.ObjectMeta.Annotations["annotation-key"], "annotation-value")
					assert.Equal(t, newApplication.ObjectMeta.Annotations["annotation-key2"], "annotation-value2")
					assert.Equal(t, newApplication.ObjectMeta.Labels["label-key"], "label-value")
					assert.Equal(t, newApplication.ObjectMeta.Labels["label-key2"], "label-value2")
					assert.Equal(t, newApplication.ObjectMeta.Name, "application-one")
					assert.Equal(t, newApplication.ObjectMeta.Namespace, "default")
				}
			}
		})
	}
}

func TestRenderTemplateKeys(t *testing.T) {
	t.Run("fasttemplate", func(t *testing.T) {
		application := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]string{
					"annotation-{{key}}": "annotation-{{value}}",
				},
			},
		}

		params := map[string]interface{}{
			"key":   "some-key",
			"value": "some-value",
		}

		render := Render{}
		newApplication, err := render.RenderTemplateParams(application, nil, params, false)
		require.NoError(t, err)
		require.Contains(t, newApplication.ObjectMeta.Annotations, "annotation-some-key")
		assert.Equal(t, newApplication.ObjectMeta.Annotations["annotation-some-key"], "annotation-some-value")
	})
	t.Run("gotemplate", func(t *testing.T) {
		application := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]string{
					"annotation-{{ .key }}": "annotation-{{ .value }}",
				},
			},
		}

		params := map[string]interface{}{
			"key":   "some-key",
			"value": "some-value",
		}

		render := Render{}
		newApplication, err := render.RenderTemplateParams(application, nil, params, true)
		require.NoError(t, err)
		require.Contains(t, newApplication.ObjectMeta.Annotations, "annotation-some-key")
		assert.Equal(t, newApplication.ObjectMeta.Annotations["annotation-some-key"], "annotation-some-value")
	})
}

func TestRenderTemplateParamsFinalizers(t *testing.T) {

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
			application := testutils.UpdateData(emptyApplication(), "/metadata/finalizers", c.existingFinalizers)

			params := map[string]interface{}{
				"one": "two",
			}

			// Render the cloned application, into a new application
			render := Render{}

			res, err := render.RenderTemplateParams(application.(map[string]interface{}), c.syncPolicy, params, true)
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

		_ = CheckInvalidGenerators(&c.appSet)
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
