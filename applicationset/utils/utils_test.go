package utils

import (
	"crypto/x509"
	"encoding/json"
	"os"
	"path"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	argoappsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
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
		ObjectMeta: metav1.ObjectMeta{
			Annotations:       map[string]string{"annotation-key": "annotation-value", "annotation-key2": "annotation-value2"},
			Labels:            map[string]string{"label-key": "label-value", "label-key2": "label-value2"},
			CreationTimestamp: metav1.NewTime(time.Now()),
			UID:               types.UID("d546da12-06b7-4f9a-8ea2-3adb16a20e2b"),
			Name:              "application-one",
			Namespace:         "default",
		},
		Spec: argoappsv1.ApplicationSpec{
			Source: &argoappsv1.ApplicationSource{
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
			for fieldName, getPtrFunc := range fieldMap {
				// Clone the template application
				application := emptyApplication.DeepCopy()

				// Set the value of the target field, to the test value
				*getPtrFunc(application) = test.fieldVal

				// Render the cloned application, into a new application
				render := Render{}
				newApplication, err := render.RenderTemplateParams(application, nil, test.params, false, nil)

				// Retrieve the value of the target field from the newApplication, then verify that
				// the target field has been templated into the expected value
				actualValue := *getPtrFunc(newApplication)
				assert.Equal(t, test.expectedVal, actualValue, "Field '%s' had an unexpected value. expected: '%s' value: '%s'", fieldName, test.expectedVal, actualValue)
				assert.Equal(t, "annotation-value", newApplication.ObjectMeta.Annotations["annotation-key"])
				assert.Equal(t, "annotation-value2", newApplication.ObjectMeta.Annotations["annotation-key2"])
				assert.Equal(t, "label-value", newApplication.ObjectMeta.Labels["label-key"])
				assert.Equal(t, "label-value2", newApplication.ObjectMeta.Labels["label-key2"])
				assert.Equal(t, "application-one", newApplication.ObjectMeta.Name)
				assert.Equal(t, "default", newApplication.ObjectMeta.Namespace)
				assert.Equal(t, newApplication.ObjectMeta.UID, types.UID("d546da12-06b7-4f9a-8ea2-3adb16a20e2b"))
				assert.Equal(t, newApplication.ObjectMeta.CreationTimestamp, application.ObjectMeta.CreationTimestamp)
				require.NoError(t, err)
			}
		})
	}
}

func TestRenderHelmValuesObjectJson(t *testing.T) {
	params := map[string]interface{}{
		"test": "Hello world",
	}

	application := &argoappsv1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:       map[string]string{"annotation-key": "annotation-value", "annotation-key2": "annotation-value2"},
			Labels:            map[string]string{"label-key": "label-value", "label-key2": "label-value2"},
			CreationTimestamp: metav1.NewTime(time.Now()),
			UID:               types.UID("d546da12-06b7-4f9a-8ea2-3adb16a20e2b"),
			Name:              "application-one",
			Namespace:         "default",
		},
		Spec: argoappsv1.ApplicationSpec{
			Source: &argoappsv1.ApplicationSource{
				Path:           "",
				RepoURL:        "",
				TargetRevision: "",
				Chart:          "",
				Helm: &argoappsv1.ApplicationSourceHelm{
					ValuesObject: &runtime.RawExtension{
						Raw: []byte(`{
								"some": {
									"string": "{{.test}}"
								}
							  }`),
					},
				},
			},
			Destination: argoappsv1.ApplicationDestination{
				Server:    "",
				Namespace: "",
				Name:      "",
			},
			Project: "",
		},
	}

	// Render the cloned application, into a new application
	render := Render{}
	newApplication, err := render.RenderTemplateParams(application, nil, params, true, []string{})

	require.NoError(t, err)
	assert.NotNil(t, newApplication)

	var unmarshaled interface{}
	err = json.Unmarshal(newApplication.Spec.Source.Helm.ValuesObject.Raw, &unmarshaled)

	require.NoError(t, err)
	assert.Equal(t, "Hello world", unmarshaled.(map[string]interface{})["some"].(map[string]interface{})["string"])
}

