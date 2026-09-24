package cert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareSSLCertDirForRepositoryCA(t *testing.T) {
	caPath := filepath.Join(t.TempDir(), "repo.pem")
	require.NoError(t, os.WriteFile(caPath, []byte("custom-repository-ca\n"), 0o644))
	destDir := filepath.Join(t.TempDir(), "ca-extra")

	sslCertDir, err := PrepareSSLCertDirForRepositoryCA(caPath, destDir)
	require.NoError(t, err)
	assert.Contains(t, sslCertDir, destDir)
	for _, part := range defaultSystemCertDirs {
		assert.Contains(t, sslCertDir, part)
	}
	entries, err := os.ReadDir(destDir)
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	data, err := os.ReadFile(filepath.Join(destDir, entries[0].Name()))
	require.NoError(t, err)
	assert.Contains(t, string(data), "custom-repository-ca")
}

func TestBuildSSLCertDir_preservesExisting(t *testing.T) {
	existing := t.TempDir()
	t.Setenv("SSL_CERT_DIR", existing)
	custom := t.TempDir()
	value := BuildSSLCertDir(custom)
	assert.True(t, strings.HasPrefix(value, custom+":"))
	assert.Contains(t, value, existing)
}

func TestUpsertEnvVars_replacesKey(t *testing.T) {
	out := UpsertEnvVars([]string{"FOO=bar", "SSL_CERT_DIR=old"}, "SSL_CERT_DIR=new")
	assert.Equal(t, []string{"FOO=bar", "SSL_CERT_DIR=new"}, out)
}
