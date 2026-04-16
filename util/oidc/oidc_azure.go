package oidc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/util/cache"
	"github.com/argoproj/argo-cd/v3/util/crypto"
	jwtutil "github.com/argoproj/argo-cd/v3/util/jwt"
)

const (
	// AzureGroupsOverageResponseCachePrefix is the cache key prefix for Azure AD groups overage responses.
	AzureGroupsOverageResponseCachePrefix = "azure_groups_overage_response"

	// requiredGraphScope is the OAuth2 scope needed to call the Microsoft Graph API for group membership.
	requiredGraphScope = "User.Read"

	// memberGroupsEndpoint is the Graph API endpoint that returns group IDs for the signed-in user.
	memberGroupsEndpoint = "/me/getMemberGroups"
)

// GetUserGroupsFromAzureOverageClaim detects the Azure AD groups overage claim and, when present,
// fetches the user's group memberships from the Microsoft Graph API. Results are encrypted and cached.
// Returns nil groups (not an error) when no overage is detected.
func (a *ClientApp) GetUserGroupsFromAzureOverageClaim(ctx context.Context, groupClaims jwt.MapClaims, graphAPIURL string) ([]string, error) {
	sub := jwtutil.StringField(groupClaims, "sub")

	if !hasAzureGroupsClaimOverflow(groupClaims) {
		log.Debugf("Azure AD groups overage claim not detected for user %s", sub)
		return nil, nil
	}

	log.Infof("Azure AD groups overage claim detected for user %s, resolving via Graph API", sub)

	// Check cache first
	clientCacheKey := FormatAzureGroupsOverageResponseCacheKey(sub)
	cachedGroupsBytes, err := a.GetValueFromEncryptedCache(ctx, clientCacheKey)
	if err != nil {
		log.Warnf("failed to read Azure groups overage cache for %s: %v", sub, err)
	}
	if cachedGroupsBytes != nil {
		var groups []string
		if err := json.Unmarshal(cachedGroupsBytes, &groups); err == nil {
			return groups, nil
		}
		log.Errorf("cannot unmarshal cached Azure groups for %s: %v", sub, err)
	}

	// Get access token from cache
	accessTokenBytes, err := a.GetValueFromEncryptedCache(ctx, FormatAccessTokenCacheKey(sub))
	if err != nil {
		return nil, fmt.Errorf("could not read access token from cache for %s: %w", sub, err)
	}
	if accessTokenBytes == nil {
		return nil, fmt.Errorf("no access token cached for %s, user must re-authenticate", sub)
	}

	accessTokenStr := string(accessTokenBytes)
	if !hasUserReadScope(accessTokenStr) {
		return nil, errors.New("access token missing User.Read scope; ensure the User.Read delegated permission is granted on the app registration in Azure AD")
	}

	graphURL := strings.TrimSuffix(graphAPIURL, "/") + memberGroupsEndpoint
	groups, err := a.fetchGroupsFromGraphAPI(ctx, accessTokenStr, graphURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch groups from Graph API for %s: %w", sub, err)
	}

	log.Infof("Successfully resolved %d groups for user %s via Graph API", len(groups), sub)

	// Cache the result
	a.cacheAzureGroupsOverageResponse(clientCacheKey, groups, groupClaims)

	return groups, nil
}

// cacheAzureGroupsOverageResponse encrypts and caches the resolved groups.
func (a *ClientApp) cacheAzureGroupsOverageResponse(cacheKey string, groups []string, claims jwt.MapClaims) {
	rawGroups, err := json.Marshal(groups)
	if err != nil {
		log.Errorf("could not marshal groups to json for caching: %v", err)
		return
	}
	encGroups, err := crypto.Encrypt(rawGroups, a.encryptionKey)
	if err != nil {
		log.Errorf("could not encrypt groups for caching: %v", err)
		return
	}

	var cacheExpiry time.Duration
	tokenExpiry := GetTokenExpiration(claims)
	settingExpiry := a.settings.AzureUserGroupOverageClaimCacheExpiration()
	if settingExpiry > 0 && settingExpiry < tokenExpiry {
		cacheExpiry = settingExpiry
	} else {
		cacheExpiry = tokenExpiry
	}

	if err := a.clientCache.Set(&cache.Item{
		Key:    cacheKey,
		Object: encGroups,
		CacheActionOpts: cache.CacheActionOpts{
			Expiration: cacheExpiry,
		},
	}); err != nil {
		log.Errorf("could not cache Azure groups overage response: %v", err)
	}
}

// fetchGroupsFromGraphAPI calls Microsoft Graph API POST /me/getMemberGroups to retrieve group IDs.
// securityEnabledOnly is set to true to return only security groups (not distribution lists),
// which is appropriate for RBAC evaluation.
func (a *ClientApp) fetchGroupsFromGraphAPI(ctx context.Context, accessToken, graphURL string) ([]string, error) {
	requestBody := map[string]bool{
		"securityEnabledOnly": true,
	}
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create Graph API request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("graph API request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var response struct {
			Value []string `json:"value"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return nil, fmt.Errorf("failed to decode Graph API response: %w", err)
		}
		return response.Value, nil

	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, fmt.Errorf("insufficient permissions for Graph API (status %d)", resp.StatusCode)

	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("graph API rate limited (status %d)", resp.StatusCode)

	default:
		return nil, fmt.Errorf("graph API request failed with status %d", resp.StatusCode)
	}
}

// hasAzureGroupsClaimOverflow checks if the ID token contains Azure AD groups overage indicators.
// Azure AD sets _claim_names and _claim_sources when the user has more than 200 group memberships.
func hasAzureGroupsClaimOverflow(claims jwt.MapClaims) bool {
	claimSources, hasClaimSources := claims["_claim_sources"]
	claimNames, hasClaimNames := claims["_claim_names"]

	if !hasClaimSources || !hasClaimNames || claimSources == nil || claimNames == nil {
		return false
	}

	claimNamesMap, ok := claimNames.(map[string]any)
	if !ok {
		return false
	}

	_, hasGroups := claimNamesMap["groups"]
	return hasGroups
}

// hasUserReadScope checks if the access token contains the User.Read scope needed for Graph API calls.
// The access token is parsed without verification since we only need to inspect the scp claim.
func hasUserReadScope(accessToken string) bool {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	claims := jwt.MapClaims{}

	_, _, err := parser.ParseUnverified(accessToken, &claims)
	if err != nil {
		log.Warnf("Failed to parse access token for scope check: %v", err)
		return false
	}

	scpRaw, exists := claims["scp"]
	if !exists {
		return false
	}

	switch scp := scpRaw.(type) {
	case string:
		for scope := range strings.FieldsSeq(scp) {
			if strings.EqualFold(scope, requiredGraphScope) {
				return true
			}
		}
	case []any:
		for _, scopeInterface := range scp {
			if scopeStr, ok := scopeInterface.(string); ok {
				if strings.EqualFold(scopeStr, requiredGraphScope) {
					return true
				}
			}
		}
	}

	return false
}

// FormatAzureGroupsOverageResponseCacheKey returns the cache key for storing Azure AD groups overage responses.
func FormatAzureGroupsOverageResponseCacheKey(sub string) string {
	return fmt.Sprintf("%s_%s", AzureGroupsOverageResponseCachePrefix, sub)
}
