package jwt

import (
	"testing"

	jwtgo "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasDistributedClaims(t *testing.T) {
	tests := []struct {
		name     string
		claims   jwtgo.MapClaims
		expected bool
	}{
		{
			name: "has distributed claims",
			claims: jwtgo.MapClaims{
				"sub": "user123",
				"_claim_names": map[string]any{
					"groups": "src1",
				},
				"_claim_sources": map[string]any{
					"src1": map[string]any{
						"endpoint": "https://example.com/claims",
					},
				},
			},
			expected: true,
		},
		{
			name: "no distributed claims",
			claims: jwtgo.MapClaims{
				"sub":    "user123",
				"groups": []string{"admin"},
			},
			expected: false,
		},
		{
			name: "only claim names, no sources",
			claims: jwtgo.MapClaims{
				"sub": "user123",
				"_claim_names": map[string]any{
					"groups": "src1",
				},
			},
			expected: false,
		},
		{
			name: "only claim sources, no names",
			claims: jwtgo.MapClaims{
				"sub": "user123",
				"_claim_sources": map[string]any{
					"src1": map[string]any{
						"endpoint": "https://example.com/claims",
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasDistributedClaims(tt.claims)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetClaimSources(t *testing.T) {
	tests := []struct {
		name        string
		claims      jwtgo.MapClaims
		expected    map[string]ClaimSource
		expectError bool
	}{
		{
			name: "valid claim sources",
			claims: jwtgo.MapClaims{
				"sub": "user123",
				"_claim_names": map[string]any{
					"groups": "src1",
				},
				"_claim_sources": map[string]any{
					"src1": map[string]any{
						"endpoint":     "https://example.com/claims",
						"access_token": "token123",
					},
					"src2": map[string]any{
						"endpoint": "https://other.com/claims",
					},
				},
			},
			expected: map[string]ClaimSource{
				"src1": {
					Endpoint:    "https://example.com/claims",
					AccessToken: "token123",
				},
				"src2": {
					Endpoint: "https://other.com/claims",
				},
			},
			expectError: false,
		},
		{
			name: "no distributed claims",
			claims: jwtgo.MapClaims{
				"sub": "user123",
			},
			expected:    nil,
			expectError: false,
		},
		{
			name: "invalid claim sources format",
			claims: jwtgo.MapClaims{
				"sub": "user123",
				"_claim_names": map[string]any{
					"groups": "src1",
				},
				"_claim_sources": "invalid",
			},
			expected:    nil,
			expectError: true,
		},
		{
			name: "source without endpoint",
			claims: jwtgo.MapClaims{
				"sub": "user123",
				"_claim_names": map[string]any{
					"groups": "src1",
				},
				"_claim_sources": map[string]any{
					"src1": map[string]any{
						"access_token": "token123",
					},
				},
			},
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetClaimSources(tt.claims)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetDistributedClaimNames(t *testing.T) {
	tests := []struct {
		name        string
		claims      jwtgo.MapClaims
		expected    map[string]string
		expectError bool
	}{
		{
			name: "valid claim names",
			claims: jwtgo.MapClaims{
				"sub": "user123",
				"_claim_names": map[string]any{
					"groups":  "src1",
					"roles":   "src2",
					"profile": "src1",
				},
				"_claim_sources": map[string]any{
					"src1": map[string]any{
						"endpoint": "https://example.com/claims",
					},
				},
			},
			expected: map[string]string{
				"groups":  "src1",
				"roles":   "src2",
				"profile": "src1",
			},
			expectError: false,
		},
		{
			name: "no distributed claims",
			claims: jwtgo.MapClaims{
				"sub": "user123",
			},
			expected:    nil,
			expectError: false,
		},
		{
			name: "invalid claim names format",
			claims: jwtgo.MapClaims{
				"sub": "user123",
				"_claim_names": "invalid",
				"_claim_sources": map[string]any{
					"src1": map[string]any{
						"endpoint": "https://example.com/claims",
					},
				},
			},
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetDistributedClaimNames(tt.claims)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestDistributedClaimsAzureAdScenario(t *testing.T) {
	// Test case simulating Azure AD distributed claims for groups
	claims := jwtgo.MapClaims{
		"sub":   "user@example.com",
		"name":  "Test User",
		"email": "user@example.com",
		"_claim_names": map[string]any{
			"groups": "src1",
		},
		"_claim_sources": map[string]any{
			"src1": map[string]any{
				"endpoint":     "https://graph.microsoft.com/v1.0/me/memberOf",
				"access_token": "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9...",
			},
		},
	}

	// Test detection
	assert.True(t, HasDistributedClaims(claims))

	// Test source extraction
	sources, err := GetClaimSources(claims)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	assert.Equal(t, "https://graph.microsoft.com/v1.0/me/memberOf", sources["src1"].Endpoint)
	assert.Equal(t, "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9...", sources["src1"].AccessToken)

	// Test claim names extraction
	claimNames, err := GetDistributedClaimNames(claims)
	require.NoError(t, err)
	require.Len(t, claimNames, 1)
	assert.Equal(t, "src1", claimNames["groups"])
}