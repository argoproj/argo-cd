package client

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTempRepoPath(t *testing.T) {
	path := TempRepoPath("foo")
	info, err := os.Stat(path)
	assert.NoError(t, err)
	assert.True(t, info.IsDir())
}
