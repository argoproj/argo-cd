//go:build linux

package commit

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSecureMkdirAllLinux(t *testing.T) {
	root := t.TempDir()

	unsafePath := "test/dir"
	fullPath, err := SecureMkdirAll(root, unsafePath, os.ModePerm)
	require.NoError(t, err)

	expectedPath := path.Join(root, unsafePath)
	require.Equal(t, expectedPath, fullPath)
}
