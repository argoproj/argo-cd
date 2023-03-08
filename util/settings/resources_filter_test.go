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
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"allowlisted-resource"}}},
	}

	assert.True(t, filter.IsExcludedResource("non-allowlisted-resource", "", ""))
	assert.False(t, filter.IsExcludedResource("allowlisted-resource", "", ""))
}

func TestResourceInclusionsExclusionNonMutex(t *testing.T) {
	filter := ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"allowlisted-resource"}}},
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"allowlisted-resource"}, Kinds: []string{"denylisted-resource"}}},
	}

	assert.True(t, filter.IsExcludedResource("allowlisted-resource", "denylisted-resource", ""))
	assert.False(t, filter.IsExcludedResource("allowlisted-resource", "", ""))
	assert.False(t, filter.IsExcludedResource("allowlisted-resource", "non-denylisted-resource", ""))

	filter = ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"allowlisted-resource"}, Kinds: []string{"allowlisted-kind"}}},
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"allowlisted-resource"}}},
	}

	assert.True(t, filter.IsExcludedResource("allowlisted-resource", "allowlisted-kind", ""))
	assert.True(t, filter.IsExcludedResource("allowlisted-resource", "", ""))
	assert.True(t, filter.IsExcludedResource("allowlisted-resource", "non-allowlisted-kind", ""))

	filter = ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"foo-bar"}, Kinds: []string{"allowlisted-kind"}}},
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"allowlisted-resource"}}},
	}

	assert.True(t, filter.IsExcludedResource("not-allowlisted-resource", "allowlisted-kind", ""))
	assert.True(t, filter.IsExcludedResource("not-allowlisted-resource", "", ""))
}

func TestResourceInclusionsExclusionMultiCluster(t *testing.T) {
	filter := ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"allowlisted-resource"}, Clusters: []string{"cluster-one"}}},
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"allowlisted-resource"}, Clusters: []string{"cluster-two"}}},
	}

	assert.False(t, filter.IsExcludedResource("allowlisted-resource", "", "cluster-one"))
	assert.True(t, filter.IsExcludedResource("allowlisted-resource", "", "cluster-two"))
	assert.False(t, filter.IsExcludedResource("allowlisted-resource", "", "cluster-three"))
}
