package plugins

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiscover(t *testing.T) {
	plugins, err := Discover()
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"test-dummy", "helm-v3"}, plugins)
}
