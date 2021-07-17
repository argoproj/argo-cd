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

	assert.True(t, settings.IsExcludedResourceWithAnnotation("events.k8s.io", "", "", map[string]string{}))
	assert.True(t, settings.IsExcludedResourceWithAnnotation("metrics.k8s.io", "", "", map[string]string{}))
	assert.False(t, settings.IsExcludedResourceWithAnnotation("rubbish.io", "", "", map[string]string{}))
}

func TestResourceInclusions(t *testing.T) {
	filter := ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}}},
	}

	assert.True(t, filter.IsExcludedResource("non-whitelisted-resource", "", ""))
	assert.False(t, filter.IsExcludedResource("whitelisted-resource", "", ""))

	assert.True(t, filter.IsExcludedResourceWithAnnotation("non-whitelisted-resource", "", "", map[string]string{}))
	assert.False(t, filter.IsExcludedResourceWithAnnotation("whitelisted-resource", "", "", map[string]string{}))
}

func TestResourceInclusionsExclusionNonMutex(t *testing.T) {
	filter := ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}}},
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}, Kinds: []string{"blacklisted-kind"}}},
	}

	assert.True(t, filter.IsExcludedResource("whitelisted-resource", "blacklisted-kind", ""))
	assert.False(t, filter.IsExcludedResource("whitelisted-resource", "", ""))
	assert.False(t, filter.IsExcludedResource("whitelisted-resource", "non-blacklisted-kind", ""))

	assert.True(t, filter.IsExcludedResourceWithAnnotation("whitelisted-resource", "blacklisted-kind", "", map[string]string{}))
	assert.False(t, filter.IsExcludedResourceWithAnnotation("whitelisted-resource", "", "", map[string]string{}))
	assert.False(t, filter.IsExcludedResourceWithAnnotation("whitelisted-resource", "non-blacklisted-kind", "", map[string]string{}))

	filter = ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}, Kinds: []string{"whitelisted-kind"}}},
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}}},
	}

	assert.True(t, filter.IsExcludedResource("whitelisted-resource", "whitelisted-kind", ""))
	assert.True(t, filter.IsExcludedResource("whitelisted-resource", "", ""))
	assert.True(t, filter.IsExcludedResource("whitelisted-resource", "non-whitelisted-kind", ""))

	assert.True(t, filter.IsExcludedResourceWithAnnotation("whitelisted-resource", "whitelisted-kind", "", map[string]string{}))
	assert.True(t, filter.IsExcludedResourceWithAnnotation("whitelisted-resource", "", "", map[string]string{}))
	assert.True(t, filter.IsExcludedResourceWithAnnotation("whitelisted-resource", "non-whitelisted-kind", "", map[string]string{}))

	filter = ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"foo-bar"}, Kinds: []string{"whitelisted-kind"}}},
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}}},
	}

	assert.True(t, filter.IsExcludedResource("not-whitelisted-resource", "whitelisted-kind", ""))
	assert.True(t, filter.IsExcludedResource("not-whitelisted-resource", "", ""))

	assert.True(t, filter.IsExcludedResourceWithAnnotation("not-whitelisted-resource", "whitelisted-kind", "", map[string]string{}))
	assert.True(t, filter.IsExcludedResourceWithAnnotation("not-whitelisted-resource", "", "", map[string]string{}))
}

func TestResourceInclusionsExclusionMultiCluster(t *testing.T) {
	filter := ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}, Clusters: []string{"cluster-one"}}},
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}, Clusters: []string{"cluster-two"}}},
	}

	assert.False(t, filter.IsExcludedResource("whitelisted-resource", "", "cluster-one"))
	assert.True(t, filter.IsExcludedResource("whitelisted-resource", "", "cluster-two"))
	assert.False(t, filter.IsExcludedResource("whitelisted-resource", "", "cluster-three"))

	assert.False(t, filter.IsExcludedResourceWithAnnotation("whitelisted-resource", "", "cluster-one", map[string]string{}))
	assert.True(t, filter.IsExcludedResourceWithAnnotation("whitelisted-resource", "", "cluster-two", map[string]string{}))
	assert.False(t, filter.IsExcludedResourceWithAnnotation("whitelisted-resource", "", "cluster-three", map[string]string{}))
}

func TestResourceInclusionsExclusionAnnotation(t *testing.T) {
	filter := ResourcesFilter{
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}, Annotations: []map[string]string{{"whitelisted-annotation": "included"}}}},
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"whitelisted-resource"}, Annotations: []map[string]string{{"whitelisted-annotation": "excluded"}}}},
	}

	assert.False(t, filter.IsExcludedResourceWithAnnotation("whitelisted-resource", "", "", map[string]string{"whitelisted-annotation": "included"}))
	assert.True(t, filter.IsExcludedResourceWithAnnotation("whitelisted-resource", "", "", map[string]string{"whitelisted-annotation": "excluded"}))
}
