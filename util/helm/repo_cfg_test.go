package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelmRepoCfg(t *testing.T) {
	t.Run("Unnamed", func(t *testing.T) {
		_, err := RepoCfgFactory{}.NewRepoCfg("http://0.0.0.0", "", "", "", nil, nil, nil)
		assert.EqualError(t, err, "must name repo")
	})

	t.Run("GarbageRepo", func(t *testing.T) {
		_, err := RepoCfgFactory{}.NewRepoCfg("http://0.0.0.0", "test", "", "", nil, nil, nil)
		assert.Error(t, err)
	})

	client, err := RepoCfgFactory{}.NewRepoCfg("https://kubernetes-charts.storage.googleapis.com", "test", "", "", nil, nil, nil)
	assert.NoError(t, err)

	const latestWordpressVersion = "5.7.1"

	t.Run("ResolveLatestRevision", func(t *testing.T) {
		resolvedRevision, err := client.ResolveRevision("wordpress", "")
		assert.NoError(t, err)
		assert.Equal(t, latestWordpressVersion, resolvedRevision)

	})
	t.Run("ResolveSpecificRevision", func(t *testing.T) {
		resolvedRevision, err := client.ResolveRevision("workpress", latestWordpressVersion)
		assert.NoError(t, err)
		assert.Equal(t, latestWordpressVersion, resolvedRevision)

	})
	t.Run("GetSpecificVersion", func(t *testing.T) {
		_, _, err = client.GetAppCfg("wordpress", latestWordpressVersion)
		assert.NoError(t, err)
	})
}
