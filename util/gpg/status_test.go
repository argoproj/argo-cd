package gpg

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerificationFailureMessage(t *testing.T) {
	tests := []struct {
		code   string
		keyID  string
		expect string
	}{
		{"BADSIG", "ABC123", "bad signature (key_id=ABC123)"},
		{"EXPSIG", "DEF456", "expired signature (key_id=DEF456)"},
		{"EXPKEYSIG", "GHI789", "signed with expired key (key_id=GHI789)"},
		{"REVKEYSIG", "JKL012", "signed with revoked key (key_id=JKL012)"},
		{"ERRSIG", "MNO345", "signed with key not in keyring (key_id=MNO345)"},
		{"UNKNOWN", "PQR678", "gpg verification failed (key_id=PQR678)"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			assert.Equal(t, tt.expect, VerificationFailureMessage(tt.code, tt.keyID))
		})
	}
}

func TestParseStatusOutputStrict(t *testing.T) {
	tests := []struct {
		name    string
		status  string
		wantErr string
		code    string
		keyID   string
	}{
		{
			name:   "valid GOODSIG",
			status: "[GNUPG:] NEWSIG\n[GNUPG:] GOODSIG 9515ACB41E14E7F9 User <user@example.com>",
			code:   "GOODSIG",
			keyID:  "9515ACB41E14E7F9",
		},
		{
			name:   "valid ERRSIG",
			status: "[GNUPG:] NEWSIG\n[GNUPG:] ERRSIG D56C4FCA57A46444 1 10 00",
			code:   "ERRSIG",
			keyID:  "D56C4FCA57A46444",
		},
		{
			name:    "no status line",
			status:  "Buahahaha!",
			wantErr: "no GPG status line found",
		},
		{
			name:    "empty",
			status:  "",
			wantErr: "no GPG status line found",
		},
		{
			name:   "skips non-matching lines",
			status: "random line\n[GNUPG:] BADSIG ABCDEF1234567890 key",
			code:   "BADSIG",
			keyID:  "ABCDEF1234567890",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, keyID, err := ParseStatusOutputStrict(tt.status)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				if tt.wantErr == "no GPG status line found" {
					assert.True(t, errors.Is(err, ErrNoStatusFound))
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.code, code)
			assert.Equal(t, tt.keyID, keyID)
		})
	}
}
