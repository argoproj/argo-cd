package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_BinaryName(t *testing.T) {
	require.Equal(t, binaryName, BinaryName())
}

func Test_Version(t *testing.T) {
	assert.Regexp(t, `^v[0-9]+\.[0-9]+\.[0-9]+(\-[a-z]+)*(\+[a-z0-9]+)*$`, Version())
}

func Test_Useragent(t *testing.T) {
	assert.Regexp(t, `^[a-z\-]+:\sv[0-9]+\.[0-9]+\.[0-9]+(-[a-z]+)*(\+[a-z0-9]+)*$`, Useragent())
}
