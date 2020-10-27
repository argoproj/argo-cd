package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsExcludedResource(t *testing.T) {
	settings := &ResourcesFilter{}
	assert.True(t, settings.IsExcludedResource("events.k8s.io", "", ""))
	assert.True(t, settings.IsExcludedResource("metrics.k8s.io", "", ""))
	assert.False(t, settings.IsExcludedResource("rubbish.io", "", ""))
}

func TestResourceInclusions(t *testing.T) {
	filter := ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}}},
	}

	assert.True(t, filter.IsExcludedResource("non-whitelisted-resource", "", ""))
	assert.False(t, filter.IsExcludedResource("whitelisted-resource", "", ""))
}

func TestResourceInclusionsExclusionNonMutex(t *testing.T) {
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
	filter := ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}, Clusters: []string{"cluster-one"}}},
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}, Clusters: []string{"cluster-two"}}},
	}

	assert.False(t, filter.IsExcludedResource("whitelisted-resource", "", "cluster-one"))
	assert.True(t, filter.IsExcludedResource("whitelisted-resource", "", "cluster-two"))
	assert.False(t, filter.IsExcludedResource("whitelisted-resource", "", "cluster-three"))
}
