package commit

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeSecureTempDir(t *testing.T) {
	tempDir, cleanup, err := makeSecureTempDir()
	if cleanup != nil {
		t.Cleanup(func() {
			err := cleanup()
			require.NoError(t, err)
		})
	}
	require.NoError(t, err)

	// Verify that the temporary directory exists
	_, err = os.Stat(tempDir)
	require.NoError(t, err)

	// Verify that the temporary directory is a directory
	fileInfo, err := os.Stat(tempDir)
	require.NoError(t, err)
	assert.True(t, fileInfo.IsDir())

	// Verify that the temporary directory is empty
	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	assert.Empty(t, files)
}
