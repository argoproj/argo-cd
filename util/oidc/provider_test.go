package oidc

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/util/settings"
)

func TestVerifyJWT(t *testing.T) {
	tests := []struct {
		name        string
		jwtConfig   *settings.JWTConfig
		token       string
		expectError bool
	}{
		{
			name: "Valid JWT",
			jwtConfig: &settings.JWTConfig{
				HeaderName:    "X-Test-JWT",
				EmailClaim:    "email",
				UsernameClaim: "sub",
				JWKSetURL:     "https://test.example.com/.well-known/jwks.json",
				CacheTTL:      "1h",
			},
			token:       "valid.jwt.token",
			expectError: false,
		},
		{
			name:        "Missing JWT Config",
			jwtConfig:   nil,
			token:       "valid.jwt.token",
			expectError: true,
		},
		{
			name: "Invalid Cache TTL",
			jwtConfig: &settings.JWTConfig{
				HeaderName: "X-Test-JWT",
				JWKSetURL:  "https://test.example.com/.well-known/jwks.json",
				CacheTTL:   "invalid",
			},
			token:       "valid.jwt.token",
			expectError: false, // Should use default TTL
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewOIDCProvider("https://test.example.com", nil)
			settings := &settings.ArgoCDSettings{
				JWTConfig: tt.jwtConfig,
			}

			_, err := provider.VerifyJWT(tt.token, settings)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
