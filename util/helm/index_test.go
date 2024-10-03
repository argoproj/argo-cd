package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var index = Index{
	Entries: map[string]Entries{
		"argo-cd": {
			Tags: []string{
				"~0.7.3",
				"0.7.1",
				"0.5.4",
				"0.5.3",
				"0.7.2",
				"0.5.2",
				"~0.5.2",
				"0.5.1",
				"0.5.0",
			}},
	},
}

func TestIndex_GetEntries(t *testing.T) {
	t.Run("NotFound", func(t *testing.T) {
		_, err := index.GetTags("foo")
		require.EqualError(t, err, "chart 'foo' not found in index")
	})
	t.Run("Found", func(t *testing.T) {
		ts, err := index.GetTags("argo-cd")
		require.NoError(t, err)
		assert.Len(t, ts, len(index.Entries["argo-cd"].Tags))
	})
}
