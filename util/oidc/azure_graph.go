package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	log "github.com/sirupsen/logrus"

	jwtutil "github.com/argoproj/argo-cd/v3/util/jwt"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// Graph API endpoint for getting member groups
var graphAPIGetMemberGroupsEndpoint = "https://graph.microsoft.com/v1.0/me/getMemberGroups"

const (
	// Default timeout for Graph API calls
	defaultGraphAPITimeout = 5 * time.Second

	// Default maximum groups limit
	defaultMaxGroupsLimit = 1000

	// Required scope for Graph API calls
	requiredScope = "User.Read"
)

// hasGroupsOverflow checks if there is a groups overflow that needs to be resolved
func hasGroupsOverflow(claims jwt.MapClaims) (bool, error) {
	// Check if _claim_names exists
	claimNamesRaw, exists := claims["_claim_names"]
	if !exists {
		return false, nil // No overflow indicators
	}

	// Parse _claim_names as JSON string
	claimNamesStr, ok := claimNamesRaw.(string)
	if !ok {
		return false, errors.New("_claim_names is not a string")
	}

	var claimNames map[string]string
	if err := json.Unmarshal([]byte(claimNamesStr), &claimNames); err != nil {
		return false, fmt.Errorf("failed to parse _claim_names JSON: %w", err)
	}

	// Check if groups key exists (if not, no groups overflow to resolve)
	groupsSource, hasGroups := claimNames["groups"]
	if !hasGroups {
		return false, nil // No groups overflow
	}

	// Validate that _claim_sources exists and contains the referenced source
	claimSourcesRaw, exists := claims["_claim_sources"]
	if !exists {
		return false, errors.New("_claim_sources not found")
	}

	claimSources, ok := claimSourcesRaw.(map[string]any)
	if !ok {
		return false, errors.New("_claim_sources is not a map")
	}

	// Check if the referenced source exists in _claim_sources
	if _, hasSource := claimSources[groupsSource]; !hasSource {
		return false, fmt.Errorf("_claim_sources does not contain referenced source '%s'", groupsSource)
	}

	// Check if groups claim already exists (should not happen with overflow)
	if _, hasGroupsClaim := claims["groups"]; hasGroupsClaim {
		return false, errors.New("groups claim already exists, overflow indicators may be invalid")
	}

	return true, nil // Valid groups overflow detected
}

// hasUserReadScope checks if the access token contains the User.Read scope
func hasUserReadScope(accessToken string) bool {
	// Parse the access token JWT without verification (we only need the claims)
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	claims := jwt.MapClaims{}

	_, _, err := parser.ParseUnverified(accessToken, &claims)
	if err != nil {
		log.Warnf("Failed to parse access token for scope check: %v", err)
		return false
	}

	// Check for scp claim
	scpRaw, exists := claims["scp"]
	if !exists {
		return false
	}

	// Handle both string and array formats
	switch scp := scpRaw.(type) {
	case string:
		// Split by spaces and check for User.Read
		scopes := strings.Fields(scp)
		for _, scope := range scopes {
			if strings.EqualFold(scope, requiredScope) {
				return true
			}
		}
	case []any:
		// Handle array format
		for _, scopeInterface := range scp {
			if scopeStr, ok := scopeInterface.(string); ok {
				if strings.EqualFold(scopeStr, requiredScope) {
					return true
				}
			}
		}
	}

	return false
}

// fetchGroupsFromGraphAPI calls Microsoft Graph API to get user's groups
func fetchGroupsFromGraphAPI(ctx context.Context, accessToken string, timeout time.Duration) ([]string, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: timeout,
	}

	// Prepare request body
	requestBody := map[string]bool{
		"securityEnabledOnly": true,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphAPIGetMemberGroupsEndpoint, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create Graph API request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("graph API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle different response codes
	switch resp.StatusCode {
	case http.StatusOK:
		// Success - parse response
		var response struct {
			Value []string `json:"value"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return nil, fmt.Errorf("failed to decode Graph API response: %w", err)
		}

		return response.Value, nil

	case http.StatusUnauthorized, http.StatusForbidden:
		// No permission - this is expected for some users
		return nil, fmt.Errorf("insufficient permissions for Graph API (status %d)", resp.StatusCode)

	case http.StatusTooManyRequests:
		// Rate limited
		return nil, fmt.Errorf("graph API rate limited (status %d)", resp.StatusCode)

	default:
		// Other errors
		return nil, fmt.Errorf("graph API request failed with status %d", resp.StatusCode)
	}
}

// resolveAzureGroupsOverflow is the main orchestration function for resolving groups overflow
func resolveAzureGroupsOverflow(ctx context.Context, idTokenClaims jwt.MapClaims, accessToken string, config *settings.AzureOIDCConfig) ([]string, error) {
	// Check if overflow resolution is enabled
	if config == nil || !config.EnableGroupsOverflowResolution {
		return nil, errors.New("groups overflow resolution is disabled")
	}

	// Check for groups overflow
	hasOverflow, err := hasGroupsOverflow(idTokenClaims)
	if err != nil {
		return nil, fmt.Errorf("invalid overflow indicators: %w", err)
	}
	if !hasOverflow {
		return nil, errors.New("no groups overflow to resolve")
	}

	// Check if access token has User.Read scope
	if !hasUserReadScope(accessToken) {
		return nil, errors.New("access token missing User.Read scope")
	}

	// Get max groups limit (default to 1000 if not configured)
	maxLimit := config.MaxGroupsLimit
	if maxLimit <= 0 {
		maxLimit = defaultMaxGroupsLimit
	}

	// Create timeout context
	timeout := defaultGraphAPITimeout
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Call Graph API
	groups, err := fetchGroupsFromGraphAPI(timeoutCtx, accessToken, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch groups from Graph API: %w", err)
	}

	// Apply business logic limit
	if len(groups) > maxLimit {
		log.Warnf("User has %d groups, exceeds limit of %d, discarding all groups", len(groups), maxLimit)
		return nil, fmt.Errorf("group count %d exceeds maximum limit %d", len(groups), maxLimit)
	}

	return groups, nil
}

// ResolveAzureGroupsOverflow is the public function to resolve Azure groups overflow
func ResolveAzureGroupsOverflow(ctx context.Context, idTokenClaims jwt.MapClaims, accessToken string, config *settings.AzureOIDCConfig) ([]string, error) {
	userSub := jwtutil.StringField(idTokenClaims, "sub")

	// Log overflow detection
	log.Infof("Azure AD groups overflow detected for user %s, resolving via Graph API", userSub)

	// Call the internal resolution function
	groups, err := resolveAzureGroupsOverflow(ctx, idTokenClaims, accessToken, config)
	if err != nil {
		log.Errorf("Failed to resolve groups via Graph API for user %s: %v", userSub, err)
		return nil, err
	}

	// Log successful resolution
	log.Infof("Successfully resolved %d groups for user %s", len(groups), userSub)

	return groups, nil
}
