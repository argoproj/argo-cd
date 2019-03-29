package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepo(t *testing.T) {

	repo, err := RepoFactory{}.GetRepo("https://kubernetes-charts.storage.googleapis.com", "test", "", "", nil, nil, nil)
	assert.NoError(t, err)

	const latestWordpressVersion = "5.7.1"

	t.Run("ResolveLatestRevision", func(t *testing.T) {
		resolvedRevision, err := repo.ResolveRevision("wordpress", "")
		assert.NoError(t, err)
		assert.Equal(t, latestWordpressVersion, resolvedRevision)

	})
	t.Run("ResolveSpecificRevision", func(t *testing.T) {
		resolvedRevision, err := repo.ResolveRevision("workpress", latestWordpressVersion)
		assert.NoError(t, err)
		assert.Equal(t, latestWordpressVersion, resolvedRevision)

	})
	t.Run("GetSpecificVersion", func(t *testing.T) {
		_, _, err = repo.GetTemplate("wordpress", latestWordpressVersion)
		assert.NoError(t, err)
	})
}
