package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateUpdatePasswordFlags(t *testing.T) {
	tests := []struct {
		name            string
		stdin           bool
		localUser       bool
		currentPassword string
		newPassword     string
		wantErr         string
	}{
		{
			name: "no stdin, no checks",
		},
		{
			name:  "no stdin even with --new-password is fine",
			stdin: false, newPassword: "secret",
		},
		{
			name:            "stdin with --current-password is fine",
			stdin:           true,
			localUser:       true,
			currentPassword: "old",
		},
		{
			name:            "stdin combined with --new-password is rejected",
			stdin:           true,
			localUser:       true,
			currentPassword: "old",
			newPassword:     "new",
			wantErr:         "--stdin cannot be combined with --new-password",
		},
		{
			name:      "local user with stdin and no --current-password is rejected",
			stdin:     true,
			localUser: true,
			wantErr:   "--current-password is required when --stdin is used",
		},
		{
			name:  "SSO user with stdin needs no --current-password",
			stdin: true,
		},
		{
			name:        "--new-password conflict reported before missing --current-password",
			stdin:       true,
			localUser:   true,
			newPassword: "new",
			wantErr:     "--stdin cannot be combined with --new-password",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUpdatePasswordFlags(tt.stdin, tt.localUser, tt.currentPassword, tt.newPassword)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}
