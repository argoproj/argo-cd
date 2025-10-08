package oidc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	jwtgo "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/settings"
)

// Create a mock settings that implements the methods we need
func createMockSettings(azureGroupsOverflowEnabled bool, timeout time.Duration) *settings.ArgoCDSettings {
	timeoutStr := timeout.String()
	enabledStr := "false"
	if azureGroupsOverflowEnabled {
		enabledStr = "true"
	}

	oidcConfigRaw := fmt.Sprintf(`
name: Test
issuer: https://example.com
clientID: test-client
enableAzureGroupsOverflow: %s
azureGroupsOverflowTimeout: "%s"
`, enabledStr, timeoutStr)

	return &settings.ArgoCDSettings{
		OIDCConfigRAW: oidcConfigRaw,
	}
}

func TestFetchAzureGroupsOverflow(t *testing.T) {
	// Create a mock server for Microsoft Graph API endpoint
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// We expect the call to be made to our constructed Microsoft Graph endpoint
		// Since we ignore the legacy endpoint from _claim_sources
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/me/getMemberObjects") // app+user token uses /me endpoint

		// Verify Content-Type
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Check authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Verify request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var requestBody map[string]interface{}
		if err := json.Unmarshal(body, &requestBody); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if securityEnabled, exists := requestBody["securityEnabledOnly"]; exists {
			assert.Equal(t, true, securityEnabled)
		}

		// Return Microsoft Graph API response format
		response := map[string]interface{}{
			"value": []string{"group1", "group2", "admin"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create mock settings
	mockSettings := createMockSettings(true, 10*time.Second)

	// Create client app with custom transport to redirect Microsoft Graph calls to our mock
	transport := &mockTransport{
		mockServer: mockServer,
	}
	httpClient := &http.Client{Transport: transport}

	client := &ClientApp{
		client:   httpClient,
		settings: mockSettings,
	}

	// Test claims with Azure groups overflow
	// Note: We need oid claim for constructing Microsoft Graph endpoint
	claims := jwtgo.MapClaims{
		"sub":   "user123",
		"email": "user@example.com",
		"oid":   "22222222-2222-2222-2222-222222222222",
		"_claim_names": map[string]interface{}{
			"groups": "src1",
		},
		"_claim_sources": map[string]interface{}{
			"src1": map[string]interface{}{
				// This endpoint will be ignored as per Microsoft's recommendation
				// We construct our own Microsoft Graph endpoint based on idtyp claim
				"endpoint":     "https://graph.windows.net/old-tenant/users/old-user/getMemberObjects",
				"access_token": "test-token",
			},
		},
	}

	// Test fetching Azure groups overflow
	enrichedClaims, err := client.FetchAzureGroupsOverflow(claims)
	require.NoError(t, err)

	// Verify claims were enriched
	assert.Equal(t, "user123", enrichedClaims["sub"])
	assert.Equal(t, "user@example.com", enrichedClaims["email"])

	// Check groups were added
	groupsRaw, exists := enrichedClaims["groups"]
	require.True(t, exists, "groups claim should exist")

	groups, ok := groupsRaw.([]string)
	require.True(t, ok, "groups should be a string array")

	assert.Contains(t, groups, "group1")
	assert.Contains(t, groups, "group2")
	assert.Contains(t, groups, "admin")
}

func TestFetchAzureGroupsOverflowDisabled(t *testing.T) {
	// Create mock settings with Azure groups overflow disabled
	mockSettings := createMockSettings(false, 10*time.Second)

	client := &ClientApp{
		client:   http.DefaultClient,
		settings: mockSettings,
	}

	claims := jwtgo.MapClaims{
		"sub": "user123",
		"_claim_names": map[string]interface{}{
			"groups": "src1",
		},
		"_claim_sources": map[string]interface{}{
			"src1": map[string]interface{}{
				"endpoint": "https://graph.microsoft.com/v1.0/me/getMemberObjects",
			},
		},
	}

	// Should return original claims unchanged
	enrichedClaims, err := client.FetchAzureGroupsOverflow(claims)
	require.NoError(t, err)
	assert.Equal(t, claims, enrichedClaims)
}

func TestFetchAzureGroupsOverflowTimeout(t *testing.T) {
	// Create a mock server that hangs
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Longer than timeout
	}))
	defer mockServer.Close()

	// Create mock settings with short timeout
	mockSettings := createMockSettings(true, 100*time.Millisecond)

	client := &ClientApp{
		client:   http.DefaultClient,
		settings: mockSettings,
	}

	claims := jwtgo.MapClaims{
		"sub": "user123",
		"_claim_names": map[string]interface{}{
			"groups": "src1",
		},
		"_claim_sources": map[string]interface{}{
			"src1": map[string]interface{}{
				"endpoint": mockServer.URL,
			},
		},
	}

	// Should return original claims due to timeout (graceful fallback)
	enrichedClaims, err := client.FetchAzureGroupsOverflow(claims)
	require.NoError(t, err)

	// Original claims should be preserved
	assert.Equal(t, "user123", enrichedClaims["sub"])
	// groups claim should not be present since fetch failed
	_, hasGroups := enrichedClaims["groups"]
	assert.False(t, hasGroups)
}