func TestRenderHelmValuesObjectYaml(t *testing.T) {
	params := map[string]interface{}{
		"test": "Hello world",
	}

	application := &argoappsv1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:       map[string]string{"annotation-key": "annotation-value", "annotation-key2": "annotation-value2"},
			Labels:            map[string]string{"label-key": "label-value", "label-key2": "label-value2"},
			CreationTimestamp: metav1.NewTime(time.Now()),
			UID:               types.UID("d546da12-06b7-4f9a-8ea2-3adb16a20e2b"),
			Name:              "application-one",
			Namespace:         "default",
		},
		Spec: argoappsv1.ApplicationSpec{
			Source: &argoappsv1.ApplicationSource{
				Path:           "",
				RepoURL:        "",
				TargetRevision: "",
				Chart:          "",
				Helm: &argoappsv1.ApplicationSourceHelm{
					ValuesObject: &runtime.RawExtension{
						Raw: []byte(`some:
  string: "{{.test}}"`),
					},
				},
			},
			Destination: argoappsv1.ApplicationDestination{
				Server:    "",
				Namespace: "",
				Name:      "",
			},
			Project: "",
		},
	}

	// Render the cloned application, into a new application
	render := Render{}
	newApplication, err := render.RenderTemplateParams(application, nil, params, true, []string{})

	require.NoError(t, err)
	assert.NotNil(t, newApplication)

	var unmarshaled interface{}
	err = json.Unmarshal(newApplication.Spec.Source.Helm.ValuesObject.Raw, &unmarshaled)

	require.NoError(t, err)
	assert.Equal(t, "Hello world", unmarshaled.(map[string]interface{})["some"].(map[string]interface{})["string"])
}

func TestRenderTemplateParamsGoTemplate(t *testing.T) {
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
		ObjectMeta: metav1.ObjectMeta{
			Annotations:       map[string]string{"annotation-key": "annotation-value", "annotation-key2": "annotation-value2"},
			Labels:            map[string]string{"label-key": "label-value", "label-key2": "label-value2"},
			CreationTimestamp: metav1.NewTime(time.Now()),
			UID:               types.UID("d546da12-06b7-4f9a-8ea2-3adb16a20e2b"),
			Name:              "application-one",
			Namespace:         "default",
		},
		Spec: argoappsv1.ApplicationSpec{
			Source: &argoappsv1.ApplicationSource{
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
		name            string
		fieldVal        string
		params          map[string]interface{}
		expectedVal     string
		errorMessage    string
		templateOptions []string
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
		{
			name:        "lookup missing value with missingkey=default",
			fieldVal:    `--> {{.doesnotexist}} <--`,
			expectedVal: `--> <no value> <--`,
			params: map[string]interface{}{
				// if no params are passed then for some reason templating is skipped
				"unused": "this is not used",
			},
		},
		{
			name:        "lookup missing value with missingkey=error",
			fieldVal:    `--> {{.doesnotexist}} <--`,
			expectedVal: "",
			params: map[string]interface{}{
				// if no params are passed then for some reason templating is skipped
				"unused": "this is not used",
			},
			templateOptions: []string{"missingkey=error"},
			errorMessage:    `failed to execute go template --> {{.doesnotexist}} <--: template: :1:6: executing "" at <.doesnotexist>: map has no entry for key "doesnotexist"`,
		},
		{
			name:        "toYaml",
			fieldVal:    `{{ toYaml . | indent 2 }}`,
			expectedVal: "  foo:\n    bar:\n      bool: true\n      number: 2\n      str: Hello world",
			params: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
						"bool":   true,
						"number": 2,
						"str":    "Hello world",
					},
				},
			},
		},
		{
			name:         "toYaml Error",
			fieldVal:     `{{ toYaml . | indent 2 }}`,
			expectedVal:  "  foo:\n    bar:\n      bool: true\n      number: 2\n      str: Hello world",
			errorMessage: "failed to execute go template {{ toYaml . | indent 2 }}: template: :1:3: executing \"\" at <toYaml .>: error calling toYaml: error marshaling into JSON: json: unsupported type: func(*string)",
			params: map[string]interface{}{
				"foo": func(test *string) {
				},
			},
		},
		{
			name:        "fromYaml",
			fieldVal:    `{{ get (fromYaml .value) "hello" }}`,
			expectedVal: "world",
			params: map[string]interface{}{
				"value": "hello: world",
			},
		},
		{
			name:         "fromYaml error",
			fieldVal:     `{{ get (fromYaml .value) "hello" }}`,
			expectedVal:  "world",
			errorMessage: "failed to execute go template {{ get (fromYaml .value) \"hello\" }}: template: :1:8: executing \"\" at <fromYaml .value>: error calling fromYaml: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type map[string]interface {}",
			params: map[string]interface{}{
				"value": "non\n compliant\n yaml",
			},
		},
		{
			name:        "fromYamlArray",
			fieldVal:    `{{ fromYamlArray .value | last }}`,
			expectedVal: "bonjour tout le monde",
			params: map[string]interface{}{
				"value": "- hello world\n- bonjour tout le monde",
			},
		},
		{
			name:         "fromYamlArray error",
			fieldVal:     `{{ fromYamlArray .value | last }}`,
			expectedVal:  "bonjour tout le monde",
			errorMessage: "failed to execute go template {{ fromYamlArray .value | last }}: template: :1:3: executing \"\" at <fromYamlArray .value>: error calling fromYamlArray: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type []interface {}",
			params: map[string]interface{}{
				"value": "non\n compliant\n yaml",
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
				newApplication, err := render.RenderTemplateParams(application, nil, test.params, true, test.templateOptions)

				// Retrieve the value of the target field from the newApplication, then verify that
				// the target field has been templated into the expected value
				if test.errorMessage != "" {
					require.Error(t, err)
					assert.Equal(t, test.errorMessage, err.Error())
				} else {
					require.NoError(t, err)
					actualValue := *getPtrFunc(newApplication)
					assert.Equal(t, test.expectedVal, actualValue, "Field '%s' had an unexpected value. expected: '%s' value: '%s'", fieldName, test.expectedVal, actualValue)
					assert.Equal(t, "annotation-value", newApplication.ObjectMeta.Annotations["annotation-key"])
					assert.Equal(t, "annotation-value2", newApplication.ObjectMeta.Annotations["annotation-key2"])
					assert.Equal(t, "label-value", newApplication.ObjectMeta.Labels["label-key"])
					assert.Equal(t, "label-value2", newApplication.ObjectMeta.Labels["label-key2"])
					assert.Equal(t, "application-one", newApplication.ObjectMeta.Name)
					assert.Equal(t, "default", newApplication.ObjectMeta.Namespace)
					assert.Equal(t, newApplication.ObjectMeta.UID, types.UID("d546da12-06b7-4f9a-8ea2-3adb16a20e2b"))
					assert.Equal(t, newApplication.ObjectMeta.CreationTimestamp, application.ObjectMeta.CreationTimestamp)
				}
			}
		})
	}
}

