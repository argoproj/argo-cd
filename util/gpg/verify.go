package gpg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/argoproj/argo-cd/v3/common"
)

func gnuPGEnviron() []string {
	return append(os.Environ(), "GNUPGHOME="+common.GetGnuPGHomePath(), "LANG=C.UTF-8")
}

// VerifyCleartextSignedMessage verifies a PGP cleartext-signed message (e.g. Helm .prov file).
// It returns the signer's key ID (long form) on success. Uses --status-fd for reliable parsing.
// Parses the same GPG status-fd format as Git (GOODSIG, ERRSIG, BADSIG, etc.)
func VerifyCleartextSignedMessage(ctx context.Context, clearsigned []byte) (signerKeyID string, err error) {
	f, err := os.CreateTemp("", "gpg-verify-")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(clearsigned); err != nil {
		return "", err
	}
	if err := f.Sync(); err != nil {
		return "", err
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return "", err
	}
	defer pr.Close()
	defer pw.Close()

	cmd := exec.CommandContext(ctx, "gpg", "--no-permission-warning", "--verify", "--status-fd", "3", f.Name())
	cmd.Env = gnuPGEnviron()
	cmd.ExtraFiles = []*os.File{pw}
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return "", err
	}
	pw.Close()
	var sb strings.Builder
	buf := make([]byte, 512)
	for {
		n, err := pr.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	_ = cmd.Wait()

	status := sb.String()
	code, keyID, err := ParseStatusOutputStrict(status)
	if err != nil {
		if errors.Is(err, ErrNoStatusFound) {
			return "", fmt.Errorf("gpg verify did not report GOODSIG (status-fd output: %q)", status)
		}
		return "", err
	}
	if code == "GOODSIG" {
		return keyID, nil
	}
	return "", errors.New(VerificationFailureMessage(code, keyID))
}
