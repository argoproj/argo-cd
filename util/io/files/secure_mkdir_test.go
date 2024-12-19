package files

import (
	"os"
	"path"
	"path/filepath"
	"strings"
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

	// Should fail because there is a file at the path
	_, err = SecureMkdirAll(root, unsafePath, os.ModePerm)
	require.Error(t, err)
}

func TestSecureMkdirAllDotDotPath(t *testing.T) {
	root := t.TempDir()
	unsafePath := "../outside"

	fullPath, err := SecureMkdirAll(root, unsafePath, os.ModePerm)
	require.NoError(t, err)

	expectedPath := filepath.Join(root, "outside")
	assert.Equal(t, expectedPath, fullPath)

	info, err := os.Stat(fullPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	relPath, err := filepath.Rel(root, fullPath)
	require.NoError(t, err)
	assert.False(t, strings.HasPrefix(relPath, ".."))
}
