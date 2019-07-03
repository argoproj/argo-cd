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
		apps, err := repo.LsFiles("wordpress")
		assert.NoError(t, err)
		assert.Len(t, apps, 1)
	})

	t.Run("LsRemote", func(t *testing.T) {
		unresolvedRevision := ""
		resolvedRevision, err := repo.LsRemote("wordpress", unresolvedRevision)
		assert.NoError(t, err)
		assert.NotEqual(t, unresolvedRevision, resolvedRevision)
	})

	t.Run("LsRemote2", func(t *testing.T) {
		resolvedRevision, err := repo.LsRemote("wordpress", latestWordpressVersion)
		assert.NoError(t, err)
		assert.Equal(t, latestWordpressVersion, resolvedRevision)
	})

	t.Run("Checkout", func(t *testing.T) {
		err := repo.Checkout("wordpress", latestWordpressVersion)
		assert.NoError(t, err)
	})

	t.Run("CheckoutUnresolvedRevision", func(t *testing.T) {
		err = repo.Checkout("wordpress", "")
		assert.EqualError(t, err, "invalid resolved revision \"\", must be resolved")
	})

	t.Run("CheckoutUnknownChart", func(t *testing.T) {
		err = repo.Checkout("wordpress1", latestWordpressVersion)
		assert.EqualError(t, err, "unknown chart \"wordpress1\"")
	})
}
