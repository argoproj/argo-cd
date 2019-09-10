package repo

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkDir(t *testing.T) {
	workDir, err := WorkDir("foo")
	assert.NoError(t, err)
	info, err := os.Stat(workDir)
	assert.NoError(t, err)
	assert.True(t, info.IsDir())
	_, err = WorkDir("foo")
	assert.NoError(t, err)
}
