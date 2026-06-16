package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/util/gpgsign/gpgsigntest"
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

// setSharedGnuPGHome points ARGOCD_GNUPGHOME at a throwaway dir so
// setupSigningKey can initialize a real keyring without touching the host's.
func setSharedGnuPGHome(t *testing.T) {
	t.Helper()
	t.Setenv(common.EnvGnuPGHome, gpgsigntest.ShortTempDir(t))
}

func TestSetupSigningKey(t *testing.T) {
	t.Run("imports an unprotected key and returns its config", func(t *testing.T) {
		keyData, wantFP := gpgsigntest.GenerateSigningKey(t, "")
		setSharedGnuPGHome(t)

		keyPath := filepath.Join(gpgsigntest.ShortTempDir(t), "signingKey")
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

		_, err := setupSigningKey(filepath.Join(gpgsigntest.ShortTempDir(t), "does-not-exist"), "")
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to read signing key")
	})

	t.Run("fails when the key file is empty", func(t *testing.T) {
		if _, err := exec.LookPath("gpg"); err != nil {
			t.Skip("gpg not available")
		}
		setSharedGnuPGHome(t)

		keyPath := filepath.Join(gpgsigntest.ShortTempDir(t), "empty")
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
		keyPath := filepath.Join(gpgsigntest.ShortTempDir(t), "signingKey")
		require.NoError(t, os.WriteFile(keyPath, []byte("not-a-real-key"), 0o600))

		_, err := setupSigningKey(keyPath, filepath.Join(gpgsigntest.ShortTempDir(t), "no-passphrase"))
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to read signing key passphrase")
	})

	t.Run("fails when the key data is not a valid GPG key", func(t *testing.T) {
		if _, err := exec.LookPath("gpg"); err != nil {
			t.Skip("gpg not available")
		}
		setSharedGnuPGHome(t)

		keyPath := filepath.Join(gpgsigntest.ShortTempDir(t), "signingKey")
		require.NoError(t, os.WriteFile(keyPath, []byte("not-a-real-key"), 0o600))

		_, err := setupSigningKey(keyPath, "")
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to import signing key")
	})
}
