package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepo(t *testing.T) {
	repo, err := NewRepo("https://kubernetes-charts.storage.googleapis.com", "test", "", "", nil, nil, nil)
	assert.NoError(t, err)
	err = repo.Init()
	assert.NoError(t, err)

	// TODO - this changes regularly
	const latestWordpressVersion = "5.8.0"

	t.Run("List", func(t *testing.T) {
		apps, err := repo.ListApps("")
		assert.NoError(t, err)
		assert.Contains(t, apps, "wordpress")
	})

	t.Run("ResolveAppRevision", func(t *testing.T) {
		unresolvedRevision := ""
		resolvedRevision, err := repo.ResolveAppRevision("wordpress", unresolvedRevision)
		assert.NoError(t, err)
		assert.NotEqual(t, unresolvedRevision, resolvedRevision)
	})

	t.Run("ResolveAppRevision/Latest", func(t *testing.T) {
		resolvedRevision, err := repo.ResolveAppRevision("wordpress", latestWordpressVersion)
		assert.NoError(t, err)
		assert.Equal(t, latestWordpressVersion, resolvedRevision)
	})

	t.Run("GetApp", func(t *testing.T) {
		appPath, err := repo.GetApp("wordpress", latestWordpressVersion)
		assert.NoError(t, err)
		assert.NotEmpty(t, appPath)
	})

	t.Run("Checkout/UnresolvedRevision", func(t *testing.T) {
		_, err := repo.GetApp("wordpress", "")
		assert.EqualError(t, err, "invalid resolved revision \"\", must be resolved")
	})

	t.Run("Checkout/UnknownChart", func(t *testing.T) {
		_, err := repo.GetApp("garbage", latestWordpressVersion)
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
