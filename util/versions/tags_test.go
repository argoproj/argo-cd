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
		version, err := MaxVersion("0.5.3", tags)
		require.NoError(t, err)
		assert.Equal(t, "0.5.3", version)
	})
	t.Run("Exact nonsemver", func(t *testing.T) {
		version, err := MaxVersion("2024.03-LTS-RC19", tags)
		require.NoError(t, err)
		assert.Equal(t, "2024.03-LTS-RC19", version)
	})
	t.Run("Exact missing", func(t *testing.T) {
		// Passing an exact version which is not in the list of tags still returns that version
		version, err := MaxVersion("99.99", []string{})
		require.NoError(t, err)
		assert.Equal(t, "99.99", version)
	})
	t.Run("Constraint", func(t *testing.T) {
		version, err := MaxVersion("> 0.5.3", tags)
		require.NoError(t, err)
		assert.Equal(t, "0.7.2", version)
	})
	t.Run("Constraint", func(t *testing.T) {
		version, err := MaxVersion("> 0.0.0", tags)
		require.NoError(t, err)
		assert.Equal(t, "0.7.2", version)
	})
	t.Run("Constraint", func(t *testing.T) {
		version, err := MaxVersion(">0.5.0,<0.7.0", tags)
		require.NoError(t, err)
		assert.Equal(t, "0.5.4", version)
	})
	t.Run("Constraint", func(t *testing.T) {
		version, err := MaxVersion("0.7.*", tags)
		require.NoError(t, err)
		assert.Equal(t, "0.7.2", version)
	})
	t.Run("Constraint", func(t *testing.T) {
		version, err := MaxVersion("*", tags)
		require.NoError(t, err)
		assert.Equal(t, "0.7.2", version)
	})
	t.Run("Constraint missing", func(t *testing.T) {
		_, err := MaxVersion("0.7.*", []string{})
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

func TestExtractPrefix(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedPrefix string
		expectedStrip  string
	}{
		{
			name:           "no prefix",
			input:          "v1.0.0",
			expectedPrefix: "",
			expectedStrip:  "v1.0.0",
		},
		{
			name:           "simple prefix",
			input:          "prod/v1.0.0",
			expectedPrefix: "prod/",
			expectedStrip:  "v1.0.0",
		},
		{
			name:           "prefix with wildcard",
			input:          "prod/v1.0.*",
			expectedPrefix: "prod/",
			expectedStrip:  "v1.0.*",
		},
		{
			name:           "prefix with star only",
			input:          "prod/*",
			expectedPrefix: "prod/",
			expectedStrip:  "*",
		},
		{
			name:           "prefix with gt constraint",
			input:          "> prod/v1.0.0",
			expectedPrefix: "prod/",
			expectedStrip:  "> v1.0.0",
		},
		{
			name:           "prefix with gte constraint",
			input:          ">= prod/v1.0.0",
			expectedPrefix: "prod/",
			expectedStrip:  ">= v1.0.0",
		},
		{
			name:           "consistent prefix in range",
			input:          ">= prod/v1.0.0 < prod/v2.0.0",
			expectedPrefix: "prod/",
			expectedStrip:  ">= v1.0.0 < v2.0.0",
		},
		{
			name:           "mixed prefixes - no extraction",
			input:          "> prod/v1.0.0 < dev/v2.0.0",
			expectedPrefix: "",
			expectedStrip:  "> prod/v1.0.0 < dev/v2.0.0",
		},
		{
			name:           "deep nested prefix",
			input:          "foo/bar/baz/v1.0.0",
			expectedPrefix: "foo/bar/baz/",
			expectedStrip:  "v1.0.0",
		},
		{
			name:           "deep nested prefix with constraint",
			input:          "> foo/bar/v1.0.0 < foo/bar/v2.0.0",
			expectedPrefix: "foo/bar/",
			expectedStrip:  "> v1.0.0 < v2.0.0",
		},
		{
			name:           "inconsistent - one has prefix one doesn't",
			input:          "> prod/v1.0.0 < v2.0.0",
			expectedPrefix: "",
			expectedStrip:  "> prod/v1.0.0 < v2.0.0",
		},
		{
			name:           "tilde constraint with prefix",
			input:          "~prod/v1.0.0",
			expectedPrefix: "prod/",
			expectedStrip:  "~v1.0.0",
		},
		{
			name:           "caret constraint with prefix",
			input:          "^prod/v1.0.0",
			expectedPrefix: "prod/",
			expectedStrip:  "^v1.0.0",
		},
		{
			name:           "empty string",
			input:          "",
			expectedPrefix: "",
			expectedStrip:  "",
		},
		{
			name:           "just operators",
			input:          "> < >=",
			expectedPrefix: "",
			expectedStrip:  "> < >=",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prefix, stripped := extractPrefixes(tc.input)
			assert.Equal(t, tc.expectedPrefix, prefix, "prefix mismatch")
			assert.Equal(t, tc.expectedStrip, stripped, "stripped mismatch")
		})
	}
}