func TestRenderGeneratorParams_does_not_panic(t *testing.T) {
	// This test verifies that the RenderGeneratorParams function does not panic when the value in a map is a non-
	// nillable type. This is a regression test.
	render := Render{}
	params := map[string]interface{}{
		"branch": "master",
	}
	generator := &argoappsv1.ApplicationSetGenerator{
		Plugin: &argoappsv1.PluginGenerator{
			ConfigMapRef: argoappsv1.PluginConfigMapRef{
				Name: "cm-plugin",
			},
			Input: argoappsv1.PluginInput{
				Parameters: map[string]apiextensionsv1.JSON{
					"branch": {
						Raw: []byte(`"{{.branch}}"`),
					},
					"repo": {
						Raw: []byte(`"argo-test"`),
					},
				},
			},
		},
	}
	_, err := render.RenderGeneratorParams(generator, params, true, []string{})
	require.NoError(t, err)
}

func TestRenderTemplateKeys(t *testing.T) {
	t.Run("fasttemplate", func(t *testing.T) {
		application := &argoappsv1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"annotation-{{key}}": "annotation-{{value}}",
				},
			},
		}

		params := map[string]interface{}{
			"key":   "some-key",
			"value": "some-value",
		}

		render := Render{}
		newApplication, err := render.RenderTemplateParams(application, nil, params, false, nil)
		require.NoError(t, err)
		require.Contains(t, newApplication.ObjectMeta.Annotations, "annotation-some-key")
		assert.Equal(t, "annotation-some-value", newApplication.ObjectMeta.Annotations["annotation-some-key"])
	})
	t.Run("gotemplate", func(t *testing.T) {
		application := &argoappsv1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"annotation-{{ .key }}": "annotation-{{ .value }}",
				},
			},
		}

		params := map[string]interface{}{
			"key":   "some-key",
			"value": "some-value",
		}

		render := Render{}
		newApplication, err := render.RenderTemplateParams(application, nil, params, true, nil)
		require.NoError(t, err)
		require.Contains(t, newApplication.ObjectMeta.Annotations, "annotation-some-key")
		assert.Equal(t, "annotation-some-value", newApplication.ObjectMeta.Annotations["annotation-some-key"])
	})
}

