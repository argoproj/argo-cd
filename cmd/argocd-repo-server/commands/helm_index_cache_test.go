package commands

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/common"
)

func TestHelmIndexCacheDirectoryFlag(t *testing.T) {
	temporary := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, temporary)
	}
	t.Setenv(common.EnvHelmIndexCacheEnabled, "")
	t.Setenv(common.EnvHelmIndexCacheDir, "")
	command := NewCommand()
	directory, err := command.Flags().GetString("helm-index-cache-dir")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(temporary, "helm-index-cache"), directory, "the default must respect the system temporary directory")
	enabled, err := command.Flags().GetBool("helm-index-cache-enabled")
	require.NoError(t, err)
	require.False(t, enabled, "having a default directory must not enable dependency reuse")
	t.Setenv(common.EnvHelmIndexCacheEnabled, "true")
	command = NewCommand()
	enabled, err = command.Flags().GetBool("helm-index-cache-enabled")
	require.NoError(t, err)
	require.True(t, enabled)
	directory, err = command.Flags().GetString("helm-index-cache-dir")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(temporary, "helm-index-cache"), directory, "enabling reuse must not require a directory override")
	t.Setenv(common.EnvHelmIndexCacheDir, "/tmp/private-helm-indexes")
	command = NewCommand()
	directory, err = command.Flags().GetString("helm-index-cache-dir")
	require.NoError(t, err)
	require.Equal(t, "/tmp/private-helm-indexes", directory)
	require.NoError(t, command.Flags().Set("helm-index-cache-dir", "/tmp/explicit-indexes"))
	directory, err = command.Flags().GetString("helm-index-cache-dir")
	require.NoError(t, err)
	require.Equal(t, "/tmp/explicit-indexes", directory)
	require.NoError(t, command.Flags().Set("helm-index-cache-enabled", "false"))
	enabled, err = command.Flags().GetBool("helm-index-cache-enabled")
	require.NoError(t, err)
	require.False(t, enabled, "the explicit flag must override environment enablement")
}
