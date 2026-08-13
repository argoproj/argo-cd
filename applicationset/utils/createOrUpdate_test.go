package utils

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
)

func Test_applyIgnoreDifferences(t *testing.T) {
	t.Parallel()

	appMeta := metav1.TypeMeta{
		APIVersion: v1alpha1.ApplicationSchemaGroupVersionKind.GroupVersion().String(),
		Kind:       v1alpha1.ApplicationSchemaGroupVersionKind.Kind,
	}
	testCases := []struct {
		name              string
		ignoreDifferences v1alpha1.ApplicationSetIgnoreDifferences
		foundApp          string
		generatedApp      string
		expectedApp       string
	}{
		{
			name: "empty ignoreDifferences",
			foundApp: `
spec: {}`,
			generatedApp: `
spec: {}`,
			expectedApp: `
spec: {}`,
		},
		{
			// For this use case: https://github.com/argoproj/argo-cd/issues/9101#issuecomment-1191138278
			name: "ignore target revision with jq",
			ignoreDifferences: v1alpha1.ApplicationSetIgnoreDifferences{
				{JQPathExpressions: []string{".spec.source.targetRevision"}},
			},
			foundApp: `
spec:
  source:
    targetRevision: foo`,
			generatedApp: `
spec:
  source:
    targetRevision: bar`,
			expectedApp: `
spec:
  source:
    targetRevision: foo`,
		},
		{
			// For this use case: https://github.com/argoproj/argo-cd/issues/9101#issuecomment-1103593714
			name: "ignore helm parameter with jq",
			ignoreDifferences: v1alpha1.ApplicationSetIgnoreDifferences{
				{JQPathExpressions: []string{`.spec.source.helm.parameters | select(.name == "image.tag")`}},
			},
			foundApp: `
spec:
  source:
    helm:
      parameters:
      - name: image.tag
        value: test
      - name: another
        value: value`,
			generatedApp: `
spec:
  source:
    helm:
      parameters:
      - name: image.tag
        value: v1.0.0
      - name: another
        value: value`,
			expectedApp: `
spec:
  source:
    helm:
      parameters:
      - name: image.tag
        value: test
      - name: another
        value: value`,
		},
		{
			// For this use case: https://github.com/argoproj/argo-cd/issues/9101#issuecomment-1191138278
			name: "ignore auto-sync in appset when it's not in the cluster with jq",
			ignoreDifferences: v1alpha1.ApplicationSetIgnoreDifferences{
				{JQPathExpressions: []string{".spec.syncPolicy.automated"}},
			},
			foundApp: `
spec:
  syncPolicy:
    retry:
      limit: 5`,
			generatedApp: `
spec:
  syncPolicy:
    automated:
      selfHeal: true
    retry:
      limit: 5`,
			expectedApp: `
spec:
  syncPolicy:
    retry:
      limit: 5`,
		},
		{
			name: "ignore auto-sync in the cluster when it's not in the appset with jq",
			ignoreDifferences: v1alpha1.ApplicationSetIgnoreDifferences{
				{JQPathExpressions: []string{".spec.syncPolicy.automated"}},
			},
			foundApp: `
spec:
  syncPolicy:
    automated:
      selfHeal: true
    retry:
      limit: 5`,
			generatedApp: `
spec:
  syncPolicy:
    retry:
      limit: 5`,
			expectedApp: `
spec:
  syncPolicy:
    automated:
      selfHeal: true
    retry:
      limit: 5`,
		},
		{
			// For this use case: https://github.com/argoproj/argo-cd/issues/9101#issuecomment-1420656537
			name: "ignore a one-off annotation with jq",
			ignoreDifferences: v1alpha1.ApplicationSetIgnoreDifferences{
				{JQPathExpressions: []string{`.metadata.annotations | select(.["foo.bar"] == "baz")`}},
			},
			foundApp: `
metadata:
  annotations:
    foo.bar: baz
    some.other: annotation`,
			generatedApp: `
metadata:
  annotations:
    some.other: annotation`,
			expectedApp: `
metadata:
  annotations:
    foo.bar: baz
    some.other: annotation`,
		},
		{
			// For this use case: https://github.com/argoproj/argo-cd/issues/9101#issuecomment-1515672638
			name: "ignore the source.plugin field with a json pointer",
			ignoreDifferences: v1alpha1.ApplicationSetIgnoreDifferences{
				{JSONPointers: []string{"/spec/source/plugin"}},
			},
			foundApp: `
spec:
  source:
    plugin:
      parameters:
      - name: url
        string: https://example.com`,
			generatedApp: `
spec:
  source:
    plugin:
      parameters:
      - name: url
        string: https://example.com/wrong`,
			expectedApp: `
spec:
  source:
    plugin:
      parameters:
      - name: url
        string: https://example.com`,
		},
		{
			// For this use case: https://github.com/argoproj/argo-cd/pull/14743#issuecomment-1761954799
			name: "ignore parameters added to a multi-source app in the cluster",
			ignoreDifferences: v1alpha1.ApplicationSetIgnoreDifferences{
				{JQPathExpressions: []string{`.spec.sources[] | select(.repoURL | contains("test-repo")).helm.parameters`}},
			},
			foundApp: `
spec:
  sources:
  - repoURL: https://git.example.com/test-org/test-repo
    helm:
      parameters:
      - name: test
        value: hi`,
			generatedApp: `
spec:
  sources:
  - repoURL: https://git.example.com/test-org/test-repo`,
			expectedApp: `
spec:
  sources:
  - repoURL: https://git.example.com/test-org/test-repo
    helm:
      parameters:
      - name: test
        value: hi`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			foundApp := v1alpha1.Application{TypeMeta: appMeta}
			err := yaml.Unmarshal([]byte(tc.foundApp), &foundApp)
			require.NoError(t, err, tc.foundApp)
			generatedApp := v1alpha1.Application{TypeMeta: appMeta}
			err = yaml.Unmarshal([]byte(tc.generatedApp), &generatedApp)
			require.NoError(t, err, tc.generatedApp)
			diffConfig, err := BuildIgnoreDiffConfig(tc.ignoreDifferences, normalizers.IgnoreNormalizerOpts{})
			require.NoError(t, err)
			err = applyIgnoreDifferences(diffConfig, &foundApp, &generatedApp)
			require.NoError(t, err)
			yamlFound, err := yaml.Marshal(tc.foundApp)
			require.NoError(t, err)
			yamlExpected, err := yaml.Marshal(tc.expectedApp)
			require.NoError(t, err)
			assert.YAMLEq(t, string(yamlExpected), string(yamlFound))
		})
	}
}

