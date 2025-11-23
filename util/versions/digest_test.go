package versions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsDigest(t *testing.T) {
	assert.False(t, IsDigest("*"))
	assert.False(t, IsDigest("1.*"))
	assert.False(t, IsDigest("1.0.*"))
	assert.False(t, IsDigest("1.0"))
	assert.False(t, IsDigest("1.0.0"))
	assert.False(t, IsDigest("sha256:12345"))
	assert.False(t, IsDigest("sha256:zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"))
	assert.True(t, IsDigest("sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"))
}
