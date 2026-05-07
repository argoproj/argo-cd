package versions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var tags = []string{
	"0.7.1",
	"0.5.4",
	"0.5.3",
	"0.7.2",
	"0.5.2",
	"0.5.1",
	"0.5.0",
	"2024.03-LTS-RC19",
}

func TestTags_MaxVersion(t *testing.T) {
	t.Run("Exact", func(t *testing.T) {
		version, err := MaxVersion("0.5.3", tags, "")
		require.NoError(t, err)
		assert.Equal(t, "0.5.3", version)
	})
	t.Run("Exact nonsemver", func(t *testing.T) {
		version, err := MaxVersion("2024.03-LTS-RC19", tags, "")
		require.NoError(t, err)
		assert.Equal(t, "2024.03-LTS-RC19", version)
	})
	t.Run("Exact missing", func(t *testing.T) {
		// Passing an exact version which is not in the list of tags still returns that version
		version, err := MaxVersion("99.99", []string{}, "")
		require.NoError(t, err)
		assert.Equal(t, "99.99", version)
	})
	t.Run("Constraint", func(t *testing.T) {
		version, err := MaxVersion("> 0.5.3", tags, "")
		require.NoError(t, err)
		assert.Equal(t, "0.7.2", version)
	})
	t.Run("Constraint", func(t *testing.T) {
		version, err := MaxVersion("> 0.0.0", tags, "")
		require.NoError(t, err)
		assert.Equal(t, "0.7.2", version)
	})
	t.Run("Constraint", func(t *testing.T) {
		version, err := MaxVersion(">0.5.0,<0.7.0", tags, "")
		require.NoError(t, err)
		assert.Equal(t, "0.5.4", version)
	})
	t.Run("Constraint", func(t *testing.T) {
		version, err := MaxVersion("0.7.*", tags, "")
		require.NoError(t, err)
		assert.Equal(t, "0.7.2", version)
	})
	t.Run("Constraint", func(t *testing.T) {
		version, err := MaxVersion("*", tags, "")
		require.NoError(t, err)
		assert.Equal(t, "0.7.2", version)
	})
	t.Run("Constraint missing", func(t *testing.T) {
		_, err := MaxVersion("0.7.*", []string{}, "")
		require.Error(t, err)
	})
}

func TestTags_IsConstraint(t *testing.T) {
	t.Run("Exact", func(t *testing.T) {
		assert.False(t, IsConstraint("0.5.3"))
	})
	t.Run("Exact nonsemver", func(t *testing.T) {
		assert.False(t, IsConstraint("2024.03-LTS-RC19"))
	})
	t.Run("Constraint", func(t *testing.T) {
		assert.True(t, IsConstraint("= 0.5.3"))
	})
	t.Run("Constraint", func(t *testing.T) {
		assert.True(t, IsConstraint("> 0.5.3"))
	})
	t.Run("Constraint", func(t *testing.T) {
		assert.True(t, IsConstraint(">0.5.0,<0.7.0"))
	})
	t.Run("Constraint", func(t *testing.T) {
		assert.True(t, IsConstraint("0.7.*"))
	})
	t.Run("Constraint", func(t *testing.T) {
		assert.True(t, IsConstraint("*"))
	})
}

