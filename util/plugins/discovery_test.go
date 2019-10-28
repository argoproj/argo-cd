package plugins

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNames(t *testing.T) {
	names := Names()
	assert.Len(t, names, 2)
}

func TestGet(t *testing.T) {
	t.Run("Dummy", func(t *testing.T) {
		plugin := Get("test-dummy")
		assert.NotNil(t, plugin)
	})
	t.Run("Helm", func(t *testing.T) {
		plugin := Get("helm-v3")
		assert.NotNil(t, plugin)
	})
}
