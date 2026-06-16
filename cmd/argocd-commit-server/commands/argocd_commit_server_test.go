package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/common"
)

func TestValidateSigningFlags(t *testing.T) {
	tests := []struct {
		name           string
		keyPath        string
		passphraseFile string
		wantErr        bool
	}{
		{
			name:           "no key and no passphrase is valid (signing disabled)",
			keyPath:        "",
			passphraseFile: "",
			wantErr:        false,
		},
		{
			name:           "key without passphrase is valid (unprotected key)",
			keyPath:        "/app/config/gpg/signing/signingKey",
			passphraseFile: "",
			wantErr:        false,
		},
		{
			name:           "key with passphrase is valid (protected key)",
			keyPath:        "/app/config/gpg/signing/signingKey",
			passphraseFile: "/app/config/gpg/signing/passphrase",
			wantErr:        false,
		},
		{
			name:           "passphrase without key is rejected",
			keyPath:        "",
			passphraseFile: "/app/config/gpg/signing/passphrase",
			wantErr:        true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSigningFlags(tt.keyPath, tt.passphraseFile)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "signing-key-path")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// shortTempDir creates a temp dir under /tmp (not t.TempDir's potentially long
// /var/folders path on macOS) so the gpg-agent unix socket path stays under the
// 108-byte limit.
func shortTempDir(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("/tmp", "signingkey-")
	require.NoError(t, err)
	require.NoError(t, os.Chmod(d, 0o700))
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	return d
}

// setSharedGnuPGHome points ARGOCD_GNUPGHOME at a throwaway dir so
// setupSigningKey can initialize a real keyring without touching the host's.
func setSharedGnuPGHome(t *testing.T) {
	t.Helper()
	t.Setenv(common.EnvGnuPGHome, shortTempDir(t))
}

// generateUnprotectedSigningKey creates a throwaway, passphrase-less GPG key in
// its own GNUPGHOME and returns the ASCII-armored secret key and its 40-char
// fingerprint. Unprotected so the happy-path test doesn't depend on
// gpg-preset-passphrase being installed.
func generateUnprotectedSigningKey(t *testing.T) (keyData []byte, fingerprint string) {
	t.Helper()
	if _, err := exec.LookPath("gpg"); err != nil {
		t.Skip("gpg not available")
	}

	srcHome := shortTempDir(t)
	recipe := "%no-protection\n" +
		"Key-Type: RSA\nKey-Length: 2048\nKey-Usage: sign\n" +
		"Name-Real: Argo CD Commit Server Test\nName-Email: commit-server@argo-cd.invalid\n" +
		"Expire-Date: 0\n%commit\n"
	recipePath := filepath.Join(srcHome, "recipe")
	require.NoError(t, os.WriteFile(recipePath, []byte(recipe), 0o600))

	env := append(os.Environ(), "GNUPGHOME="+srcHome, "LANG=C.UTF-8")
	ctx := t.Context()
	cmd := exec.CommandContext(ctx, "gpg", "--no-permission-warning", "--batch", "--pinentry-mode", "loopback", "--gen-key", recipePath)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "gen-key failed: %s", string(out))

	cmd = exec.CommandContext(ctx, "gpg", "--no-permission-warning", "--with-colons", "--list-secret-keys")
	cmd.Env = env
	listing, err := cmd.Output()
	require.NoError(t, err)
	inSec := false
	for line := range strings.SplitSeq(string(listing), "\n") {
		fields := strings.Split(line, ":")
		switch {
		case len(fields) > 0 && fields[0] == "sec":
			inSec = true
		case len(fields) >= 10 && fields[0] == "fpr" && inSec:
			fingerprint = fields[9]
		}
		if fingerprint != "" {
			break
		}
	}
	require.NotEmpty(t, fingerprint)

	cmd = exec.CommandContext(ctx, "gpg", "--no-permission-warning", "--batch", "--pinentry-mode", "loopback", "--armor", "--export-secret-keys", fingerprint)
	cmd.Env = env
	keyData, err = cmd.Output()
	require.NoError(t, err)
	return keyData, fingerprint
}

func TestSetupSigningKey(t *testing.T) {
	t.Run("imports an unprotected key and returns its config", func(t *testing.T) {
		keyData, wantFP := generateUnprotectedSigningKey(t)
		setSharedGnuPGHome(t)

		keyPath := filepath.Join(shortTempDir(t), "signingKey")
		require.NoError(t, os.WriteFile(keyPath, keyData, 0o600))

		cfg, err := setupSigningKey(keyPath, "")
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, wantFP, cfg.Fingerprint)
		assert.Equal(t, wantFP[len(wantFP)-16:], cfg.KeyID)
	})

	t.Run("fails when the key file does not exist", func(t *testing.T) {
		// gpg is still required because setupSigningKey initializes GnuPG before
		// it ever reads the key file.
		if _, err := exec.LookPath("gpg"); err != nil {
			t.Skip("gpg not available")
		}
		setSharedGnuPGHome(t)

		_, err := setupSigningKey(filepath.Join(shortTempDir(t), "does-not-exist"), "")
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to read signing key")
	})

	t.Run("fails when the key file is empty", func(t *testing.T) {
		if _, err := exec.LookPath("gpg"); err != nil {
			t.Skip("gpg not available")
		}
		setSharedGnuPGHome(t)

		keyPath := filepath.Join(shortTempDir(t), "empty")
		require.NoError(t, os.WriteFile(keyPath, []byte("   \n"), 0o600))

		_, err := setupSigningKey(keyPath, "")
		require.Error(t, err)
		assert.ErrorContains(t, err, "is empty")
	})

	t.Run("fails when the passphrase file does not exist", func(t *testing.T) {
		if _, err := exec.LookPath("gpg"); err != nil {
			t.Skip("gpg not available")
		}
		setSharedGnuPGHome(t)

		// The passphrase is read before the key is imported, so a non-empty
		// placeholder key is enough to reach the passphrase-read error.
		keyPath := filepath.Join(shortTempDir(t), "signingKey")
		require.NoError(t, os.WriteFile(keyPath, []byte("not-a-real-key"), 0o600))

		_, err := setupSigningKey(keyPath, filepath.Join(shortTempDir(t), "no-passphrase"))
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to read signing key passphrase")
	})

	t.Run("fails when the key data is not a valid GPG key", func(t *testing.T) {
		if _, err := exec.LookPath("gpg"); err != nil {
			t.Skip("gpg not available")
		}
		setSharedGnuPGHome(t)

		keyPath := filepath.Join(shortTempDir(t), "signingKey")
		require.NoError(t, os.WriteFile(keyPath, []byte("not-a-real-key"), 0o600))

		_, err := setupSigningKey(keyPath, "")
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to import signing key")
	})
}
