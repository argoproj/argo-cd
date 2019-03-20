package repos

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelmClient(t *testing.T) {
	tmp, err := ioutil.TempDir("", "helm")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmp) }()

	client, err := factory{}.newHelmClient("https://kubernetes-charts.storage.googleapis.com", "test", tmp, "", "", nil, nil, nil)
	assert.NoError(t, err)

	t.Run("Test", func(t *testing.T) {
		assert.NoError(t, client.Test())
	})

	const latestWordpressVersion = "5.7.0"

	t.Run("ResolveLatestRevision", func(t *testing.T) {
		resolvedRevision, err := client.ResolveRevision("wordpress", "")
		assert.NoError(t, err)
		assert.Equal(t, latestWordpressVersion, resolvedRevision)

	})
	t.Run("ResolveSpecificRevision", func(t *testing.T) {
		resolvedRevision, err := client.ResolveRevision("", latestWordpressVersion)
		assert.NoError(t, err)
		assert.Equal(t, latestWordpressVersion, resolvedRevision)

	})
	t.Run("CheckoutLatestVersion", func(t *testing.T) {
		checkedOutRevision, err := client.Checkout("wordpress", "")
		assert.NoError(t, err)
		assert.Equal(t, latestWordpressVersion, checkedOutRevision)

	})
	t.Run("CheckoutSpecificVersion", func(t *testing.T) {
		_, err = client.Checkout("wordpress", latestWordpressVersion)
		assert.NoError(t, err)

		checkedOutRevision, err := client.Checkout("wordpress", latestWordpressVersion)
		assert.NoError(t, err)
		assert.Equal(t, latestWordpressVersion, checkedOutRevision)

	})
}
