//go:build !linux

package commit

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecureMkdirAllDefault(t *testing.T) {
	root := t.TempDir()

	unsafePath := "test/dir"
	fullPath, err := SecureMkdirAll(root, unsafePath, os.ModePerm)
	require.NoError(t, err)

	expectedPath := path.Join(root, unsafePath)
	assert.Equal(t, expectedPath, fullPath)
}

func TestSecureMkdirAllWithExistingDir(t *testing.T) {
	root := t.TempDir()
	unsafePath := "existing/dir"

	fullPath, err := SecureMkdirAll(root, unsafePath, os.ModePerm)
	require.NoError(t, err)

	newPath, err := SecureMkdirAll(root, unsafePath, os.ModePerm)
	require.NoError(t, err)
	assert.Equal(t, fullPath, newPath)
}

func TestSecureMkdirAllWithFile(t *testing.T) {
	root := t.TempDir()
	unsafePath := "file.txt"

	filePath := filepath.Join(root, unsafePath)
	err := os.WriteFile(filePath, []byte("test"), os.ModePerm)
	require.NoError(t, err)

	_, err = SecureMkdirAll(root, unsafePath, os.ModePerm)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create directory")
}
