package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
	"gopkg.in/square/go-jose.v2"

	"github.com/argoproj/argo-cd/v2/util/settings"
)

func generateTestTokenWithKey(privateKey *rsa.PrivateKey, claims map[string]interface{}) string {
	// Create token with custom headers
	token := jwt.New(jwt.SigningMethodRS256)
	token.Header["kid"] = "test-key-id"

	// Set default claims
	defaultClaims := jwt.MapClaims{
		"sub":   "test-user",
		"email": "test@example.com",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
		"iss":   "https://test.example.com",
	}

	// Merge provided claims with defaults
	for k, v := range claims {
		defaultClaims[k] = v
	}

	token.Claims = defaultClaims

	// Sign with provided private key
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		log.Fatalf("Failed to sign token: %v", err)
	}
	return tokenString
}

func TestVerifyJWT(t *testing.T) {
	const (
		kid           = "test-key-id"
		validAudience = "test-audience"
	)

	// Generate key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey := &privateKey.PublicKey

	// Create test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Mock server received request: %s %s", r.Method, r.URL.Path)

		if r.URL.Path == "/.well-known/jwks.json" {
			// Use jose library to create the JWKS
			jwk := jose.JSONWebKey{
				Key:       publicKey,
				KeyID:     kid,
				Algorithm: string(jose.RS256),
				Use:       "sig",
			}
			jwks := jose.JSONWebKeySet{
				Keys: []jose.JSONWebKey{jwk},
			}

			// Serve the JWKS
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(jwks)
			require.NoError(t, err)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	tests := []struct {
		name        string
		jwtConfig   *settings.JWTConfig
		claims      map[string]interface{}
		expectError bool
	}{
		{
			name: "Valid Token",
			jwtConfig: &settings.JWTConfig{
				HeaderName:    "X-Test-JWT",
				EmailClaim:    "email",
				UsernameClaim: "sub",
				JWKSetURL:     ts.URL + "/.well-known/jwks.json",
				CacheTTL:      "invalid", // This will trigger the warning
			},
			claims: map[string]interface{}{
				"iss": ts.URL, // Match issuer with test server URL
			},
			expectError: false,
		},
		{
			name: "Valid Token with Matching Audience",
			jwtConfig: &settings.JWTConfig{
				HeaderName:    "X-Test-JWT",
				EmailClaim:    "email",
				UsernameClaim: "sub",
				JWKSetURL:     ts.URL + "/.well-known/jwks.json",
				Audience:      validAudience,
			},
			claims: map[string]interface{}{
				"iss": ts.URL,
				"aud": validAudience,
			},
			expectError: false,
		},
		{
			name: "Invalid Audience",
			jwtConfig: &settings.JWTConfig{
				HeaderName:    "X-Test-JWT",
				EmailClaim:    "email",
				UsernameClaim: "sub",
				JWKSetURL:     ts.URL + "/.well-known/jwks.json",
				Audience:      validAudience,
			},
			claims: map[string]interface{}{
				"iss": ts.URL,
				"aud": "wrong-audience",
			},
			expectError: true,
		},
		{
			name: "Valid Token with Multiple Audiences",
			jwtConfig: &settings.JWTConfig{
				HeaderName:    "X-Test-JWT",
				EmailClaim:    "email",
				UsernameClaim: "sub",
				JWKSetURL:     ts.URL + "/.well-known/jwks.json",
				Audience:      validAudience,
			},
			claims: map[string]interface{}{
				"iss": ts.URL,
				"aud": []interface{}{validAudience, "other-audience"},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			// Generate token for each test case
			tokenString := generateTestTokenWithKey(privateKey, tt.claims)
			t.Logf("Generated Token: %s", tokenString)

			provider := NewOIDCProvider(ts.URL, nil)
			settings := &settings.ArgoCDSettings{
				JWTConfig: tt.jwtConfig,
			}

			// Verify the JWT using the updated VerifyJWT method
			token, err := provider.VerifyJWT(tokenString, settings)
			if tt.expectError {
				require.Error(t, err)
				t.Logf("Expected error: %v", err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, token)
				claims, ok := token.Claims.(jwt.MapClaims)
				require.True(t, ok, "claims are not of type MapClaims")
				require.Equal(t, "test@example.com", claims["email"])
				require.Equal(t, "test-user", claims["sub"])
				t.Logf("JWT verified successfully.")
			}
		})
	}
}
