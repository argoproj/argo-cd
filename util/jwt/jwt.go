package jwt

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	jwtgo "github.com/golang-jwt/jwt/v5"
)

// MapClaims converts a jwt.Claims to a MapClaims
func MapClaims(claims jwtgo.Claims) (jwtgo.MapClaims, error) {
	if mapClaims, ok := claims.(*jwtgo.MapClaims); ok {
		return *mapClaims, nil
	}
	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return nil, err
	}
	var mapClaims jwtgo.MapClaims
	err = json.Unmarshal(claimsBytes, &mapClaims)
	if err != nil {
		return nil, err
	}
	return mapClaims, nil
}

// StringField extracts a field from the claims as a string
func StringField(claims jwtgo.MapClaims, fieldName string) string {
	if fieldIf, ok := claims[fieldName]; ok {
		if field, ok := fieldIf.(string); ok {
			return field
		}
	}
	return ""
}

// Float64Field extracts a field from the claims as a float64
func Float64Field(claims jwtgo.MapClaims, fieldName string) float64 {
	if fieldIf, ok := claims[fieldName]; ok {
		if field, ok := fieldIf.(float64); ok {
			return field
		}
	}
	return 0
}

// GetScopeValues extracts the values of specified scopes from the claims
func GetScopeValues(claims jwtgo.MapClaims, scopes []string) []string {
	groups := make([]string, 0)
	for i := range scopes {
		scopeIf, ok := claims[scopes[i]]
		if !ok {
			continue
		}

		switch val := scopeIf.(type) {
		case []any:
			for _, groupIf := range val {
				group, ok := groupIf.(string)
				if ok {
					groups = append(groups, group)
				}
			}
		case []string:
			groups = append(groups, val...)
		case string:
			groups = append(groups, val)
		}
	}

	return groups
}

func numField(m jwtgo.MapClaims, key string) (int64, error) {
	field, ok := m[key]
	if !ok {
		return 0, fmt.Errorf("token does not have %s claim", key)
	}
	switch val := field.(type) {
	case float64:
		return int64(val), nil
	case json.Number:
		return val.Int64()
	case int64:
		return val, nil
	default:
		return 0, fmt.Errorf("%s '%v' is not a number", key, val)
	}
}

// IssuedAt returns the issued at as an int64
func IssuedAt(m jwtgo.MapClaims) (int64, error) {
	return numField(m, "iat")
}

// IssuedAtTime returns the issued at as a time.Time
func IssuedAtTime(m jwtgo.MapClaims) (time.Time, error) {
	iat, err := IssuedAt(m)
	return time.Unix(iat, 0), err
}

// ExpirationTime returns the expiration as a time.Time
func ExpirationTime(m jwtgo.MapClaims) (time.Time, error) {
	exp, err := numField(m, "exp")
	return time.Unix(exp, 0), err
}

func Claims(in any) jwtgo.Claims {
	claims, ok := in.(jwtgo.Claims)
	if ok {
		return claims
	}
	return nil
}

// IsMember returns whether or not the user's claims is a member of any of the groups
func IsMember(claims jwtgo.Claims, groups []string, scopes []string) bool {
	mapClaims, err := MapClaims(claims)
	if err != nil {
		return false
	}
	// O(n^2) loop
	for _, userGroup := range GetGroups(mapClaims, scopes) {
		for _, group := range groups {
			if userGroup == group {
				return true
			}
		}
	}
	return false
}

func GetGroups(mapClaims jwtgo.MapClaims, scopes []string) []string {
	return GetScopeValues(mapClaims, scopes)
}

func IsValid(token string) bool {
	return len(strings.SplitN(token, ".", 3)) == 3
}

// GetUserIdentifier returns a consistent user identifier, checking federated_claims.user_id when Dex is in use
func GetUserIdentifier(c jwtgo.MapClaims) string {
	if c == nil {
		return ""
	}

	// Fallback to sub if federated_claims.user_id is not set.
	fallback := StringField(c, "sub")

	f := c["federated_claims"]
	if f == nil {
		return fallback
	}
	federatedClaims, ok := f.(map[string]any)
	if !ok {
		return fallback
	}

	userId, ok := federatedClaims["user_id"].(string)
	if !ok || userId == "" {
		return fallback
	}

	return userId
}

