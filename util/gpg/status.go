// Package gpg provides shared GPG-related utilities used by both Git verification
// and source integrity (e.g. Helm provenance) verification.
package gpg

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrNoStatusFound is returned when no GPG status line is found in the output.
var ErrNoStatusFound = errors.New("no GPG status line found")

// StatusSigRegex matches [GNUPG:] signature status lines (GOODSIG, BADSIG, etc.).
// Format: [GNUPG:] CODE KEYID ...
// See https://github.com/gpg/gnupg/blob/master/doc/DETAILS#general-status-codes
var StatusSigRegex = regexp.MustCompile(`(?m)^\[GNUPG:\] (GOODSIG|BADSIG|EXPSIG|EXPKEYSIG|REVKEYSIG|ERRSIG) ([0-9A-Fa-f]+)`)

// VerificationFailureMessage returns the user-facing error message for a GPG verification failure.
func VerificationFailureMessage(code, keyID string) string {
	var phrase string
	switch code {
	case "BADSIG":
		phrase = "bad signature"
	case "EXPSIG":
		phrase = "expired signature"
	case "EXPKEYSIG":
		phrase = "signed with expired key"
	case "REVKEYSIG":
		phrase = "signed with revoked key"
	case "ERRSIG":
		phrase = "signed with key not in keyring"
	default:
		phrase = "gpg verification failed"
	}
	return fmt.Sprintf("%s (key_id=%s)", phrase, keyID)
}

// ParseStatusOutputStrict parses GPG output line-by-line. Returns the first line's
// (code, keyID). Errors if any line has multiple status matches (preserves Git
// verify-tag --raw behavior).
func ParseStatusOutputStrict(status string) (code, keyID string, err error) {
	for _, line := range strings.Split(status, "\n") {
		matches := StatusSigRegex.FindAllStringSubmatch(line, -1)
		switch len(matches) {
		case 0:
			continue
		case 1:
			if len(matches[0]) >= 3 {
				return matches[0][1], matches[0][2], nil
			}
			continue
		default:
			return "", "", fmt.Errorf("too many matches parsing line %q", line)
		}
	}
	return "", "", fmt.Errorf("%w", ErrNoStatusFound)
}

// IsNoPubKey returns true if the status output indicates the signing key was not in the keyring.
func IsNoPubKey(status string) bool {
	return strings.Contains(status, "[GNUPG:] NO_PUBKEY ")
}
