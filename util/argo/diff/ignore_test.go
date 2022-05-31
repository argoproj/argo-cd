package diff_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo/diff"
)

func TestIgnoreDiffConfig_HasIgnoreDifference(t *testing.T) {
	getOverride := func(gk string) map[string]v1alpha1.ResourceOverride {
		return map[string]v1alpha1.ResourceOverride{
			gk: {
				IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{
					ManagedFieldsManagers: []string{"manager1", "manager2"},
					JQPathExpressions:     []string{"some.jq.path.expr"},
					JSONPointers:          []string{"some.json.pointer"},
				},
			},
		}
	}
	getIgnoreDiff := func(group, kind, name, namespace string) v1alpha1.ResourceIgnoreDifferences {
		return v1alpha1.ResourceIgnoreDifferences{
			Group:                 group,
			Kind:                  kind,
			Name:                  name,
			Namespace:             namespace,
			JSONPointers:          []string{"ignore.diff.json.pointer"},
			JQPathExpressions:     []string{"ignore.diff.jq.path"},
			ManagedFieldsManagers: []string{"ignoreDiffManager1", "ignoreDiffManager2"},
		}
	}
	t.Run("will return ignore diffs from resource override", func(t *testing.T) {
		// given
		gk := "apps/Deployment"
		override := getOverride(gk)
		ignoreDiff := getIgnoreDiff("apps", "Deployment", "", "")
		ignoreDiffs := []v1alpha1.ResourceIgnoreDifferences{ignoreDiff}
		ignoreConfig := diff.NewIgnoreDiffConfig(ignoreDiffs, override)
		expectedManagedFields := append(override[gk].IgnoreDifferences.ManagedFieldsManagers, ignoreDiff.ManagedFieldsManagers...)
		expectedJSONPointers := append(override[gk].IgnoreDifferences.JSONPointers, ignoreDiff.JSONPointers...)
		expectedJQExpression := append(override[gk].IgnoreDifferences.JQPathExpressions, ignoreDiff.JQPathExpressions...)

		// when
		ok, actual := ignoreConfig.HasIgnoreDifference("apps", "Deployment", "app-name", "default")

		// then
		assert.True(t, ok)
		assert.NotNil(t, actual)
		assert.Equal(t, expectedManagedFields, actual.ManagedFieldsManagers)
		assert.Equal(t, expectedJSONPointers, actual.JSONPointers)
		assert.Equal(t, expectedJQExpression, actual.JQPathExpressions)
	})
	t.Run("will return ignore diffs from resource override with wildcard", func(t *testing.T) {
		// given
		gk := "*/*"
		override := getOverride(gk)
		ignoreDiff := getIgnoreDiff("apps", "Deployment", "", "")
		ignoreDiffs := []v1alpha1.ResourceIgnoreDifferences{ignoreDiff}
		ignoreConfig := diff.NewIgnoreDiffConfig(ignoreDiffs, override)
		expectedManagedFields := append(override[gk].IgnoreDifferences.ManagedFieldsManagers, ignoreDiff.ManagedFieldsManagers...)
		expectedJSONPointers := append(override[gk].IgnoreDifferences.JSONPointers, ignoreDiff.JSONPointers...)
		expectedJQExpression := append(override[gk].IgnoreDifferences.JQPathExpressions, ignoreDiff.JQPathExpressions...)

		// when
		ok, actual := ignoreConfig.HasIgnoreDifference("apps", "Deployment", "app-name", "default")

		// then
		assert.True(t, ok)
		assert.NotNil(t, actual)
		assert.Equal(t, expectedManagedFields, actual.ManagedFieldsManagers)
		assert.Equal(t, expectedJSONPointers, actual.JSONPointers)
		assert.Equal(t, expectedJQExpression, actual.JQPathExpressions)
	})
	t.Run("will return ignore diffs from application resource", func(t *testing.T) {
		// given
		ignoreDiff := getIgnoreDiff("apps", "Deployment", "app-name", "default")
		ignoreDiffs := []v1alpha1.ResourceIgnoreDifferences{ignoreDiff}
		ignoreConfig := diff.NewIgnoreDiffConfig(ignoreDiffs, nil)

		// when
		ok, actual := ignoreConfig.HasIgnoreDifference("apps", "Deployment", "app-name", "default")

		// then
		assert.True(t, ok)
		assert.NotNil(t, actual)
		assert.Equal(t, ignoreDiff.ManagedFieldsManagers, actual.ManagedFieldsManagers)
		assert.Equal(t, ignoreDiff.JSONPointers, actual.JSONPointers)
		assert.Equal(t, ignoreDiff.JQPathExpressions, actual.JQPathExpressions)
	})
	t.Run("will return ignore diffs from application resource with no app name and namespace configured", func(t *testing.T) {
		// given
		ignoreDiff := getIgnoreDiff("apps", "Deployment", "", "")
		ignoreDiffs := []v1alpha1.ResourceIgnoreDifferences{ignoreDiff}
		ignoreConfig := diff.NewIgnoreDiffConfig(ignoreDiffs, nil)

		// when
		ok, actual := ignoreConfig.HasIgnoreDifference("apps", "Deployment", "app-name", "default")

		// then
		assert.True(t, ok)
		assert.NotNil(t, actual)
		assert.Equal(t, ignoreDiff.ManagedFieldsManagers, actual.ManagedFieldsManagers)
		assert.Equal(t, ignoreDiff.JSONPointers, actual.JSONPointers)
		assert.Equal(t, ignoreDiff.JQPathExpressions, actual.JQPathExpressions)
	})
	t.Run("will return ignore diffs for all resources from group", func(t *testing.T) {
		// given
		ignoreDiff := getIgnoreDiff("apps", "*", "", "")
		ignoreDiffs := []v1alpha1.ResourceIgnoreDifferences{ignoreDiff}
		ignoreConfig := diff.NewIgnoreDiffConfig(ignoreDiffs, nil)

		// when
		ok, actual := ignoreConfig.HasIgnoreDifference("apps", "Deployment", "app-name", "default")

		// then
		assert.True(t, ok)
		require.NotNil(t, actual)
		assert.Equal(t, ignoreDiff.ManagedFieldsManagers, actual.ManagedFieldsManagers)
		assert.Equal(t, ignoreDiff.JSONPointers, actual.JSONPointers)
		assert.Equal(t, ignoreDiff.JQPathExpressions, actual.JQPathExpressions)
	})
	t.Run("will return ignore diffs for all resources", func(t *testing.T) {
		// given
		ignoreDiff := getIgnoreDiff("*", "*", "", "")
		ignoreDiffs := []v1alpha1.ResourceIgnoreDifferences{ignoreDiff}
		ignoreConfig := diff.NewIgnoreDiffConfig(ignoreDiffs, nil)

		// when
		ok, actual := ignoreConfig.HasIgnoreDifference("apps", "Deployment", "app-name", "default")

		// then
		assert.True(t, ok)
		require.NotNil(t, actual)
		assert.Equal(t, ignoreDiff.ManagedFieldsManagers, actual.ManagedFieldsManagers)
		assert.Equal(t, ignoreDiff.JSONPointers, actual.JSONPointers)
		assert.Equal(t, ignoreDiff.JQPathExpressions, actual.JQPathExpressions)
	})
	t.Run("no ignore diffs if namespace do not match", func(t *testing.T) {
		// given
		ignoreDiff := getIgnoreDiff("apps", "Deployment", "app-name", "default")
		ignoreDiffs := []v1alpha1.ResourceIgnoreDifferences{ignoreDiff}
		ignoreConfig := diff.NewIgnoreDiffConfig(ignoreDiffs, nil)

		// when
		ok, actual := ignoreConfig.HasIgnoreDifference("apps", "Deployment", "app-name", "another-namespace")

		// then
		assert.False(t, ok)
		require.Nil(t, actual)
	})
	t.Run("no ignore diffs if name do not match", func(t *testing.T) {
		// given
		ignoreDiff := getIgnoreDiff("apps", "Deployment", "app-name", "default")
		ignoreDiffs := []v1alpha1.ResourceIgnoreDifferences{ignoreDiff}
		ignoreConfig := diff.NewIgnoreDiffConfig(ignoreDiffs, nil)

		// when
		ok, actual := ignoreConfig.HasIgnoreDifference("apps", "Deployment", "another-app", "default")

		// then
		assert.False(t, ok)
		require.Nil(t, actual)
	})
	t.Run("no ignore diffs if resource do not match", func(t *testing.T) {
		// given
		ignoreDiff := getIgnoreDiff("apps", "Deployment", "app-name", "default")
		ignoreDiffs := []v1alpha1.ResourceIgnoreDifferences{ignoreDiff}
		ignoreConfig := diff.NewIgnoreDiffConfig(ignoreDiffs, nil)

		// when
		ok, actual := ignoreConfig.HasIgnoreDifference("apps", "Service", "app-name", "default")

		// then
		assert.False(t, ok)
		require.Nil(t, actual)
	})
	t.Run("no ignore diffs if group do not match", func(t *testing.T) {
		// given
		ignoreDiff := getIgnoreDiff("apps", "Deployment", "app-name", "default")
		ignoreDiffs := []v1alpha1.ResourceIgnoreDifferences{ignoreDiff}
		ignoreConfig := diff.NewIgnoreDiffConfig(ignoreDiffs, nil)

		// when
		ok, actual := ignoreConfig.HasIgnoreDifference("another-group", "Deployment", "app-name", "default")

		// then
		assert.False(t, ok)
		require.Nil(t, actual)
	})
	t.Run("will merge ignore differences correctly removing duplicated configs", func(t *testing.T) {
		// given
		gk := "*/*"
		override := getOverride(gk)
		ignoreDiff := getIgnoreDiff("*", "*", "", "")
		expectedManagers := append(ignoreDiff.ManagedFieldsManagers, "repeated-manager")
		expectedManagers = append(expectedManagers, override[gk].IgnoreDifferences.ManagedFieldsManagers...)
		expectedJSONPointers := append(ignoreDiff.JSONPointers, "repeated-jsonpointer")
		expectedJSONPointers = append(expectedJSONPointers, override[gk].IgnoreDifferences.JSONPointers...)
		expectedJQPath := append(ignoreDiff.JQPathExpressions, "repeated-jqpath")
		expectedJQPath = append(expectedJQPath, override[gk].IgnoreDifferences.JQPathExpressions...)
		ignoreDiff.ManagedFieldsManagers = append(ignoreDiff.ManagedFieldsManagers, []string{"repeated-manager", "repeated-manager"}...)
		ignoreDiff.JSONPointers = append(ignoreDiff.JSONPointers, []string{"repeated-jsonpointer", "repeated-jsonpointer"}...)
		ignoreDiff.JQPathExpressions = append(ignoreDiff.JQPathExpressions, []string{"repeated-jqpath", "repeated-jqpath"}...)
		ignoreDiffs := []v1alpha1.ResourceIgnoreDifferences{ignoreDiff}
		ignoreConfig := diff.NewIgnoreDiffConfig(ignoreDiffs, override)

		// when
		ok, actual := ignoreConfig.HasIgnoreDifference("apps", "Deployment", "app-name", "default")

		// then
		assert.True(t, ok)
		require.NotNil(t, actual)
		assert.ElementsMatch(t, expectedManagers, actual.ManagedFieldsManagers)
		assert.ElementsMatch(t, expectedJSONPointers, actual.JSONPointers)
		assert.ElementsMatch(t, expectedJQPath, actual.JQPathExpressions)
	})

}
