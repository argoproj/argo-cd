package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepo(t *testing.T) {
	repo, err := RepoFactory{}.GetRepo("https://kubernetes-charts.storage.googleapis.com", "test", "", "", nil, nil, nil)
	assert.NoError(t, err)

	// TODO - this changes regularly
	const latestWordpressVersion = "5.8.0"

	t.Run("FindApps", func(t *testing.T) {
		apps, err := repo.FindApps("")
		assert.NoError(t, err)
		assert.Equal(t, "helm", apps["wordpress"])
	})

	t.Run("ResolveLatestRevision", func(t *testing.T) {
		unresolvedRevision := ""
		resolvedRevision, err := repo.ResolveRevision("wordpress", unresolvedRevision)
		assert.NoError(t, err)
		assert.NotEqual(t, unresolvedRevision, resolvedRevision)
	})

	t.Run("ResolveSpecificRevision", func(t *testing.T) {
		resolvedRevision, err := repo.ResolveRevision("wordpress", latestWordpressVersion)
		assert.NoError(t, err)
		assert.Equal(t, latestWordpressVersion, resolvedRevision)
	})

	t.Run("GetApp", func(t *testing.T) {
		_, tool, err := repo.GetTemplate("wordpress", latestWordpressVersion)
		assert.NoError(t, err)
		assert.Equal(t, "helm", tool)
	})

	t.Run("GetAppUnresolvedRevision", func(t *testing.T) {
		_, _, err = repo.GetTemplate("wordpress", "")
		assert.EqualError(t, err, "invalid resolved revision \"\", must be resolved")
	})

	t.Run("GetAppUnknownCHart", func(t *testing.T) {
		_, _, err = repo.GetTemplate("wordpress1", latestWordpressVersion)
		assert.EqualError(t, err, "unknown chart \"wordpress1\"")
	})
}
