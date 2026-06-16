package gpgsign

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/common"
)

// shortTempDir creates a temp dir under /tmp (not t.TempDir's potentially
// long /var/folders path on macOS) so the gpg-agent unix socket path stays
// under 108 bytes.
func shortTempDir(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("/tmp", "gpgsign-")
	require.NoError(t, err)
	require.NoError(t, os.Chmod(d, 0o700))
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	return d
}

// generateTestSigningKey creates a throwaway protected/unprotected GPG key
// in its own GNUPGHOME and exports the secret key block. Returns the ASCII
// armored secret key and its 40-char fingerprint.
func generateTestSigningKey(t *testing.T, passphrase string) ([]byte, string) {
	t.Helper()
	if _, err := exec.LookPath("gpg"); err != nil {
		t.Skip("gpg not available")
	}

	srcHome := shortTempDir(t)
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

	env := append(os.Environ(), "GNUPGHOME="+srcHome, "LANG=C.UTF-8")

	ctx := t.Context()
	cmd := exec.CommandContext(ctx, "gpg", "--no-permission-warning", "--batch", "--pinentry-mode", "loopback", "--gen-key", recipePath)
	cmd.Env = env
	stderr, err := cmd.CombinedOutput()
	require.NoError(t, err, "gen-key failed: %s", string(stderr))

	cmd = exec.CommandContext(ctx, "gpg", "--no-permission-warning", "--with-colons", "--list-secret-keys")
	cmd.Env = env
	out, err := cmd.Output()
	require.NoError(t, err)
	fp := ""
	inSec := false
	for line := range strings.SplitSeq(string(out), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "sec" {
			inSec = true
		} else if fields[0] == "fpr" && inSec {
			fp = fields[9]
			break
		}
	}
	require.NotEmpty(t, fp)

	args := []string{"--no-permission-warning", "--batch", "--pinentry-mode", "loopback", "--armor", "--export-secret-keys"}
	if passphrase != "" {
		args = append(args, "--passphrase", passphrase)
	}
	args = append(args, fp)
	cmd = exec.CommandContext(ctx, "gpg", args...)
	cmd.Env = env
	keyData, err := cmd.Output()
	require.NoError(t, err)
	return keyData, fp
}

func setSharedGnuPGHome(t *testing.T) {
	t.Helper()
	home := shortTempDir(t)
	t.Setenv(common.EnvGnuPGHome, home)
}

// generateTestSigningKeyWithSubkey creates a key whose primary is cert-only
// and whose signatures are produced by a dedicated signing subkey — the
// common real-world layout that the trailing-16 primary check used to reject.
// Returns the armored secret key, the primary fingerprint, and the signing
// subkey's long key ID.
func generateTestSigningKeyWithSubkey(t *testing.T) (keyData []byte, primaryFP, subkeyID string) {
	t.Helper()
	if _, err := exec.LookPath("gpg"); err != nil {
		t.Skip("gpg not available")
	}

	srcHome := shortTempDir(t)
	recipe := "%no-protection\n" +
		"Key-Type: RSA\nKey-Length: 2048\nKey-Usage: cert\n" +
		"Subkey-Type: RSA\nSubkey-Length: 2048\nSubkey-Usage: sign\n" +
		"Name-Real: Argo CD Subkey Test\nName-Email: subkey@argo-cd.invalid\n" +
		"Expire-Date: 0\n%commit\n"
	recipePath := filepath.Join(srcHome, "recipe")
	require.NoError(t, os.WriteFile(recipePath, []byte(recipe), 0o600))

	env := append(os.Environ(), "GNUPGHOME="+srcHome, "LANG=C.UTF-8")
	ctx := t.Context()
	cmd := exec.CommandContext(ctx, "gpg", "--no-permission-warning", "--batch", "--pinentry-mode", "loopback", "--gen-key", recipePath)
	cmd.Env = env
	stderr, err := cmd.CombinedOutput()
	require.NoError(t, err, "gen-key failed: %s", string(stderr))

	cmd = exec.CommandContext(ctx, "gpg", "--no-permission-warning", "--with-colons", "--list-secret-keys")
	cmd.Env = env
	out, err := cmd.Output()
	require.NoError(t, err)
	inSec := false
	for line := range strings.SplitSeq(string(out), "\n") {
		fields := strings.Split(line, ":")
		switch {
		case len(fields) > 0 && fields[0] == "sec":
			inSec = true
		case len(fields) >= 10 && fields[0] == "fpr":
			if inSec && primaryFP == "" {
				primaryFP = fields[9]
			}
		case len(fields) >= 12 && fields[0] == "ssb":
			if strings.Contains(fields[11], "s") {
				subkeyID = fields[4]
			}
		}
	}
	require.NotEmpty(t, primaryFP)
	require.NotEmpty(t, subkeyID, "expected a signing subkey id")

	cmd = exec.CommandContext(ctx, "gpg", "--no-permission-warning", "--batch", "--pinentry-mode", "loopback", "--armor", "--export-secret-keys", primaryFP)
	cmd.Env = env
	keyData, err = cmd.Output()
	require.NoError(t, err)
	return keyData, primaryFP, subkeyID
}

