package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExcludeResource(t *testing.T) {
	apiGroup := "foo.com"
	kind := "bar"
	cluster := "baz.com"
	annotation := map[string]string{"qux": "excluded"}

	// matches with missing values
	assert.True(t, FilteredResource{Kinds: []string{kind}, Clusters: []string{cluster}, Annotations: []map[string]string{annotation}}.Match(apiGroup, kind, cluster, annotation))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Clusters: []string{cluster}, Annotations: []map[string]string{annotation}}.Match(apiGroup, kind, cluster, annotation))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{"*"}, Annotations: []map[string]string{annotation}}.Match(apiGroup, kind, cluster, annotation))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster}}.Match(apiGroup, kind, cluster, annotation))

	// simple matches
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster}, Annotations: []map[string]string{annotation}}.Match(apiGroup, kind, cluster, annotation))
	assert.True(t, FilteredResource{APIGroups: []string{"*.com"}, Kinds: []string{kind}, Clusters: []string{cluster}, Annotations: []map[string]string{annotation}}.Match(apiGroup, kind, cluster, annotation))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{"*"}, Clusters: []string{cluster}, Annotations: []map[string]string{annotation}}.Match(apiGroup, kind, cluster, annotation))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{"*.com"}, Annotations: []map[string]string{annotation}}.Match(apiGroup, kind, cluster, annotation))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster}, Annotations: []map[string]string{{"qux": "*"}}}.Match(apiGroup, kind, cluster, annotation))

	// negative matches
	assert.False(t, FilteredResource{APIGroups: []string{""}, Kinds: []string{kind}, Clusters: []string{cluster}, Annotations: []map[string]string{annotation}}.Match(apiGroup, kind, cluster, annotation))
	assert.False(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{""}, Clusters: []string{cluster}, Annotations: []map[string]string{annotation}}.Match(apiGroup, kind, cluster, annotation))
	assert.False(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{""}, Annotations: []map[string]string{annotation}}.Match(apiGroup, kind, cluster, annotation))
	assert.False(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{""}, Annotations: []map[string]string{{"qux": ""}}}.Match(apiGroup, kind, cluster, annotation))

	// complex matches
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup, apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster}, Annotations: []map[string]string{annotation}}.Match(apiGroup, kind, cluster, annotation))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind, kind}, Clusters: []string{cluster}, Annotations: []map[string]string{annotation}}.Match(apiGroup, kind, cluster, annotation))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster, cluster}, Annotations: []map[string]string{annotation}}.Match(apiGroup, kind, cluster, annotation))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster}, Annotations: []map[string]string{annotation, annotation}}.Match(apiGroup, kind, cluster, annotation))

	// rubbish patterns
	assert.False(t, FilteredResource{APIGroups: []string{"["}, Kinds: []string{""}, Clusters: []string{""}, Annotations: []map[string]string{{"qux": ""}}}.Match("", "", "", map[string]string{}))
	assert.False(t, FilteredResource{APIGroups: []string{""}, Kinds: []string{"["}, Clusters: []string{""}, Annotations: []map[string]string{{"qux": ""}}}.Match("", "", "", map[string]string{}))
	assert.False(t, FilteredResource{APIGroups: []string{""}, Kinds: []string{""}, Clusters: []string{"["}, Annotations: []map[string]string{{"qux": ""}}}.Match("", "", "", map[string]string{}))
	assert.False(t, FilteredResource{APIGroups: []string{""}, Kinds: []string{""}, Clusters: []string{""}, Annotations: []map[string]string{{"qux": "["}}}.Match("", "", "", map[string]string{}))
}
