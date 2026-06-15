// Package gpgsign provides helpers used by the commit server to import a
// GPG signing key into the shared GNUPGHOME and resolve the imported key's
// fingerprint.
//
// Verification and trust-management of GPG keys are intentionally NOT covered
// here — that's util/sourceintegrity's concern. This package is the minimal
// sign-side counterpart used only by argocd-commit-server.
package gpgsign

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/common"
	executil "github.com/argoproj/argo-cd/v3/util/exec"
)

// Config holds the resolved signing identity. KeyID is the long key ID
// (16 hex chars, the trailing 64 bits of the fingerprint) of the imported
// primary key. The key material itself lives in the shared GNUPGHOME at
// common.GetGnuPGHomePath(). GPGProgram, when non-empty, is the path to a
// wrapper script that git's gpg.program must be set to so that loopback
// pinentry (and an optional passphrase file) are used during signing.
//
// SigningKeyIDs lists the long key IDs of every signing-capable (sub)key of
// the imported key — primary plus any signing subkeys. git/gpg may produce
// the signature with a dedicated signing subkey rather than the primary, so
// the post-commit verification must accept any of these IDs.
type Config struct {
	KeyID         string
	Fingerprint   string
	GPGProgram    string
	SigningKeyIDs []string
}

func gpgEnv() []string {
	return append(os.Environ(), "GNUPGHOME="+common.GetGnuPGHomePath(), "LANG=C.UTF-8")
}

// ImportSigningKey imports an ASCII-armored or binary private key into the
// shared GNUPGHOME and returns the resolved Config. passphrase may be empty
// for unprotected keys. Re-importing a key already present in the keyring is
// safe — gpg treats it as an update and still reports the fingerprint.
//
// The passphrase is fed to gpg via a 0600 temp file (--passphrase-file) so it
// does not appear on the command line and stays out of /proc/<pid>/cmdline
// for the duration of the import.
func ImportSigningKey(ctx context.Context, keyData []byte, passphrase string) (*Config, error) {
	if len(keyData) == 0 {
		return nil, errors.New("signing key data is empty")
	}

	args := []string{
		"--no-permission-warning",
		"--batch",
		"--pinentry-mode", "loopback",
		"--status-fd", "1",
	}
	var cleanup func()
	if passphrase != "" {
		ppFile, c, err := writePassphraseFile(passphrase)
		if err != nil {
			return nil, err
		}
		cleanup = c
		args = append(args, "--passphrase-file", ppFile)
	}
	args = append(args, "--import")

	cmd := exec.CommandContext(ctx, "gpg", args...)
	cmd.Env = gpgEnv()
	cmd.Stdin = strings.NewReader(string(keyData))
	out, err := executil.Run(cmd)
	// The passphrase file is only needed for the import above; remove it
	// immediately once gpg has run, regardless of the outcome, rather than
	// holding it open until the function returns.
	if cleanup != nil {
		cleanup()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to import signing key: %w", err)
	}

	fp, err := parseImportFingerprint(out)
	if err != nil {
		return nil, err
	}
	signingIDs, err := signingKeyIDs(ctx, fp)
	if err != nil {
		return nil, err
	}
	return &Config{
		Fingerprint:   fp,
		KeyID:         fp[len(fp)-16:],
		SigningKeyIDs: signingIDs,
	}, nil
}

// writePassphraseFile creates a 0600 file holding the passphrase and returns
// its path plus a cleanup func that removes it. Used to avoid leaking the
// secret through --passphrase on argv.
func writePassphraseFile(passphrase string) (string, func(), error) {
	f, err := os.CreateTemp("", "argocd-gpg-passphrase-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("failed to create passphrase temp file: %w", err)
	}
	cleanup := func() {
		if err := os.Remove(f.Name()); err != nil && !os.IsNotExist(err) {
			log.WithError(err).Warnf("failed to remove passphrase temp file %q", f.Name())
		}
	}
	if _, err := f.WriteString(passphrase); err != nil {
		_ = f.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("failed to write passphrase temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("failed to close passphrase temp file: %w", err)
	}
	return f.Name(), cleanup, nil
}

// parseImportFingerprint extracts the fingerprint from gpg's --status-fd
// IMPORT_OK status line: "[GNUPG:] IMPORT_OK <flags> <fingerprint>".
func parseImportFingerprint(statusOut string) (string, error) {
	const prefix = "[GNUPG:] IMPORT_OK "
	for line := range strings.SplitSeq(statusOut, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, prefix))
		if len(fields) >= 2 && len(fields[1]) == 40 {
			return fields[1], nil
		}
	}
	return "", errors.New("could not determine fingerprint from gpg import output (no IMPORT_OK status with a 40-char fingerprint)")
}

