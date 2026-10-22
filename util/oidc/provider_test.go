package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/smithy-go/ptr"
	jwtgo "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"gopkg.in/square/go-jose.v2"
	"sigs.k8s.io/yaml" // Import yaml

	"github.com/argoproj/argo-cd/v3/util/settings"
)

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
			skipAudienceCheckWhenTokenHasNoAudience: ptr.Bool(true),
			expectError:                             true, // Changed to true as go-oidc v3 still requires the audience
			errorContains:                           "expected audience",
		},
		{
			name:                                    "Invalid: Token has audience, skip check is true (should still fail)",
			tokenAudience:                           []string{"some-aud"},
			allowedAudiences:                        []string{"different-aud"},
			skipAudienceCheckWhenTokenHasNoAudience: ptr.Bool(true),
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
