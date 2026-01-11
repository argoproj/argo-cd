package generators

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj/argo-cd/v3/applicationset/services/plugin"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestPluginGenerateParams(t *testing.T) {
	testCases := []struct {
		name                 string
		configmap            *corev1.ConfigMap
		secret               *corev1.Secret
		configNamespace      string
		inputParameters      map[string]apiextensionsv1.JSON
		values               map[string]string
		gotemplate           bool
		expected             []map[string]any
		content              []byte
		expectedError        string
		allowedPluginGenUrls []string
	}{
		{
			name: "simple case",
			configmap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &corev1.Secret{
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
			expected: []map[string]any{
				{
					"key1":                 "val1",
					"key2.key2_1":          "val2_1",
					"key2.key2_2.key2_2_1": "val2_2_1",
					"key3":                 "123",
					"generator": map[string]any{
						"input": argoprojiov1alpha1.PluginInput{
							Parameters: argoprojiov1alpha1.PluginParameters{
								"pkey1": {Raw: []byte(`"val1"`)},
								"pkey2": {Raw: []byte(`"val2"`)},
							},
						},
					},
				},
			},
			expectedError: "",
		},
		{
			name: "simple case with values",
			configmap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &corev1.Secret{
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
			expected: []map[string]any{
				{
					"key1":                 "val1",
					"key2.key2_1":          "val2_1",
					"key2.key2_2.key2_2_1": "val2_2_1",
					"key3":                 "123",
					"values.valuekey1":     "valuevalue1",
					"values.valuekey2":     "templated-val1",
					"generator": map[string]any{
						"input": argoprojiov1alpha1.PluginInput{
							Parameters: argoprojiov1alpha1.PluginParameters{
								"pkey1": {Raw: []byte(`"val1"`)},
								"pkey2": {Raw: []byte(`"val2"`)},
							},
						},
					},
				},
			},
			expectedError: "",
		},
		{
			name: "simple case with gotemplate",
			configmap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &corev1.Secret{
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
			expected: []map[string]any{
				{
					"key1": "val1",
					"key2": map[string]any{
						"key2_1": "val2_1",
						"key2_2": map[string]any{
							"key2_2_1": "val2_2_1",
						},
					},
					"key3": float64(123),
					"generator": map[string]any{
						"input": argoprojiov1alpha1.PluginInput{
							Parameters: argoprojiov1alpha1.PluginParameters{
								"pkey1": {Raw: []byte(`"val1"`)},
								"pkey2": {Raw: []byte(`"val2"`)},
							},
						},
					},
				},
			},
			expectedError: "",
		},
		{
			name: "simple case with appended params",
			configmap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &corev1.Secret{
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
			expected: []map[string]any{
				{
					"key1":                 "val1",
					"key2.key2_1":          "val2_1",
					"key2.key2_2.key2_2_1": "val2_2_1",
					"key3":                 "123",
					"pkey2":                "valplugin",
					"generator": map[string]any{
						"input": argoprojiov1alpha1.PluginInput{
							Parameters: argoprojiov1alpha1.PluginParameters{
								"pkey1": {Raw: []byte(`"val1"`)},
								"pkey2": {Raw: []byte(`"val2"`)},
							},
						},
					},
				},
			},
			expectedError: "",
		},
		{
			name: "no params",
			configmap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &corev1.Secret{
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
			expected: []map[string]any{
				{
					"key1":                 "val1",
					"key2.key2_1":          "val2_1",
					"key2.key2_2.key2_2_1": "val2_2_1",
					"key3":                 "123",
					"generator": map[string]any{
						"input": map[string]map[string]any{
							"parameters": {},
						},
					},
				},
			},
			expectedError: "",
		},
		{
			name: "empty return",
			configmap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &corev1.Secret{
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
			expected:        []map[string]any{},
			expectedError:   "",
		},
		{
			name: "wrong return",
			configmap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &corev1.Secret{
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
			expected:        []map[string]any{},
			expectedError:   "error listing params: error get api 'set': invalid character 'w' looking for beginning of value: wrong body ...",
		},
		{
			name: "external secret",
			configmap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin-secret:plugin.token",
				},
			},
			secret: &corev1.Secret{
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
			expected: []map[string]any{
				{
					"key1":                 "val1",
					"key2.key2_1":          "val2_1",
					"key2.key2_2.key2_2_1": "val2_2_1",
					"key3":                 "123",
					"pkey2":                "valplugin",
					"generator": map[string]any{
						"input": argoprojiov1alpha1.PluginInput{
							Parameters: argoprojiov1alpha1.PluginParameters{
								"pkey1": {Raw: []byte(`"val1"`)},
								"pkey2": {Raw: []byte(`"val2"`)},
							},
						},
					},
				},
			},
			expectedError: "",
		},
		{
			name: "no secret",
			configmap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &corev1.Secret{},
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
			expected: []map[string]any{
				{
					"key1":                 "val1",
					"key2.key2_1":          "val2_1",
					"key2.key2_2.key2_2_1": "val2_2_1",
					"key3":                 "123",
					"generator": map[string]any{
						"input": argoprojiov1alpha1.PluginInput{
							Parameters: argoprojiov1alpha1.PluginParameters{
								"pkey1": {Raw: []byte(`"val1"`)},
								"pkey2": {Raw: []byte(`"val2"`)},
							},
						},
					},
				},
			},
			expectedError: "error getting plugin from generator: error fetching Secret token: error fetching secret default/argocd-secret: secrets \"argocd-secret\" not found",
		},
		{
			name:      "no configmap",
			configmap: &corev1.ConfigMap{},
			secret: &corev1.Secret{
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
			expected: []map[string]any{
				{
					"key1":                 "val1",
					"key2.key2_1":          "val2_1",
					"key2.key2_2.key2_2_1": "val2_2_1",
					"key3":                 "123",
					"generator": map[string]any{
						"input": argoprojiov1alpha1.PluginInput{
							Parameters: argoprojiov1alpha1.PluginParameters{
								"pkey1": {Raw: []byte(`"val1"`)},
								"pkey2": {Raw: []byte(`"val2"`)},
							},
						},
					},
				},
			},
			expectedError: "error getting plugin from generator: error fetching ConfigMap: configmaps \"\" not found",
		},
		{
			name: "no baseUrl",
			configmap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"token": "$plugin.token",
				},
			},
			secret: &corev1.Secret{
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
			expected: []map[string]any{
				{
					"key1":                 "val1",
					"key2.key2_1":          "val2_1",
					"key2.key2_2.key2_2_1": "val2_2_1",
					"key3":                 "123",
					"generator": map[string]any{
						"input": argoprojiov1alpha1.PluginInput{
							Parameters: argoprojiov1alpha1.PluginParameters{
								"pkey1": {Raw: []byte(`"val1"`)},
								"pkey2": {Raw: []byte(`"val2"`)},
							},
						},
					},
				},
			},
			expectedError: "error getting plugin from generator: error fetching ConfigMap: baseUrl not found in ConfigMap",
		},
		{
			name: "no token",
			configmap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "default",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
				},
			},
			secret: &corev1.Secret{},
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
			expected: []map[string]any{
				{
					"key1":                 "val1",
					"key2.key2_1":          "val2_1",
					"key2.key2_2.key2_2_1": "val2_2_1",
					"key3":                 "123",
					"generator": map[string]any{
						"input": argoprojiov1alpha1.PluginInput{
							Parameters: argoprojiov1alpha1.PluginParameters{
								"pkey1": {Raw: []byte(`"val1"`)},
								"pkey2": {Raw: []byte(`"val2"`)},
							},
						},
					},
				},
			},
			expectedError: "error getting plugin from generator: error fetching ConfigMap: token not found in ConfigMap",
		},
		{
			name: "Config in another namespace",
			configmap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "other-ns",
					Labels: map[string]string{
						"argocd.argoproj.io/cm-type": "plugin-generator",
					},
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-secret",
					Namespace: "other-ns",
				},
				Data: map[string][]byte{
					"plugin.token": []byte("my-secret"),
				},
			},
			configNamespace: "other-ns",
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
			expected: []map[string]any{
				{
					"key1":                 "val1",
					"key2.key2_1":          "val2_1",
					"key2.key2_2.key2_2_1": "val2_2_1",
					"key3":                 "123",
					"generator": map[string]any{
						"input": argoprojiov1alpha1.PluginInput{
							Parameters: argoprojiov1alpha1.PluginParameters{
								"pkey1": {Raw: []byte(`"val1"`)},
								"pkey2": {Raw: []byte(`"val2"`)},
							},
						},
					},
				},
			},
			expectedError: "",
		},
		{
			name: "Config in another namespace and not specified in the plugin generator",
			configmap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "other-ns",
					Labels: map[string]string{
						"argocd.argoproj.io/cm-type": "plugin-generator",
					},
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-secret",
					Namespace: "other-ns",
				},
				Data: map[string][]byte{
					"plugin.token": []byte("my-secret"),
				},
			},
			configNamespace: "",
			inputParameters: map[string]apiextensionsv1.JSON{
				"pkey1": {Raw: []byte(`"val1"`)},
				"pkey2": {Raw: []byte(`"val2"`)},
			},
			gotemplate:    false,
			content:       []byte{},
			expected:      []map[string]any{},
			expectedError: "error getting plugin from generator: error fetching ConfigMap: configmaps \"first-plugin-cm\" not found",
		},
		{
			name: "Config in another namespace and missing the label",
			configmap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "other-ns",
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-secret",
					Namespace: "other-ns",
				},
				Data: map[string][]byte{
					"plugin.token": []byte("my-secret"),
				},
			},
			configNamespace: "other-ns",
			inputParameters: map[string]apiextensionsv1.JSON{
				"pkey1": {Raw: []byte(`"val1"`)},
				"pkey2": {Raw: []byte(`"val2"`)},
			},
			gotemplate:    false,
			content:       []byte{},
			expected:      []map[string]any{},
			expectedError: "error getting plugin from generator: error fetching ConfigMap: configMap with name first-plugin-cm not found in namespace other-ns. Check if it's correctly labelled with argocd.argoproj.io/cm-type=plugin-generator",
		},
		{
			name: "Config in another namespace and with invalid label value",
			configmap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "other-ns",
					Labels: map[string]string{
						"argocd.argoproj.io/cm-type": "INVALID",
					},
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-secret",
					Namespace: "other-ns",
				},
				Data: map[string][]byte{
					"plugin.token": []byte("my-secret"),
				},
			},
			configNamespace: "other-ns",
			inputParameters: map[string]apiextensionsv1.JSON{
				"pkey1": {Raw: []byte(`"val1"`)},
				"pkey2": {Raw: []byte(`"val2"`)},
			},
			gotemplate:    false,
			content:       []byte{},
			expected:      []map[string]any{},
			expectedError: "error getting plugin from generator: error fetching ConfigMap: configMap with name first-plugin-cm not found in namespace other-ns. Check if it's correctly labelled with argocd.argoproj.io/cm-type=plugin-generator",
		},
		{
			name: "Config in another namespace and allowed in the list of urls",
			configmap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plugin-cm",
					Namespace: "other-ns",
					Labels: map[string]string{
						"argocd.argoproj.io/cm-type": "plugin-generator",
					},
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-secret",
					Namespace: "other-ns",
				},
				Data: map[string][]byte{
					"plugin.token": []byte("my-secret"),
				},
			},
			configNamespace: "other-ns",
			inputParameters: map[string]apiextensionsv1.JSON{
				"pkey1": {Raw: []byte(`"val1"`)},
				"pkey2": {Raw: []byte(`"val2"`)},
			},
			gotemplate: false,
			allowedPluginGenUrls: []string{
				"https://someurl",
				"http://127.0.0.1:*",
			},
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
			expected: []map[string]any{
				{
					"key1":                 "val1",
					"key2.key2_1":          "val2_1",
					"key2.key2_2.key2_2_1": "val2_2_1",
					"key3":                 "123",
					"generator": map[string]any{
						"input": argoprojiov1alpha1.PluginInput{
							Parameters: argoprojiov1alpha1.PluginParameters{
								"pkey1": {Raw: []byte(`"val1"`)},
								"pkey2": {Raw: []byte(`"val2"`)},
							},
						},
					},
				},
			},
			expectedError: "",
		},
		{
			name: "Config in another namespace and not in the allowed url list",
			configmap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-plugin-cm",
					Namespace: "other-ns",
					Labels: map[string]string{
						"argocd.argoproj.io/cm-type": "plugin-generator",
					},
				},
				Data: map[string]string{
					"baseUrl": "http://127.0.0.1",
					"token":   "$plugin.token",
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-secret",
					Namespace: "other-ns",
				},
				Data: map[string][]byte{
					"plugin.token": []byte("my-secret"),
				},
			},
			configNamespace: "other-ns",
			inputParameters: map[string]apiextensionsv1.JSON{
				"pkey1": {Raw: []byte(`"val1"`)},
				"pkey2": {Raw: []byte(`"val2"`)},
			},
			gotemplate: false,
			allowedPluginGenUrls: []string{
				"https://someurl",
				"http://*.mydomain.com",
			},
			content:       []byte{},
			expected:      []map[string]any{},
			expectedError: "error getting plugin from generator: error fetching ConfigMap: baseUrl %s not allowed",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			generatorConfig := argoprojiov1alpha1.ApplicationSetGenerator{
				Plugin: &argoprojiov1alpha1.PluginGenerator{
					ConfigMapRef: argoprojiov1alpha1.PluginConfigMapRef{Name: testCase.configmap.Name, Namespace: testCase.configNamespace},
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

			fakeClientWithCache := fake.NewClientBuilder().WithObjects([]client.Object{testCase.configmap, testCase.secret}...).Build()

			pluginGenerator := NewPluginGenerator(fakeClientWithCache, testCase.allowedPluginGenUrls)

			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "set",
					Namespace: "default",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					GoTemplate: testCase.gotemplate,
				},
			}

			got, err := pluginGenerator.GenerateParams(&generatorConfig, &applicationSetInfo, nil)
			if err != nil {
				fmt.Println(err)
			}

			if testCase.expectedError != "" {
				var errorStr string
				if strings.Contains(testCase.expectedError, "%s") {
					errorStr = fmt.Sprintf(testCase.expectedError, testCase.configmap.Data["baseUrl"])
				} else {
					errorStr = testCase.expectedError
				}
				require.EqualError(t, err, errorStr)
			} else {
				require.NoError(t, err)
				expectedJSON, err := json.Marshal(testCase.expected)
				require.NoError(t, err)
				gotJSON, err := json.Marshal(got)
				require.NoError(t, err)
				assert.JSONEq(t, string(expectedJSON), string(gotJSON))
			}
		})
	}
}
