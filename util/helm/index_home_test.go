package helm

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagerKeyMatchesHelmDownloaderKey(t *testing.T) {
	sum := sha256.Sum256([]byte("https://charts.example.com"))
	assert.Equal(t, hex.EncodeToString(sum[:]), managerKey("https://charts.example.com"))
	assert.Equal(t, "helm-manager-"+managerKey("https://charts.example.com"), managerKeyPrefix+managerKey("https://charts.example.com"))
}

func TestPrewarmIndexesInstallsBothNames(t *testing.T) {
	c, err := newCmdWithVersion(t.TempDir(), false, "", "", func(_ *exec.Cmd, _ func(_ string) string) (string, error) {
		return "", nil
	})
	require.NoError(t, err)
	c.persistentIndexStore, _ = newTestIndexStore(t, time.Minute, 1024)
	data := sampleIndexData("demo")

	require.NoError(t, c.prewarmIndexes("https://charts.example.com", data, "example"))

	// Registered name
	b, err := os.ReadFile(filepath.Join(c.helmHome, "cache", "helm", "repository", cacheIndexFileName("example")))
	require.NoError(t, err)
	assert.Equal(t, data, b)
	// Manager name
	b, err = os.ReadFile(filepath.Join(c.helmHome, "cache", "helm", "repository", cacheIndexFileName(managerKeyPrefix+managerKey("https://charts.example.com"))))
	require.NoError(t, err)
	assert.Equal(t, data, b)
}

func TestWriteRepoCredentialWritesPrivateTempFile(t *testing.T) {
	c, err := newCmdWithVersion(t.TempDir(), false, "", "", func(_ *exec.Cmd, _ func(_ string) string) (string, error) {
		return "", nil
	})
	require.NoError(t, err)

	name, err := c.writeRepoCredential([]byte("cert-bytes"))
	require.NoError(t, err)
	require.NotEmpty(t, name)

	b, err := os.ReadFile(name)
	require.NoError(t, err)
	assert.Equal(t, []byte("cert-bytes"), b)

	// Credential files live under the private home and vanish with Close.
	require.Contains(t, name, c.helmHome)
}

func TestWriteRepoCredentialEmptyData(t *testing.T) {
	c, err := newCmdWithVersion(t.TempDir(), false, "", "", func(_ *exec.Cmd, _ func(_ string) string) (string, error) {
		return "", nil
	})
	require.NoError(t, err)
	name, err := c.writeRepoCredential(nil)
	require.NoError(t, err)
	assert.Empty(t, name)
}

func TestWriteRepoEntriesCreatesAndMerges(t *testing.T) {
	c, err := newCmdWithVersion(t.TempDir(), false, "", "", func(_ *exec.Cmd, _ func(_ string) string) (string, error) {
		return "", nil
	})
	require.NoError(t, err)

	first := []helmRepoEntry{{Name: "example", URL: "https://charts.example.com"}}
	require.NoError(t, c.writeRepoEntries(first))

	// Same entry again: idempotent.
	require.NoError(t, c.writeRepoEntries(first))

	// Additional entry merges.
	require.NoError(t, c.writeRepoEntries([]helmRepoEntry{{Name: "other", URL: "https://other.example.com"}}))

	// Same name different URL: conflict error.
	err = c.writeRepoEntries([]helmRepoEntry{{Name: "example", URL: "https://elsewhere.example.com"}})
	assert.Error(t, err, "name conflict must be rejected")
}

func TestWriteRepoEntriesFilePermissionsAndShape(t *testing.T) {
	c, err := newCmdWithVersion(t.TempDir(), false, "", "", func(_ *exec.Cmd, _ func(_ string) string) (string, error) {
		return "", nil
	})
	require.NoError(t, err)
	require.NoError(t, c.writeRepoEntries([]helmRepoEntry{{Name: "example", URL: "https://charts.example.com"}}))

	repoFile := filepath.Join(c.helmHome, "config", "helm", "repositories.yaml")
	info, err := os.Stat(repoFile)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