// HasAzureGroupsOverflow checks if the JWT indicates Azure AD groups overflow (distributed claims with groups)
func HasAzureGroupsOverflow(claims jwtgo.MapClaims) bool {
	claimNamesRaw, hasClaimNames := claims["_claim_names"]
	_, hasClaimSources := claims["_claim_sources"]
	
	if !hasClaimNames || !hasClaimSources {
		return false
	}

	// Check if "groups" is in the distributed claims
	claimNamesMap, ok := claimNamesRaw.(map[string]any)
	if !ok {
		return false
	}

	_, hasGroupsClaim := claimNamesMap["groups"]
	return hasGroupsClaim
}

// AzureGroupsOverflowInfo represents information needed to fetch Azure AD groups
type AzureGroupsOverflowInfo struct {
	GraphEndpoint string
	AccessToken   string
}

// GetAzureGroupsOverflowInfo extracts Azure AD groups overflow information from JWT claims
func GetAzureGroupsOverflowInfo(claims jwtgo.MapClaims) (*AzureGroupsOverflowInfo, error) {
	if !HasAzureGroupsOverflow(claims) {
		return nil, nil
	}

	claimNamesRaw, _ := claims["_claim_names"]
	claimSourcesRaw, _ := claims["_claim_sources"]

	claimNamesMap, ok := claimNamesRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("_claim_names is not a map")
	}

	claimSourcesMap, ok := claimSourcesRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("_claim_sources is not a map")
	}

	// Get the source name for groups claim
	groupsSourceNameRaw, exists := claimNamesMap["groups"]
	if !exists {
		return nil, fmt.Errorf("groups claim not found in _claim_names")
	}

	groupsSourceName, ok := groupsSourceNameRaw.(string)
	if !ok {
		return nil, fmt.Errorf("groups source name is not a string")
	}

	// Get the source information
	sourceRaw, exists := claimSourcesMap[groupsSourceName]
	if !exists {
		return nil, fmt.Errorf("source %s not found in _claim_sources", groupsSourceName)
	}

	sourceMap, ok := sourceRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("source %s is not a map", groupsSourceName)
	}

	endpoint, hasEndpoint := sourceMap["endpoint"].(string)
	if !hasEndpoint {
		return nil, fmt.Errorf("endpoint not found in source %s", groupsSourceName)
	}

	// Convert older Azure endpoints to Microsoft Graph API v1.0
	graphEndpoint := convertToMicrosoftGraphEndpoint(endpoint)

	info := &AzureGroupsOverflowInfo{
		GraphEndpoint: graphEndpoint,
	}

	if accessToken, hasToken := sourceMap["access_token"].(string); hasToken {
		info.AccessToken = accessToken
	}

	return info, nil
}

// convertToMicrosoftGraphEndpoint converts legacy Azure AD endpoints to Microsoft Graph API v1.0
func convertToMicrosoftGraphEndpoint(endpoint string) string {
	// Convert legacy graph.windows.net to graph.microsoft.com
	if strings.Contains(endpoint, "graph.windows.net") {
		// Extract tenant ID from the old URL
		// e.g., https://graph.windows.net/11111111-1111-1111-1111-111111111111/users/22222222-2222-2222-2222-222222222222/getMemberObjects
		parts := strings.Split(endpoint, "/")
		if len(parts) >= 5 {
			tenantId := parts[3]
			userId := parts[5]
			return fmt.Sprintf("https://graph.microsoft.com/v1.0/%s/users/%s/getMemberObjects", tenantId, userId)
		}
	}
	
	// If already using graph.microsoft.com or if it's a generic endpoint, ensure it uses v1.0 and getMemberObjects
	if strings.Contains(endpoint, "graph.microsoft.com") {
		// Handle various Microsoft Graph endpoint formats
		if strings.Contains(endpoint, "/me/") {
			// Convert /me/ endpoints to direct user endpoints if possible
			return endpoint
		}
		if !strings.Contains(endpoint, "getMemberObjects") && !strings.Contains(endpoint, "GetMemberGroups") {
			// Default to /me/getMemberObjects for Microsoft Graph
			return "https://graph.microsoft.com/v1.0/me/getMemberObjects"
		}
	}
	
	return endpoint
}
