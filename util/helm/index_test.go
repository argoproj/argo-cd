package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var index = Index{
	Entries: map[string]Entries{
		"argo-cd": {
			{Version: "~0.7.3"},
			{Version: "0.7.1"},
			{Version: "0.5.4"},
			{Version: "0.5.3"},
			{Version: "0.7.2"},
			{Version: "0.5.2"},
			{Version: "~0.5.2"},
			{Version: "0.5.1"},
			{Version: "0.5.0"},
		},
	},
}

func TestIndex_GetEntries(t *testing.T) {
	t.Run("NotFound", func(t *testing.T) {
		_, err := index.GetEntries("foo")
		require.EqualError(t, err, "chart 'foo' not found in index")
	})
	t.Run("Found", func(t *testing.T) {
		ts, err := index.GetEntries("argo-cd")
		require.NoError(t, err)
		assert.Len(t, ts, len(index.Entries["argo-cd"]))
	})
}
