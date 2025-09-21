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
		version, revisionMetadata, err := MaxVersion("0.5.3", tags)
		require.NoError(t, err)
		assert.Equal(t, "0.5.3", version)
		assert.Equal(t, RevisionResolutionDirect, revisionMetadata.ResolutionType)
		assert.Equal(t, "0.5.3", revisionMetadata.OriginalRevision)
		assert.Equal(t, "0.5.3", revisionMetadata.Resolved)
	})
	t.Run("Exact nonsemver", func(t *testing.T) {
		version, revisionMetadata, err := MaxVersion("2024.03-LTS-RC19", tags)
		require.NoError(t, err)
		assert.Equal(t, "2024.03-LTS-RC19", version)
		assert.Equal(t, RevisionResolutionDirect, revisionMetadata.ResolutionType)
		assert.Equal(t, "2024.03-LTS-RC19", revisionMetadata.OriginalRevision)
		assert.Equal(t, "2024.03-LTS-RC19", revisionMetadata.Resolved)
	})
	t.Run("Exact missing", func(t *testing.T) {
		// Passing an exact version which is not in the list of tags still returns that version
		version, revisionMetadata, err := MaxVersion("99.99", []string{})
		require.NoError(t, err)
		assert.Equal(t, "99.99", version)
		assert.Equal(t, RevisionResolutionDirect, revisionMetadata.ResolutionType)
		assert.Equal(t, "99.99", revisionMetadata.OriginalRevision)
		assert.Equal(t, "99.99", revisionMetadata.Resolved)
	})
	t.Run("Constraint", func(t *testing.T) {
		version, revisionMetadata, err := MaxVersion("> 0.5.3", tags)
		require.NoError(t, err)
		assert.Equal(t, "0.7.2", version)
		assert.Equal(t, RevisionResolutionRange, revisionMetadata.ResolutionType)
		assert.Equal(t, "> 0.5.3", revisionMetadata.OriginalRevision)
		assert.Equal(t, "0.7.2", revisionMetadata.Resolved)
	})
	t.Run("Constraint", func(t *testing.T) {
		version, revisionMetadata, err := MaxVersion("> 0.0.0", tags)
		require.NoError(t, err)
		assert.Equal(t, "0.7.2", version)
		assert.Equal(t, RevisionResolutionRange, revisionMetadata.ResolutionType)
	})
	t.Run("Constraint", func(t *testing.T) {
		version, revisionMetadata, err := MaxVersion(">0.5.0,<0.7.0", tags)
		require.NoError(t, err)
		assert.Equal(t, "0.5.4", version)
		assert.Equal(t, RevisionResolutionRange, revisionMetadata.ResolutionType)
	})
	t.Run("Constraint", func(t *testing.T) {
		version, revisionMetadata, err := MaxVersion("0.7.*", tags)
		require.NoError(t, err)
		assert.Equal(t, "0.7.2", version)
		assert.Equal(t, RevisionResolutionRange, revisionMetadata.ResolutionType)
	})
	t.Run("Constraint", func(t *testing.T) {
		version, revisionMetadata, err := MaxVersion("*", tags)
		require.NoError(t, err)
		assert.Equal(t, "0.7.2", version)
		assert.Equal(t, RevisionResolutionRange, revisionMetadata.ResolutionType)
	})
	t.Run("Constraint missing", func(t *testing.T) {
		_, _, err := MaxVersion("0.7.*", []string{})
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
