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

	client, err := factory{}.newHelmClient("https://kubernetes-charts.storage.googleapis.com", "tmp", tmp, "", "", nil, nil, nil)
	assert.NoError(t, err)

	t.Run("Test", func(t *testing.T) {
		assert.NoError(t, client.Test())
	})
	t.Run("ResolveRevision", func(t *testing.T) {
		resolvedRevision, err := client.ResolveRevision("5.4.0")
		assert.NoError(t, err)
		assert.Equal(t, "5.4.0", resolvedRevision)

	})
	t.Run("Checkout", func(t *testing.T) {
		_, err = client.Checkout("wordpress", "5.4.0")
		assert.NoError(t, err)

		checkedOutRevision, err := client.Checkout("wordpress", "5.4.0")
		assert.NoError(t, err)
		assert.Equal(t, "5.4.0", checkedOutRevision)

	})
}
