package generators

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type possiblyErroringFakeCtrlRuntimeClient struct {
	client.Client
	shouldError bool
}

func (p *possiblyErroringFakeCtrlRuntimeClient) List(ctx context.Context, secretList client.ObjectList, opts ...client.ListOption) error {
	if p.shouldError {
		return fmt.Errorf("could not list Secrets")
	}
	return p.Client.List(ctx, secretList, opts...)
}

func TestGenerateParams(t *testing.T) {
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
				"name":   []byte("production_01/west"),
				"server": []byte("https://production-01.example.com"),
			},
			Type: corev1.SecretType("Opaque"),
		},
	}
	testCases := []struct {
		name     string
		selector metav1.LabelSelector
		values   map[string]string
		expected []map[string]interface{}
		// clientError is true if a k8s client error should be simulated
		clientError   bool
		expectedError error
	}{
		{
			name:     "no label selector",
			selector: metav1.LabelSelector{},
			values: map[string]string{
				"lol1":  "lol",
				"lol2":  "{{values.lol1}}{{values.lol1}}",
				"lol3":  "{{values.lol2}}{{values.lol2}}{{values.lol2}}",
				"foo":   "bar",
				"bar":   "{{ metadata.annotations.foo.argoproj.io }}",
				"bat":   "{{ metadata.labels.environment }}",
				"aaa":   "{{ server }}",
				"no-op": "{{ this-does-not-exist }}",
			}, expected: []map[string]interface{}{
				{
					"values.lol1": "lol", "values.lol2": "{{values.lol1}}{{values.lol1}}", "values.lol3": "{{values.lol2}}{{values.lol2}}{{values.lol2}}", "values.foo": "bar", "values.bar": "production", "values.no-op": "{{ this-does-not-exist }}", "values.bat": "production", "values.aaa": "https://production-01.example.com", "name": "production_01/west", "nameNormalized": "production-01-west", "server": "https://production-01.example.com", "metadata.labels.environment": "production", "metadata.labels.org": "bar",
					"metadata.labels.argocd.argoproj.io/secret-type": "cluster", "metadata.annotations.foo.argoproj.io": "production",
				},

				{
					"values.lol1": "lol", "values.lol2": "{{values.lol1}}{{values.lol1}}", "values.lol3": "{{values.lol2}}{{values.lol2}}{{values.lol2}}", "values.foo": "bar", "values.bar": "staging", "values.no-op": "{{ this-does-not-exist }}", "values.bat": "staging", "values.aaa": "https://staging-01.example.com", "name": "staging-01", "nameNormalized": "staging-01", "server": "https://staging-01.example.com", "metadata.labels.environment": "staging", "metadata.labels.org": "foo",
					"metadata.labels.argocd.argoproj.io/secret-type": "cluster", "metadata.annotations.foo.argoproj.io": "staging",
				},

				{"values.lol1": "lol", "values.lol2": "{{values.lol1}}{{values.lol1}}", "values.lol3": "{{values.lol2}}{{values.lol2}}{{values.lol2}}", "values.foo": "bar", "values.bar": "{{ metadata.annotations.foo.argoproj.io }}", "values.no-op": "{{ this-does-not-exist }}", "values.bat": "{{ metadata.labels.environment }}", "values.aaa": "https://kubernetes.default.svc", "nameNormalized": "in-cluster", "name": "in-cluster", "server": "https://kubernetes.default.svc"},
			},
			clientError:   false,
			expectedError: nil,
		},
		{
			name: "secret type label selector",
			selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"argocd.argoproj.io/secret-type": "cluster",
				},
			},
			values: nil,
			expected: []map[string]interface{}{
				{
					"name": "production_01/west", "nameNormalized": "production-01-west", "server": "https://production-01.example.com", "metadata.labels.environment": "production", "metadata.labels.org": "bar",
					"metadata.labels.argocd.argoproj.io/secret-type": "cluster", "metadata.annotations.foo.argoproj.io": "production",
				},

				{
					"name": "staging-01", "nameNormalized": "staging-01", "server": "https://staging-01.example.com", "metadata.labels.environment": "staging", "metadata.labels.org": "foo",
					"metadata.labels.argocd.argoproj.io/secret-type": "cluster", "metadata.annotations.foo.argoproj.io": "staging",
				},
			},
			clientError:   false,
			expectedError: nil,
		},
		{
			name: "production-only",
			selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"environment": "production",
				},
			},
			values: map[string]string{
				"foo": "bar",
			},
			expected: []map[string]interface{}{
				{
					"values.foo": "bar", "name": "production_01/west", "nameNormalized": "production-01-west", "server": "https://production-01.example.com", "metadata.labels.environment": "production", "metadata.labels.org": "bar",
					"metadata.labels.argocd.argoproj.io/secret-type": "cluster", "metadata.annotations.foo.argoproj.io": "production",
				},
			},
			clientError:   false,
			expectedError: nil,
		},
		{
			name: "production or staging",
			selector: metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "environment",
						Operator: "In",
						Values: []string{
							"production",
							"staging",
						},
					},
				},
			},
			values: map[string]string{
				"foo": "bar",
			},
			expected: []map[string]interface{}{
				{
					"values.foo": "bar", "name": "staging-01", "nameNormalized": "staging-01", "server": "https://staging-01.example.com", "metadata.labels.environment": "staging", "metadata.labels.org": "foo",
					"metadata.labels.argocd.argoproj.io/secret-type": "cluster", "metadata.annotations.foo.argoproj.io": "staging",
				},
				{
					"values.foo": "bar", "name": "production_01/west", "nameNormalized": "production-01-west", "server": "https://production-01.example.com", "metadata.labels.environment": "production", "metadata.labels.org": "bar",
					"metadata.labels.argocd.argoproj.io/secret-type": "cluster", "metadata.annotations.foo.argoproj.io": "production",
				},
			},
			clientError:   false,
			expectedError: nil,
		},
		{
			name: "production or staging with match labels",
			selector: metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "environment",
						Operator: "In",
						Values: []string{
							"production",
							"staging",
						},
					},
				},
				MatchLabels: map[string]string{
					"org": "foo",
				},
			},
			values: map[string]string{
				"name": "baz",
			},
			expected: []map[string]interface{}{
				{
					"values.name": "baz", "name": "staging-01", "nameNormalized": "staging-01", "server": "https://staging-01.example.com", "metadata.labels.environment": "staging", "metadata.labels.org": "foo",
					"metadata.labels.argocd.argoproj.io/secret-type": "cluster", "metadata.annotations.foo.argoproj.io": "staging",
				},
			},
			clientError:   false,
			expectedError: nil,
		},
		{
			name:          "simulate client error",
			selector:      metav1.LabelSelector{},
			values:        nil,
			expected:      nil,
			clientError:   true,
			expectedError: fmt.Errorf("error getting cluster secrets: could not list Secrets"),
		},
	}

	// convert []client.Object to []runtime.Object, for use by kubefake package
	runtimeClusters := []runtime.Object{}
	for _, clientCluster := range clusters {
		runtimeClusters = append(runtimeClusters, clientCluster)
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			appClientset := kubefake.NewSimpleClientset(runtimeClusters...)

			fakeClient := fake.NewClientBuilder().WithObjects(clusters...).Build()
			cl := &possiblyErroringFakeCtrlRuntimeClient{
				fakeClient,
				testCase.clientError,
			}

			clusterGenerator := NewClusterGenerator(cl, context.Background(), appClientset, "namespace")

			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{},
			}

			got, err := clusterGenerator.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{
				Clusters: &argoprojiov1alpha1.ClusterGenerator{
					Selector: testCase.selector,
					Values:   testCase.values,
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

func TestGenerateParamsGoTemplate(t *testing.T) {
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
				"name":   []byte("production_01/west"),
				"server": []byte("https://production-01.example.com"),
			},
			Type: corev1.SecretType("Opaque"),
		},
	}
	testCases := []struct {
		name     string
		selector metav1.LabelSelector
		values   map[string]string
		expected []map[string]interface{}
		// clientError is true if a k8s client error should be simulated
		clientError   bool
		expectedError error
	}{
		{
			name:     "no label selector",
			selector: metav1.LabelSelector{},
			values: map[string]string{
				"lol1":  "lol",
				"lol2":  "{{ .values.lol1 }}{{ .values.lol1 }}",
				"lol3":  "{{ .values.lol2 }}{{ .values.lol2 }}{{ .values.lol2 }}",
				"foo":   "bar",
				"bar":   "{{ if not (empty .metadata) }}{{index .metadata.annotations \"foo.argoproj.io\" }}{{ end }}",
				"bat":   "{{ if not (empty .metadata) }}{{.metadata.labels.environment}}{{ end }}",
				"aaa":   "{{ .server }}",
				"no-op": "{{ .thisDoesNotExist }}",
			}, expected: []map[string]interface{}{
				{
					"name":           "production_01/west",
					"nameNormalized": "production-01-west",
					"server":         "https://production-01.example.com",
					"metadata": map[string]interface{}{
						"labels": map[string]string{
							"argocd.argoproj.io/secret-type": "cluster",
							"environment":                    "production",
							"org":                            "bar",
						},
						"annotations": map[string]string{
							"foo.argoproj.io": "production",
						},
					},
					"values": map[string]string{
						"lol1":  "lol",
						"lol2":  "<no value><no value>",
						"lol3":  "<no value><no value><no value>",
						"foo":   "bar",
						"bar":   "production",
						"bat":   "production",
						"aaa":   "https://production-01.example.com",
						"no-op": "<no value>",
					},
				},
				{
					"name":           "staging-01",
					"nameNormalized": "staging-01",
					"server":         "https://staging-01.example.com",
					"metadata": map[string]interface{}{
						"labels": map[string]string{
							"argocd.argoproj.io/secret-type": "cluster",
							"environment":                    "staging",
							"org":                            "foo",
						},
						"annotations": map[string]string{
							"foo.argoproj.io": "staging",
						},
					},
					"values": map[string]string{
						"lol1":  "lol",
						"lol2":  "<no value><no value>",
						"lol3":  "<no value><no value><no value>",
						"foo":   "bar",
						"bar":   "staging",
						"bat":   "staging",
						"aaa":   "https://staging-01.example.com",
						"no-op": "<no value>",
					},
				},
				{
					"nameNormalized": "in-cluster",
					"name":           "in-cluster",
					"server":         "https://kubernetes.default.svc",
					"values": map[string]string{
						"lol1":  "lol",
						"lol2":  "<no value><no value>",
						"lol3":  "<no value><no value><no value>",
						"foo":   "bar",
						"bar":   "",
						"bat":   "",
						"aaa":   "https://kubernetes.default.svc",
						"no-op": "<no value>",
					},
				},
			},
			clientError:   false,
			expectedError: nil,
		},
		{
			name: "secret type label selector",
			selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"argocd.argoproj.io/secret-type": "cluster",
				},
			},
			values: nil,
			expected: []map[string]interface{}{
				{
					"name":           "production_01/west",
					"nameNormalized": "production-01-west",
					"server":         "https://production-01.example.com",
					"metadata": map[string]interface{}{
						"labels": map[string]string{
							"argocd.argoproj.io/secret-type": "cluster",
							"environment":                    "production",
							"org":                            "bar",
						},
						"annotations": map[string]string{
							"foo.argoproj.io": "production",
						},
					},
				},
				{
					"name":           "staging-01",
					"nameNormalized": "staging-01",
					"server":         "https://staging-01.example.com",
					"metadata": map[string]interface{}{
						"labels": map[string]string{
							"argocd.argoproj.io/secret-type": "cluster",
							"environment":                    "staging",
							"org":                            "foo",
						},
						"annotations": map[string]string{
							"foo.argoproj.io": "staging",
						},
					},
				},
			},
			clientError:   false,
			expectedError: nil,
		},
		{
			name: "production-only",
			selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"environment": "production",
				},
			},
			values: map[string]string{
				"foo": "bar",
			},
			expected: []map[string]interface{}{
				{
					"name":           "production_01/west",
					"nameNormalized": "production-01-west",
					"server":         "https://production-01.example.com",
					"metadata": map[string]interface{}{
						"labels": map[string]string{
							"argocd.argoproj.io/secret-type": "cluster",
							"environment":                    "production",
							"org":                            "bar",
						},
						"annotations": map[string]string{
							"foo.argoproj.io": "production",
						},
					},
					"values": map[string]string{
						"foo": "bar",
					},
				},
			},
			clientError:   false,
			expectedError: nil,
		},
		{
			name: "production or staging",
			selector: metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "environment",
						Operator: "In",
						Values: []string{
							"production",
							"staging",
						},
					},
				},
			},
			values: map[string]string{
				"foo": "bar",
			},
			expected: []map[string]interface{}{
				{
					"name":           "production_01/west",
					"nameNormalized": "production-01-west",
					"server":         "https://production-01.example.com",
					"metadata": map[string]interface{}{
						"labels": map[string]string{
							"argocd.argoproj.io/secret-type": "cluster",
							"environment":                    "production",
							"org":                            "bar",
						},
						"annotations": map[string]string{
							"foo.argoproj.io": "production",
						},
					},
					"values": map[string]string{
						"foo": "bar",
					},
				},
				{
					"name":           "staging-01",
					"nameNormalized": "staging-01",
					"server":         "https://staging-01.example.com",
					"metadata": map[string]interface{}{
						"labels": map[string]string{
							"argocd.argoproj.io/secret-type": "cluster",
							"environment":                    "staging",
							"org":                            "foo",
						},
						"annotations": map[string]string{
							"foo.argoproj.io": "staging",
						},
					},
					"values": map[string]string{
						"foo": "bar",
					},
				},
			},
			clientError:   false,
			expectedError: nil,
		},
		{
			name: "production or staging with match labels",
			selector: metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "environment",
						Operator: "In",
						Values: []string{
							"production",
							"staging",
						},
					},
				},
				MatchLabels: map[string]string{
					"org": "foo",
				},
			},
			values: map[string]string{
				"name": "baz",
			},
			expected: []map[string]interface{}{
				{
					"name":           "staging-01",
					"nameNormalized": "staging-01",
					"server":         "https://staging-01.example.com",
					"metadata": map[string]interface{}{
						"labels": map[string]string{
							"argocd.argoproj.io/secret-type": "cluster",
							"environment":                    "staging",
							"org":                            "foo",
						},
						"annotations": map[string]string{
							"foo.argoproj.io": "staging",
						},
					},
					"values": map[string]string{
						"name": "baz",
					},
				},
			},
			clientError:   false,
			expectedError: nil,
		},
		{
			name:          "simulate client error",
			selector:      metav1.LabelSelector{},
			values:        nil,
			expected:      nil,
			clientError:   true,
			expectedError: fmt.Errorf("error getting cluster secrets: could not list Secrets"),
		},
	}

	// convert []client.Object to []runtime.Object, for use by kubefake package
	runtimeClusters := []runtime.Object{}
	for _, clientCluster := range clusters {
		runtimeClusters = append(runtimeClusters, clientCluster)
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			appClientset := kubefake.NewSimpleClientset(runtimeClusters...)

			fakeClient := fake.NewClientBuilder().WithObjects(clusters...).Build()
			cl := &possiblyErroringFakeCtrlRuntimeClient{
				fakeClient,
				testCase.clientError,
			}

			clusterGenerator := NewClusterGenerator(cl, context.Background(), appClientset, "namespace")

			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					GoTemplate: true,
				},
			}

			got, err := clusterGenerator.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{
				Clusters: &argoprojiov1alpha1.ClusterGenerator{
					Selector: testCase.selector,
					Values:   testCase.values,
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

func TestSanitizeClusterName(t *testing.T) {
	t.Run("valid DNS-1123 subdomain name", func(t *testing.T) {
		assert.Equal(t, "cluster-name", utils.SanitizeName("cluster-name"))
	})
	t.Run("invalid DNS-1123 subdomain name", func(t *testing.T) {
		invalidName := "-.--CLUSTER/name  -./.-"
		assert.Equal(t, "cluster-name", utils.SanitizeName(invalidName))
	})
}
