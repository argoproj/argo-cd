package generators

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

const (
	resourceApiVersion = "mallard.io/v1"
	resourceKind       = "ducks"
	resourceName       = "quak"
)

func TestGenerateParamsForDuckType(t *testing.T) {
	clusters := []client.Object{
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
				"name":   []byte("production-01"),
				"server": []byte("https://production-01.example.com"),
			},
			Type: corev1.SecretType("Opaque"),
		},
	}

	duckType := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": resourceApiVersion,
			"kind":       "Duck",
			"metadata": map[string]interface{}{
				"name":      resourceName,
				"namespace": "namespace",
				"labels":    map[string]interface{}{"duck": "all-species"},
			},
			"status": map[string]interface{}{
				"decisions": []interface{}{
					map[string]interface{}{
						"clusterName": "staging-01",
					},
					map[string]interface{}{
						"clusterName": "production-01",
					},
				},
			},
		},
	}

	duckTypeProdOnly := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": resourceApiVersion,
			"kind":       "Duck",
			"metadata": map[string]interface{}{
				"name":      resourceName,
				"namespace": "namespace",
				"labels":    map[string]interface{}{"duck": "spotted"},
			},
			"status": map[string]interface{}{
				"decisions": []interface{}{
					map[string]interface{}{
						"clusterName": "production-01",
					},
				},
			},
		},
	}

	duckTypeEmpty := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": resourceApiVersion,
			"kind":       "Duck",
			"metadata": map[string]interface{}{
				"name":      resourceName,
				"namespace": "namespace",
				"labels":    map[string]interface{}{"duck": "canvasback"},
			},
			"status": map[string]interface{}{},
		},
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-configmap",
			Namespace: "namespace",
		},
		Data: map[string]string{
			"apiVersion":    resourceApiVersion,
			"kind":          resourceKind,
			"statusListKey": "decisions",
			"matchKey":      "clusterName",
		},
	}

	testCases := []struct {
		name          string
		configMapRef  string
		resourceName  string
		labelSelector metav1.LabelSelector
		resource      *unstructured.Unstructured
		values        map[string]string
		expected      []map[string]interface{}
		expectedError error
	}{
		{
			name:          "no duck resource",
			resourceName:  "",
			resource:      duckType,
			values:        nil,
			expected:      []map[string]interface{}{},
			expectedError: fmt.Errorf("There is a problem with the definition of the ClusterDecisionResource generator"),
		},
		/*** This does not work with the FAKE runtime client, fieldSelectors are broken.
		{
			name:          "invalid name for duck resource",
			resourceName:  resourceName + "-different",
			resource:      duckType,
			values:        nil,
			expected:      []map[string]string{},
			expectedError: fmt.Errorf("duck.mallard.io \"quak\" not found"),
		},
		***/
		{
			name:         "duck type generator resourceName",
			resourceName: resourceName,
			resource:     duckType,
			values:       nil,
			expected: []map[string]interface{}{
				{"clusterName": "production-01", "name": "production-01", "server": "https://production-01.example.com"},

				{"clusterName": "staging-01", "name": "staging-01", "server": "https://staging-01.example.com"},
			},
			expectedError: nil,
		},
		{
			name:         "production-only",
			resourceName: resourceName,
			resource:     duckTypeProdOnly,
			values: map[string]string{
				"foo": "bar",
			},
			expected: []map[string]interface{}{
				{"clusterName": "production-01", "values.foo": "bar", "name": "production-01", "server": "https://production-01.example.com"},
			},
			expectedError: nil,
		},
		{
			name:          "duck type empty status",
			resourceName:  resourceName,
			resource:      duckTypeEmpty,
			values:        nil,
			expected:      nil,
			expectedError: nil,
		},
		{
			name:          "duck type empty status labelSelector.matchLabels",
			resourceName:  "",
			labelSelector: metav1.LabelSelector{MatchLabels: map[string]string{"duck": "canvasback"}},
			resource:      duckTypeEmpty,
			values:        nil,
			expected:      nil,
			expectedError: nil,
		},
		{
			name:          "duck type generator labelSelector.matchLabels",
			resourceName:  "",
			labelSelector: metav1.LabelSelector{MatchLabels: map[string]string{"duck": "all-species"}},
			resource:      duckType,
			values:        nil,
			expected: []map[string]interface{}{
				{"clusterName": "production-01", "name": "production-01", "server": "https://production-01.example.com"},

				{"clusterName": "staging-01", "name": "staging-01", "server": "https://staging-01.example.com"},
			},
			expectedError: nil,
		},
		{
			name:          "production-only labelSelector.matchLabels",
			resourceName:  "",
			resource:      duckTypeProdOnly,
			labelSelector: metav1.LabelSelector{MatchLabels: map[string]string{"duck": "spotted"}},
			values: map[string]string{
				"foo": "bar",
			},
			expected: []map[string]interface{}{
				{"clusterName": "production-01", "values.foo": "bar", "name": "production-01", "server": "https://production-01.example.com"},
			},
			expectedError: nil,
		},
		{
			name:         "duck type generator labelSelector.matchExpressions",
			resourceName: "",
			labelSelector: metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "duck",
					Operator: "In",
					Values:   []string{"all-species", "marbled"},
				},
			}},
			resource: duckType,
			values:   nil,
			expected: []map[string]interface{}{
				{"clusterName": "production-01", "name": "production-01", "server": "https://production-01.example.com"},

				{"clusterName": "staging-01", "name": "staging-01", "server": "https://staging-01.example.com"},
			},
			expectedError: nil,
		},
		{
			name:         "duck type generator resourceName and labelSelector.matchExpressions",
			resourceName: resourceName,
			labelSelector: metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "duck",
					Operator: "In",
					Values:   []string{"all-species", "marbled"},
				},
			}},
			resource:      duckType,
			values:        nil,
			expected:      nil,
			expectedError: fmt.Errorf("There is a problem with the definition of the ClusterDecisionResource generator"),
		},
	}

	// convert []client.Object to []runtime.Object, for use by kubefake package
	runtimeClusters := []runtime.Object{}
	for _, clientCluster := range clusters {
		runtimeClusters = append(runtimeClusters, clientCluster)
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			appClientset := kubefake.NewSimpleClientset(append(runtimeClusters, configMap)...)

			gvrToListKind := map[schema.GroupVersionResource]string{{
				Group:    "mallard.io",
				Version:  "v1",
				Resource: "ducks",
			}: "DuckList"}

			fakeDynClient := dynfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, testCase.resource)

			duckTypeGenerator := NewDuckTypeGenerator(context.Background(), fakeDynClient, appClientset, "namespace")

			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{},
			}

			got, err := duckTypeGenerator.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{
				ClusterDecisionResource: &argoprojiov1alpha1.DuckTypeGenerator{
					ConfigMapRef:  "my-configmap",
					Name:          testCase.resourceName,
					LabelSelector: testCase.labelSelector,
					Values:        testCase.values,
				},
			}, &applicationSetInfo, nil)

			if testCase.expectedError != nil {
				require.EqualError(t, err, testCase.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.ElementsMatch(t, testCase.expected, got)
			}
		})
	}
}

