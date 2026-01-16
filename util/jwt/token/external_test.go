package token

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	jwtgo "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"gopkg.in/square/go-jose.v2"

	"github.com/argoproj/argo-cd/v3/util/settings"
)

// Helper function to generate a JWT for testing
func generateTestToken(signingMethod jwtgo.SigningMethod, key any, kid string, claims map[string]any) string {
	token := jwtgo.New(signingMethod)
	if kid != "" {
		token.Header["kid"] = kid
	}

	defaultClaims := jwtgo.MapClaims{
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

	tokenString, err := token.SignedString(key)
	if err != nil {
		log.Fatalf("Failed to sign token: %v", err)
	}
	return tokenString
}

func TestVerify(t *testing.T) {
	const (
		kid           = "test-key-id"
		validAudience = "test-audience"
		validIssuer   = "https://test.example.com"
	)

	// Generate key pairs
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey := &privateKey.PublicKey

	wrongPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// --- Mock JWKS Server ---
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/jwks.json" {
			jwk := jose.JSONWebKey{
				Key:       publicKey, // Serve the correct public key
				KeyID:     kid,
				Algorithm: string(jose.RS256),
				Use:       "sig",
			}
			jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}}
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(jwks)
			require.NoError(t, err)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()
	// --- End Mock JWKS Server ---

	// --- Base JWT Config ---
	baseJwtConfig := &settings.JWTConfig{
		HeaderName:    "X-Test-JWT",
		EmailClaim:    "email",
		UsernameClaim: "sub",
		JWKSetURL:     ts.URL + "/.well-known/jwks.json",
		CacheTTL:      "1m",
		Audience:      validAudience,
		Issuer:        validIssuer,
	}
	// --- End Base JWT Config ---

	// --- Base Claims ---
	baseClaims := map[string]any{
		"iss": validIssuer,
		"aud": validAudience,
		"exp": time.Now().Add(time.Hour).Unix(),
		"nbf": time.Now().Add(-time.Minute).Unix(),
		"iat": time.Now().Add(-time.Minute).Unix(),
	}
	// --- End Base Claims ---

	// --- Test Cases ---
	tests := []struct {
		name          string
		jwtConfig     *settings.JWTConfig // Optional override
		claims        map[string]any      // Optional override
		signingMethod jwtgo.SigningMethod // Default: RS256
		signingKey    any                 // Default: privateKey
		signingKid    string              // Default: kid
		expectError   bool
		errorContains string // Substring to check in error message
	}{
		// --- Success Cases ---
		{
			name: "Valid Token",
		},
		{
			name: "Valid Token with Multiple Audiences",
			claims: map[string]any{
				"aud": []any{validAudience, "other-audience"},
			},
		},
		{
			name:      "Valid Token with No Audience Configured",
			jwtConfig: &settings.JWTConfig{Audience: ""}, // Override base config
			claims:    map[string]any{"aud": validAudience},
		},
		{
			name:      "Valid Token with No Issuer Configured",
			jwtConfig: &settings.JWTConfig{Issuer: ""}, // Override base config
			claims:    map[string]any{"iss": validIssuer},
		},
		{
			name:      "Valid Token with No Audience Claim and No Audience Configured",
			jwtConfig: &settings.JWTConfig{Audience: ""}, // Override base config
			claims:    map[string]any{"aud": nil},
		},
		{
			name:      "Valid Token with No Issuer Claim and No Issuer Configured",
			jwtConfig: &settings.JWTConfig{Issuer: ""}, // Override base config
			claims:    map[string]any{"iss": nil},
		},
		{
			name:      "Valid Token with groups claim",
			jwtConfig: &settings.JWTConfig{GroupsClaim: "groups"}, // Override base config
			claims:    map[string]any{"groups": []string{"group1", "group2"}},
		},
		{
			name:      "Valid Token with nested groups claim",
			jwtConfig: &settings.JWTConfig{GroupsClaim: "nested.groups"}, // Override base config
			claims:    map[string]any{"nested": map[string]any{"groups": []string{"group1", "group2"}}},
		},
		// --- Failure Cases: Claims ---
		{
			name:          "Invalid Audience Claim",
			claims:        map[string]any{"aud": "wrong-audience"},
			expectError:   true,
			errorContains: "invalid audience claim in external JWT",
		},
		{
			name:          "Invalid Issuer Claim",
			claims:        map[string]any{"iss": "wrong-issuer"},
			expectError:   true,
			errorContains: "invalid issuer claim in external JWT",
		},
		{
			name:          "Expired Token",
			claims:        map[string]any{"exp": time.Now().Add(-time.Hour).Unix()},
			expectError:   true,
			errorContains: jwtgo.ErrTokenExpired.Error(),
		},
		{
			name:          "Token Not Yet Valid (nbf)",
			claims:        map[string]any{"nbf": time.Now().Add(time.Hour).Unix()},
			expectError:   true,
			errorContains: jwtgo.ErrTokenNotValidYet.Error(),
		},
		{
			name:          "Missing Required Email Claim",
			jwtConfig:     &settings.JWTConfig{EmailClaim: "missing_email"}, // Override base config
			expectError:   false,                                            // Currently only logs a warning
			errorContains: "",                                               // No error expected, just a log
		},
		{
			name:          "Missing Required Username Claim",
			jwtConfig:     &settings.JWTConfig{UsernameClaim: "missing_user"}, // Override base config
			expectError:   false,                                              // Currently only logs a warning
			errorContains: "",                                                 // No error expected, just a log
		},
		// --- Failure Cases: Signature & Algorithm ---
		{
			name:          "Invalid Signature (Wrong Key)",
			signingKey:    wrongPrivateKey,
			expectError:   true,
			errorContains: "failed to parse/verify external JWT",
		},
		{
			name:          "Algorithm None (Bypass Attempt)",
			signingMethod: jwtgo.SigningMethodNone,
			signingKey:    jwtgo.UnsafeAllowNoneSignatureType,
			expectError:   true,
			errorContains: "failed to parse/verify external JWT",
		},
		{
			name:          "Mismatched Default Signing Algorithm (e.g., HS256 VS RS256)",
			signingMethod: jwtgo.SigningMethodHS256,
			signingKey:    []byte("some-secret"),
			expectError:   true,
			errorContains: "failed to parse/verify external JWT",
		},
		{
			name:          "Mismatched Configured Signing Algorithm (e.g., ES256 VS RS256)",
			jwtConfig:     &settings.JWTConfig{SigningAlgorithm: "ES256"},
			signingMethod: jwtgo.SigningMethodHS256,
			signingKey:    []byte("some-secret"),
			expectError:   true,
			errorContains: "failed to parse/verify external JWT",
		},
		// --- Failure Cases: Key ID (kid) ---
		{
			name:          "Missing KID in Token Header",
			signingKid:    "",
			expectError:   true,
			errorContains: "kid header not found in external JWT",
		},
		{
			name:          "KID in Token Header Not Found in JWKS",
			signingKid:    "non-existent-kid", // Generate token with wrong kid
			expectError:   true,
			errorContains: "no key found for kid in external JWT",
		},
	}
	// --- End Test Cases ---

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			// --- Prepare Test Data ---
			currentClaims := make(map[string]any)
			for k, v := range baseClaims {
				currentClaims[k] = v
			}
			if tt.claims != nil {
				for k, v := range tt.claims {
					if v == nil {
						delete(currentClaims, k) // Allow removing claims for testing
					} else {
						currentClaims[k] = v
					}
				}
			}

			currentJwtConfig := *baseJwtConfig // Copy base config
			if tt.jwtConfig != nil {
				// Apply overrides selectively
				if tt.jwtConfig.Audience != "" || baseJwtConfig.Audience != "" { // Check if override or base has Audience
					currentJwtConfig.Audience = tt.jwtConfig.Audience
				}
				if tt.jwtConfig.Issuer != "" || baseJwtConfig.Issuer != "" { // Check if override or base has Issuer
					currentJwtConfig.Issuer = tt.jwtConfig.Issuer
				}
				if tt.jwtConfig.EmailClaim != "" {
					currentJwtConfig.EmailClaim = tt.jwtConfig.EmailClaim
				}
				if tt.jwtConfig.UsernameClaim != "" {
					currentJwtConfig.UsernameClaim = tt.jwtConfig.UsernameClaim
				}
				if tt.jwtConfig.GroupsClaim != "" {
					currentJwtConfig.GroupsClaim = tt.jwtConfig.GroupsClaim
				}
				// Add other fields if needed
			}

			// Declare signingMethod as the interface type
			var signingMethod jwtgo.SigningMethod = jwtgo.SigningMethodRS256
			if tt.signingMethod != nil {
				signingMethod = tt.signingMethod // Now assigning interface to interface variable
			}

			signingKey := any(privateKey)
			if tt.signingKey != nil {
				signingKey = tt.signingKey
			}

			signingKid := kid
			if tt.signingKid != "" {
				signingKid = tt.signingKid
			} else if tt.name == "Missing KID in Token Header" {
				signingKid = ""
			}
			// --- End Prepare Test Data ---

			// Generate token
			tokenString := generateTestToken(signingMethod, signingKey, signingKid, currentClaims)
			t.Logf("Test: %s, Generated Token: %s", tt.name, tokenString)

			// Create provider and settings
			// Use a real issuer URL for the provider, JWKS URL is in JWTConfig
			verifier := NewExternalTokenVerifier(http.DefaultClient)
			argoSettings := &settings.ArgoCDSettings{
				JWTConfig: &currentJwtConfig,
			}

			// Verify the JWT
			jwtClaims, err := verifier.Verify(t.Context(), tokenString, argoSettings)

			// Assertions
			if tt.expectError {
				require.Error(t, err, "Expected an error but got none")
				if tt.errorContains != "" {
					require.ErrorContains(t, err, tt.errorContains, "Error message mismatch")
				}
				t.Logf("Expected error occurred: %v", err)
			} else {
				require.NoError(t, err, "Expected no error but got: %v", err)
				require.NotNil(t, jwtClaims, "Claims should not be nil on success")
				claims, ok := jwtClaims.(jwtgo.MapClaims)
				require.True(t, ok, "Claims are not of type MapClaims")

				// Basic claim checks on success
				if currentJwtConfig.EmailClaim != "" && currentClaims[currentJwtConfig.EmailClaim] != nil {
					require.Contains(t, claims, currentJwtConfig.EmailClaim, "Email claim missing")
				}
				if currentJwtConfig.UsernameClaim != "" && currentClaims[currentJwtConfig.UsernameClaim] != nil {
					require.Contains(t, claims, currentJwtConfig.UsernameClaim, "Username claim missing")
				}
				if currentJwtConfig.GroupsClaim != "" {
					groupsPath := strings.Split(currentJwtConfig.GroupsClaim, ".")
					if currentClaims[groupsPath[0]] != nil {
						// groups were extracted
						require.Contains(t, claims, "groups", "Groups claim missing")
						// groups match
						var expectedGroups []string
						if len(groupsPath) > 1 {
							expectedGroups = currentClaims[groupsPath[0]].(map[string]any)[groupsPath[1]].([]string)
						} else {
							expectedGroups = currentClaims[groupsPath[0]].([]string)
						}
						require.Equal(t, expectedGroups, claims["groups"], "Groups claim value mismatch")
					}
				}
				t.Logf("JWT verified successfully.")
			}
		})
	}
}

