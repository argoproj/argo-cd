package normalizers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

const cnpgClusterManagedRolesYAML = `
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: example
spec:
  managed:
    roles:
      - connectionLimit: -1
        ensure: present
        inherit: true
        login: true
        name: dev_knowledge_graph
        passwordSecret:
          name: cnpg-dev-knowledge-graph-role
      - connectionLimit: 10
        ensure: absent
        inherit: false
        login: true
        name: custom_role
`

func mustUnstructured(t *testing.T, manifest string) *unstructured.Unstructured {
	t.Helper()
	obj := &unstructured.Unstructured{}
	require.NoError(t, yaml.Unmarshal([]byte(manifest), obj))
	return obj
}

func roleByName(t *testing.T, obj *unstructured.Unstructured, name string) map[string]any {
	t.Helper()
	roles, has, err := unstructured.NestedSlice(obj.Object, "spec", "managed", "roles")
	require.NoError(t, err)
	require.True(t, has)
	for _, r := range roles {
		role, ok := r.(map[string]any)
		require.True(t, ok)
		if role["name"] == name {
			return role
		}
	}
	t.Fatalf("role %q not found", name)
	return nil
}

// TestLoadBuiltinIgnoreDifferences verifies the embedded CNPG Cluster
// ignoreDifferences.yaml is discovered and parsed.
func TestLoadBuiltinIgnoreDifferences(t *testing.T) {
	builtins, err := loadBuiltinIgnoreDifferences()
	require.NoError(t, err)

	cnpg, ok := builtins["postgresql.cnpg.io/Cluster"]
	require.True(t, ok, "expected built-in ignoreDifferences for postgresql.cnpg.io/Cluster")
	assert.ElementsMatch(t, []string{
		`.spec.managed.roles[]? | select(.connectionLimit == -1) | .connectionLimit`,
		`.spec.managed.roles[]? | select(.inherit == true) | .inherit`,
		`.spec.managed.roles[]? | select(.ensure == "present") | .ensure`,
	}, cnpg.JQPathExpressions)
}

// TestBuiltinIgnoreDifferencesCNPGCluster verifies the normalizer strips
// API-server-defaulted fields from managed roles while preserving user-set
// values that differ from the defaults.
func TestBuiltinIgnoreDifferencesCNPGCluster(t *testing.T) {
	normalizer, err := NewIgnoreNormalizer(nil, map[string]v1alpha1.ResourceOverride{}, IgnoreNormalizerOpts{})
	require.NoError(t, err)

	obj := mustUnstructured(t, cnpgClusterManagedRolesYAML)
	require.NoError(t, normalizer.Normalize(obj))

	// Role with default values: connectionLimit/inherit/ensure should be stripped.
	defaulted := roleByName(t, obj, "dev_knowledge_graph")
	assert.NotContains(t, defaulted, "connectionLimit", "default connectionLimit should be stripped")
	assert.NotContains(t, defaulted, "inherit", "default inherit should be stripped")
	assert.NotContains(t, defaulted, "ensure", "default ensure should be stripped")
	assert.Equal(t, true, defaulted["login"])
	assert.Equal(t, "cnpg-dev-knowledge-graph-role", defaulted["passwordSecret"].(map[string]any)["name"])

	// Role with non-default values: nothing should be stripped.
	custom := roleByName(t, obj, "custom_role")
	assert.Equal(t, int64(10), custom["connectionLimit"], "user-set connectionLimit must be preserved")
	assert.Equal(t, false, custom["inherit"], "user-set inherit must be preserved")
	assert.Equal(t, "absent", custom["ensure"], "user-set ensure must be preserved")
}

// TestBuiltinIgnoreDifferencesConfigMapPrecedence verifies that a ConfigMap
// override for the same group/kind takes precedence over the built-in
// (replace semantics, consistent with built-in health.lua).
func TestBuiltinIgnoreDifferencesConfigMapPrecedence(t *testing.T) {
	overrides := map[string]v1alpha1.ResourceOverride{
		"postgresql.cnpg.io/Cluster": {
			IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{JSONPointers: []string{"/spec/example"}},
		},
	}

	builtinIgnores, err := getBuiltinResourceIgnoreDifferences(overrides)
	require.NoError(t, err)
	for _, item := range builtinIgnores {
		if item.Group == "postgresql.cnpg.io" && item.Kind == "Cluster" {
			t.Fatal("built-in CNPG ignoreDifferences should be skipped when ConfigMap overrides it")
		}
	}

	// A ConfigMap override without ignoreDifferences must not suppress the built-in.
	overridesWithoutIgnore := map[string]v1alpha1.ResourceOverride{
		"postgresql.cnpg.io/Cluster": {HealthLua: "foo"},
	}
	builtinIgnores, err = getBuiltinResourceIgnoreDifferences(overridesWithoutIgnore)
	require.NoError(t, err)
	found := false
	for _, item := range builtinIgnores {
		if item.Group == "postgresql.cnpg.io" && item.Kind == "Cluster" {
			found = true
		}
	}
	assert.True(t, found, "built-in CNPG ignoreDifferences should apply when ConfigMap has no ignoreDifferences for it")
}

// TestBuiltinIgnoreDifferencesWildcardMatch verifies that a wildcard built-in
// entry — the form produced by converting "_"-prefixed dirs to "*" (matching
// health.lua) — normalizes resources with multi-segment group names. The
// normalizer calls glob.Match without separators, so "*" spans ".".
func TestBuiltinIgnoreDifferencesWildcardMatch(t *testing.T) {
	normalizer, err := NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:        "*.crossplane.io",
		Kind:         "*",
		JSONPointers: []string{"/metadata/annotations/toStrip"},
	}}, map[string]v1alpha1.ResourceOverride{}, IgnoreNormalizerOpts{})
	require.NoError(t, err)

	obj := mustUnstructured(t, `
apiVersion: compute.aws.crossplane.io/v1beta1
kind: Foo
metadata:
  name: example
  annotations:
    toStrip: value
    keep: other
`)
	require.NoError(t, normalizer.Normalize(obj))

	_, hasStrip := obj.GetAnnotations()["toStrip"]
	assert.False(t, hasStrip, "wildcard group *.crossplane.io should match compute.aws.crossplane.io")
	assert.Contains(t, obj.GetAnnotations(), "keep")
}