func Test_Render_Replace_no_panic_on_missing_closing_brace(t *testing.T) {
	r := &Render{}
	assert.NotPanics(t, func() {
		_, err := r.Replace("{{properly.closed}} {{improperly.closed}", nil, false, []string{})
		require.Error(t, err)
	})
}

func TestRenderTemplateParamsFinalizers(t *testing.T) {
	emptyApplication := &argoappsv1.Application{
		Spec: argoappsv1.ApplicationSpec{
			Source: &argoappsv1.ApplicationSource{
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
		syncPolicy         *argoappsv1.ApplicationSetSyncPolicy
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
			syncPolicy:         &argoappsv1.ApplicationSetSyncPolicy{},
			expectedFinalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		{
			testName:           "preserveResourcesOnDeletion should not have a finalizer",
			existingFinalizers: nil,
			syncPolicy: &argoappsv1.ApplicationSetSyncPolicy{
				PreserveResourcesOnDeletion: true,
			},
			expectedFinalizers: nil,
		},
		{
			testName:           "user-specified finalizer should overwrite preserveResourcesOnDeletion",
			existingFinalizers: []string{"resources-finalizer.argocd.argoproj.io/background"},
			syncPolicy: &argoappsv1.ApplicationSetSyncPolicy{
				PreserveResourcesOnDeletion: true,
			},
			expectedFinalizers: []string{"resources-finalizer.argocd.argoproj.io/background"},
		},
	} {
		t.Run(c.testName, func(t *testing.T) {
			// Clone the template application
			application := emptyApplication.DeepCopy()
			application.Finalizers = c.existingFinalizers

			params := map[string]interface{}{
				"one": "two",
			}

			// Render the cloned application, into a new application
			render := Render{}

			res, err := render.RenderTemplateParams(application, c.syncPolicy, params, true, nil)
			require.NoError(t, err)

			assert.ElementsMatch(t, res.Finalizers, c.expectedFinalizers)
		})
	}
}

func TestCheckInvalidGenerators(t *testing.T) {
	scheme := runtime.NewScheme()
	err := argoappsv1.AddToScheme(scheme)
	require.NoError(t, err)
	err = argoappsv1.AddToScheme(scheme)
	require.NoError(t, err)

	for _, c := range []struct {
		testName    string
		appSet      argoappsv1.ApplicationSet
		expectedMsg string
	}{
		{
			testName: "invalid generator, without annotation",
			appSet: argoappsv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app-set",
					Namespace: "namespace",
				},
				Spec: argoappsv1.ApplicationSetSpec{
					Generators: []argoappsv1.ApplicationSetGenerator{
						{
							List:     &argoappsv1.ListGenerator{},
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
							Git:      &argoappsv1.GitGenerator{},
						},
					},
				},
			},
			expectedMsg: "ApplicationSet test-app-set contains unrecognized generators",
		},
		{
			testName: "invalid generator, with annotation",
			appSet: argoappsv1.ApplicationSet{
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
				Spec: argoappsv1.ApplicationSetSpec{
					Generators: []argoappsv1.ApplicationSetGenerator{
						{
							List:     &argoappsv1.ListGenerator{},
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
							Git:      &argoappsv1.GitGenerator{},
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
		assert.GreaterOrEqual(t, len(hook.Entries), 1, c.testName)
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
	err := argoappsv1.AddToScheme(scheme)
	require.NoError(t, err)
	err = argoappsv1.AddToScheme(scheme)
	require.NoError(t, err)

	for _, c := range []struct {
		testName        string
		appSet          argoappsv1.ApplicationSet
		expectedInvalid bool
		expectedNames   map[string]bool
	}{
		{
			testName: "valid generators, with annotation",
			appSet: argoappsv1.ApplicationSet{
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
				Spec: argoappsv1.ApplicationSetSpec{
					Generators: []argoappsv1.ApplicationSetGenerator{
						{
							List:     &argoappsv1.ListGenerator{},
							Clusters: nil,
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: &argoappsv1.ClusterGenerator{},
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      &argoappsv1.GitGenerator{},
						},
					},
				},
			},
			expectedInvalid: false,
			expectedNames:   map[string]bool{},
		},
		{
			testName: "invalid generators, no annotation",
			appSet: argoappsv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: argoappsv1.ApplicationSetSpec{
					Generators: []argoappsv1.ApplicationSetGenerator{
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
			appSet: argoappsv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: argoappsv1.ApplicationSetSpec{
					Generators: []argoappsv1.ApplicationSetGenerator{
						{
							List:     nil,
							Clusters: &argoappsv1.ClusterGenerator{},
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
							Git:      &argoappsv1.GitGenerator{},
						},
					},
				},
			},
			expectedInvalid: true,
			expectedNames:   map[string]bool{},
		},
		{
			testName: "valid and invalid generators, with annotation",
			appSet: argoappsv1.ApplicationSet{
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
				Spec: argoappsv1.ApplicationSetSpec{
					Generators: []argoappsv1.ApplicationSetGenerator{
						{
							List:     nil,
							Clusters: &argoappsv1.ClusterGenerator{},
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
							Git:      &argoappsv1.GitGenerator{},
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
			appSet: argoappsv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
					Annotations: map[string]string{
						"kubectl.kubernetes.io/last-applied-configuration": `{
						}`,
					},
				},
				Spec: argoappsv1.ApplicationSetSpec{
					Generators: []argoappsv1.ApplicationSetGenerator{
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
			appSet: argoappsv1.ApplicationSet{
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
				Spec: argoappsv1.ApplicationSetSpec{
					Generators: []argoappsv1.ApplicationSetGenerator{
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
			appSet: argoappsv1.ApplicationSet{
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
				Spec: argoappsv1.ApplicationSetSpec{
					Generators: []argoappsv1.ApplicationSetGenerator{
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
			appSet: argoappsv1.ApplicationSet{
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
				Spec: argoappsv1.ApplicationSetSpec{
					Generators: []argoappsv1.ApplicationSetGenerator{
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

func TestSlugify(t *testing.T) {
	for _, c := range []struct {
		branch           string
		smartTruncate    bool
		length           int
		expectedBasePath string
	}{
		{
			branch:           "feat/a_really+long_pull_request_name_to_test_argo_slugification_and_branch_name_shortening_feature",
			smartTruncate:    false,
			length:           50,
			expectedBasePath: "feat-a-really-long-pull-request-name-to-test-argo",
		},
		{
			branch:           "feat/a_really+long_pull_request_name_to_test_argo_slugification_and_branch_name_shortening_feature",
			smartTruncate:    true,
			length:           53,
			expectedBasePath: "feat-a-really-long-pull-request-name-to-test-argo",
		},
		{
			branch:           "feat/areallylongpullrequestnametotestargoslugificationandbranchnameshorteningfeature",
			smartTruncate:    true,
			length:           50,
			expectedBasePath: "feat",
		},
		{
			branch:           "feat/areallylongpullrequestnametotestargoslugificationandbranchnameshorteningfeature",
			smartTruncate:    false,
			length:           50,
			expectedBasePath: "feat-areallylongpullrequestnametotestargoslugifica",
		},
	} {
		result := SlugifyName(c.length, c.smartTruncate, c.branch)
		assert.Equal(t, c.expectedBasePath, result, c.branch)
	}
}

func TestGetTLSConfig(t *testing.T) {
	temppath := t.TempDir()
	certFromFile := `
-----BEGIN CERTIFICATE-----
MIIFvTCCA6WgAwIBAgIUGrTmW3qc39zqnE08e3qNDhUkeWswDQYJKoZIhvcNAQEL
BQAwbjELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAklMMRAwDgYDVQQHDAdDaGljYWdv
MRQwEgYDVQQKDAtDYXBvbmUsIEluYzEQMA4GA1UECwwHU3BlY09wczEYMBYGA1UE
AwwPZm9vLmV4YW1wbGUuY29tMB4XDTE5MDcwODEzNTUwNVoXDTIwMDcwNzEzNTUw
NVowbjELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAklMMRAwDgYDVQQHDAdDaGljYWdv
MRQwEgYDVQQKDAtDYXBvbmUsIEluYzEQMA4GA1UECwwHU3BlY09wczEYMBYGA1UE
AwwPZm9vLmV4YW1wbGUuY29tMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKC
AgEA3csSO13w7qQXKeSLNcpeuAe6wAjXYbRkRl6ariqzTEDcFTKmy2QiXJTKoEGn
bvwxq0T91var7rxY88SGL/qi8Zmo0tVSR0XvKSKcghFIkQOTyDmVgMPZGCvixt4q
gQ7hUVSk4KkFmtcqBVuvnzI1d/DKfZAGKdmGcfRpuAsnVhac3swP0w4Tl1BFrK9U
vuIkz4KwXG77s5oB8rMUnyuLasLsGNpvpvXhkcQRhp6vpcCO2bS7kOTTelAPIucw
P37qkOEdZdiWCLrr57dmhg6tmcVlmBMg6JtmfLxn2HQd9ZrCKlkWxMk5NYs6CAW5
kgbDZUWQTAsnHeoJKbcgtPkIbxDRxNpPukFMtbA4VEWv1EkODXy9FyEKDOI/PV6K
/80oLkgCIhCkP2mvwSFheU0RHTuZ0o0vVolP5TEOq5iufnDN4wrxqb12o//XLRc0
RiLqGVVxhFdyKCjVxcLfII9AAp5Tse4PMh6bf6jDfB3OMvGkhMbJWhKXdR2NUTl0
esKawMPRXIn5g3oBdNm8kyRsTTnvB567pU8uNSmA8j3jxfGCPynI8JdiwKQuW/+P
WgLIflgxqAfG85dVVOsFmF9o5o24dDslvv9yHnHH102c6ijPCg1EobqlyFzqqxOD
Wf2OPjIkzoTH+O27VRugnY/maIU1nshNO7ViRX5zIxEUtNMCAwEAAaNTMFEwHQYD
VR0OBBYEFNY4gDLgPBidogkmpO8nq5yAq5g+MB8GA1UdIwQYMBaAFNY4gDLgPBid
ogkmpO8nq5yAq5g+MA8GA1UdEwEB/wQFMAMBAf8wDQYJKoZIhvcNAQELBQADggIB
AJ0WGioNtGNg3m6ywpmxNThorQD5ZvDMlmZlDVk78E2wfNyMhwbVhKhlAnONv0wv
kmsGjibY75nRZ+EK9PxSJ644841fryQXQ+bli5fhr7DW3uTKwaRsnzETJXRJuljq
6+c6Zyg1/mqwnyx7YvPgVh3w496DYx/jm6Fm1IEq3BzOmn6H/gGPq3gbURzEqI3h
P+kC2vJa8RZWrpa05Xk/Q1QUkErDX9vJghb9z3+GgirISZQzqWRghII/znv3NOE6
zoIgaaWNFn8KPeBVpUoboH+IhpgibsnbTbI0G7AMtFq6qm3kn/4DZ2N2tuh1G2tT
zR2Fh7hJbU7CrqxANrgnIoHG/nLSvzE24ckLb0Vj69uGQlwnZkn9fz6F7KytU+Az
NoB2rjufaB0GQi1azdboMvdGSOxhSCAR8otWT5yDrywCqVnEvjw0oxKmuRduNe2/
6AcG6TtK2/K+LHuhymiAwZM2qE6VD2odvb+tCzDkZOIeoIz/JcVlNpXE9FuVl250
9NWvugeghq7tUv81iJ8ninBefJ4lUfxAehTPQqX+zXcfxgjvMRCi/ig73nLyhmjx
r2AaraPFgrprnxUibP4L7jxdr+iiw5bWN9/B81PodrS7n5TNtnfnpZD6X6rThqOP
xO7Tr5lAo74vNUkF2EHNaI28/RGnJPm2TIxZqy4rNH6L
-----END CERTIFICATE-----
`

	certFromCM := `
-----BEGIN CERTIFICATE-----
MIIDOTCCAiGgAwIBAgIQSRJrEpBGFc7tNb1fb5pKFzANBgkqhkiG9w0BAQsFADAS
MRAwDgYDVQQKEwdBY21lIENvMCAXDTcwMDEwMTAwMDAwMFoYDzIwODQwMTI5MTYw
MDAwWjASMRAwDgYDVQQKEwdBY21lIENvMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A
MIIBCgKCAQEA6Gba5tHV1dAKouAaXO3/ebDUU4rvwCUg/CNaJ2PT5xLD4N1Vcb8r
bFSW2HXKq+MPfVdwIKR/1DczEoAGf/JWQTW7EgzlXrCd3rlajEX2D73faWJekD0U
aUgz5vtrTXZ90BQL7WvRICd7FlEZ6FPOcPlumiyNmzUqtwGhO+9ad1W5BqJaRI6P
YfouNkwR6Na4TzSj5BrqUfP0FwDizKSJ0XXmh8g8G9mtwxOSN3Ru1QFc61Xyeluk
POGKBV/q6RBNklTNe0gI8usUMlYyoC7ytppNMW7X2vodAelSu25jgx2anj9fDVZu
h7AXF5+4nJS4AAt0n1lNY7nGSsdZas8PbQIDAQABo4GIMIGFMA4GA1UdDwEB/wQE
AwICpDATBgNVHSUEDDAKBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MB0GA1Ud
DgQWBBStsdjh3/JCXXYlQryOrL4Sh7BW5TAuBgNVHREEJzAlggtleGFtcGxlLmNv
bYcEfwAAAYcQAAAAAAAAAAAAAAAAAAAAATANBgkqhkiG9w0BAQsFAAOCAQEAxWGI
5NhpF3nwwy/4yB4i/CwwSpLrWUa70NyhvprUBC50PxiXav1TeDzwzLx/o5HyNwsv
cxv3HdkLW59i/0SlJSrNnWdfZ19oTcS+6PtLoVyISgtyN6DpkKpdG1cOkW3Cy2P2
+tK/tKHRP1Y/Ra0RiDpOAmqn0gCOFGz8+lqDIor/T7MTpibL3IxqWfPrvfVRHL3B
grw/ZQTTIVjjh4JBSW3WyWgNo/ikC1lrVxzl4iPUGptxT36Cr7Zk2Bsg0XqwbOvK
5d+NTDREkSnUbie4GeutujmX3Dsx88UiV6UY/4lHJa6I5leHUNOHahRbpbWeOfs/
WkBKOclmOV2xlTVuPw==
-----END CERTIFICATE-----
`

	rootCAPath := path.Join(temppath, "foo.example.com")
	err := os.WriteFile(rootCAPath, []byte(certFromFile), 0o666)
	if err != nil {
		panic(err)
	}

	testCases := []struct {
		name                    string
		scmRootCAPath           string
		insecure                bool
		caCerts                 []byte
		validateCertInTlsConfig bool
	}{
		{
			name:                    "Insecure mode configured, SCM Root CA Path not set",
			scmRootCAPath:           "",
			insecure:                true,
			caCerts:                 nil,
			validateCertInTlsConfig: false,
		},
		{
			name:                    "SCM Root CA Path set, Insecure mode set to false",
			scmRootCAPath:           rootCAPath,
			insecure:                false,
			caCerts:                 nil,
			validateCertInTlsConfig: true,
		},
		{
			name:                    "SCM Root CA Path set, Insecure mode set to true",
			scmRootCAPath:           rootCAPath,
			insecure:                true,
			caCerts:                 nil,
			validateCertInTlsConfig: true,
		},
		{
			name:                    "Cert passed, Insecure mode set to false",
			scmRootCAPath:           "",
			insecure:                false,
			caCerts:                 []byte(certFromCM),
			validateCertInTlsConfig: true,
		},
		{
			name:                    "SCM Root CA Path set, cert passed, Insecure mode set to false",
			scmRootCAPath:           rootCAPath,
			insecure:                false,
			caCerts:                 []byte(certFromCM),
			validateCertInTlsConfig: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			certPool := x509.NewCertPool()
			tlsConfig := GetTlsConfig(testCase.scmRootCAPath, testCase.insecure, testCase.caCerts)
			assert.Equal(t, testCase.insecure, tlsConfig.InsecureSkipVerify)
			if testCase.caCerts != nil {
				ok := certPool.AppendCertsFromPEM([]byte(certFromCM))
				assert.True(t, ok)
			}
			if testCase.scmRootCAPath != "" {
				ok := certPool.AppendCertsFromPEM([]byte(certFromFile))
				assert.True(t, ok)
			}
			assert.NotNil(t, tlsConfig)
			if testCase.validateCertInTlsConfig {
				assert.True(t, tlsConfig.RootCAs.Equal(certPool))
			}
		})
	}
}
