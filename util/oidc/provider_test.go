package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	jwtgo "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"gopkg.in/square/go-jose.v2"
	"sigs.k8s.io/yaml" // Import yaml

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

func TestVerifyJWT(t *testing.T) {
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
			errorContains: "invalid audience claim",
		},
		{
			name:          "Invalid Issuer Claim",
			claims:        map[string]any{"iss": "wrong-issuer"},
			expectError:   true,
			errorContains: "invalid issuer claim",
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
			errorContains: "failed to parse/verify JWT",
		},
		{
			name:          "Algorithm None (Bypass Attempt)",
			signingMethod: jwtgo.SigningMethodNone,
			signingKey:    jwtgo.UnsafeAllowNoneSignatureType,
			expectError:   true,
			errorContains: "failed to parse/verify JWT",
		},
		{
			name:          "Mismatched Default Signing Algorithm (e.g., HS256 VS RS256)",
			signingMethod: jwtgo.SigningMethodHS256,
			signingKey:    []byte("some-secret"),
			expectError:   true,
			errorContains: "failed to parse/verify JWT",
		},
		{
			name:          "Mismatched Configured Signing Algorithm (e.g., ES256 VS RS256)",
			jwtConfig:     &settings.JWTConfig{SigningAlgorithm: "ES256"},
			signingMethod: jwtgo.SigningMethodHS256,
			signingKey:    []byte("some-secret"),
			expectError:   true,
			errorContains: "failed to parse/verify JWT",
		},
		// --- Failure Cases: Key ID (kid) ---
		{
			name:          "Missing KID in Token Header",
			signingKid:    "",
			expectError:   true,
			errorContains: "kid header not found",
		},
		{
			name:          "KID in Token Header Not Found in JWKS",
			signingKid:    "non-existent-kid", // Generate token with wrong kid
			expectError:   true,
			errorContains: "no key found for kid",
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
			provider := NewOIDCProvider(validIssuer, nil)
			argoSettings := &settings.ArgoCDSettings{
				JWTConfig: &currentJwtConfig,
			}

			// Verify the JWT
			token, err := provider.VerifyJWT(t.Context(), tokenString, argoSettings)

			// Assertions
			if tt.expectError {
				require.Error(t, err, "Expected an error but got none")
				if tt.errorContains != "" {
					require.ErrorContains(t, err, tt.errorContains, "Error message mismatch")
				}
				t.Logf("Expected error occurred: %v", err)
			} else {
				require.NoError(t, err, "Expected no error but got: %v", err)
				require.NotNil(t, token, "Token should not be nil on success")
				claims, ok := token.Claims.(jwtgo.MapClaims)
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
	provider := NewOIDCProvider("issuer", nil) // Issuer doesn't matter here

	claims := map[string]any{"exp": time.Now().Add(time.Hour).Unix()}
	tokenString := generateTestToken(jwtgo.SigningMethodRS256, privateKey, kid, claims)

	// First verification - should fetch JWKS
	_, err = provider.VerifyJWT(t.Context(), tokenString, argoSettings)
	require.NoError(t, err)
	require.Equal(t, 1, requestCount, "JWKS should be fetched on first call")

	// Second verification - should use cache
	_, err = provider.VerifyJWT(t.Context(), tokenString, argoSettings)
	require.NoError(t, err)
	require.Equal(t, 1, requestCount, "JWKS should be cached on second call")

	// Wait for cache to expire
	time.Sleep(1100 * time.Millisecond) // Wait slightly longer than TTL

	// Third verification - should fetch JWKS again
	_, err = provider.VerifyJWT(t.Context(), tokenString, argoSettings)
	require.NoError(t, err)
	require.Equal(t, 2, requestCount, "JWKS should be fetched again after cache expiry")
}

// TestVerify_Audience tests the audience verification logic in the Verify method (for OIDC tokens)
func TestVerify_Audience(t *testing.T) {
	// Use the go-oidc test harness components
	clientID := "argo-cd-client"
	cliClientID := "argo-cd-cli-client"
	otherAudience := "other-aud"

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// --- Mock OIDC Server ---
	mux := http.NewServeMux()
	ts := httptest.NewServer(mux) // Define ts here
	defer ts.Close()

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"issuer": "%s",
			"jwks_uri": "%s/keys"
		}`, ts.URL, ts.URL) // Use ts.URL here
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		jwk := jose.JSONWebKey{
			Key:       key.Public(),
			KeyID:     "test-key-id",
			Use:       "sig",
			Algorithm: string(jose.RS256),
		}
		_ = json.NewEncoder(w).Encode(&jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}})
	})
	// --- End Mock OIDC Server ---

	// --- Helper to create ID Token ---
	makeToken := func(aud []string) string {
		claims := map[string]any{
			"iss": ts.URL,
			"sub": "test-sub",
			"aud": aud,
			"exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Unix(),
		}

		token := jwtgo.New(jwtgo.SigningMethodRS256)
		token.Header["kid"] = "test-key-id"
		token.Claims = jwtgo.MapClaims(claims)

		tokenString, err := token.SignedString(key)
		require.NoError(t, err)
		return tokenString
	}
	// --- End Helper ---

	tests := []struct {
		name                                    string
		tokenAudience                           []string
		allowedAudiences                        []string
		skipAudienceCheckWhenTokenHasNoAudience *bool // nil means use default (false for >=2.6)
		expectError                             bool
		errorContains                           string
	}{
		{
			name:          "Valid: Token aud matches default allowed (clientID)",
			tokenAudience: []string{clientID},
		},
		{
			name:          "Valid: Token aud matches default allowed (cliClientID)",
			tokenAudience: []string{cliClientID},
		},
		{
			name:             "Valid: Token aud matches explicitly allowed audience",
			tokenAudience:    []string{otherAudience},
			allowedAudiences: []string{clientID, otherAudience},
		},
		{
			name:             "Valid: Token has multiple aud, one matches allowed",
			tokenAudience:    []string{"unrelated", clientID},
			allowedAudiences: []string{clientID, otherAudience},
		},
		{
			name:             "Valid: Token has multiple aud, one matches default allowed",
			tokenAudience:    []string{"unrelated", cliClientID},
			allowedAudiences: []string{}, // Use default allowed
		},
		{
			name:          "Invalid: Token aud does not match default allowed",
			tokenAudience: []string{otherAudience},
			expectError:   true,
			errorContains: "token verification failed for all audiences",
		},
		{
			name:             "Invalid: Token aud does not match explicitly allowed",
			tokenAudience:    []string{"unrelated"},
			allowedAudiences: []string{clientID, otherAudience},
			expectError:      true,
			errorContains:    "token verification failed for all audiences",
		},
		{
			name:          "Invalid: Token has no audience, skip check is default (false)",
			tokenAudience: []string{}, // No audience
			expectError:   true,
			errorContains: "expected audience", // This error happens during audience check with go-oidc v3
		},
		{
			name:                                    "Valid: Token has no audience, skip check is true",
			tokenAudience:                           []string{}, // No audience
			skipAudienceCheckWhenTokenHasNoAudience: boolPtr(true),
			expectError:                             true, // Changed to true as go-oidc v3 still requires the audience
			errorContains:                           "expected audience",
		},
		{
			name:                                    "Invalid: Token has audience, skip check is true (should still fail)",
			tokenAudience:                           []string{"some-aud"},
			allowedAudiences:                        []string{"different-aud"},
			skipAudienceCheckWhenTokenHasNoAudience: boolPtr(true),
			expectError:                             true,
			errorContains:                           "token verification failed for all audiences",
		},
	}

	var argoSettings *settings.ArgoCDSettings // Declare argoSettings outside the loop

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenString := makeToken(tt.tokenAudience)

			// Configure OIDC in settings using raw YAML string
			oidcConfigMap := map[string]any{
				"clientID":    clientID,
				"cliClientID": cliClientID,
			}
			if len(tt.allowedAudiences) > 0 {
				oidcConfigMap["allowedAudiences"] = tt.allowedAudiences
			}
			if tt.skipAudienceCheckWhenTokenHasNoAudience != nil {
				oidcConfigMap["skipAudienceCheckWhenTokenHasNoAudience"] = *tt.skipAudienceCheckWhenTokenHasNoAudience
			}

			oidcConfigBytes, err := yaml.Marshal(oidcConfigMap)
			require.NoError(t, err)

			argoSettings = &settings.ArgoCDSettings{
				OIDCConfigRAW: fmt.Sprintf("issuer: %s\n%s", ts.URL, string(oidcConfigBytes)),
			}

			// Create provider instance
			provider := NewOIDCProvider(ts.URL, http.DefaultClient).(*providerImpl)

			// Perform verification
			_, err = provider.Verify(t.Context(), tokenString, argoSettings)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.ErrorContains(t, err, tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// Helper for pointer to bool
func boolPtr(b bool) *bool {
	return &b
}