func TestFetchAzureGroupsOverflowUnauthorized(t *testing.T) {
	// Create a mock server that returns 401
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer mockServer.Close()

	mockSettings := createMockSettings(true, 10*time.Second)

	client := &ClientApp{
		client:   http.DefaultClient,
		settings: mockSettings,
	}

	claims := jwtgo.MapClaims{
		"sub": "user123",
		"_claim_names": map[string]interface{}{
			"groups": "src1",
		},
		"_claim_sources": map[string]interface{}{
			"src1": map[string]interface{}{
				"endpoint":     mockServer.URL,
				"access_token": "invalid-token",
			},
		},
	}

	// Should return original claims due to authorization failure (graceful fallback)
	enrichedClaims, err := client.FetchAzureGroupsOverflow(claims)
	require.NoError(t, err)
	assert.Equal(t, claims["sub"], enrichedClaims["sub"])
}

func TestFetchAzureGroupsOverflowNoAccessToken(t *testing.T) {
	mockSettings := createMockSettings(true, 10*time.Second)

	client := &ClientApp{
		client:   http.DefaultClient,
		settings: mockSettings,
	}

	claims := jwtgo.MapClaims{
		"sub": "user123",
		"_claim_names": map[string]interface{}{
			"groups": "src1",
		},
		"_claim_sources": map[string]interface{}{
			"src1": map[string]interface{}{
				"endpoint": "https://graph.microsoft.com/v1.0/me/getMemberObjects",
				// No access_token
			},
		},
	}

	// Should return original claims when no access token is provided (graceful fallback)
	enrichedClaims, err := client.FetchAzureGroupsOverflow(claims)
	require.NoError(t, err)
	assert.Equal(t, "user123", enrichedClaims["sub"])
	// groups should not be added due to missing access token
	_, hasGroups := enrichedClaims["groups"]
	assert.False(t, hasGroups)
}

func TestFetchAzureGroupsOverflowRealScenario(t *testing.T) {
	// Create a mock Microsoft Graph API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// We expect the call to be made to our constructed Microsoft Graph endpoint
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/me/getMemberObjects") // app+user token uses /me endpoint

		// Verify Content-Type
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify Authorization header
		auth := r.Header.Get("Authorization")
		assert.Equal(t, "Bearer azure-access-token", auth)

		// Verify request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var requestBody map[string]interface{}
		if err := json.Unmarshal(body, &requestBody); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if securityEnabled, exists := requestBody["securityEnabledOnly"]; exists {
			assert.Equal(t, true, securityEnabled)
		}

		// Return Microsoft Graph API response format
		response := map[string]interface{}{
			"value": []string{
				"12345678-1234-1234-1234-123456789abc",
				"87654321-4321-4321-4321-cba987654321",
				"11111111-2222-3333-4444-555555555555",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	mockSettings := createMockSettings(true, 10*time.Second)

	// Create client app with custom transport to redirect Microsoft Graph calls to our mock
	transport := &mockTransport{
		mockServer: mockServer,
	}
	httpClient := &http.Client{Transport: transport}

	client := &ClientApp{
		client:   httpClient,
		settings: mockSettings,
	}

	// Test claims with Azure AD groups overflow
	claims := jwtgo.MapClaims{
		"sub":   "user@contoso.com",
		"email": "user@contoso.com",
		"oid":   "33333333-3333-3333-3333-333333333333", // Need oid for Microsoft Graph endpoint construction
		"_claim_names": map[string]interface{}{
			"groups": "src1",
		},
		"_claim_sources": map[string]interface{}{
			"src1": map[string]interface{}{
				// This endpoint will be ignored per Microsoft's recommendation
				"endpoint":     "https://graph.windows.net/old-tenant/users/old-user/getMemberObjects",
				"access_token": "azure-access-token",
			},
		},
	}

	// Test fetching Azure groups overflow
	enrichedClaims, err := client.FetchAzureGroupsOverflow(claims)
	require.NoError(t, err)

	// Verify claims were enriched with Azure AD groups
	assert.Equal(t, "user@contoso.com", enrichedClaims["sub"])
	assert.Equal(t, "user@contoso.com", enrichedClaims["email"])

	groups, ok := enrichedClaims["groups"].([]string)
	require.True(t, ok, "groups should be a string array")
	assert.Len(t, groups, 3)
	assert.Contains(t, groups, "12345678-1234-1234-1234-123456789abc")
	assert.Contains(t, groups, "87654321-4321-4321-4321-cba987654321")
	assert.Contains(t, groups, "11111111-2222-3333-4444-555555555555")
}

// mockTransport redirects Microsoft Graph API calls to our mock server
type mockTransport struct {
	mockServer *httptest.Server
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect any Microsoft Graph API calls to our mock server
	if strings.Contains(req.URL.Host, "graph.microsoft.com") {
		// Replace the host but keep the path
		req.URL.Scheme = "http"
		req.URL.Host = strings.TrimPrefix(t.mockServer.URL, "http://")
	}
	return http.DefaultTransport.RoundTrip(req)
}
