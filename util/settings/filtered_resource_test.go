package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExcludeResource(t *testing.T) {
	apiGroup := "foo.com"
	kind := "bar"
	cluster := "baz.com"

	// matches with missing values
	assert.True(t, FilteredResource{Kinds: []string{kind}, Clusters: []string{cluster}}.Match(apiGroup, kind, cluster))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Clusters: []string{cluster}}.Match(apiGroup, kind, cluster))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{"*"}}.Match(apiGroup, kind, cluster))

	// simple matches
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster}}.Match(apiGroup, kind, cluster))
	assert.True(t, FilteredResource{APIGroups: []string{"*.com"}, Kinds: []string{kind}, Clusters: []string{cluster}}.Match(apiGroup, kind, cluster))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{"*"}, Clusters: []string{cluster}}.Match(apiGroup, kind, cluster))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{"*.com"}}.Match(apiGroup, kind, cluster))

	// negative matches
	assert.False(t, FilteredResource{APIGroups: []string{""}, Kinds: []string{kind}, Clusters: []string{cluster}}.Match(apiGroup, kind, cluster))
	assert.False(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{""}, Clusters: []string{cluster}}.Match(apiGroup, kind, cluster))
	assert.False(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{""}}.Match(apiGroup, kind, cluster))

	// complex matches
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup, apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster}}.Match(apiGroup, kind, cluster))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind, kind}, Clusters: []string{cluster}}.Match(apiGroup, kind, cluster))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster, cluster}}.Match(apiGroup, kind, cluster))

	// rubbish patterns
	assert.False(t, FilteredResource{APIGroups: []string{"["}, Kinds: []string{""}, Clusters: []string{""}}.Match("", "", ""))
	assert.False(t, FilteredResource{APIGroups: []string{""}, Kinds: []string{"["}, Clusters: []string{""}}.Match("", "", ""))
	assert.False(t, FilteredResource{APIGroups: []string{""}, Kinds: []string{""}, Clusters: []string{"["}}.Match("", "", ""))
}
