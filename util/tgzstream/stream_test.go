package tgzstream

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/io/files"
)

func TestCloseAndDeleteTempFileRemovesParentDirectory(t *testing.T) {
	tempRoot := t.TempDir()
	t.Setenv("TMP", tempRoot)
	t.Setenv("TEMP", tempRoot)
	t.Setenv("TMPDIR", tempRoot)
	parent, err := files.CreateTempDir(os.TempDir())
	require.NoError(t, err)
	file, err := os.CreateTemp(parent, "archive")
	require.NoError(t, err)

	CloseAndDeleteTempFile(file)

	_, err = os.Stat(parent)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestCloseAndDeleteTempFilePreservesUnownedParentDirectory(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "shared")
	require.NoError(t, os.Mkdir(parent, 0o755))
	file, err := os.CreateTemp(parent, "archive")
	require.NoError(t, err)

	CloseAndDeleteTempFile(file)

	_, err = os.Stat(parent)
	assert.NoError(t, err)
}

func TestIsOwnedTempDir(t *testing.T) {
	tempRoot := t.TempDir()
	t.Setenv("TMP", tempRoot)
	t.Setenv("TEMP", tempRoot)
	t.Setenv("TMPDIR", tempRoot)

	owned, err := files.CreateTempDir(os.TempDir())
	require.NoError(t, err)
	assert.True(t, isOwnedTempDir(owned))
	assert.False(t, isOwnedTempDir(tempRoot))

	unowned := filepath.Join(tempRoot, "not-a-uuid")
	require.NoError(t, os.Mkdir(unowned, 0o755))
	assert.False(t, isOwnedTempDir(unowned))
	assert.False(t, isOwnedTempDir(filepath.Join(owned, "child")))
}

func TestCloseAndDeletePreservesParentDirectory(t *testing.T) {
	parent := t.TempDir()
	file, err := os.CreateTemp(parent, "archive")
	require.NoError(t, err)

	CloseAndDelete(file)

	_, err = os.Stat(parent)
	assert.NoError(t, err)
}

func TestCompressFilesCleanupRemovesTemporaryDirectory(t *testing.T) {
	appPath := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(appPath, "config.yaml"), []byte("kind: ConfigMap\n"), 0o600))

	file, filesWritten, checksum, err := CompressFiles(appPath, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, filesWritten)
	assert.NotEmpty(t, checksum)
	parent := filepath.Dir(file.Name())
	require.DirExists(t, parent)

	CloseAndDeleteTempFile(file)

	assert.NoDirExists(t, parent)
}

func TestCompressFilesCleansTemporaryDirectoryOnFailure(t *testing.T) {
	tempRoot := t.TempDir()
	appPath := t.TempDir()
	t.Setenv("TMP", tempRoot)
	t.Setenv("TEMP", tempRoot)
	t.Setenv("TMPDIR", tempRoot)

	_, _, _, err := CompressFiles(filepath.Join(appPath, "missing"), nil, nil)
	require.Error(t, err)

	entries, err := os.ReadDir(tempRoot)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
