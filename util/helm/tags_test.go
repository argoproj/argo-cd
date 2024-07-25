package helm

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var tags = TagsList{
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
	},
}

func TestTagsList_MaxVersion(t *testing.T) {
	t.Run("NotFound", func(t *testing.T) {
		constraints, _ := semver.NewConstraint("0.8.1")
		_, err := tags.MaxVersion(constraints)
		assert.EqualError(t, err, "constraint not found in 9 tags")
	})
	t.Run("Exact", func(t *testing.T) {
		constraints, _ := semver.NewConstraint("0.5.3")
		version, err := tags.MaxVersion(constraints)
		require.NoError(t, err)
		assert.Equal(t, semver.MustParse("0.5.3"), version)
	})
	t.Run("Constraint", func(t *testing.T) {
		constraints, _ := semver.NewConstraint("> 0.5.3")
		version, err := tags.MaxVersion(constraints)
		require.NoError(t, err)
		assert.Equal(t, semver.MustParse("0.7.2"), version)
	})
}
