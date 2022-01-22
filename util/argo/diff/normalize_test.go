package diff_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test"
	"github.com/argoproj/argo-cd/v2/util/argo/diff"
	"github.com/argoproj/argo-cd/v2/util/argo/testdata"
)

func TestNormalize(t *testing.T) {
	type fixture struct {
		diffConfig diff.DiffConfig
		lives      []*unstructured.Unstructured
		targets    []*unstructured.Unstructured
	}
	setup := func(t *testing.T, ignores []v1alpha1.ResourceIgnoreDifferences) *fixture {
		t.Helper()
		dc, err := diff.NewDiffConfigBuilder().
			WithDiffSettings(ignores, nil, true).
			WithNoCache().
			Build()
		require.NoError(t, err)
		live := test.YamlToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml)
		target := test.YamlToUnstructured(testdata.DesiredDeploymentYaml)
		return &fixture{
			diffConfig: dc,
			lives:      []*unstructured.Unstructured{live},
			targets:    []*unstructured.Unstructured{target},
		}
	}
	t.Run("will normalize resources removing the fields owned by managers", func(t *testing.T) {
		// given
		ignore := v1alpha1.ResourceIgnoreDifferences{
			Group:                 "*",
			Kind:                  "*",
			ManagedFieldsManagers: []string{"revision-history-manager"},
		}
		ignores := []v1alpha1.ResourceIgnoreDifferences{ignore}
		f := setup(t, ignores)

		// when
		result, err := diff.Normalize(f.lives, f.targets, f.diffConfig)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(result.Targets))
		_, ok, err := unstructured.NestedFloat64(result.Targets[0].Object, "spec", "revisionHistoryLimit")
		require.NoError(t, err)
		require.False(t, ok)
		_, ok, err = unstructured.NestedFloat64(result.Lives[0].Object, "spec", "revisionHistoryLimit")
		require.NoError(t, err)
		require.False(t, ok)
	})
	t.Run("will correctly normalize with multiple ignore configurations", func(t *testing.T) {
		// given
		ignores := []v1alpha1.ResourceIgnoreDifferences{
			{
				Group:        "apps",
				Kind:         "Deployment",
				JSONPointers: []string{"/spec/replicas"},
			},
			{
				Group:                 "*",
				Kind:                  "*",
				ManagedFieldsManagers: []string{"revision-history-manager"},
			},
		}
		f := setup(t, ignores)

		// when
		normalized, err := diff.Normalize(f.lives, f.targets, f.diffConfig)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(normalized.Targets))
		_, ok, err := unstructured.NestedFloat64(normalized.Targets[0].Object, "spec", "revisionHistoryLimit")
		require.NoError(t, err)
		require.False(t, ok)
		_, ok, err = unstructured.NestedFloat64(normalized.Lives[0].Object, "spec", "revisionHistoryLimit")
		require.NoError(t, err)
		require.False(t, ok)
		_, ok, err = unstructured.NestedInt64(normalized.Targets[0].Object, "spec", "replicas")
		require.NoError(t, err)
		require.False(t, ok)
		_, ok, err = unstructured.NestedInt64(normalized.Lives[0].Object, "spec", "replicas")
		require.NoError(t, err)
		require.False(t, ok)
	})
	t.Run("will not modify resources if ignore difference is not configured", func(t *testing.T) {
		// given
		ignores := []v1alpha1.ResourceIgnoreDifferences{}
		f := setup(t, ignores)

		// when
		result, err := diff.Normalize(f.lives, f.targets, f.diffConfig)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(result.Targets))
		assert.Equal(t, f.lives[0], result.Lives[0])
		assert.Equal(t, f.targets[0], result.Targets[0])
	})
}