func TestParseSigningKeyIDs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		out           string
		wantIDs       []string
		wantHasSecret bool
	}{
		{
			name: "sign+cert primary, encrypt subkey",
			out: "sec:u:3072:1:1234567890ABCDEF:1700000000:::u:::scESC:::+:::23::0:\n" +
				"fpr:::::::::AAAA1111BBBB2222CCCC33331234567890ABCDEF:\n" +
				"ssb:u:3072:1:FEDCBA0987654321:1700000000::::::e:::+:::23:\n",
			wantIDs:       []string{"1234567890ABCDEF"},
			wantHasSecret: true,
		},
		{
			name: "cert-only primary, signing subkey",
			out: "sec:u:3072:1:1111222233334444:1700000000:::u:::cSC:::+:::23::0:\n" +
				"fpr:::::::::DDDD4444EEEE5555FFFF66661111222233334444:\n" +
				"ssb:u:3072:1:5555666677778888:1700000000::::::s:::+:::23:\n",
			wantIDs:       []string{"5555666677778888"},
			wantHasSecret: true,
		},
		{
			name:          "public only",
			out:           "pub:u:3072:1:9999000011112222:1700000000:::u:::scESC::::::23::0:\n",
			wantIDs:       nil,
			wantHasSecret: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ids, hasSecret := parseSigningKeyIDs(tt.out)
			assert.Equal(t, tt.wantIDs, ids)
			assert.Equal(t, tt.wantHasSecret, hasSecret)
		})
	}
}

func TestMatchesSigningKey(t *testing.T) {
	t.Parallel()
	cfg := &Config{SigningKeyIDs: []string{"55556666AABBCCDD"}}
	// %GK returning the long key id.
	assert.True(t, cfg.MatchesSigningKey("55556666AABBCCDD"))
	// %GK returning a full fingerprint that ends in the long key id.
	assert.True(t, cfg.MatchesSigningKey("DDDD4444EEEE5555FFFF666655556666AABBCCDD"))
	// Case-insensitive (gpg occasionally lowercases).
	assert.True(t, cfg.MatchesSigningKey("55556666aabbccdd"))
	// Different key.
	assert.False(t, cfg.MatchesSigningKey("DEADBEEFDEADBEEF"))
	// Empty.
	assert.False(t, cfg.MatchesSigningKey(""))
}

func TestImportSigningKey_SigningSubkey(t *testing.T) {
	keyData, primaryFP, subkeyID := generateTestSigningKeyWithSubkey(t)
	setSharedGnuPGHome(t)

	cfg, err := ImportSigningKey(context.Background(), keyData, "")
	require.NoError(t, err)
	assert.Equal(t, primaryFP, cfg.Fingerprint)
	// The signing-capable key is the subkey, not the cert-only primary.
	assert.Contains(t, cfg.SigningKeyIDs, subkeyID)
	assert.NotContains(t, cfg.SigningKeyIDs, primaryFP[len(primaryFP)-16:],
		"cert-only primary must not be treated as a signing key")
	// A commit signed by the subkey (git emits the subkey id as %GK) verifies.
	assert.True(t, cfg.MatchesSigningKey(subkeyID))
}

func TestImportSigningKey_Unprotected(t *testing.T) {
	keyData, wantFP := generateTestSigningKey(t, "")
	setSharedGnuPGHome(t)

	cfg, err := ImportSigningKey(context.Background(), keyData, "")
	require.NoError(t, err)
	assert.Equal(t, wantFP, cfg.Fingerprint)
	assert.Equal(t, wantFP[len(wantFP)-16:], cfg.KeyID)
	// Sign-capable primary, no subkey: the primary long id is the signer.
	assert.Contains(t, cfg.SigningKeyIDs, wantFP[len(wantFP)-16:])
	assert.True(t, cfg.MatchesSigningKey(cfg.KeyID))
}

func TestImportSigningKey_WithPassphrase(t *testing.T) {
	keyData, wantFP := generateTestSigningKey(t, "s3cret")
	setSharedGnuPGHome(t)

	cfg, err := ImportSigningKey(context.Background(), keyData, "s3cret")
	require.NoError(t, err)
	assert.Equal(t, wantFP, cfg.Fingerprint)
}

func TestImportSigningKey_EmptyKey(t *testing.T) {
	setSharedGnuPGHome(t)
	_, err := ImportSigningKey(context.Background(), nil, "")
	require.Error(t, err)
}

