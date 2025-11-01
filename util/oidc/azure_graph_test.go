package oidc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/settings"
)

func TestHasGroupsOverflow(t *testing.T) {
	tests := []struct {
		name      string
		claims    jwt.MapClaims
		expected  bool
		expectErr bool
		errMsg    string
	}{
		{
			name:      "no overflow indicators",
			claims:    jwt.MapClaims{"sub": "user123"},
			expected:  false,
			expectErr: false,
		},
		{
			name: "only _claim_sources",
			claims: jwt.MapClaims{
				"sub":            "user123",
				"_claim_sources": map[string]any{"src1": "https://example.com"},
			},
			expected:  false,
			expectErr: false,
		},
		{
			name: "only _claim_names",
			claims: jwt.MapClaims{
				"sub":          "user123",
				"_claim_names": `{"groups":"src1"}`,
			},
			expected:  false,
			expectErr: true,
			errMsg:    "_claim_sources not found",
		},
		{
			name: "valid groups overflow",
			claims: jwt.MapClaims{
				"sub":            "user123",
				"_claim_sources": map[string]any{"src1": "https://example.com"},
				"_claim_names":   `{"groups":"src1"}`,
			},
			expected:  true,
			expectErr: false,
		},
		{
			name: "multiple keys in _claim_names (valid)",
			claims: jwt.MapClaims{
				"sub":            "user123",
				"_claim_sources": map[string]any{"src1": "https://example.com", "src2": "https://example2.com"},
				"_claim_names":   `{"groups":"src1","roles":"src2"}`,
			},
			expected:  true,
			expectErr: false,
		},
		{
			name: "no groups key in _claim_names",
			claims: jwt.MapClaims{
				"sub":            "user123",
				"_claim_sources": map[string]any{"src1": "https://example.com"},
				"_claim_names":   `{"roles":"src1"}`,
			},
			expected:  false,
			expectErr: false,
		},
		{
			name: "missing _claim_sources",
			claims: jwt.MapClaims{
				"sub":          "user123",
				"_claim_names": `{"groups":"src1"}`,
			},
			expected:  false,
			expectErr: true,
			errMsg:    "_claim_sources not found",
		},
		{
			name: "_claim_sources not a map",
			claims: jwt.MapClaims{
				"sub":            "user123",
				"_claim_sources": "invalid",
				"_claim_names":   `{"groups":"src1"}`,
			},
			expected:  false,
			expectErr: true,
			errMsg:    "_claim_sources is not a map",
		},
		{
			name: "referenced source not in _claim_sources",
			claims: jwt.MapClaims{
				"sub":            "user123",
				"_claim_sources": map[string]any{"src2": "https://example.com"},
				"_claim_names":   `{"groups":"src1"}`,
			},
			expected:  false,
			expectErr: true,
			errMsg:    "_claim_sources does not contain referenced source 'src1'",
		},
		{
			name: "groups claim already exists",
			claims: jwt.MapClaims{
				"sub":            "user123",
				"_claim_sources": map[string]any{"src1": "https://example.com"},
				"_claim_names":   `{"groups":"src1"}`,
				"groups":         []string{"group1", "group2"},
			},
			expected:  false,
			expectErr: true,
			errMsg:    "groups claim already exists, overflow indicators may be invalid",
		},
		{
			name: "_claim_names not a string",
			claims: jwt.MapClaims{
				"sub":            "user123",
				"_claim_sources": map[string]any{"src1": "https://example.com"},
				"_claim_names":   map[string]string{"groups": "src1"},
			},
			expected:  false,
			expectErr: true,
			errMsg:    "_claim_names is not a string",
		},
		{
			name: "invalid JSON in _claim_names",
			claims: jwt.MapClaims{
				"sub":            "user123",
				"_claim_sources": map[string]any{"src1": "https://example.com"},
				"_claim_names":   `{"groups":"src1"`, // Missing closing brace
			},
			expected:  false,
			expectErr: true,
			errMsg:    "failed to parse _claim_names JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasOverflow, err := hasGroupsOverflow(tt.claims)
			assert.Equal(t, tt.expected, hasOverflow)
			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidateOverflowIndicators is now covered by TestHasGroupsOverflow

func TestHasUserReadScope(t *testing.T) {
	tests := []struct {
		name        string
		accessToken string
		expected    bool
		description string
	}{
		{
			name:        "valid token with User.Read scope",
			accessToken: createTestJWT(t, jwt.MapClaims{"scp": "User.Read profile email"}),
			expected:    true,
			description: "should find User.Read in space-separated scopes",
		},
		{
			name:        "valid token with User.Read in array",
			accessToken: createTestJWT(t, jwt.MapClaims{"scp": []string{"User.Read", "profile", "email"}}),
			expected:    true,
			description: "should find User.Read in array format",
		},
		{
			name:        "valid token without User.Read scope",
			accessToken: createTestJWT(t, jwt.MapClaims{"scp": "profile email"}),
			expected:    false,
			description: "should not find User.Read when not present",
		},
		{
			name:        "token with case insensitive User.Read",
			accessToken: createTestJWT(t, jwt.MapClaims{"scp": "user.read profile email"}),
			expected:    true,
			description: "should find User.Read case insensitively",
		},
		{
			name:        "token without scp claim",
			accessToken: createTestJWT(t, jwt.MapClaims{"sub": "user123"}),
			expected:    false,
			description: "should return false when scp claim is missing",
		},
		{
			name:        "invalid token",
			accessToken: "invalid.jwt.token",
			expected:    false,
			description: "should return false for invalid token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasUserReadScope(tt.accessToken)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

func TestFetchGroupsFromGraphAPI(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectedGroups []string
		expectErr      bool
		errMsg         string
	}{
		{
			name: "successful response with groups",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				response := map[string]any{
					"value": []string{"group1", "group2", "group3"},
				}
				_ = json.NewEncoder(w).Encode(response)
			},
			expectedGroups: []string{"group1", "group2", "group3"},
			expectErr:      false,
		},
		{
			name: "successful response with empty groups",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				response := map[string]any{
					"value": []string{},
				}
				_ = json.NewEncoder(w).Encode(response)
			},
			expectedGroups: []string{},
			expectErr:      false,
		},
		{
			name: "unauthorized response",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			expectedGroups: nil,
			expectErr:      true,
			errMsg:         "insufficient permissions for Graph API",
		},
		{
			name: "forbidden response",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
			expectedGroups: nil,
			expectErr:      true,
			errMsg:         "insufficient permissions for Graph API",
		},
		{
			name: "rate limited response",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusTooManyRequests)
			},
			expectedGroups: nil,
			expectErr:      true,
			errMsg:         "graph API rate limited",
		},
		{
			name: "server error response",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectedGroups: nil,
			expectErr:      true,
			errMsg:         "graph API request failed with status 500",
		},
		{
			name: "invalid JSON response",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("invalid json"))
			},
			expectedGroups: nil,
			expectErr:      true,
			errMsg:         "failed to decode Graph API response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			// Override the Graph API endpoint for testing
			originalEndpoint := graphAPIGetMemberGroupsEndpoint
			graphAPIGetMemberGroupsEndpoint = server.URL
			defer func() {
				graphAPIGetMemberGroupsEndpoint = originalEndpoint
			}()

			ctx := context.Background()
			accessToken := "test-access-token"
			timeout := 5 * time.Second

			groups, err := fetchGroupsFromGraphAPI(ctx, accessToken, timeout)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, groups)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedGroups, groups)
			}
		})
	}
}

