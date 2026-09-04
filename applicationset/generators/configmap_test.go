package generators

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestConfigMapGenerateParams(t *testing.T) {
	testCases := []struct {
		name          string
		configMap     *corev1.ConfigMap
		generator     *argoprojiov1alpha1.ConfigMapGenerator
		gotemplate    bool
		expected      []map[string]any
		expectedError string
	}{
		{
			name: "exposes ConfigMap data as parameters",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "env-config", Namespace: "argocd"},
				Data:       map[string]string{"cluster": "prod", "region": "us-east-1"},
			},
			generator: &argoprojiov1alpha1.ConfigMapGenerator{ConfigMapRef: "env-config"},
			expected: []map[string]any{
				{"cluster": "prod", "region": "us-east-1"},
			},
		},
		{
			name: "appends values alongside ConfigMap data (fasttemplate)",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "env-config", Namespace: "argocd"},
				Data:       map[string]string{"cluster": "prod"},
			},
			generator: &argoprojiov1alpha1.ConfigMapGenerator{
				ConfigMapRef: "env-config",
				Values:       map[string]string{"environment": "production"},
			},
			expected: []map[string]any{
				{"cluster": "prod", "values.environment": "production"},
			},
		},
		{
			name: "appends values alongside ConfigMap data (goTemplate)",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "env-config", Namespace: "argocd"},
				Data:       map[string]string{"cluster": "prod"},
			},
			generator: &argoprojiov1alpha1.ConfigMapGenerator{
				ConfigMapRef: "env-config",
				Values:       map[string]string{"environment": "{{.cluster}}"},
			},
			gotemplate: true,
			expected: []map[string]any{
				{"cluster": "prod", "values": map[string]string{"environment": "prod"}},
			},
		},
		{
			name: "empty ConfigMap data yields a single empty parameter set",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "env-config", Namespace: "argocd"},
			},
			generator: &argoprojiov1alpha1.ConfigMapGenerator{ConfigMapRef: "env-config"},
			expected:  []map[string]any{{}},
		},
		{
			name: "missing configMapRef is an error",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "env-config", Namespace: "argocd"},
			},
			generator:     &argoprojiov1alpha1.ConfigMapGenerator{},
			expectedError: "ConfigMap generator requires configMapRef to be set",
		},
		{
			name: "referenced ConfigMap does not exist is an error",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "env-config", Namespace: "argocd"},
			},
			generator:     &argoprojiov1alpha1.ConfigMapGenerator{ConfigMapRef: "does-not-exist"},
			expectedError: "error fetching ConfigMap argocd/does-not-exist",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithObjects([]client.Object{tc.configMap}...).Build()

			g := NewConfigMapGenerator(fakeClient, "argocd")

			appSet := &argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{Name: "set"},
				Spec:       argoprojiov1alpha1.ApplicationSetSpec{GoTemplate: tc.gotemplate},
			}

			got, err := g.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{ConfigMap: tc.generator}, appSet, nil)

			if tc.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, got)
			}
		})
	}
}

func TestConfigMapGenerateParamsNilGuards(t *testing.T) {
	g := NewConfigMapGenerator(fake.NewClientBuilder().Build(), "argocd")

	_, err := g.GenerateParams(nil, &argoprojiov1alpha1.ApplicationSet{}, nil)
	require.ErrorIs(t, err, ErrEmptyAppSetGenerator)

	_, err = g.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{}, &argoprojiov1alpha1.ApplicationSet{}, nil)
	require.ErrorIs(t, err, ErrEmptyAppSetGenerator)
}

func TestConfigMapGetTemplate(t *testing.T) {
	g := NewConfigMapGenerator(fake.NewClientBuilder().Build(), "argocd")
	tmpl := argoprojiov1alpha1.ApplicationSetTemplate{
		ApplicationSetTemplateMeta: argoprojiov1alpha1.ApplicationSetTemplateMeta{Name: "{{.cluster}}"},
	}
	got := g.GetTemplate(&argoprojiov1alpha1.ApplicationSetGenerator{
		ConfigMap: &argoprojiov1alpha1.ConfigMapGenerator{Template: tmpl},
	})
	assert.Equal(t, "{{.cluster}}", got.Name)
}
