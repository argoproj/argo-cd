package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExcludeResource(t *testing.T) {
	apiGroup := "foo.com"
	kind := "bar"
	cluster := "baz.com"
	namespace := "qux"

	// matches with missing values
	assert.True(t, FilteredResource{Kinds: []string{kind}, Clusters: []string{cluster}, Namespaces: []string{namespace}}.Match(apiGroup, kind, cluster, namespace))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Clusters: []string{cluster}, Namespaces: []string{namespace}}.Match(apiGroup, kind, cluster, namespace))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Namespaces: []string{namespace}}.Match(apiGroup, kind, cluster, namespace))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster}}.Match(apiGroup, kind, cluster, namespace))

	// simple matches
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster}, Namespaces: []string{namespace}}.Match(apiGroup, kind, cluster, namespace))
	assert.True(t, FilteredResource{APIGroups: []string{"*.com"}, Kinds: []string{kind}, Clusters: []string{cluster}, Namespaces: []string{namespace}}.Match(apiGroup, kind, cluster, namespace))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{"*"}, Clusters: []string{cluster}, Namespaces: []string{namespace}}.Match(apiGroup, kind, cluster, namespace))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{"*.com"}, Namespaces: []string{namespace}}.Match(apiGroup, kind, cluster, namespace))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster}, Namespaces: []string{"*"}}.Match(apiGroup, kind, cluster, namespace))

	// negative matches
	assert.False(t, FilteredResource{APIGroups: []string{""}, Kinds: []string{kind}, Clusters: []string{cluster}, Namespaces: []string{namespace}}.Match(apiGroup, kind, cluster, namespace))
	assert.False(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{""}, Clusters: []string{cluster}, Namespaces: []string{namespace}}.Match(apiGroup, kind, cluster, namespace))
	assert.False(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{""}, Namespaces: []string{namespace}}.Match(apiGroup, kind, cluster, namespace))
	assert.False(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster}, Namespaces: []string{""}}.Match(apiGroup, kind, cluster, namespace))

	// complex matches
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup, apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster}, Namespaces: []string{namespace}}.Match(apiGroup, kind, cluster, namespace))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind, kind}, Clusters: []string{cluster}, Namespaces: []string{namespace}}.Match(apiGroup, kind, cluster, namespace))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster, cluster}, Namespaces: []string{namespace}}.Match(apiGroup, kind, cluster, namespace))
	assert.True(t, FilteredResource{APIGroups: []string{apiGroup}, Kinds: []string{kind}, Clusters: []string{cluster}, Namespaces: []string{namespace, namespace}}.Match(apiGroup, kind, cluster, namespace))

	// rubbish patterns
	assert.False(t, FilteredResource{APIGroups: []string{"["}, Kinds: []string{""}, Clusters: []string{""}, Namespaces: []string{""}}.Match("", "", "", ""))
	assert.False(t, FilteredResource{APIGroups: []string{""}, Kinds: []string{"["}, Clusters: []string{""}, Namespaces: []string{""}}.Match("", "", "", ""))
	assert.False(t, FilteredResource{APIGroups: []string{""}, Kinds: []string{""}, Clusters: []string{"["}, Namespaces: []string{""}}.Match("", "", "", ""))
	assert.False(t, FilteredResource{APIGroups: []string{""}, Kinds: []string{""}, Clusters: []string{""}, Namespaces: []string{"["}}.Match("", "", "", ""))
}
