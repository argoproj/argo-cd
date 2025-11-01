package jwt

import (
	"testing"

	jwtgo "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasAzureGroupsOverflow(t *testing.T) {
	tests := []struct {
		name     string
		claims   jwtgo.MapClaims
		expected bool
	}{
		{
			name: "has Azure groups overflow",
			claims: jwtgo.MapClaims{
				"sub": "user123",
				"_claim_names": map[string]any{
					"groups": "src1",
				},
				"_claim_sources": map[string]any{
					"src1": map[string]any{
						"endpoint": "https://graph.microsoft.com/v1.0/me/getMemberGroups",
					},
				},
			},
			expected: true,
		},
		{
			name: "no Azure groups overflow - normal groups",
			claims: jwtgo.MapClaims{
				"sub":    "user123",
				"groups": []string{"admin"},
			},
			expected: false,
		},
		{
			name: "has distributed claims but not for groups",
			claims: jwtgo.MapClaims{
				"sub": "user123",
				"_claim_names": map[string]any{
					"roles": "src1",
				},
				"_claim_sources": map[string]any{
					"src1": map[string]any{
						"endpoint": "https://example.com/claims",
					},
				},
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
			result := HasAzureGroupsOverflow(tt.claims)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetAzureGroupsOverflowInfo(t *testing.T) {
	tests := []struct {
		name        string
		claims      jwtgo.MapClaims
		expected    *AzureGroupsOverflowInfo
		expectError bool
	}{
		{
			name: "valid Azure groups overflow info",
			claims: jwtgo.MapClaims{
				"sub": "user123",
				"_claim_names": map[string]any{
					"groups": "src1",
				},
				"_claim_sources": map[string]any{
					"src1": map[string]any{
						"endpoint":     "https://graph.microsoft.com/v1.0/me/getMemberGroups",
						"access_token": "token123",
					},
				},
			},
			expected: &AzureGroupsOverflowInfo{
				GraphEndpoint: "https://graph.microsoft.com/v1.0/me/getMemberGroups",
				AccessToken:   "token123",
			},
			expectError: false,
		},
		{
			name: "app+user token ignores legacy endpoint and constructs correct URL",
			claims: jwtgo.MapClaims{
				"sub": "user123",
				"oid": "22222222-2222-2222-2222-222222222222",
				"_claim_names": map[string]any{
					"groups": "src1",
				},
				"_claim_sources": map[string]any{
					"src1": map[string]any{
						"endpoint":     "https://graph.windows.net/11111111-1111-1111-1111-111111111111/users/22222222-2222-2222-2222-222222222222/getMemberObjects",
						"access_token": "token123",
					},
				},
				// No idtyp claim means app+user token
			},
			expected: &AzureGroupsOverflowInfo{
				GraphEndpoint: "https://graph.microsoft.com/v1.0/me/getMemberGroups",
				AccessToken:   "token123",
			},
			expectError: false,
		},
		{
			name: "app-only token constructs correct URL with user ID",
			claims: jwtgo.MapClaims{
				"sub":   "user123",
				"oid":   "22222222-2222-2222-2222-222222222222",
				"idtyp": "app",
				"_claim_names": map[string]any{
					"groups": "src1",
				},
				"_claim_sources": map[string]any{
					"src1": map[string]any{
						"endpoint":     "https://graph.windows.net/11111111-1111-1111-1111-111111111111/users/22222222-2222-2222-2222-222222222222/getMemberObjects",
						"access_token": "token123",
					},
				},
			},
			expected: &AzureGroupsOverflowInfo{
				GraphEndpoint: "https://graph.microsoft.com/v1.0/users/22222222-2222-2222-2222-222222222222/getMemberGroups",
				AccessToken:   "token123",
			},
			expectError: false,
		},
		{
			name: "no Azure groups overflow",
			claims: jwtgo.MapClaims{
				"sub": "user123",
			},
			expected:    nil,
			expectError: false,
		},
		{
			name: "groups not in distributed claims",
			claims: jwtgo.MapClaims{
				"sub": "user123",
				"_claim_names": map[string]any{
					"roles": "src1",
				},
				"_claim_sources": map[string]any{
					"src1": map[string]any{
						"endpoint": "https://example.com/claims",
					},
				},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetAzureGroupsOverflowInfo(tt.claims)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestConstructMicrosoftGraphGroupsEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		claims   jwtgo.MapClaims
		expected string
	}{
		{
			name: "app+user token (normal user authentication)",
			claims: jwtgo.MapClaims{
				"sub": "user@example.com",
				"oid": "22222222-2222-2222-2222-222222222222",
				// No idtyp claim means it's an app+user token
			},
				expected: "https://graph.microsoft.com/v1.0/me/getMemberGroups",
		},
		{
			name: "app-only token with oid claim",
			claims: jwtgo.MapClaims{
				"sub":   "service-principal-id",
				"oid":   "33333333-3333-3333-3333-333333333333",
				"idtyp": "app",
			},
				expected: "https://graph.microsoft.com/v1.0/users/33333333-3333-3333-3333-333333333333/getMemberGroups",
		},
		{
			name: "app-only token without oid claim (fallback)",
			claims: jwtgo.MapClaims{
				"sub":   "service-principal-id",
				"idtyp": "app",
			},
				expected: "https://graph.microsoft.com/v1.0/me/getMemberGroups",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := constructMicrosoftGraphGroupsEndpoint(tt.claims)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAzureGroupsOverflowScenario(t *testing.T) {
	// Test case simulating Azure AD groups overflow
	claims := jwtgo.MapClaims{
		"sub":   "user@example.com",
		"name":  "Test User",
		"email": "user@example.com",
		"_claim_names": map[string]any{
			"groups": "src1",
		},
		"_claim_sources": map[string]any{
			"src1": map[string]any{
				"endpoint":     "https://graph.microsoft.com/v1.0/me/getMemberGroups",
				"access_token": "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9...",
			},
		},
	}

	// Test detection
	assert.True(t, HasAzureGroupsOverflow(claims))

	// Test Azure groups overflow info extraction
	azureInfo, err := GetAzureGroupsOverflowInfo(claims)
	require.NoError(t, err)
	require.NotNil(t, azureInfo)
	assert.Equal(t, "https://graph.microsoft.com/v1.0/me/getMemberGroups", azureInfo.GraphEndpoint)
	assert.Equal(t, "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9...", azureInfo.AccessToken)
}
