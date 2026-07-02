// Package gpgsigntest provides GPG test fixtures shared by the commit-server
// signing tests across packages. It lives outside the gpgsign package so it is
// not part of gpgsign's public API; because only _test.go files import it, it
// never gets linked into a production binary.
package gpgsigntest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// ShortTempDir creates a temp dir under /tmp (not t.TempDir's potentially long
// /var/folders path on macOS) so the gpg-agent unix socket path stays under the
// 108-byte limit. It is removed automatically when the test ends.
func ShortTempDir(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("/tmp", "gpgsigntest-")
	require.NoError(t, err)
	require.NoError(t, os.Chmod(d, 0o700))
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	return d
}

// GenerateSigningKey creates a throwaway GPG key in its own temporary
// GNUPGHOME and returns the ASCII-armored secret key block and the key's
// 40-char fingerprint. Pass an empty passphrase for an unprotected key (which
// signs without gpg-preset-passphrase). The temporary GNUPGHOME used for
// generation is removed before returning; the caller owns nothing. The test is
// skipped when gpg is unavailable.
func GenerateSigningKey(t *testing.T, passphrase string) (keyData []byte, fingerprint string) {
	t.Helper()
	if _, err := exec.LookPath("gpg"); err != nil {
		t.Skip("gpg not available")
	}

	srcHome := ShortTempDir(t)

	recipe := strings.Builder{}
	if passphrase == "" {
		recipe.WriteString("%no-protection\n")
	}
	recipe.WriteString("Key-Type: RSA\nKey-Length: 2048\nKey-Usage: sign\n")
	recipe.WriteString("Name-Real: Argo CD Test\nName-Email: test@argo-cd.invalid\n")
	recipe.WriteString("Expire-Date: 0\n")
	if passphrase != "" {
		recipe.WriteString("Passphrase: " + passphrase + "\n")
	}
	recipe.WriteString("%commit\n")

	recipePath := filepath.Join(srcHome, "recipe")
	require.NoError(t, os.WriteFile(recipePath, []byte(recipe.String()), 0o600))

	ctx := t.Context()
	env := append(os.Environ(), "GNUPGHOME="+srcHome, "LANG=C.UTF-8")

	cmd := exec.CommandContext(ctx, "gpg", "--no-permission-warning", "--batch", "--pinentry-mode", "loopback", "--gen-key", recipePath)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "gen-key failed: %s", out)

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
	require.NotEmpty(t, fingerprint, "could not parse fingerprint from: %q", listing)

	args := []string{"--no-permission-warning", "--batch", "--pinentry-mode", "loopback", "--armor", "--export-secret-keys"}
	if passphrase != "" {
		args = append(args, "--passphrase", passphrase)
	}
	args = append(args, fingerprint)
	cmd = exec.CommandContext(ctx, "gpg", args...)
	cmd.Env = env
	keyData, err = cmd.Output()
	require.NoError(t, err)
	return keyData, fingerprint
}
