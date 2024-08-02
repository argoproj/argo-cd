//go:build !linux
// +build !linux

package commit

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path"
	"testing"
)

func TestSecureMkdirAllDefault(t *testing.T) {
	root := t.TempDir()

	// hydratePath
	hydratePath := "test/dir"
	fullHydratePath, err := SecureMkdirAll(root, hydratePath, os.ModePerm)
	require.NoError(t, err)

	expectedPath := path.Join(root, hydratePath)
	assert.Equal(t, expectedPath, fullHydratePath)
}
