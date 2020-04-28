package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsExcludedResource(t *testing.T) {
	settings := &ResourcesFilter{}
	assert.True(t, settings.IsExcludedResource("events.k8s.io", "", "", map[string]string{}))
	assert.True(t, settings.IsExcludedResource("metrics.k8s.io", "", "", map[string]string{}))
	assert.False(t, settings.IsExcludedResource("rubbish.io", "", "", map[string]string{}))
}

func TestResourceInclusions(t *testing.T) {
	filter := ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}}},
	}

	assert.True(t, filter.IsExcludedResource("non-whitelisted-resource", "", "", map[string]string{}))
	assert.False(t, filter.IsExcludedResource("whitelisted-resource", "", "", map[string]string{}))
}

func TestResourceInclusionsExclusionNonMutex(t *testing.T) {
	filter := ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}}},
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}, Kinds: []string{"blacklisted-kind"}}},
	}

	assert.True(t, filter.IsExcludedResource("whitelisted-resource", "blacklisted-kind", "", map[string]string{}))
	assert.False(t, filter.IsExcludedResource("whitelisted-resource", "", "", map[string]string{}))
	assert.False(t, filter.IsExcludedResource("whitelisted-resource", "non-blacklisted-kind", "", map[string]string{}))

	filter = ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}, Kinds: []string{"whitelisted-kind"}}},
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}}},
	}

	assert.True(t, filter.IsExcludedResource("whitelisted-resource", "whitelisted-kind", "", map[string]string{}))
	assert.True(t, filter.IsExcludedResource("whitelisted-resource", "", "", map[string]string{}))
	assert.True(t, filter.IsExcludedResource("whitelisted-resource", "non-whitelisted-kind", "", map[string]string{}))

	filter = ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"foo-bar"}, Kinds: []string{"whitelisted-kind"}}},
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}}},
	}

	assert.True(t, filter.IsExcludedResource("not-whitelisted-resource", "whitelisted-kind", "", map[string]string{}))
	assert.True(t, filter.IsExcludedResource("not-whitelisted-resource", "", "", map[string]string{}))

	filter = ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}, Kinds: []string{"whitelisted-kind"}}},
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}, Kinds: []string{"whitelisted-kind"}, Labels: []map[string]string{map[string]string{"blacklisted-label": "excluded"}}}},
	}

	assert.True(t, filter.IsExcludedResource("whitelisted-resource", "whitelisted-kind", "", map[string]string{"blacklisted-label": "excluded"}))
	assert.False(t, filter.IsExcludedResource("whitelisted-resource", "whitelisted-kind", "", map[string]string{}))
	assert.False(t, filter.IsExcludedResource("whitelisted-resource", "whitelisted-kind", "", map[string]string{"blacklisted-list": "included"}))
}