// WriteSignWrapper writes a small POSIX shell wrapper to dir that invokes gpg
// with --batch and loopback pinentry. If passphraseFile is non-empty, the
// wrapper additionally passes --passphrase-file. Returns the absolute path to
// the script. The script must be set as git's gpg.program when signing.
//
// We need the wrapper because `git commit -S` shells out to gpg without
// piping stdin, so the only way to feed a passphrase to a protected key is
// for gpg itself to read it from a file.
func WriteSignWrapper(dir, passphraseFile string) (string, error) {
	script := "#!/bin/sh\nexec gpg --no-permission-warning --batch --pinentry-mode loopback"
	if passphraseFile != "" {
		// Validate the path at startup before embedding it into shell source.
		if !filepath.IsAbs(passphraseFile) || passphraseFile != filepath.Clean(passphraseFile) {
			return "", fmt.Errorf("passphrase file path must be clean and absolute: %q", passphraseFile)
		}
		// Quote the path so paths with spaces still work. We've already
		// rejected empty here.
		script += " --passphrase-file '" + strings.ReplaceAll(passphraseFile, "'", `'\''`) + "'"
	}
	script += " \"$@\"\n"

	path := filepath.Join(dir, "argocd-gpg-sign-wrapper.sh")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		return "", fmt.Errorf("failed to write gpg sign wrapper: %w", err)
	}
	return path, nil
}

// signingKeyIDs lists the secret keys for the imported fingerprint and returns
// the long key IDs of every signing-capable (sub)key. It doubles as the
// secret-material guard: if no "sec" record is present the key is public-only
// and we reject it (IMPORT_OK is reported even for a public key). git/gpg may
// sign with a dedicated signing subkey rather than the primary, so the caller
// must accept any returned ID when verifying the freshly created commit.
func signingKeyIDs(ctx context.Context, fingerprint string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "gpg",
		"--no-permission-warning",
		"--with-colons",
		"--list-secret-keys",
		fingerprint,
	)
	cmd.Env = gpgEnv()
	out, err := executil.Run(cmd)
	if err != nil {
		return nil, fmt.Errorf("imported key %s is not a secret key: %w", fingerprint, err)
	}
	ids, hasSecret := parseSigningKeyIDs(out)
	if !hasSecret {
		return nil, fmt.Errorf("imported key %s does not contain secret material — provide the private key, not the public one", fingerprint)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("imported key %s has no signing-capable (sub)key", fingerprint)
	}
	return ids, nil
}

// parseSigningKeyIDs scans gpg --with-colons --list-secret-keys output and
// returns the long key IDs of all signing-capable sec/ssb records, plus
// whether any secret ("sec") record was seen at all. In the colon format the
// record type is field 1, the long key ID is field 5, and the per-key
// capability letters are field 12; a lowercase "s" there means that specific
// (sub)key can create signatures.
func parseSigningKeyIDs(colonOut string) (ids []string, hasSecret bool) {
	for line := range strings.SplitSeq(colonOut, "\n") {
		fields := strings.Split(line, ":")
		if len(fields) < 12 {
			continue
		}
		recordType := fields[0]
		if recordType != "sec" && recordType != "ssb" {
			continue
		}
		if recordType == "sec" {
			hasSecret = true
		}
		keyID := fields[4]
		capabilities := fields[11]
		if keyID != "" && strings.Contains(capabilities, "s") {
			ids = append(ids, keyID)
		}
	}
	return ids, hasSecret
}

// MatchesSigningKey reports whether keyID (git's %GK for a signed commit)
// corresponds to one of the imported key's signing-capable (sub)keys. The
// comparison is a case-insensitive suffix match so that either a 16-char long
// key ID or a 40-char fingerprint emitted by %GK is accepted.
func (c *Config) MatchesSigningKey(keyID string) bool {
	got := strings.ToUpper(strings.TrimSpace(keyID))
	if got == "" {
		return false
	}
	for _, id := range c.SigningKeyIDs {
		if strings.HasSuffix(got, strings.ToUpper(id)) {
			return true
		}
	}
	return false
}