func TestMaxVersion_WithTagPrefix(t *testing.T) {
	prefixedTags := []string{
		"prod/v1.0.0",
		"prod/v1.0.1",
		"prod/v1.1.0",
		"staging/v1.0.0",
		"staging/v2.0.0",
		"v3.0.0",
	}

	t.Run("patch wildcard with prefix", func(t *testing.T) {
		version, err := MaxVersion("v1.0.*", prefixedTags, "prod/")
		require.NoError(t, err)
		assert.Equal(t, "prod/v1.0.1", version)
	})

	t.Run("minor wildcard with prefix", func(t *testing.T) {
		version, err := MaxVersion("v1.*", prefixedTags, "prod/")
		require.NoError(t, err)
		assert.Equal(t, "prod/v1.1.0", version)
	})

	t.Run("star wildcard with prefix - matches all versions in prefix", func(t *testing.T) {
		version, err := MaxVersion("*", prefixedTags, "prod/")
		require.NoError(t, err)
		assert.Equal(t, "prod/v1.1.0", version)
	})

	t.Run("x.x.x wildcard syntax with prefix", func(t *testing.T) {
		// 1.x.x is equivalent to 1.* - matches any 1.x.x version
		version, err := MaxVersion("v1.x.x", prefixedTags, "prod/")
		require.NoError(t, err)
		assert.Equal(t, "prod/v1.1.0", version)
	})

	t.Run("x.x wildcard syntax with prefix", func(t *testing.T) {
		// 1.0.x is equivalent to 1.0.* - matches any 1.0.x version
		version, err := MaxVersion("v1.0.x", prefixedTags, "prod/")
		require.NoError(t, err)
		assert.Equal(t, "prod/v1.0.1", version)
	})

	t.Run("two-segment version with prefix is exact match", func(t *testing.T) {
		tagsWithTwoSegment := []string{
			"prod/1.0",
			"prod/1.0.0",
			"prod/1.0.1",
		}
		version, err := MaxVersion("1.0", tagsWithTwoSegment, "prod/")
		require.NoError(t, err)
		assert.Equal(t, "prod/1.0", version)
	})

	t.Run("gt constraint with prefix", func(t *testing.T) {
		version, err := MaxVersion("> v1.0.0", prefixedTags, "staging/")
		require.NoError(t, err)
		assert.Equal(t, "staging/v2.0.0", version)
	})

	t.Run("range constraint with prefix", func(t *testing.T) {
		version, err := MaxVersion(">= v1.0.0 < v1.1.0", prefixedTags, "prod/")
		require.NoError(t, err)
		assert.Equal(t, "prod/v1.0.1", version)
	})

	t.Run("no prefix still works", func(t *testing.T) {
		version, err := MaxVersion("v3.*", prefixedTags, "")
		require.NoError(t, err)
		assert.Equal(t, "v3.0.0", version)
	})

	t.Run("exact version with prefix", func(t *testing.T) {
		version, err := MaxVersion("v1.0.0", prefixedTags, "prod/")
		require.NoError(t, err)
		assert.Equal(t, "prod/v1.0.0", version)
	})

	t.Run("non-matching prefix returns error", func(t *testing.T) {
		_, err := MaxVersion("v1.0.*", prefixedTags, "dev/")
		require.Error(t, err)
	})

	t.Run("prerelease constraint with prefix", func(t *testing.T) {
		prereleaseTags := []string{
			"prod/1.0.2-20260309-192056-549",
			"prod/1.0.2-20260309-193104-150",
		}
		version, err := MaxVersion("*-0", prereleaseTags, "prod/")
		require.NoError(t, err)
		assert.Equal(t, "prod/1.0.2-20260309-193104-150", version)
	})

	t.Run("non-semver tag in mix is skipped", func(t *testing.T) {
		// "prod/latest" cannot be parsed as semver and should be ignored
		prereleaseTags := []string{
			"prod/1.0.2-20260309-192056-549",
			"prod/1.0.2-20260309-193104-150",
			"prod/latest",
		}
		version, err := MaxVersion("*-0", prereleaseTags, "prod/")
		require.NoError(t, err)
		assert.Equal(t, "prod/1.0.2-20260309-193104-150", version)
	})

	t.Run("prerelease constraint with bare timestamp tags", func(t *testing.T) {
		prereleaseTags := []string{
			"prod/20260309-192056-549",
			"prod/20260309-193104-150",
		}
		version, err := MaxVersion("*-0", prereleaseTags, "prod/")
		require.NoError(t, err)
		assert.Equal(t, "prod/20260309-193104-150", version)
	})

	t.Run("release tag vs bare timestamp tags", func(t *testing.T) {
		prereleaseTags := []string{
			"prod/20260309-192056-549",
			"prod/20260309-193104-150",
			"prod/1.0.0",
		}
		version, err := MaxVersion("*-0", prereleaseTags, "prod/")
		require.NoError(t, err)
		t.Logf("winner: %s", version)
	})

	t.Run("deep nested prefix", func(t *testing.T) {
		nestedTags := []string{
			"foo/bar/v1.0.0",
			"foo/bar/v1.0.1",
			"foo/baz/v1.0.0",
		}
		version, err := MaxVersion("v1.0.*", nestedTags, "foo/bar/")
		require.NoError(t, err)
		assert.Equal(t, "foo/bar/v1.0.1", version)
	})
}
