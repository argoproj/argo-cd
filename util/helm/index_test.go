package helm

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var index = Index{
	Entries: map[string]Entries{
		"argo-cd": {
			{Version: "~0.7.3"},
			{Version: "0.7.2"},
			{Version: "0.7.1"},
			{Version: "0.5.4"},
			{Version: "0.5.3"},
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
		entries, err := index.GetEntries("argo-cd")
		require.NoError(t, err)
		assert.Len(t, entries, 9)
	})
}

func TestEntries_MaxVersion(t *testing.T) {
	entries, _ := index.GetEntries("argo-cd")
	t.Run("NotFound", func(t *testing.T) {
		constraints, _ := semver.NewConstraint("0.8.1")
		_, err := entries.MaxVersion(constraints)
		require.EqualError(t, err, "constraint not found in index")
	})
	t.Run("Exact", func(t *testing.T) {
		constraints, _ := semver.NewConstraint("0.5.3")
		version, err := entries.MaxVersion(constraints)
		require.NoError(t, err)
		assert.Equal(t, semver.MustParse("0.5.3"), version)
	})
	t.Run("Constraint", func(t *testing.T) {
		constraints, _ := semver.NewConstraint("> 0.5.3")
		version, err := entries.MaxVersion(constraints)
		require.NoError(t, err)
		assert.Equal(t, semver.MustParse("0.7.2"), version)
	})
	t.Run("Constraint", func(t *testing.T) {
		constraints, _ := semver.NewConstraint("> 0.0.0")
		version, err := entries.MaxVersion(constraints)
		require.NoError(t, err)
		assert.Equal(t, semver.MustParse("0.7.2"), version)
	})
	t.Run("Constraint", func(t *testing.T) {
		constraints, _ := semver.NewConstraint(">0.5.0,<0.7.0")
		version, err := entries.MaxVersion(constraints)
		require.NoError(t, err)
		assert.Equal(t, semver.MustParse("0.5.4"), version)
	})
	t.Run("Constraint", func(t *testing.T) {
		constraints, _ := semver.NewConstraint("0.7.*")
		version, err := entries.MaxVersion(constraints)
		require.NoError(t, err)
		assert.Equal(t, semver.MustParse("0.7.2"), version)
	})
	t.Run("Constraint", func(t *testing.T) {
		constraints, _ := semver.NewConstraint("*")
		version, err := entries.MaxVersion(constraints)
		require.NoError(t, err)
		assert.Equal(t, semver.MustParse("0.7.2"), version)
	})
}