func Test_helmSourcesHaveNullValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		app      *v1alpha1.Application
		expected bool
	}{
		{
			name: "no helm source",
			app: &v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{
					Source: &v1alpha1.ApplicationSource{},
				},
			},
			expected: false,
		},
		{
			name: "helm source without valuesObject",
			app: &v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{
					Source: &v1alpha1.ApplicationSource{
						Helm: &v1alpha1.ApplicationSourceHelm{},
					},
				},
			},
			expected: false,
		},
		{
			name: "helm source with valuesObject without nulls",
			app: &v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{
					Source: &v1alpha1.ApplicationSource{
						Helm: &v1alpha1.ApplicationSourceHelm{
							ValuesObject: &runtime.RawExtension{
								Raw: []byte(`{"memory":"64Mi"}`),
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "helm source with valuesObject with top-level null",
			app: &v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{
					Source: &v1alpha1.ApplicationSource{
						Helm: &v1alpha1.ApplicationSourceHelm{
							ValuesObject: &runtime.RawExtension{
								Raw: []byte(`{"cpu":null,"memory":"64Mi"}`),
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "helm source with valuesObject with nested null",
			app: &v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{
					Source: &v1alpha1.ApplicationSource{
						Helm: &v1alpha1.ApplicationSourceHelm{
							ValuesObject: &runtime.RawExtension{
								Raw: []byte(`{"recommender":{"resources":{"limits":{"cpu":null,"memory":"64Mi"}}}}`),
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "multi-source with null in one source",
			app: &v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{
					Sources: v1alpha1.ApplicationSources{
						{
							Helm: &v1alpha1.ApplicationSourceHelm{
								ValuesObject: &runtime.RawExtension{
									Raw: []byte(`{"key":"value"}`),
								},
							},
						},
						{
							Helm: &v1alpha1.ApplicationSourceHelm{
								ValuesObject: &runtime.RawExtension{
									Raw: []byte(`{"key":null}`),
								},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "string containing null word is not a false positive",
			app: &v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{
					Source: &v1alpha1.ApplicationSource{
						Helm: &v1alpha1.ApplicationSourceHelm{
							ValuesObject: &runtime.RawExtension{
								Raw: []byte(`{"message":"this is not null at all"}`),
							},
						},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := helmSourcesHaveNullValues(tt.app)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_jsonContainsNull(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		json     string
		expected bool
	}{
		{"empty object", `{}`, false},
		{"no nulls", `{"a":"b"}`, false},
		{"top-level null", `{"a":null}`, true},
		{"nested null", `{"a":{"b":null}}`, true},
		{"null in array", `{"a":[null]}`, true},
		{"deeply nested null", `{"a":{"b":{"c":{"d":null}}}}`, true},
		{"string with null word", `{"a":"null"}`, false},
		{"number value", `{"a":0}`, false},
		{"boolean false", `{"a":false}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := jsonContainsNull([]byte(tt.json))
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCreateOrUpdate_IgnoreDifferencesWithNullValuesObject exercises the interaction
// between ignoreDifferences and the null-values fallback path. applyIgnoreDifferences
// mutates the desired object in place to strip ignored fields; if that mutated object
// is then sent to a full Update (the fallback used when valuesObject contains nulls),
// ignored fields are cleared on the live resource and ignoreDifferences semantics are
// violated. CreateOrUpdate must preserve a pre-ignore copy of the desired object and
// use that copy on the Update path.
func TestCreateOrUpdate_IgnoreDifferencesWithNullValuesObject(t *testing.T) {
	t.Parallel()

	appMeta := metav1.TypeMeta{
		APIVersion: v1alpha1.ApplicationSchemaGroupVersionKind.GroupVersion().String(),
		Kind:       v1alpha1.ApplicationSchemaGroupVersionKind.Kind,
	}

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	// Both the live and the generated app carry a value for an ignored label. The
	// generated valuesObject contains a null entry, forcing the Update fallback. Without
	// the fix, applyIgnoreDifferences strips the label from the desired object in
	// place; the subsequent Update would then write a label-less object and clear the
	// label on the live resource. With the fix, the pre-ignore copy of the desired
	// object is used for the Update so the label is written through.
	live := &v1alpha1.Application{
		TypeMeta: appMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "argocd",
			Labels: map[string]string{
				"team": "platform-live",
			},
		},
		Spec: v1alpha1.ApplicationSpec{
			Source: &v1alpha1.ApplicationSource{
				RepoURL: "https://example.com/chart",
				Helm: &v1alpha1.ApplicationSourceHelm{
					ValuesObject: &runtime.RawExtension{
						Raw: []byte(`{"replicas":1}`),
					},
				},
			},
		},
	}

	desired := &v1alpha1.Application{
		TypeMeta: appMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "argocd",
			Labels: map[string]string{
				"team": "platform-desired",
			},
		},
		Spec: v1alpha1.ApplicationSpec{
			Source: &v1alpha1.ApplicationSource{
				RepoURL: "https://example.com/chart",
				Helm: &v1alpha1.ApplicationSourceHelm{
					ValuesObject: &runtime.RawExtension{
						Raw: []byte(`{"replicas":2,"resources":{"limits":{"cpu":null}}}`),
					},
				},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(live).Build()

	ignoreDifferences := v1alpha1.ApplicationSetIgnoreDifferences{
		{JSONPointers: []string{"/metadata/labels/team"}},
	}
	diffConfig, err := BuildIgnoreDiffConfig(ignoreDifferences, normalizers.IgnoreNormalizerOpts{})
	require.NoError(t, err)

	target := &v1alpha1.Application{
		TypeMeta:   appMeta,
		ObjectMeta: metav1.ObjectMeta{Name: "test-app", Namespace: "argocd"},
	}
	logCtx := log.NewEntry(log.New())
	result, err := CreateOrUpdate(t.Context(), logCtx, c, diffConfig, target, func() error {
		// c.Get clears TypeMeta on the fake client. Restore it so the diff/normalize
		// machinery in applyIgnoreDifferences resolves the IgnoreDifferences rule for
		// this Kind/Group; otherwise the rule silently no-ops and the test would not
		// actually exercise the in-place mutation that this fix protects against.
		target.TypeMeta = appMeta
		target.Labels = map[string]string{}
		for k, v := range desired.Labels {
			target.Labels[k] = v
		}
		desired.Spec.DeepCopyInto(&target.Spec)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "updated", string(result))

	// Read the live object back. The label that ignoreDifferences ignores must NOT
	// have been cleared by the Update fallback (it should hold the desired value), and
	// the null entry in valuesObject must survive on the live resource.
	got := &v1alpha1.Application{}
	require.NoError(t, c.Get(t.Context(), types.NamespacedName{Name: "test-app", Namespace: "argocd"}, got))

	assert.NotEmpty(t, got.Labels["team"],
		"ignored label must not be cleared by the null-values Update fallback")
	require.NotNil(t, got.Spec.Source)
	require.NotNil(t, got.Spec.Source.Helm)
	require.NotNil(t, got.Spec.Source.Helm.ValuesObject)
	assert.Contains(t, string(got.Spec.Source.Helm.ValuesObject.Raw), "null",
		"null entry in valuesObject must be preserved on the live resource")
}
