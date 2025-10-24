package oidc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/settings"
)

// TestAzureGroupsOverflowIntegration tests the complete end-to-end flow
func TestAzureGroupsOverflowIntegration(t *testing.T) {
	// Create mock Graph API server
	graphServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and headers
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.NotEmpty(t, r.Header.Get("Authorization"))

		// Verify request body
		var requestBody map[string]bool
		err := json.NewDecoder(r.Body).Decode(&requestBody)
		require.NoError(t, err)
		assert.True(t, requestBody["securityEnabledOnly"])

		// Return successful response with groups
		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{
			"value": []string{"group1", "group2", "group3", "group4", "group5"},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer graphServer.Close()

	// Override the Graph API endpoint for testing
	originalEndpoint := graphAPIGetMemberGroupsEndpoint
	graphAPIGetMemberGroupsEndpoint = graphServer.URL
	defer func() {
		graphAPIGetMemberGroupsEndpoint = originalEndpoint
	}()

	// Test configuration
	config := &settings.AzureOIDCConfig{
		EnableGroupsOverflowResolution: true,
		MaxGroupsLimit:                 1000,
	}

	// Test ID token claims with overflow indicators
	idTokenClaims := jwt.MapClaims{
		"sub":            "user123",
		"email":          "user@example.com",
		"_claim_sources": map[string]any{"src1": "https://example.com"},
		"_claim_names":   `{"groups":"src1"}`,
	}

	// Test access token with User.Read scope
	accessToken := createTestJWT(t, jwt.MapClaims{
		"scp": "User.Read profile email",
		"sub": "user123",
	})

	// Test the complete resolution flow
	ctx := context.Background()
	groups, err := ResolveAzureGroupsOverflow(ctx, idTokenClaims, accessToken, config)

	// Verify results
	require.NoError(t, err)
	assert.Equal(t, []string{"group1", "group2", "group3", "group4", "group5"}, groups)
}

// TestAzureGroupsOverflowIntegrationWithFailure tests the flow when Graph API fails
func TestAzureGroupsOverflowIntegrationWithFailure(t *testing.T) {
	// Create mock Graph API server that returns an error
	graphServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer graphServer.Close()

	// Override the Graph API endpoint for testing
	originalEndpoint := graphAPIGetMemberGroupsEndpoint
	graphAPIGetMemberGroupsEndpoint = graphServer.URL
	defer func() {
		graphAPIGetMemberGroupsEndpoint = originalEndpoint
	}()

	// Test configuration
	config := &settings.AzureOIDCConfig{
		EnableGroupsOverflowResolution: true,
		MaxGroupsLimit:                 1000,
	}

	// Test ID token claims with overflow indicators
	idTokenClaims := jwt.MapClaims{
		"sub":            "user123",
		"email":          "user@example.com",
		"_claim_sources": map[string]any{"src1": "https://example.com"},
		"_claim_names":   `{"groups":"src1"}`,
	}

	// Test access token with User.Read scope
	accessToken := createTestJWT(t, jwt.MapClaims{
		"scp": "User.Read profile email",
		"sub": "user123",
	})

	// Test the complete resolution flow
	ctx := context.Background()
	groups, err := ResolveAzureGroupsOverflow(ctx, idTokenClaims, accessToken, config)

	// Verify that error is returned and no groups are resolved
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient permissions for Graph API")
	assert.Nil(t, groups)
}

// TestAzureGroupsOverflowIntegrationExceedsLimit tests the flow when groups exceed the limit
func TestAzureGroupsOverflowIntegrationExceedsLimit(t *testing.T) {
	// Create mock Graph API server that returns too many groups
	graphServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Return more than 1000 groups
		groups := make([]string, 1001)
		for i := 0; i < 1001; i++ {
			groups[i] = "group" + string(rune(i))
		}

		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{
			"value": groups,
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer graphServer.Close()

	// Override the Graph API endpoint for testing
	originalEndpoint := graphAPIGetMemberGroupsEndpoint
	graphAPIGetMemberGroupsEndpoint = graphServer.URL
	defer func() {
		graphAPIGetMemberGroupsEndpoint = originalEndpoint
	}()

	// Test configuration with low limit
	config := &settings.AzureOIDCConfig{
		EnableGroupsOverflowResolution: true,
		MaxGroupsLimit:                 1000,
	}

	// Test ID token claims with overflow indicators
	idTokenClaims := jwt.MapClaims{
		"sub":            "user123",
		"email":          "user@example.com",
		"_claim_sources": map[string]any{"src1": "https://example.com"},
		"_claim_names":   `{"groups":"src1"}`,
	}

	// Test access token with User.Read scope
	accessToken := createTestJWT(t, jwt.MapClaims{
		"scp": "User.Read profile email",
		"sub": "user123",
	})

	// Test the complete resolution flow
	ctx := context.Background()
	groups, err := ResolveAzureGroupsOverflow(ctx, idTokenClaims, accessToken, config)

	// Verify that error is returned due to exceeding limit
	require.Error(t, err)
	assert.Contains(t, err.Error(), "group count 1001 exceeds maximum limit 1000")
	assert.Nil(t, groups)
}

// TestAzureGroupsOverflowIntegrationDisabled tests the flow when overflow resolution is disabled
func TestAzureGroupsOverflowIntegrationDisabled(t *testing.T) {
	// Test configuration with overflow resolution disabled
	config := &settings.AzureOIDCConfig{
		EnableGroupsOverflowResolution: false,
		MaxGroupsLimit:                 1000,
	}

	// Test ID token claims with overflow indicators
	idTokenClaims := jwt.MapClaims{
		"sub":            "user123",
		"email":          "user@example.com",
		"_claim_sources": map[string]any{"src1": "https://example.com"},
		"_claim_names":   `{"groups":"src1"}`,
	}

	// Test access token with User.Read scope
	accessToken := createTestJWT(t, jwt.MapClaims{
		"scp": "User.Read profile email",
		"sub": "user123",
	})

	// Test the complete resolution flow
	ctx := context.Background()
	groups, err := ResolveAzureGroupsOverflow(ctx, idTokenClaims, accessToken, config)

	// Verify that error is returned because resolution is disabled
	require.Error(t, err)
	assert.Contains(t, err.Error(), "groups overflow resolution is disabled")
	assert.Nil(t, groups)
}

// TestAzureGroupsOverflowIntegrationNoOverflow tests the flow when no overflow is detected
func TestAzureGroupsOverflowIntegrationNoOverflow(t *testing.T) {
	// Test configuration
	config := &settings.AzureOIDCConfig{
		EnableGroupsOverflowResolution: true,
		MaxGroupsLimit:                 1000,
	}

	// Test ID token claims without overflow indicators
	idTokenClaims := jwt.MapClaims{
		"sub":   "user123",
		"email": "user@example.com",
		// No _claim_sources or _claim_names
	}

	// Test access token with User.Read scope
	accessToken := createTestJWT(t, jwt.MapClaims{
		"scp": "User.Read profile email",
		"sub": "user123",
	})

	// Test the complete resolution flow
	ctx := context.Background()
	groups, err := ResolveAzureGroupsOverflow(ctx, idTokenClaims, accessToken, config)

	// Verify that error is returned because no overflow is detected
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no groups overflow to resolve")
	assert.Nil(t, groups)
}

// TestAzureGroupsOverflowIntegrationMissingScope tests the flow when access token lacks User.Read scope
func TestAzureGroupsOverflowIntegrationMissingScope(t *testing.T) {
	// Test configuration
	config := &settings.AzureOIDCConfig{
		EnableGroupsOverflowResolution: true,
		MaxGroupsLimit:                 1000,
	}

	// Test ID token claims with overflow indicators
	idTokenClaims := jwt.MapClaims{
		"sub":            "user123",
		"email":          "user@example.com",
		"_claim_sources": map[string]any{"src1": "https://example.com"},
		"_claim_names":   `{"groups":"src1"}`,
	}

	// Test access token without User.Read scope
	accessToken := createTestJWT(t, jwt.MapClaims{
		"scp": "profile email", // Missing User.Read
		"sub": "user123",
	})

	// Test the complete resolution flow
	ctx := context.Background()
	groups, err := ResolveAzureGroupsOverflow(ctx, idTokenClaims, accessToken, config)

	// Verify that error is returned because User.Read scope is missing
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access token missing User.Read scope")
	assert.Nil(t, groups)
}

// TestAzureGroupsOverflowIntegrationExistingGroups tests the flow when groups claim already exists
func TestAzureGroupsOverflowIntegrationExistingGroups(t *testing.T) {
	// Test configuration
	config := &settings.AzureOIDCConfig{
		EnableGroupsOverflowResolution: true,
		MaxGroupsLimit:                 1000,
	}

	// Test ID token claims with existing groups and overflow indicators
	idTokenClaims := jwt.MapClaims{
		"sub":            "user123",
		"email":          "user@example.com",
		"groups":         []string{"existing-group1", "existing-group2"}, // Existing groups
		"_claim_sources": map[string]any{"src1": "https://example.com"},
		"_claim_names":   `{"groups":"src1"}`,
	}

	// Test access token with User.Read scope
	accessToken := createTestJWT(t, jwt.MapClaims{
		"scp": "User.Read profile email",
		"sub": "user123",
	})

	// Test the complete resolution flow
	ctx := context.Background()
	groups, err := ResolveAzureGroupsOverflow(ctx, idTokenClaims, accessToken, config)

	// Verify that error is returned because groups claim already exists
	require.Error(t, err)
	assert.Contains(t, err.Error(), "groups claim already exists")
	assert.Nil(t, groups)
}

// TestAzureGroupsOverflowIntegrationInvalidClaims tests the flow with invalid overflow indicators
func TestAzureGroupsOverflowIntegrationInvalidClaims(t *testing.T) {
	// Test configuration
	config := &settings.AzureOIDCConfig{
		EnableGroupsOverflowResolution: true,
		MaxGroupsLimit:                 1000,
	}

	// Test ID token claims with invalid overflow indicators (referenced source not in _claim_sources)
	idTokenClaims := jwt.MapClaims{
		"sub":            "user123",
		"email":          "user@example.com",
		"_claim_names":   `{"groups":"src1"}`,
		"_claim_sources": map[string]any{"src2": "https://example.com"}, // src1 not present
	}

	// Test access token with User.Read scope
	accessToken := createTestJWT(t, jwt.MapClaims{
		"scp": "User.Read profile email",
		"sub": "user123",
	})

	// Test the complete resolution flow
	ctx := context.Background()
	groups, err := ResolveAzureGroupsOverflow(ctx, idTokenClaims, accessToken, config)

	// Verify that error is returned because overflow indicators are invalid
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid overflow indicators")
	assert.Nil(t, groups)
}
