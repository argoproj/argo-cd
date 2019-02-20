package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExcludeResource(t *testing.T) {
	apiGroup := "foo.com"
	kind := "bar"
	cluster := "baz.com"

	// simple matches
	assert.True(t, ExcludedResource{ApiGroups: []string{"foo.com"}, Kinds: []string{"bar"}, Clusters: []string{"baz.com"}}.Match(apiGroup, kind, cluster))
	assert.True(t, ExcludedResource{ApiGroups: []string{"*.com"}, Kinds: []string{"bar"}, Clusters: []string{"baz.com"}}.Match(apiGroup, kind, cluster))
	assert.True(t, ExcludedResource{ApiGroups: []string{"foo.com"}, Kinds: []string{"*"}, Clusters: []string{"baz.com"}}.Match(apiGroup, kind, cluster))
	assert.True(t, ExcludedResource{ApiGroups: []string{"foo.com"}, Kinds: []string{"bar"}, Clusters: []string{"*.com"}}.Match(apiGroup, kind, cluster))
	// negative matches
	assert.False(t, ExcludedResource{ApiGroups: []string{}, Kinds: []string{"bar"}, Clusters: []string{"baz.com"}}.Match(apiGroup, kind, cluster))
	assert.False(t, ExcludedResource{ApiGroups: []string{"foo.com"}, Kinds: []string{""}, Clusters: []string{"baz.com"}}.Match(apiGroup, kind, cluster))
	assert.False(t, ExcludedResource{ApiGroups: []string{"foo.com"}, Kinds: []string{"bar"}, Clusters: []string{}}.Match(apiGroup, kind, cluster))
	// complex matches
	assert.True(t, ExcludedResource{ApiGroups: []string{"foo.com", "foo.com"}, Kinds: []string{"bar"}, Clusters: []string{"baz.com"}}.Match(apiGroup, kind, cluster))
	assert.True(t, ExcludedResource{ApiGroups: []string{"foo.com"}, Kinds: []string{"bar", "bar"}, Clusters: []string{"baz.com"}}.Match(apiGroup, kind, cluster))
	assert.True(t, ExcludedResource{ApiGroups: []string{"foo.com"}, Kinds: []string{"bar"}, Clusters: []string{"baz.com", "baz.com"}}.Match(apiGroup, kind, cluster))
	// rubbish patterns
	assert.False(t, ExcludedResource{ApiGroups: []string{"["}, Kinds: []string{""}, Clusters: []string{""}}.Match("", "", ""))
	assert.False(t, ExcludedResource{ApiGroups: []string{""}, Kinds: []string{"["}, Clusters: []string{""}}.Match("", "", ""))
	assert.False(t, ExcludedResource{ApiGroups: []string{""}, Kinds: []string{""}, Clusters: []string{"["}}.Match("", "", ""))
}

