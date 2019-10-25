package plugins

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiscover(t *testing.T) {
	plugins:= Discover()
	assert.ElementsMatch(t, []string{"test-dummy", "helm-v3"}, plugins)
}