func TestMaxVersion_WithPrefix(t *testing.T) {
	prefixedTags := []string{
		"prod/v1.0.0",
		"prod/v1.0.1",
		"prod/v1.1.0",
		"staging/v1.0.0",
		"staging/v2.0.0",
		"v3.0.0",
	}

	t.Run("patch wildcard with prefix", func(t *testing.T) {
		version, err := MaxVersion("prod/v1.0.*", prefixedTags)
		require.NoError(t, err)
		assert.Equal(t, "prod/v1.0.1", version)
	})

	t.Run("minor wildcard with prefix", func(t *testing.T) {
		version, err := MaxVersion("prod/v1.*", prefixedTags)
		require.NoError(t, err)
		assert.Equal(t, "prod/v1.1.0", version)
	})

	t.Run("star wildcard with prefix - matches all versions in prefix", func(t *testing.T) {
		version, err := MaxVersion("prod/*", prefixedTags)
		require.NoError(t, err)
		assert.Equal(t, "prod/v1.1.0", version)
	})

	t.Run("x.x.x wildcard syntax with prefix", func(t *testing.T) {
		// 1.x.x is equivalent to 1.* - matches any 1.x.x version
		version, err := MaxVersion("prod/v1.x.x", prefixedTags)
		require.NoError(t, err)
		assert.Equal(t, "prod/v1.1.0", version)
	})

	t.Run("x.x wildcard syntax with prefix", func(t *testing.T) {
		// 1.0.x is equivalent to 1.0.* - matches any 1.0.x version
		version, err := MaxVersion("prod/v1.0.x", prefixedTags)
		require.NoError(t, err)
		assert.Equal(t, "prod/v1.0.1", version)
	})

	t.Run("two-segment version with prefix is exact match", func(t *testing.T) {
		// Two-segment versions like 1.0 are treated as exact versions, not constraints
		// They must match a tag exactly (semver interprets 1.0 as 1.0.0)
		tagsWithTwoSegment := []string{
			"prod/1.0",
			"prod/1.0.0",
			"prod/1.0.1",
		}
		version, err := MaxVersion("prod/1.0", tagsWithTwoSegment)
		require.NoError(t, err)
		assert.Equal(t, "prod/1.0", version)
	})

	t.Run("gt constraint with prefix", func(t *testing.T) {
		version, err := MaxVersion("> staging/v1.0.0", prefixedTags)
		require.NoError(t, err)
		assert.Equal(t, "staging/v2.0.0", version)
	})

	t.Run("range constraint with prefix", func(t *testing.T) {
		version, err := MaxVersion(">= prod/v1.0.0 < prod/v1.1.0", prefixedTags)
		require.NoError(t, err)
		assert.Equal(t, "prod/v1.0.1", version)
	})

	t.Run("no prefix still works", func(t *testing.T) {
		version, err := MaxVersion("v3.*", prefixedTags)
		require.NoError(t, err)
		assert.Equal(t, "v3.0.0", version)
	})

	t.Run("exact version with prefix", func(t *testing.T) {
		version, err := MaxVersion("prod/v1.0.0", prefixedTags)
		require.NoError(t, err)
		assert.Equal(t, "prod/v1.0.0", version)
	})

	t.Run("non-matching prefix returns error", func(t *testing.T) {
		_, err := MaxVersion("dev/v1.0.*", prefixedTags)
		require.Error(t, err)
	})

	t.Run("deep nested prefix", func(t *testing.T) {
		nestedTags := []string{
			"foo/bar/v1.0.0",
			"foo/bar/v1.0.1",
			"foo/baz/v1.0.0",
		}
		version, err := MaxVersion("foo/bar/v1.0.*", nestedTags)
		require.NoError(t, err)
		assert.Equal(t, "foo/bar/v1.0.1", version)
	})
}