func TestParseSigningKeygrips(t *testing.T) {
	t.Parallel()
	// In `gpg --with-colons --with-keygrip` output the keygrip lives in field 10
	// (index 9) of each grp record, which follows its key record.
	const (
		grip1 = "1111111111111111111111111111111111111111"
		grip2 = "2222222222222222222222222222222222222222"
	)
	tests := []struct {
		name string
		out  string
		want []string
	}{
		{
			name: "sign-capable primary, encrypt subkey",
			out: "sec:u:3072:1:1234567890ABCDEF:1700000000:::u:::scESC:::+:::23::0:\n" +
				"fpr:::::::::AAAA1111BBBB2222CCCC33331234567890ABCDEF:\n" +
				"grp:::::::::" + grip1 + ":\n" +
				"ssb:u:3072:1:FEDCBA0987654321:1700000000::::::e:::+:::23:\n" +
				"grp:::::::::" + grip2 + ":\n",
			want: []string{grip1},
		},
		{
			name: "cert-only primary, signing subkey",
			out: "sec:u:3072:1:1111222233334444:1700000000:::u:::cSC:::+:::23::0:\n" +
				"fpr:::::::::DDDD4444EEEE5555FFFF66661111222233334444:\n" +
				"grp:::::::::" + grip1 + ":\n" +
				"ssb:u:3072:1:5555666677778888:1700000000::::::s:::+:::23:\n" +
				"grp:::::::::" + grip2 + ":\n",
			want: []string{grip2},
		},
		{
			name: "both primary and subkey signing-capable",
			out: "sec:u:3072:1:1111222233334444:1700000000:::u:::scSC:::+:::23::0:\n" +
				"grp:::::::::" + grip1 + ":\n" +
				"ssb:u:3072:1:5555666677778888:1700000000::::::s:::+:::23:\n" +
				"grp:::::::::" + grip2 + ":\n",
			want: []string{grip1, grip2},
		},
		{
			name: "public only, no keygrip",
			out:  "pub:u:3072:1:9999000011112222:1700000000:::u:::scESC::::::23::0:\n",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, parseSigningKeygrips(tt.out))
		})
	}
}

func TestPresetSigningPassphrase_EnablesNonInteractiveSigning(t *testing.T) {
	const passphrase = "s3cret"
	keyData, fp := generateTestSigningKey(t, passphrase)
	setSharedGnuPGHome(t)
	home := common.GetGnuPGHomePath()

	// The agent config must be in place before importing starts the agent.
	require.NoError(t, WriteAgentConfig(home))

	ctx := context.Background()
	_, err := ImportSigningKey(ctx, keyData, passphrase)
	require.NoError(t, err)

	// gpg-preset-passphrase ships in libexec with the gpg-agent package but may
	// be absent in some minimal environments; skip rather than fail there.
	if _, err := presetPassphraseBin(ctx); err != nil {
		t.Skipf("gpg-preset-passphrase unavailable: %v", err)
	}

	require.NoError(t, PresetSigningPassphrase(ctx, fp, passphrase))

	// --pinentry-mode error makes gpg fail instead of prompting (or hanging)
	// when the passphrase isn't available, so a successful sign here proves the
	// passphrase was served from the agent's preset cache.
	cmd := exec.CommandContext(ctx, "gpg",
		"--no-permission-warning", "--batch", "--no-tty",
		"--pinentry-mode", "error",
		"-u", fp, "--detach-sign",
	)
	cmd.Env = append(os.Environ(), "GNUPGHOME="+home, "LANG=C.UTF-8")
	cmd.Stdin = strings.NewReader("hydrated manifest content")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "signing must not prompt after preset: %s", out)
}

func TestImportSigningKey_PublicKeyOnly_Rejected(t *testing.T) {
	keyData, fp := generateTestSigningKey(t, "")
	// Export only the public part to simulate user mistake.
	srcHome := shortTempDir(t)
	env := append(os.Environ(), "GNUPGHOME="+srcHome, "LANG=C.UTF-8")
	// First import secret to source home so we can re-export the public key.
	ctx := t.Context()
	cmd := exec.CommandContext(ctx, "gpg", "--no-permission-warning", "--batch", "--import")
	cmd.Env = env
	cmd.Stdin = strings.NewReader(string(keyData))
	require.NoError(t, cmd.Run())
	cmd = exec.CommandContext(ctx, "gpg", "--no-permission-warning", "--armor", "--export", fp)
	cmd.Env = env
	pubData, err := cmd.Output()
	require.NoError(t, err)

	setSharedGnuPGHome(t)
	_, err = ImportSigningKey(context.Background(), pubData, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secret")
}
