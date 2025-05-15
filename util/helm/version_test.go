package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsVersion(t *testing.T) {
	assert.False(t, IsVersion("*"))
	assert.False(t, IsVersion("1.*"))
	assert.False(t, IsVersion("1.0.*"))
	assert.True(t, IsVersion("1.0.0"))
}
