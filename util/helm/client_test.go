package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepo(t *testing.T) {
	repo, err := NewClient("https://kubernetes-charts.storage.googleapis.com", "test", "", "", nil, nil, nil)
	assert.NoError(t, err)

	// TODO - this changes regularly
	const latestWordpressVersion = "5.8.0"

	t.Run("LsFiles", func(t *testing.T) {
		apps, err := repo.LsFiles("wordpress/*.yaml")
		assert.NoError(t, err)
		assert.ElementsMatch(t, apps, []string{"wordpress/Chart.yaml"})
	})

	t.Run("ResolveRevision", func(t *testing.T) {
		unresolvedRevision := ""
		resolvedRevision, err := repo.ResolveRevision("wordpress", unresolvedRevision)
		assert.NoError(t, err)
		assert.NotEqual(t, unresolvedRevision, resolvedRevision)
	})

	t.Run("ResolveRevision/Latest", func(t *testing.T) {
		resolvedRevision, err := repo.ResolveRevision("wordpress", latestWordpressVersion)
		assert.NoError(t, err)
		assert.Equal(t, latestWordpressVersion, resolvedRevision)
	})

	t.Run("Checkout", func(t *testing.T) {
		err := repo.Checkout("wordpress", latestWordpressVersion)
		assert.NoError(t, err)

		revision, err := repo.Revision("wordpress")
		assert.NoError(t, err)
		assert.Equal(t, latestWordpressVersion, revision)
	})

	t.Run("Checkout/File", func(t *testing.T) {
		err := repo.Checkout("wordpress/Chart.yaml", latestWordpressVersion)
		assert.NoError(t, err)
	})

	t.Run("Checkout/UnresolvedRevision", func(t *testing.T) {
		err = repo.Checkout("wordpress", "")
		assert.EqualError(t, err, "invalid resolved revision \"\", must be resolved")
	})

	t.Run("Checkout/UnknownChart", func(t *testing.T) {
		err = repo.Checkout("garbage", latestWordpressVersion)
		assert.EqualError(t, err, "unknown chart \"garbage\"")
	})

	t.Run("RevisionMetadata/UnknownChart", func(t *testing.T) {
		_, err = repo.RevisionMetadata("garbage", latestWordpressVersion)
		assert.EqualError(t, err, "unknown chart \"garbage/5.8.0\"")
	})

	t.Run("RevisionMetadata/KnownChart", func(t *testing.T) {
		metaData, err := repo.RevisionMetadata("wordpress", latestWordpressVersion)
		assert.NoError(t, err)
		assert.NotEmpty(t, metaData.Date)
	})
}
