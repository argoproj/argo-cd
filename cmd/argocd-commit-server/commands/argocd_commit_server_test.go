package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