func TestResolveAzureGroupsOverflow(t *testing.T) {
	tests := []struct {
		name        string
		config      *settings.AzureOIDCConfig
		claims      jwt.MapClaims
		accessToken string
		expectErr   bool
		errMsg      string
	}{
		{
			name:        "overflow resolution disabled",
			config:      &settings.AzureOIDCConfig{EnableGroupsOverflowResolution: false},
			claims:      jwt.MapClaims{"sub": "user123"},
			accessToken: "test-token",
			expectErr:   true,
			errMsg:      "groups overflow resolution is disabled",
		},
		{
			name:        "nil config",
			config:      nil,
			claims:      jwt.MapClaims{"sub": "user123"},
			accessToken: "test-token",
			expectErr:   true,
			errMsg:      "groups overflow resolution is disabled",
		},
		{
			name:        "no overflow indicators",
			config:      &settings.AzureOIDCConfig{EnableGroupsOverflowResolution: true},
			claims:      jwt.MapClaims{"sub": "user123"},
			accessToken: "test-token",
			expectErr:   true,
			errMsg:      "no groups overflow to resolve",
		},
		{
			name:   "invalid overflow indicators",
			config: &settings.AzureOIDCConfig{EnableGroupsOverflowResolution: true},
			claims: jwt.MapClaims{
				"sub":            "user123",
				"_claim_sources": map[string]any{"src1": "https://example.com"},
				"_claim_names":   "invalid",
			},
			accessToken: "test-token",
			expectErr:   true,
			errMsg:      "invalid overflow indicators",
		},
		{
			name:   "missing User.Read scope",
			config: &settings.AzureOIDCConfig{EnableGroupsOverflowResolution: true},
			claims: jwt.MapClaims{
				"sub":            "user123",
				"_claim_sources": map[string]any{"src1": "https://example.com"},
				"_claim_names":   `{"groups":"src1"}`,
			},
			accessToken: createTestJWT(t, jwt.MapClaims{"scp": "profile email"}),
			expectErr:   true,
			errMsg:      "access token missing User.Read scope",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			groups, err := resolveAzureGroupsOverflow(ctx, tt.claims, tt.accessToken, tt.config)

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, groups)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, groups)
			}
		})
	}
}

