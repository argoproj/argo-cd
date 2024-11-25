package oidc

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/util/settings"
)

func generateTestToken(claims map[string]interface{}) string {
	// Using github.com/golang-jwt/jwt/v5
	token := jwt.New(jwt.SigningMethodHS256)
	defaultClaims := map[string]interface{}{
		"sub":   "test-user",
		"email": "test@example.com",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}

	// Merge provided claims with defaults
	for k, v := range claims {
		defaultClaims[k] = v
	}

	token.Claims = jwt.MapClaims(defaultClaims)

	// Sign with test key - real validation will be mocked
	tokenString, _ := token.SignedString([]byte("test-key"))
	return tokenString
}

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
			token:       generateTestToken(nil), // for default claims
			expectError: false,
		},
		{
			name:        "Missing JWT Config",
			jwtConfig:   nil,
			token:       generateTestToken(nil), // for default claims
			expectError: true,
		},
		{
			name: "Invalid Cache TTL",
			jwtConfig: &settings.JWTConfig{
				HeaderName: "X-Test-JWT",
				JWKSetURL:  "https://test.example.com/.well-known/jwks.json",
				CacheTTL:   "invalid",
			},
			token: generateTestToken(map[string]interface{}{
				"email": "custom@example.com",
			}),
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