func TestGenerateParamsForDuckTypeGoTemplate(t *testing.T) {
	clusters := []client.Object{
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
				"name":   []byte("production-01"),
				"server": []byte("https://production-01.example.com"),
			},
			Type: corev1.SecretType("Opaque"),
		},
	}

	duckType := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": resourceApiVersion,
			"kind":       "Duck",
			"metadata": map[string]interface{}{
				"name":      resourceName,
				"namespace": "namespace",
				"labels":    map[string]interface{}{"duck": "all-species"},
			},
			"status": map[string]interface{}{
				"decisions": []interface{}{
					map[string]interface{}{
						"clusterName": "staging-01",
					},
					map[string]interface{}{
						"clusterName": "production-01",
					},
				},
			},
		},
	}

	duckTypeProdOnly := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": resourceApiVersion,
			"kind":       "Duck",
			"metadata": map[string]interface{}{
				"name":      resourceName,
				"namespace": "namespace",
				"labels":    map[string]interface{}{"duck": "spotted"},
			},
			"status": map[string]interface{}{
				"decisions": []interface{}{
					map[string]interface{}{
						"clusterName": "production-01",
					},
				},
			},
		},
	}

	duckTypeEmpty := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": resourceApiVersion,
			"kind":       "Duck",
			"metadata": map[string]interface{}{
				"name":      resourceName,
				"namespace": "namespace",
				"labels":    map[string]interface{}{"duck": "canvasback"},
			},
			"status": map[string]interface{}{},
		},
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-configmap",
			Namespace: "namespace",
		},
		Data: map[string]string{
			"apiVersion":    resourceApiVersion,
			"kind":          resourceKind,
			"statusListKey": "decisions",
			"matchKey":      "clusterName",
		},
	}

	testCases := []struct {
		name          string
		configMapRef  string
		resourceName  string
		labelSelector metav1.LabelSelector
		resource      *unstructured.Unstructured
		values        map[string]string
		expected      []map[string]interface{}
		expectedError error
	}{
		{
			name:          "no duck resource",
			resourceName:  "",
			resource:      duckType,
			values:        nil,
			expected:      []map[string]interface{}{},
			expectedError: fmt.Errorf("There is a problem with the definition of the ClusterDecisionResource generator"),
		},
		/*** This does not work with the FAKE runtime client, fieldSelectors are broken.
		{
			name:          "invalid name for duck resource",
			resourceName:  resourceName + "-different",
			resource:      duckType,
			values:        nil,
			expected:      []map[string]string{},
			expectedError: fmt.Errorf("duck.mallard.io \"quak\" not found"),
		},
		***/
		{
			name:         "duck type generator resourceName",
			resourceName: resourceName,
			resource:     duckType,
			values:       nil,
			expected: []map[string]interface{}{
				{"clusterName": "production-01", "name": "production-01", "server": "https://production-01.example.com"},

				{"clusterName": "staging-01", "name": "staging-01", "server": "https://staging-01.example.com"},
			},
			expectedError: nil,
		},
		{
			name:         "production-only",
			resourceName: resourceName,
			resource:     duckTypeProdOnly,
			values: map[string]string{
				"foo": "bar",
			},
			expected: []map[string]interface{}{
				{"clusterName": "production-01", "values": map[string]string{"foo": "bar"}, "name": "production-01", "server": "https://production-01.example.com"},
			},
			expectedError: nil,
		},
		{
			name:          "duck type empty status",
			resourceName:  resourceName,
			resource:      duckTypeEmpty,
			values:        nil,
			expected:      nil,
			expectedError: nil,
		},
		{
			name:          "duck type empty status labelSelector.matchLabels",
			resourceName:  "",
			labelSelector: metav1.LabelSelector{MatchLabels: map[string]string{"duck": "canvasback"}},
			resource:      duckTypeEmpty,
			values:        nil,
			expected:      nil,
			expectedError: nil,
		},
		{
			name:          "duck type generator labelSelector.matchLabels",
			resourceName:  "",
			labelSelector: metav1.LabelSelector{MatchLabels: map[string]string{"duck": "all-species"}},
			resource:      duckType,
			values:        nil,
			expected: []map[string]interface{}{
				{"clusterName": "production-01", "name": "production-01", "server": "https://production-01.example.com"},

				{"clusterName": "staging-01", "name": "staging-01", "server": "https://staging-01.example.com"},
			},
			expectedError: nil,
		},
		{
			name:          "production-only labelSelector.matchLabels",
			resourceName:  "",
			resource:      duckTypeProdOnly,
			labelSelector: metav1.LabelSelector{MatchLabels: map[string]string{"duck": "spotted"}},
			values: map[string]string{
				"foo": "bar",
			},
			expected: []map[string]interface{}{
				{"clusterName": "production-01", "values": map[string]string{"foo": "bar"}, "name": "production-01", "server": "https://production-01.example.com"},
			},
			expectedError: nil,
		},
		{
			name:         "duck type generator labelSelector.matchExpressions",
			resourceName: "",
			labelSelector: metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "duck",
					Operator: "In",
					Values:   []string{"all-species", "marbled"},
				},
			}},
			resource: duckType,
			values:   nil,
			expected: []map[string]interface{}{
				{"clusterName": "production-01", "name": "production-01", "server": "https://production-01.example.com"},

				{"clusterName": "staging-01", "name": "staging-01", "server": "https://staging-01.example.com"},
			},
			expectedError: nil,
		},
		{
			name:         "duck type generator resourceName and labelSelector.matchExpressions",
			resourceName: resourceName,
			labelSelector: metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "duck",
					Operator: "In",
					Values:   []string{"all-species", "marbled"},
				},
			}},
			resource:      duckType,
			values:        nil,
			expected:      nil,
			expectedError: fmt.Errorf("There is a problem with the definition of the ClusterDecisionResource generator"),
		},
	}

	// convert []client.Object to []runtime.Object, for use by kubefake package
	runtimeClusters := []runtime.Object{}
	for _, clientCluster := range clusters {
		runtimeClusters = append(runtimeClusters, clientCluster)
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			appClientset := kubefake.NewSimpleClientset(append(runtimeClusters, configMap)...)

			gvrToListKind := map[schema.GroupVersionResource]string{{
				Group:    "mallard.io",
				Version:  "v1",
				Resource: "ducks",
			}: "DuckList"}

			fakeDynClient := dynfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, testCase.resource)

			duckTypeGenerator := NewDuckTypeGenerator(context.Background(), fakeDynClient, appClientset, "namespace")

			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					GoTemplate: true,
				},
			}

			got, err := duckTypeGenerator.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{
				ClusterDecisionResource: &argoprojiov1alpha1.DuckTypeGenerator{
					ConfigMapRef:  "my-configmap",
					Name:          testCase.resourceName,
					LabelSelector: testCase.labelSelector,
					Values:        testCase.values,
				},
			}, &applicationSetInfo, nil)

			if testCase.expectedError != nil {
				require.EqualError(t, err, testCase.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.ElementsMatch(t, testCase.expected, got)
			}
		})
	}
}
