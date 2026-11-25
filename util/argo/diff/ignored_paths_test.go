package diff_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test"
	"github.com/argoproj/argo-cd/v3/util/argo/diff"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
	"github.com/argoproj/argo-cd/v3/util/argo/testdata"
)

func TestIgnoredPathsResolver(t *testing.T) {
	t.Parallel()
	newResolver := func(t *testing.T, targets []*unstructured.Unstructured, ignores ...v1alpha1.ResourceIgnoreDifferences) *diff.IgnoredPathsResolver {
		t.Helper()
		dc, err := diff.NewDiffConfigBuilder().
			WithDiffSettings(ignores, nil, true, normalizers.IgnoreNormalizerOpts{}).
			WithNoCache().
			Build()
		require.NoError(t, err)
		r, err := diff.NewIgnoredPathsResolver(targets, dc)
		require.NoError(t, err)
		return r
	}

	t.Run("resolves jsonPointers and jqPathExpressions on both sides", func(t *testing.T) {
		t.Parallel()
		live := test.YamlToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml)
		target := test.YamlToUnstructured(testdata.DesiredDeploymentYaml)
		r := newResolver(t, []*unstructured.Unstructured{target}, v1alpha1.ResourceIgnoreDifferences{
			Group:             "apps",
			Kind:              "Deployment",
			JSONPointers:      []string{"/spec/replicas", "/spec/not/there"},
			JQPathExpressions: []string{`.spec.template.spec.containers[] | select(.name == "guestbook-ui") | .image`},
		})

		livePaths, targetPaths, err := r.IgnoredPaths(live, target)

		require.NoError(t, err)
		assert.ElementsMatch(t, [][]any{
			{"spec", "replicas"},
			{"spec", "template", "spec", "containers", 0, "image"},
		}, livePaths)
		assert.ElementsMatch(t, [][]any{
			{"spec", "replicas"},
			{"spec", "template", "spec", "containers", 0, "image"},
		}, targetPaths)
	})

	t.Run("resolves the annotation on the target", func(t *testing.T) {
		t.Parallel()
		live := test.YamlToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml)
		target := test.YamlToUnstructured(testdata.DesiredDeploymentAnnotationYaml)
		r := newResolver(t, []*unstructured.Unstructured{target})

		livePaths, targetPaths, err := r.IgnoredPaths(live, target)

		require.NoError(t, err)
		assert.Contains(t, livePaths, []any{"spec", "replicas"})
		assert.Contains(t, targetPaths, []any{"spec", "replicas"})
	})

	t.Run("resolves fields owned by managedFieldsManagers", func(t *testing.T) {
		t.Parallel()
		live := test.YamlToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml)
		target := test.YamlToUnstructured(testdata.DesiredDeploymentYaml)
		r := newResolver(t, []*unstructured.Unstructured{target}, v1alpha1.ResourceIgnoreDifferences{
			Group:                 "*",
			Kind:                  "*",
			ManagedFieldsManagers: []string{"revision-history-manager"},
		})

		livePaths, targetPaths, err := r.IgnoredPaths(live, target)

		require.NoError(t, err)
		assert.Equal(t, [][]any{{"spec", "revisionHistoryLimit"}}, livePaths)
		assert.Equal(t, [][]any{{"spec", "revisionHistoryLimit"}}, targetPaths)
	})

	t.Run("rules for other kinds do not contribute", func(t *testing.T) {
		t.Parallel()
		live := test.YamlToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml)
		target := test.YamlToUnstructured(testdata.DesiredDeploymentYaml)
		r := newResolver(t, []*unstructured.Unstructured{target}, v1alpha1.ResourceIgnoreDifferences{
			Kind:         "Service",
			JSONPointers: []string{"/spec"},
		})

		livePaths, targetPaths, err := r.IgnoredPaths(live, target)

		require.NoError(t, err)
		assert.Empty(t, livePaths)
		assert.Empty(t, targetPaths)
	})

	t.Run("nil objects yield no paths", func(t *testing.T) {
		t.Parallel()
		target := test.YamlToUnstructured(testdata.DesiredDeploymentYaml)
		r := newResolver(t, []*unstructured.Unstructured{target}, v1alpha1.ResourceIgnoreDifferences{
			Group: "apps", Kind: "Deployment", JSONPointers: []string{"/spec/replicas"},
		})

		livePaths, targetPaths, err := r.IgnoredPaths(nil, target)

		require.NoError(t, err)
		assert.Empty(t, livePaths)
		assert.Empty(t, targetPaths)
	})

	t.Run("nil diff config is rejected", func(t *testing.T) {
		t.Parallel()
		_, err := diff.NewIgnoredPathsResolver(nil, nil)
		require.Error(t, err)
	})
}
