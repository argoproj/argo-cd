package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExcludeResource(t *testing.T) {
	apiGroup := "foo.com"
	kind := "bar"
	cluster := "baz.com"
	label := map[string]string{"qux": "excluded"}

	// matches with missing values
	assert.True(t, FilteredResource{Kinds: []string{kind}, Clusters: []string{cluster}, Labels: []map[string]string{label}}.Match(apiGroup, kind, cluster, label))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Clusters: []string{cluster}, Labels: []map[string]string{label}}.Match(apiGroup, kind, cluster, label))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{"*"}, Labels: []map[string]string{label}}.Match(apiGroup, kind, cluster, label))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster}}.Match(apiGroup, kind, cluster, label))

	// simple matches
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster}, Labels: []map[string]string{label}}.Match(apiGroup, kind, cluster, label))
	assert.True(t, FilteredResource{APIGroups: []string{"*.com"}, Kinds: []string{kind}, Clusters: []string{cluster}, Labels: []map[string]string{label}}.Match(apiGroup, kind, cluster, label))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{"*"}, Clusters: []string{cluster}, Labels: []map[string]string{label}}.Match(apiGroup, kind, cluster, label))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{"*.com"}, Labels: []map[string]string{label}}.Match(apiGroup, kind, cluster, label))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster}, Labels: []map[string]string{map[string]string{"qux": "*"}}}.Match(apiGroup, kind, cluster, label))

	// negative matches
	assert.False(t, FilteredResource{APIGroups: []string{""}, Kinds: []string{kind}, Clusters: []string{cluster}, Labels: []map[string]string{label}}.Match(apiGroup, kind, cluster, label))
	assert.False(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{""}, Clusters: []string{cluster}, Labels: []map[string]string{label}}.Match(apiGroup, kind, cluster, label))
	assert.False(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{""}, Labels: []map[string]string{label}}.Match(apiGroup, kind, cluster, label))
	assert.False(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster}, Labels: []map[string]string{map[string]string{"qux": ""}}}.Match(apiGroup, kind, cluster, label))

	// complex matches
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup, apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster}, Labels: []map[string]string{label}}.Match(apiGroup, kind, cluster, label))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind, kind}, Clusters: []string{cluster}, Labels: []map[string]string{label}}.Match(apiGroup, kind, cluster, label))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster, cluster}, Labels: []map[string]string{label}}.Match(apiGroup, kind, cluster, label))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster, cluster}, Labels: []map[string]string{label, label}}.Match(apiGroup, kind, cluster, label))

	// rubbish patterns
	assert.False(t, FilteredResource{APIGroups: []string{"["}, Kinds: []string{""}, Clusters: []string{""}, Labels: []map[string]string{map[string]string{"qux": ""}}}.Match("", "", "", map[string]string{}))
	assert.False(t, FilteredResource{APIGroups: []string{""}, Kinds: []string{"["}, Clusters: []string{""}, Labels: []map[string]string{map[string]string{"qux": ""}}}.Match("", "", "", map[string]string{}))
	assert.False(t, FilteredResource{APIGroups: []string{""}, Kinds: []string{""}, Clusters: []string{"["}, Labels: []map[string]string{map[string]string{"qux": ""}}}.Match("", "", "", map[string]string{}))
	assert.False(t, FilteredResource{APIGroups: []string{""}, Kinds: []string{""}, Clusters: []string{""}, Labels: []map[string]string{map[string]string{"qux": "["}}}.Match("", "", "", map[string]string{}))
}
