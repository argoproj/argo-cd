package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/v3/util/cache"
	"github.com/argoproj/argo-cd/v3/util/crypto"
	jwtutil "github.com/argoproj/argo-cd/v3/util/jwt"
	"github.com/golang-jwt/jwt/v5"
	log "github.com/sirupsen/logrus"
)

const (
	// Required scope for Graph API calls
	requiredScope = "User.Read"
	// Graph API endpoint for getting member groups
	memberGroupsEndpoint = "/me/getMemberGroups"
)

// GetUserGroupsFromAzureOverageClaim returns the groups for a user with the Azure AD groups overage claim
func (a *ClientApp) GetUserGroupsFromAzureOverageClaim(groupClaims jwt.MapClaims, graphAPIURL string) ([]string, error) {
	sub := jwtutil.StringField(groupClaims, "sub")
	hasGroupsOverageClaim := hasGroupsOverageClaim(groupClaims)
	if !hasGroupsOverageClaim {
		log.Infof("Azure AD groups overage claim not detected for user %s", sub)
		return nil, nil
	}

	log.Infof("Azure AD groups overage claim detected for user %s, resolving via Graph API", sub)

	// in case we got it in the cache, we just return the item
	var encGroups []byte
	var groups []string
	clientCacheKey := FormatAzureGroupsOverageResponseCacheKey(sub)
	if err := a.clientCache.Get(clientCacheKey, &encGroups); err == nil {
		groupsRaw, err := crypto.Decrypt(encGroups, a.encryptionKey)
		if err != nil {
			log.Errorf("decrypting the cached groups failed (sub=%s): %s", sub, err)
		} else {
			err = json.Unmarshal(groupsRaw, &groups)
			if err == nil {
				// return the cached groups since they are not yet expired, were successfully decrypted and unmarshaled
				return groups, nil
			}
			log.Errorf("cannot unmarshal cached groups structure: %s", err)
		}
	}

	accessToken, err := a.getTokenFromCache(sub)
	if err != nil {
		return nil, fmt.Errorf("could not get accessToken from cache: %w", err)
	}

	accessTokenStr := string(accessToken)
	if !hasUserReadScope(accessTokenStr) {
		err := errors.New("access token missing User.Read scope")
		log.Errorf("Failed to resolve groups via Graph API for user %s: %v", sub, err)
		return nil, err
	}

	graphAPIGetMemberGroupsURL := strings.TrimSuffix(graphAPIURL, "/") + memberGroupsEndpoint
	groups, err = a.fetchGroupsFromGraphAPI(accessTokenStr, graphAPIGetMemberGroupsURL)
	if err != nil {
		err = fmt.Errorf("failed to fetch groups from Graph API: %w", err)
		log.Errorf("Failed to resolve groups via Graph API for user %s: %v", sub, err)
		return nil, err
	}

	log.Infof("Successfully resolved %d groups for user %s", len(groups), sub)

	// only use configured expiry if the token lives longer and the expiry is configured
	// otherwise use the expiry of the token
	var cacheExpiry time.Duration
	tokenExpiry := getTokenExpiration(groupClaims)
	settingExpiry := a.settings.AzureUserGroupOverageClaimCacheExpiration()
	if settingExpiry < tokenExpiry && settingExpiry != 0 {
		cacheExpiry = settingExpiry
	} else {
		cacheExpiry = tokenExpiry
	}

	rawGroups, err := json.Marshal(groups)
	if err != nil {
		return groups, fmt.Errorf("could not marshal claim to json: %w", err)
	}
	encGroups, err = crypto.Encrypt(rawGroups, a.encryptionKey)
	if err != nil {
		return groups, fmt.Errorf("could not encrypt user info response: %w", err)
	}

	err = a.clientCache.Set(&cache.Item{
		Key:    clientCacheKey,
		Object: encGroups,
		CacheActionOpts: cache.CacheActionOpts{
			Expiration: cacheExpiry,
		},
	})
	if err != nil {
		return groups, fmt.Errorf("could not put item to cache: %w", err)
	}

	return groups, nil
}

// fetchGroupsFromGraphAPI calls Microsoft Graph API to get user's groups
func (a *ClientApp) fetchGroupsFromGraphAPI(accessToken string, url string) ([]string, error) {
	requestBody := map[string]bool{
		"securityEnabledOnly": true,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, strings.NewReader(string(bodyBytes)))
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

// hasGroupsOverageClaim checks if there is a groups overage claim
func hasGroupsOverageClaim(claims jwt.MapClaims) bool {
	claimSources, hasClaimSources := claims["_claim_sources"]
	claimNames, hasClaimNames := claims["_claim_names"]

	if claimSources == nil || claimNames == nil || !hasClaimSources || !hasClaimNames {
		return false
	}

	if claimNamesMap, ok := claimNames.(map[string]any); ok {
		if _, hasGroups := claimNamesMap["groups"]; !hasGroups {
			return false
		}
	}

	return true
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
