//go:build linux
// +build linux

package commit

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestSecureMkdirAllLinux(t *testing.T) {
	root := t.TempDir()

	// hydratePath
	hydratePath := "test/dir"
	fullHydratePath, err := SecureMkdirAll(root, hydratePath, os.ModePerm)
	require.NoError(t, err)

	expectedPath := path.Join(root, hydratePath)
	require.Equal(t, expectedPath, fullHydratePath)
}