// TestVerifyJWT_Cache tests the JWKS caching mechanism
func TestVerifyJWT_Cache(t *testing.T) {
	const kid = "cache-test-key"
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey := &privateKey.PublicKey

	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/jwks.json" {
			requestCount++ // Increment counter on each JWKS request
			jwk := jose.JSONWebKey{Key: publicKey, KeyID: kid, Algorithm: string(jose.RS256), Use: "sig"}
			jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(jwks)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	jwtConfig := &settings.JWTConfig{
		JWKSetURL:     ts.URL + "/.well-known/jwks.json",
		HeaderName:    "JWT-Assertion",
		CacheTTL:      "1s", // Short TTL for testing
		EmailClaim:    "email",
		UsernameClaim: "sub",
	}
	argoSettings := &settings.ArgoCDSettings{JWTConfig: jwtConfig}
	verifier := NewExternalTokenVerifier(http.DefaultClient) // Issuer doesn't matter here

	claims := map[string]any{"exp": time.Now().Add(time.Hour).Unix()}
	tokenString := generateTestToken(jwtgo.SigningMethodRS256, privateKey, kid, claims)

	// First verification - should fetch JWKS
	_, err = verifier.Verify(t.Context(), tokenString, argoSettings)
	require.NoError(t, err)
	require.Equal(t, 1, requestCount, "JWKS should be fetched on first call")

	// Second verification - should use cache
	_, err = verifier.Verify(t.Context(), tokenString, argoSettings)
	require.NoError(t, err)
	require.Equal(t, 1, requestCount, "JWKS should be cached on second call")

	// Wait for cache to expire
	time.Sleep(1100 * time.Millisecond) // Wait slightly longer than TTL

	// Third verification - should fetch JWKS again
	_, err = verifier.Verify(t.Context(), tokenString, argoSettings)
	require.NoError(t, err)
	require.Equal(t, 2, requestCount, "JWKS should be fetched again after cache expiry")
}
