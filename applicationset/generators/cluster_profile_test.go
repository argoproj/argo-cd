package generators

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	v1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	clusterinventory "sigs.k8s.io/cluster-inventory-api/apis/v1alpha1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterProfileGenerateParams(t *testing.T) {
	clusterProfiles := []client.Object{
		&clusterinventory.ClusterProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "staging-01",
				Namespace: "namespace",
				Labels: map[string]string{
					"environment": "staging",
					"org":         "foo",
				},
				Annotations: map[string]string{
					"foo.argoproj.io": "staging",
				},
			},
			Status: clusterinventory.ClusterProfileStatus{
				CredentialProviders: []clusterinventory.CredentialProvider{
					{
						Name: "kubeconfig",
						Cluster: v1.Cluster{
							Server: "https://staging-01.example.com",
						},
					},
				},
			},
		},
		&clusterinventory.ClusterProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "production-01",
				Namespace: "namespace",
				Labels: map[string]string{
					"environment": "production",
					"org":         "bar",
				},
				Annotations: map[string]string{
					"foo.argoproj.io": "production",
				},
			},
			Status: clusterinventory.ClusterProfileStatus{
				CredentialProviders: []clusterinventory.CredentialProvider{
					{
						Name: "kubeconfig",
						Cluster: v1.Cluster{
							Server: "https://production-01.example.com",
						},
					},
				},
			},
		},
	}

	testCases := []struct {
		name          string
		selector      metav1.LabelSelector
		isFlatMode    bool
		values        map[string]string
		expected      []map[string]any
		clientError   bool
		expectedError error
	}{
		{
			name:     "no label selector",
			selector: metav1.LabelSelector{},
			values: map[string]string{
				"foo": "bar",
			},
			expected: []map[string]any{
				{
					"values.foo": "bar", "name": "production-01", "nameNormalized": "production-01", "server": "https://production-01.example.com", "metadata.labels.environment": "production", "metadata.labels.org": "bar",
					"metadata.annotations.foo.argoproj.io": "production", "project": "",
				},
				{
					"values.foo": "bar", "name": "staging-01", "nameNormalized": "staging-01", "server": "https://staging-01.example.com", "metadata.labels.environment": "staging", "metadata.labels.org": "foo",
					"metadata.annotations.foo.argoproj.io": "staging", "project": "",
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
			expected: []map[string]any{
				{
					"values.foo": "bar", "name": "production-01", "nameNormalized": "production-01", "server": "https://production-01.example.com", "metadata.labels.environment": "production", "metadata.labels.org": "bar",
					"metadata.annotations.foo.argoproj.io": "production", "project": "",
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
			expected: []map[string]any{
				{
					"values.foo": "bar", "name": "staging-01", "nameNormalized": "staging-01", "server": "https://staging-01.example.com", "metadata.labels.environment": "staging", "metadata.labels.org": "foo",
					"metadata.annotations.foo.argoproj.io": "staging", "project": "",
				},
				{
					"values.foo": "bar", "name": "production-01", "nameNormalized": "production-01", "server": "https://production-01.example.com", "metadata.labels.environment": "production", "metadata.labels.org": "bar",
					"metadata.annotations.foo.argoproj.io": "production", "project": "",
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
			expectedError: errors.New("error listing cluster profiles: client error"),
		},
		{
			name:     "flat mode without selectors",
			selector: metav1.LabelSelector{},
			values: map[string]string{
				"foo": "bar",
			},
			expected: []map[string]any{
				{
					"clusters": []map[string]any{
						{
							"values.foo": "bar", "name": "production-01", "nameNormalized": "production-01", "server": "https://production-01.example.com", "metadata.labels.environment": "production", "metadata.labels.org": "bar",
							"metadata.annotations.foo.argoproj.io": "production", "project": "",
						},
						{
							"values.foo": "bar", "name": "staging-01", "nameNormalized": "staging-01", "server": "https://staging-01.example.com", "metadata.labels.environment": "staging", "metadata.labels.org": "foo",
							"metadata.annotations.foo.argoproj.io": "staging", "project": "",
						},
					},
				},
			},
			isFlatMode:    true,
			clientError:   false,
			expectedError: nil,
		},
		{
			name: "production or staging with flat mode",
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
			isFlatMode: true,
			values: map[string]string{
				"foo": "bar",
			},
			expected: []map[string]any{
				{
					"clusters": []map[string]any{
						{
							"values.foo": "bar", "name": "production-01", "nameNormalized": "production-01", "server": "https://production-01.example.com", "metadata.labels.environment": "production", "metadata.labels.org": "bar",
							"metadata.annotations.foo.argoproj.io": "production", "project": "",
						},
						{
							"values.foo": "bar", "name": "staging-01", "nameNormalized": "staging-01", "server": "https://staging-01.example.com", "metadata.labels.environment": "staging", "metadata.labels.org": "foo",
							"metadata.annotations.foo.argoproj.io": "staging", "project": "",
						},
					},
				},
			},
			clientError:   false,
			expectedError: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {

			s := runtime.NewScheme()
			s.AddKnownTypes(clusterinventory.GroupVersion, &clusterinventory.ClusterProfile{}, &clusterinventory.ClusterProfileList{})
			fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(clusterProfiles...).Build()
			cl := &possiblyErroringFakeCtrlRuntimeClient{
				fakeClient,
				testCase.clientError,
			}

			clusterProfileGenerator := NewClusterProfileGenerator(context.Background(), cl, "namespace")

			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{},
			}

			got, err := clusterProfileGenerator.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{
				ClusterProfiles: &argoprojiov1alpha1.ClusterProfileGenerator{
					Selector: testCase.selector,
					Values:   testCase.values,
					FlatList: testCase.isFlatMode,
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

func TestClusterProfileGenerateParamsGoTemplate(t *testing.T) {
	clusterProfiles := []client.Object{
		&clusterinventory.ClusterProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "staging-01",
				Namespace: "namespace",
				Labels: map[string]string{
					"environment": "staging",
					"org":         "foo",
				},
				Annotations: map[string]string{
					"foo.argoproj.io": "staging",
				},
			},
			Status: clusterinventory.ClusterProfileStatus{
				CredentialProviders: []clusterinventory.CredentialProvider{
					{
						Name: "kubeconfig",
						Cluster: v1.Cluster{
							Server: "https://staging-01.example.com",
						},
					},
				},
			},
		},
		&clusterinventory.ClusterProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "production-01",
				Namespace: "namespace",
				Labels: map[string]string{
					"environment": "production",
					"org":         "bar",
				},
				Annotations: map[string]string{
					"foo.argoproj.io": "production",
				},
			},
			Status: clusterinventory.ClusterProfileStatus{
				CredentialProviders: []clusterinventory.CredentialProvider{
					{
						Name: "kubeconfig",
						Cluster: v1.Cluster{
							Server: "https://production-01.example.com",
						},
					},
				},
			},
		},
	}
	testCases := []struct {
		name          string
		selector      metav1.LabelSelector
		values        map[string]string
		isFlatMode    bool
		expected      []map[string]any
		clientError   bool
		expectedError error
	}{
		{
			name:     "no label selector",
			selector: metav1.LabelSelector{},
			values: map[string]string{
				"foo":   "bar",
				"bar":   "{{ if not (empty .metadata) }}{{index .metadata.annotations \"foo.argoproj.io\" }}{{ end }}",
				"bat":   "{{ if not (empty .metadata) }}{{.metadata.labels.environment}}{{ end }}",
				"aaa":   "{{ .server }}",
				"no-op": "{{ .thisDoesNotExist }}",
			}, expected: []map[string]any{
				{
					"name":           "production-01",
					"nameNormalized": "production-01",
					"server":         "https://production-01.example.com",
					"project":        "",
					"metadata": map[string]any{
						"labels": map[string]string{
							"environment": "production",
							"org":         "bar",
						},
						"annotations": map[string]string{
							"foo.argoproj.io": "production",
						},
					},
					"values": map[string]string{
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
					"project":        "",
					"metadata": map[string]any{
						"labels": map[string]string{
							"environment": "staging",
							"org":         "foo",
						},
						"annotations": map[string]string{
							"foo.argoproj.io": "staging",
						},
					},
					"values": map[string]string{
						"foo":   "bar",
						"bar":   "staging",
						"bat":   "staging",
						"aaa":   "https://staging-01.example.com",
						"no-op": "<no value>",
					},
				},
			},
			clientError:   false,
			expectedError: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			s := runtime.NewScheme()
			s.AddKnownTypes(clusterinventory.GroupVersion, &clusterinventory.ClusterProfile{}, &clusterinventory.ClusterProfileList{})
			fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(clusterProfiles...).Build()
			cl := &possiblyErroringFakeCtrlRuntimeClient{
				fakeClient,
				testCase.clientError,
			}

			clusterProfileGenerator := NewClusterProfileGenerator(context.Background(), cl, "namespace")

			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					GoTemplate: true,
				},
			}

			got, err := clusterProfileGenerator.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{
				ClusterProfiles: &argoprojiov1alpha1.ClusterProfileGenerator{
					Selector: testCase.selector,
					Values:   testCase.values,
					FlatList: testCase.isFlatMode,
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
