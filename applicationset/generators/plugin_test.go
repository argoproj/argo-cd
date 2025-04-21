package generators

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj/argo-cd/v2/applicationset/services/plugin"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestPluginGenerateParams(t *testing.T) {
	testCases := []struct {
		name            string
		configmap       *v1.ConfigMap
		secret          *v1.Secret
		inputParameters map[string]apiextensionsv1.JSON
		values          map[string]string
		gotemplate      bool
		expected        []map[string]interface{}
		content         []byte
		expectedError   error
	}{
		{
			name: "simple case",
			configmap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"plugin.token": []byte("my-secret"),
				},
			},
			inputParameters: map[string]apiextensionsv1.JSON{
				"pkey1": {Raw: []byte(`"val1"`)},
				"pkey2": {Raw: []byte(`"val2"`)},
			},
			gotemplate: false,
			content: []byte(`{"output": {
				"parameters": [{
					"key1": "val1",
					"key2": {
						"key2_1": "val2_1",
						"key2_2": {
							"key2_2_1": "val2_2_1"
						}
					},
					"key3": 123
                }]
			 }}`),
			expected: []map[string]interface{}{
				{
					"key1":                 "val1",
					"key2.key2_1":          "val2_1",
					"key2.key2_2.key2_2_1": "val2_2_1",
					"key3":                 "123",
					"generator": map[string]interface{}{
						"input": argoprojiov1alpha1.PluginInput{
							Parameters: argoprojiov1alpha1.PluginParameters{
								"pkey1": {Raw: []byte(`"val1"`)},
								"pkey2": {Raw: []byte(`"val2"`)},
							},
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "simple case with values",
			configmap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"plugin.token": []byte("my-secret"),
				},
			},
			inputParameters: map[string]apiextensionsv1.JSON{
				"pkey1": {Raw: []byte(`"val1"`)},
				"pkey2": {Raw: []byte(`"val2"`)},
			},
			values: map[string]string{
				"valuekey1": "valuevalue1",
				"valuekey2": "templated-{{key1}}",
			},
			gotemplate: false,
			content: []byte(`{"output": {
				"parameters": [{
					"key1": "val1",
					"key2": {
						"key2_1": "val2_1",
						"key2_2": {
							"key2_2_1": "val2_2_1"
						}
					},
					"key3": 123
                }]
			 }}`),
			expected: []map[string]interface{}{
				{
					"key1":                 "val1",
					"key2.key2_1":          "val2_1",
					"key2.key2_2.key2_2_1": "val2_2_1",
					"key3":                 "123",
					"values.valuekey1":     "valuevalue1",
					"values.valuekey2":     "templated-val1",
					"generator": map[string]interface{}{
						"input": argoprojiov1alpha1.PluginInput{
							Parameters: argoprojiov1alpha1.PluginParameters{
								"pkey1": {Raw: []byte(`"val1"`)},
								"pkey2": {Raw: []byte(`"val2"`)},
							},
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "simple case with gotemplate",
			configmap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"plugin.token": []byte("my-secret"),
				},
			},
			inputParameters: map[string]apiextensionsv1.JSON{
				"pkey1": {Raw: []byte(`"val1"`)},
				"pkey2": {Raw: []byte(`"val2"`)},
			},
			gotemplate: true,
			content: []byte(`{"output": {
				"parameters": [{
					"key1": "val1",
					"key2": {
						"key2_1": "val2_1",
						"key2_2": {
							"key2_2_1": "val2_2_1"
						}
					},
					"key3": 123
                }]
			 }}`),
			expected: []map[string]interface{}{
				{
					"key1": "val1",
					"key2": map[string]interface{}{
						"key2_1": "val2_1",
						"key2_2": map[string]interface{}{
							"key2_2_1": "val2_2_1",
						},
					},
					"key3": float64(123),
					"generator": map[string]interface{}{
						"input": argoprojiov1alpha1.PluginInput{
							Parameters: argoprojiov1alpha1.PluginParameters{
								"pkey1": {Raw: []byte(`"val1"`)},
								"pkey2": {Raw: []byte(`"val2"`)},
							},
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "simple case with appended params",
			configmap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"plugin.token": []byte("my-secret"),
				},
			},
			inputParameters: map[string]apiextensionsv1.JSON{
				"pkey1": {Raw: []byte(`"val1"`)},
				"pkey2": {Raw: []byte(`"val2"`)},
			},
			gotemplate: false,
			content: []byte(`{"output": {"parameters": [{
				"key1": "val1",
				"key2": {
					"key2_1": "val2_1",
					"key2_2": {
						"key2_2_1": "val2_2_1"
					}
				},
				"key3": 123,
				"pkey2": "valplugin"
			 }]}}`),
			expected: []map[string]interface{}{
				{
					"key1":                 "val1",
					"key2.key2_1":          "val2_1",
					"key2.key2_2.key2_2_1": "val2_2_1",
					"key3":                 "123",
					"pkey2":                "valplugin",
					"generator": map[string]interface{}{
						"input": argoprojiov1alpha1.PluginInput{
							Parameters: argoprojiov1alpha1.PluginParameters{
								"pkey1": {Raw: []byte(`"val1"`)},
								"pkey2": {Raw: []byte(`"val2"`)},
							},
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "no params",
			configmap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"plugin.token": []byte("my-secret"),
				},
			},
			inputParameters: argoprojiov1alpha1.PluginParameters{},
			gotemplate:      false,
			content: []byte(`{"output": {
				"parameters": [{
					"key1": "val1",
					"key2": {
						"key2_1": "val2_1",
						"key2_2": {
							"key2_2_1": "val2_2_1"
						}
					},
					"key3": 123
                }]
			 }}`),
			expected: []map[string]interface{}{
				{
					"key1":                 "val1",
					"key2.key2_1":          "val2_1",
					"key2.key2_2.key2_2_1": "val2_2_1",
					"key3":                 "123",
					"generator": map[string]interface{}{
						"input": map[string]map[string]interface{}{
							"parameters": {},
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "empty return",
			configmap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"plugin.token": []byte("my-secret"),
				},
			},
			inputParameters: map[string]apiextensionsv1.JSON{},
			gotemplate:      false,
			content:         []byte(`{"input": {"parameters": []}}`),
			expected:        []map[string]interface{}{},
			expectedError:   nil,
		},
		{
			name: "wrong return",
			configmap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"plugin.token": []byte("my-secret"),
				},
			},
			inputParameters: map[string]apiextensionsv1.JSON{},
			gotemplate:      false,
			content:         []byte(`wrong body ...`),
			expected:        []map[string]interface{}{},
			expectedError:   fmt.Errorf("error listing params: error get api 'set': invalid character 'w' looking for beginning of value: wrong body ..."),
		},
		{
			name: "external secret",
			configmap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin-secret:plugin.token",
				},
			},
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plugin-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"plugin.token": []byte("my-secret"),
				},
			},
			inputParameters: map[string]apiextensionsv1.JSON{
				"pkey1": {Raw: []byte(`"val1"`)},
				"pkey2": {Raw: []byte(`"val2"`)},
			},
			gotemplate: false,
			content: []byte(`{"output": {"parameters": [{
				"key1": "val1",
				"key2": {
					"key2_1": "val2_1",
					"key2_2": {
						"key2_2_1": "val2_2_1"
					}
				},
				"key3": 123,
				"pkey2": "valplugin"
			 }]}}`),
			expected: []map[string]interface{}{
				{
					"key1":                 "val1",
					"key2.key2_1":          "val2_1",
					"key2.key2_2.key2_2_1": "val2_2_1",
					"key3":                 "123",
					"pkey2":                "valplugin",
					"generator": map[string]interface{}{
						"input": argoprojiov1alpha1.PluginInput{
							Parameters: argoprojiov1alpha1.PluginParameters{
								"pkey1": {Raw: []byte(`"val1"`)},
								"pkey2": {Raw: []byte(`"val2"`)},
							},
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "no secret",
			configmap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &v1.Secret{},
			inputParameters: map[string]apiextensionsv1.JSON{
				"pkey1": {Raw: []byte(`"val1"`)},
				"pkey2": {Raw: []byte(`"val2"`)},
			},
			gotemplate: false,
			content: []byte(`{"output": {
				"parameters": [{
					"key1": "val1",
					"key2": {
						"key2_1": "val2_1",
						"key2_2": {
							"key2_2_1": "val2_2_1"
						}
					},
					"key3": 123
                }]
			 }}`),
			expected: []map[string]interface{}{
				{
					"key1":                 "val1",
					"key2.key2_1":          "val2_1",
					"key2.key2_2.key2_2_1": "val2_2_1",
					"key3":                 "123",
					"generator": map[string]interface{}{
						"input": argoprojiov1alpha1.PluginInput{
							Parameters: argoprojiov1alpha1.PluginParameters{
								"pkey1": {Raw: []byte(`"val1"`)},
								"pkey2": {Raw: []byte(`"val2"`)},
							},
						},
					},
				},
			},
			expectedError: fmt.Errorf("error getting plugin from generator: error fetching Secret token: error fetching secret default/argocd-secret: secrets \"argocd-secret\" not found"),
		},
		{
			name:      "no configmap",
			configmap: &v1.ConfigMap{},
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"plugin.token": []byte("my-secret"),
				},
			},
			inputParameters: map[string]apiextensionsv1.JSON{
				"pkey1": {Raw: []byte(`"val1"`)},
				"pkey2": {Raw: []byte(`"val2"`)},
			},
			gotemplate: false,
			content: []byte(`{"output": {
				"parameters": [{
					"key1": "val1",
					"key2": {
						"key2_1": "val2_1",
						"key2_2": {
							"key2_2_1": "val2_2_1"
						}
					},
					"key3": 123
                }]
			 }}`),
			expected: []map[string]interface{}{
				{
					"key1":                 "val1",
					"key2.key2_1":          "val2_1",
					"key2.key2_2.key2_2_1": "val2_2_1",
					"key3":                 "123",
					"generator": map[string]interface{}{
						"input": argoprojiov1alpha1.PluginInput{
							Parameters: argoprojiov1alpha1.PluginParameters{
								"pkey1": {Raw: []byte(`"val1"`)},
								"pkey2": {Raw: []byte(`"val2"`)},
							},
						},
					},
				},
			},
			expectedError: fmt.Errorf("error getting plugin from generator: error fetching ConfigMap: configmaps \"\" not found"),
		},
		{
			name: "no baseUrl",
			configmap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"token": "$plugin.token",
				},
			},
			secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"plugin.token": []byte("my-secret"),
				},
			},
			inputParameters: map[string]apiextensionsv1.JSON{
				"pkey1": {Raw: []byte(`"val1"`)},
				"pkey2": {Raw: []byte(`"val2"`)},
			},
			gotemplate: false,
			content: []byte(`{"output": {
				"parameters": [{
					"key1": "val1",
					"key2": {
						"key2_1": "val2_1",
						"key2_2": {
							"key2_2_1": "val2_2_1"
						}
					},
					"key3": 123
                }]
			 }}`),
			expected: []map[string]interface{}{
				{
					"key1":                 "val1",
					"key2.key2_1":          "val2_1",
					"key2.key2_2.key2_2_1": "val2_2_1",
					"key3":                 "123",
					"generator": map[string]interface{}{
						"input": argoprojiov1alpha1.PluginInput{
							Parameters: argoprojiov1alpha1.PluginParameters{
								"pkey1": {Raw: []byte(`"val1"`)},
								"pkey2": {Raw: []byte(`"val2"`)},
							},
						},
					},
				},
			},
			expectedError: fmt.Errorf("error getting plugin from generator: error fetching ConfigMap: baseUrl not found in ConfigMap"),
		},
		{
			name: "no token",
			configmap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
				},
			},
			secret: &v1.Secret{},
			inputParameters: map[string]apiextensionsv1.JSON{
				"pkey1": {Raw: []byte(`"val1"`)},
				"pkey2": {Raw: []byte(`"val2"`)},
			},
			gotemplate: false,
			content: []byte(`{"output": {
				"parameters": [{
					"key1": "val1",
					"key2": {
						"key2_1": "val2_1",
						"key2_2": {
							"key2_2_1": "val2_2_1"
						}
					},
					"key3": 123
                }]
			 }}`),
			expected: []map[string]interface{}{
				{
					"key1":                 "val1",
					"key2.key2_1":          "val2_1",
					"key2.key2_2.key2_2_1": "val2_2_1",
					"key3":                 "123",
					"generator": map[string]interface{}{
						"input": argoprojiov1alpha1.PluginInput{
							Parameters: argoprojiov1alpha1.PluginParameters{
								"pkey1": {Raw: []byte(`"val1"`)},
								"pkey2": {Raw: []byte(`"val2"`)},
							},
						},
					},
				},
			},
			expectedError: fmt.Errorf("error getting plugin from generator: error fetching ConfigMap: token not found in ConfigMap"),
		},
	}

	ctx := context.Background()

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			generatorConfig := argoprojiov1alpha1.ApplicationSetGenerator{
				Plugin: &argoprojiov1alpha1.PluginGenerator{
					ConfigMapRef: argoprojiov1alpha1.PluginConfigMapRef{Name: testCase.configmap.Name},
					Input: argoprojiov1alpha1.PluginInput{
						Parameters: testCase.inputParameters,
					},
					Values: testCase.values,
				},
			}

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authHeader := r.Header.Get("Authorization")
				_, tokenKey := plugin.ParseSecretKey(testCase.configmap.Data["token"])
				expectedToken := testCase.secret.Data[strings.ReplaceAll(tokenKey, "$", "")]
				if authHeader != "Bearer "+string(expectedToken) {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write(testCase.content)
				if err != nil {
					require.NoError(t, fmt.Errorf("Error Write %w", err))
				}
			})

			fakeServer := httptest.NewServer(handler)

			defer fakeServer.Close()

			if _, ok := testCase.configmap.Data["baseUrl"]; ok {
				testCase.configmap.Data["baseUrl"] = fakeServer.URL
			}

			fakeClient := kubefake.NewSimpleClientset(append([]runtime.Object{}, testCase.configmap, testCase.secret)...)

			fakeClientWithCache := fake.NewClientBuilder().WithObjects([]client.Object{testCase.configmap, testCase.secret}...).Build()

			pluginGenerator := NewPluginGenerator(fakeClientWithCache, ctx, fakeClient, "default")

			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					GoTemplate: testCase.gotemplate,
				},
			}

			got, err := pluginGenerator.GenerateParams(&generatorConfig, &applicationSetInfo, nil)
			if err != nil {
				fmt.Println(err)
			}

			if testCase.expectedError != nil {
				require.EqualError(t, err, testCase.expectedError.Error())
			} else {
				require.NoError(t, err)
				expectedJson, err := json.Marshal(testCase.expected)
				require.NoError(t, err)
				gotJson, err := json.Marshal(got)
				require.NoError(t, err)
				assert.Equal(t, string(expectedJson), string(gotJson))
			}
		})
	}
}
