package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsExcludedResource(t *testing.T) {
	t.Parallel()
	settings := &ResourcesFilter{}
	assert.True(t, settings.IsExcludedResource("events.k8s.io", "", ""))
	assert.True(t, settings.IsExcludedResource("metrics.k8s.io", "", ""))
	assert.False(t, settings.IsExcludedResource("rubbish.io", "", ""))
}

func TestResourceInclusions(t *testing.T) {
	t.Parallel()
	filter := ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}}},
	}

	assert.True(t, filter.IsExcludedResource("non-whitelisted-resource", "", ""))
	assert.False(t, filter.IsExcludedResource("whitelisted-resource", "", ""))
}

func TestResourceInclusionsExclusionNonMutex(t *testing.T) {
	t.Parallel()
	filter := ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}}},
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}, Kinds: []string{"blacklisted-kind"}}},
	}

	assert.True(t, filter.IsExcludedResource("whitelisted-resource", "blacklisted-kind", ""))
	assert.False(t, filter.IsExcludedResource("whitelisted-resource", "", ""))
	assert.False(t, filter.IsExcludedResource("whitelisted-resource", "non-blacklisted-kind", ""))

	filter = ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}, Kinds: []string{"whitelisted-kind"}}},
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}}},
	}

	assert.True(t, filter.IsExcludedResource("whitelisted-resource", "whitelisted-kind", ""))
	assert.True(t, filter.IsExcludedResource("whitelisted-resource", "", ""))
	assert.True(t, filter.IsExcludedResource("whitelisted-resource", "non-whitelisted-kind", ""))

	filter = ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"foo-bar"}, Kinds: []string{"whitelisted-kind"}}},
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}}},
	}

	assert.True(t, filter.IsExcludedResource("not-whitelisted-resource", "whitelisted-kind", ""))
	assert.True(t, filter.IsExcludedResource("not-whitelisted-resource", "", ""))
}

func TestResourceInclusionsExclusionMultiCluster(t *testing.T) {
	t.Parallel()
	filter := ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}, Clusters: []string{"cluster-one"}}},
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}, Clusters: []string{"cluster-two"}}},
	}

	assert.False(t, filter.IsExcludedResource("whitelisted-resource", "", "cluster-one"))
	assert.True(t, filter.IsExcludedResource("whitelisted-resource", "", "cluster-two"))
	assert.False(t, filter.IsExcludedResource("whitelisted-resource", "", "cluster-three"))
}

func TestGetLabelSelector(t *testing.T) {
	t.Run("no selectors configured", func(t *testing.T) {
		filter := &ResourcesFilter{}
		assert.Empty(t, filter.GetLabelSelector("", "Pod", "cluster-one"))
	})

	t.Run("selector of the matching rule is returned", func(t *testing.T) {
		filter := &ResourcesFilter{
			ResourceSelectors: []FilteredResource{
				{Kinds: []string{"Pod"}, Clusters: []string{"cluster-one"}, Selector: "!foo"},
			},
		}
		assert.Equal(t, "!foo", filter.GetLabelSelector("", "Pod", "cluster-one"))
		assert.Empty(t, filter.GetLabelSelector("", "Service", "cluster-one"))
		assert.Empty(t, filter.GetLabelSelector("", "Pod", "cluster-two"))
	})

	t.Run("rule without group, kind and cluster matches everything", func(t *testing.T) {
		filter := &ResourcesFilter{
			ResourceSelectors: []FilteredResource{{Selector: "foo=bar"}},
		}
		assert.Equal(t, "foo=bar", filter.GetLabelSelector("apps", "Deployment", "cluster-one"))
	})

	t.Run("selectors of matching rules are ANDed", func(t *testing.T) {
		filter := &ResourcesFilter{
			ResourceSelectors: []FilteredResource{
				{Selector: "foo=bar"},
				{Kinds: []string{"Pod"}, Selector: "!baz"},
				{Kinds: []string{"Service"}, Selector: "ignored=true"},
			},
		}
		assert.Equal(t, "foo=bar,!baz", filter.GetLabelSelector("", "Pod", "cluster-one"))
	})

	t.Run("rule without a selector is skipped", func(t *testing.T) {
		filter := &ResourcesFilter{
			ResourceSelectors: []FilteredResource{{Kinds: []string{"Pod"}}},
		}
		assert.Empty(t, filter.GetLabelSelector("", "Pod", "cluster-one"))
	})
}
