package commands

import (
	"path/filepath"
	"testing"
	"time"

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
	t.Setenv(common.EnvHelmIndexCacheFilterEnabled, "")
	command := NewCommand()
	directory, err := command.Flags().GetString("helm-index-cache-dir")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(temporary, "helm-index-cache"), directory, "the default must respect the system temporary directory")
	enabled, err := command.Flags().GetBool("helm-index-cache-enabled")
	require.NoError(t, err)
	require.False(t, enabled, "having a default directory must not enable dependency reuse")
	filtered, err := command.Flags().GetBool("helm-index-cache-filter-enabled")
	require.NoError(t, err)
	require.False(t, filtered, "dependency-only snapshots require a separate opt-in")
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
	t.Setenv(common.EnvHelmIndexCacheFilterEnabled, "true")
	command = NewCommand()
	filtered, err = command.Flags().GetBool("helm-index-cache-filter-enabled")
	require.NoError(t, err)
	require.True(t, filtered)
	require.NoError(t, command.Flags().Set("helm-index-cache-filter-enabled", "false"))
	filtered, err = command.Flags().GetBool("helm-index-cache-filter-enabled")
	require.NoError(t, err)
	require.False(t, filtered, "the explicit flag must force complete indexes despite environment enablement")
}

func TestHelmIndexCacheExpirationFlag(t *testing.T) {
	t.Setenv(common.EnvHelmIndexCacheExpiration, "")
	t.Setenv("ARGOCD_RECONCILIATION_TIMEOUT", "15m")
	command := NewCommand()
	expiration, err := command.Flags().GetDuration("helm-index-cache-expiration")
	require.NoError(t, err)
	require.Equal(t, time.Minute, expiration, "Helm freshness must not inherit the Git revision interval")
	require.NoError(t, command.Flags().Set("revision-cache-expiration", "30m"))
	expiration, err = command.Flags().GetDuration("helm-index-cache-expiration")
	require.NoError(t, err)
	require.Equal(t, time.Minute, expiration)
	t.Setenv(common.EnvHelmIndexCacheExpiration, "30s")
	command = NewCommand()
	expiration, err = command.Flags().GetDuration("helm-index-cache-expiration")
	require.NoError(t, err)
	require.Equal(t, 30*time.Second, expiration)
	require.NoError(t, command.Flags().Set("helm-index-cache-expiration", "3m"))
	expiration, err = command.Flags().GetDuration("helm-index-cache-expiration")
	require.NoError(t, err)
	require.Equal(t, 3*time.Minute, expiration, "the explicit flag must override environment freshness")
	require.NoError(t, command.Flags().Set("helm-index-cache-expiration", "0s"))
	expiration, err = command.Flags().GetDuration("helm-index-cache-expiration")
	require.NoError(t, err)
	require.Zero(t, expiration, "zero requests revalidation on every use")
}