func TestResolveAzureGroupsOverflowWithMockServer(t *testing.T) {
	// Test successful resolution with mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and headers
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Bearer "))

		// Verify request body
		var requestBody map[string]bool
		err := json.NewDecoder(r.Body).Decode(&requestBody)
		require.NoError(t, err)
		assert.True(t, requestBody["securityEnabledOnly"])

		// Return successful response
		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{
			"value": []string{"group1", "group2", "group3"},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Override the Graph API endpoint for testing
	originalEndpoint := graphAPIGetMemberGroupsEndpoint
	graphAPIGetMemberGroupsEndpoint = server.URL
	defer func() {
		graphAPIGetMemberGroupsEndpoint = originalEndpoint
	}()

	config := &settings.AzureOIDCConfig{
		EnableGroupsOverflowResolution: true,
		MaxGroupsLimit:                 1000,
	}

	claims := jwt.MapClaims{
		"sub":            "user123",
		"_claim_sources": map[string]any{"src1": "https://example.com"},
		"_claim_names":   `{"groups":"src1"}`,
	}

	accessToken := createTestJWT(t, jwt.MapClaims{"scp": "User.Read profile email"})

	ctx := context.Background()
	groups, err := resolveAzureGroupsOverflow(ctx, claims, accessToken, config)

	require.NoError(t, err)
	assert.Equal(t, []string{"group1", "group2", "group3"}, groups)
}

func TestResolveAzureGroupsOverflowExceedsLimit(t *testing.T) {
	// Test when groups exceed the configured limit
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return more than 1000 groups
		groups := make([]string, 1001)
		for i := 0; i < 1001; i++ {
			groups[i] = "group" + string(rune(i))
		}
		response := map[string]any{
			"value": groups,
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Override the Graph API endpoint for testing
	originalEndpoint := graphAPIGetMemberGroupsEndpoint
	graphAPIGetMemberGroupsEndpoint = server.URL
	defer func() {
		graphAPIGetMemberGroupsEndpoint = originalEndpoint
	}()

	config := &settings.AzureOIDCConfig{
		EnableGroupsOverflowResolution: true,
		MaxGroupsLimit:                 1000,
	}

	claims := jwt.MapClaims{
		"sub":            "user123",
		"_claim_sources": map[string]any{"src1": "https://example.com"},
		"_claim_names":   `{"groups":"src1"}`,
	}

	accessToken := createTestJWT(t, jwt.MapClaims{"scp": "User.Read profile email"})

	ctx := context.Background()
	groups, err := resolveAzureGroupsOverflow(ctx, claims, accessToken, config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "group count 1001 exceeds maximum limit 1000")
	assert.Nil(t, groups)
}

// Helper function to create test JWT tokens
func createTestJWT(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("test-secret"))
	require.NoError(t, err)
	return tokenString
}
