package commands

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/common"
)

func TestHelmIndexCacheDirectoryFlag(t *testing.T) {
	t.Setenv(common.EnvHelmIndexCacheDir, "")
	command := NewCommand()
	directory, err := command.Flags().GetString("helm-index-cache-dir")
	require.NoError(t, err)
	require.Empty(t, directory, "dependency reuse must remain off by default")
	t.Setenv(common.EnvHelmIndexCacheDir, "/tmp/private-helm-indexes")
	command = NewCommand()
	directory, err = command.Flags().GetString("helm-index-cache-dir")
	require.NoError(t, err)
	require.Equal(t, "/tmp/private-helm-indexes", directory)
	require.NoError(t, command.Flags().Set("helm-index-cache-dir", "/tmp/explicit-indexes"))
	directory, err = command.Flags().GetString("helm-index-cache-dir")
	require.NoError(t, err)
	require.Equal(t, "/tmp/explicit-indexes", directory)
}
