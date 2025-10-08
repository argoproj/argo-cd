package oidc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwtgo "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/settings"
)

// Create a mock settings that implements the methods we need
func createMockSettings(distributedClaimsEnabled bool, timeout time.Duration) *settings.ArgoCDSettings {
	timeoutStr := timeout.String()
	enabledStr := "false"
	if distributedClaimsEnabled {
		enabledStr = "true"
	}
	
	return &settings.ArgoCDSettings{
		OIDCConfigRAW: fmt.Sprintf(`
name: Test
issuer: https://example.com
clientID: test-client
enableDistributedClaims: %s
distributedClaimsTimeout: "%s"
`, enabledStr, timeoutStr),
	}
}

func TestFetchDistributedClaims(t *testing.T) {
	// Create a mock server for distributed claims endpoint
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Return mock group claims
		response := map[string]interface{}{
			"sub":    "user123",
			"groups": []string{"group1", "group2", "admin"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create mock settings
	mockSettings := createMockSettings(true, 10*time.Second)

	// Create client app
	client := &ClientApp{
		client:   http.DefaultClient,
		settings: mockSettings,
	}

	// Test claims with distributed claims
	claims := jwtgo.MapClaims{
		"sub":   "user123",
		"email": "user@example.com",
		"_claim_names": map[string]interface{}{
			"groups": "src1",
		},
		"_claim_sources": map[string]interface{}{
			"src1": map[string]interface{}{
				"endpoint":     mockServer.URL,
				"access_token": "test-token",
			},
		},
	}

	// Test fetching distributed claims
	enrichedClaims, err := client.FetchDistributedClaims(claims)
	require.NoError(t, err)

	// Verify claims were enriched
	assert.Equal(t, "user123", enrichedClaims["sub"])
	assert.Equal(t, "user@example.com", enrichedClaims["email"])
	
	// When JSON is unmarshaled, arrays become []interface{} instead of []string
	groupsRaw, exists := enrichedClaims["groups"]
	require.True(t, exists, "groups claim should exist")
	
	groupsInterfaceSlice, ok := groupsRaw.([]interface{})
	require.True(t, ok, "groups should be an interface array")
	
	// Convert to string slice for verification
	groups := make([]string, len(groupsInterfaceSlice))
	for i, g := range groupsInterfaceSlice {
		groups[i] = g.(string)
	}
	
	assert.Contains(t, groups, "group1")
	assert.Contains(t, groups, "group2")
	assert.Contains(t, groups, "admin")
}

func TestFetchDistributedClaimsDisabled(t *testing.T) {
	// Create mock settings with distributed claims disabled
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
				"endpoint": "https://example.com/claims",
			},
		},
	}

	// Should return original claims unchanged
	enrichedClaims, err := client.FetchDistributedClaims(claims)
	require.NoError(t, err)
	assert.Equal(t, claims, enrichedClaims)
}

func TestFetchDistributedClaimsTimeout(t *testing.T) {
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
	enrichedClaims, err := client.FetchDistributedClaims(claims)
	require.NoError(t, err)
	
	// Original claims should be preserved
	assert.Equal(t, "user123", enrichedClaims["sub"])
	// groups claim should not be present since fetch failed
	_, hasGroups := enrichedClaims["groups"]
	assert.False(t, hasGroups)
}

func TestFetchDistributedClaimsUnauthorized(t *testing.T) {
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
	enrichedClaims, err := client.FetchDistributedClaims(claims)
	require.NoError(t, err)
	assert.Equal(t, claims["sub"], enrichedClaims["sub"])
}

func TestFetchDistributedClaimsSubjectMismatch(t *testing.T) {
	// Create a mock server that returns different subject
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"sub":    "different-user", // Different from original
			"groups": []string{"admin"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
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
				"endpoint": mockServer.URL,
			},
		},
	}

	// Should return original claims due to subject mismatch (graceful fallback)
	enrichedClaims, err := client.FetchDistributedClaims(claims)
	require.NoError(t, err)
	assert.Equal(t, "user123", enrichedClaims["sub"])
	// groups should not be added due to subject mismatch
	_, hasGroups := enrichedClaims["groups"]
	assert.False(t, hasGroups)
}

func TestFetchDistributedClaimsAzureAD(t *testing.T) {
	// Create a mock Azure Graph API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a POST request
		assert.Equal(t, "POST", r.Method)
		
		// Verify Content-Type
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		
		// Verify Authorization header
		auth := r.Header.Get("Authorization")
		assert.Equal(t, "Bearer azure-access-token", auth)
		
		// Verify request body
		body, _ := io.ReadAll(r.Body)
		var requestBody map[string]interface{}
		json.Unmarshal(body, &requestBody)
		assert.Equal(t, false, requestBody["securityEnabledOnly"])

		// Return Azure Graph API response format
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

	client := &ClientApp{
		client:   http.DefaultClient,
		settings: mockSettings,
	}

	// Test claims with Azure AD distributed claims
	// Use the mock server URL but with the GetMemberGroups path to trigger Azure AD logic
	azureEndpoint := mockServer.URL + "/me/GetMemberGroups"
	
	claims := jwtgo.MapClaims{
		"sub":   "user@contoso.com",
		"email": "user@contoso.com",
		"_claim_names": map[string]interface{}{
			"groups": "src1",
		},
		"_claim_sources": map[string]interface{}{
			"src1": map[string]interface{}{
				"endpoint":     azureEndpoint,
				"access_token": "azure-access-token",
			},
		},
	}

	// Test fetching distributed claims
	enrichedClaims, err := client.FetchDistributedClaims(claims)
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