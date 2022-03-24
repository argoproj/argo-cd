package generators

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"testing"

	kubefake "k8s.io/client-go/kubernetes/fake"

	argoappsetv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"

	"github.com/stretchr/testify/assert"
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
		expected []map[string]string
		// clientError is true if a k8s client error should be simulated
		clientError   bool
		expectedError error
	}{
		{
			name:     "no label selector",
			selector: metav1.LabelSelector{},
			values:   nil,
			expected: []map[string]string{
				{"name": "production_01/west", "nameNormalized": "production-01-west", "server": "https://production-01.example.com", "metadata.labels.environment": "production", "metadata.labels.org": "bar",
					"metadata.labels.argocd.argoproj.io/secret-type": "cluster", "metadata.annotations.foo.argoproj.io": "production"},

				{"name": "staging-01", "nameNormalized": "staging-01", "server": "https://staging-01.example.com", "metadata.labels.environment": "staging", "metadata.labels.org": "foo",
					"metadata.labels.argocd.argoproj.io/secret-type": "cluster", "metadata.annotations.foo.argoproj.io": "staging"},

				{"name": "in-cluster", "server": "https://kubernetes.default.svc"},
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
			expected: []map[string]string{
				{"name": "production_01/west", "nameNormalized": "production-01-west", "server": "https://production-01.example.com", "metadata.labels.environment": "production", "metadata.labels.org": "bar",
					"metadata.labels.argocd.argoproj.io/secret-type": "cluster", "metadata.annotations.foo.argoproj.io": "production"},

				{"name": "staging-01", "nameNormalized": "staging-01", "server": "https://staging-01.example.com", "metadata.labels.environment": "staging", "metadata.labels.org": "foo",
					"metadata.labels.argocd.argoproj.io/secret-type": "cluster", "metadata.annotations.foo.argoproj.io": "staging"},
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
			expected: []map[string]string{
				{"values.foo": "bar", "name": "production_01/west", "nameNormalized": "production-01-west", "server": "https://production-01.example.com", "metadata.labels.environment": "production", "metadata.labels.org": "bar",
					"metadata.labels.argocd.argoproj.io/secret-type": "cluster", "metadata.annotations.foo.argoproj.io": "production"},
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
			expected: []map[string]string{
				{"values.foo": "bar", "name": "staging-01", "nameNormalized": "staging-01", "server": "https://staging-01.example.com", "metadata.labels.environment": "staging", "metadata.labels.org": "foo",
					"metadata.labels.argocd.argoproj.io/secret-type": "cluster", "metadata.annotations.foo.argoproj.io": "staging"},
				{"values.foo": "bar", "name": "production_01/west", "nameNormalized": "production-01-west", "server": "https://production-01.example.com", "metadata.labels.environment": "production", "metadata.labels.org": "bar",
					"metadata.labels.argocd.argoproj.io/secret-type": "cluster", "metadata.annotations.foo.argoproj.io": "production"},
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
			expected: []map[string]string{
				{"values.name": "baz", "name": "staging-01", "nameNormalized": "staging-01", "server": "https://staging-01.example.com", "metadata.labels.environment": "staging", "metadata.labels.org": "foo",
					"metadata.labels.argocd.argoproj.io/secret-type": "cluster", "metadata.annotations.foo.argoproj.io": "staging"},
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
			expectedError: fmt.Errorf("could not list Secrets"),
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

			var clusterGenerator = NewClusterGenerator(cl, context.Background(), appClientset, "namespace")

			got, err := clusterGenerator.GenerateParams(&argoappsetv1alpha1.ApplicationSetGenerator{
				Clusters: &argoappsetv1alpha1.ClusterGenerator{
					Selector: testCase.selector,
					Values:   testCase.values,
				},
			}, nil)

			if testCase.expectedError != nil {
				assert.EqualError(t, err, testCase.expectedError.Error())
			} else {
				assert.NoError(t, err)
				assert.ElementsMatch(t, testCase.expected, got)
			}

		})
	}
}

func TestSanitizeClusterName(t *testing.T) {
	t.Run("valid DNS-1123 subdomain name", func(t *testing.T) {
		assert.Equal(t, "cluster-name", sanitizeName("cluster-name"))
	})
	t.Run("invalid DNS-1123 subdomain name", func(t *testing.T) {
		invalidName := "-.--CLUSTER/name  -./.-"
		assert.Equal(t, "cluster-name", sanitizeName(invalidName))
	})
}
