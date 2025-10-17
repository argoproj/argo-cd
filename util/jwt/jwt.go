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

	// Get the source information to extract access token
	sourceRaw, exists := claimSourcesMap[groupsSourceName]
	if !exists {
		return nil, fmt.Errorf("source %s not found in _claim_sources", groupsSourceName)
	}

	sourceMap, ok := sourceRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("source %s is not a map", groupsSourceName)
	}

	// Construct the Microsoft Graph API URL based on token type
	// According to Microsoft docs, ignore the endpoint URL and construct our own
	graphEndpoint := constructMicrosoftGraphGroupsEndpoint(claims)

	info := &AzureGroupsOverflowInfo{
		GraphEndpoint: graphEndpoint,
	}

	if accessToken, hasToken := sourceMap["access_token"].(string); hasToken {
		info.AccessToken = accessToken
	}

	return info, nil
}

// constructMicrosoftGraphGroupsEndpoint constructs the correct Microsoft Graph API URL
// based on the token type as recommended by Microsoft documentation.
// See: https://learn.microsoft.com/en-us/entra/identity-platform/access-token-claims-reference#groups-overage-claim
func constructMicrosoftGraphGroupsEndpoint(claims jwtgo.MapClaims) string {
	// We now use the Microsoft Graph getMemberGroups action instead of getMemberObjects.
	// Docs: https://learn.microsoft.com/en-us/graph/api/directoryobject-getmembergroups
	// Reason: We only need group object IDs for authorization mapping; getMemberGroups
	// returns only groups (and is the recommended action for group-based authorization).
	if idtyp := StringField(claims, "idtyp"); idtyp == "app" {
		// For app-only tokens, we need to use the user ID from the oid claim
		// https://graph.microsoft.com/v1.0/users/{userId}/getMemberGroups
		if oid := StringField(claims, "oid"); oid != "" {
			return fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/getMemberGroups", oid)
		}
		// Fallback – should not normally occur – default to /me variant
		return "https://graph.microsoft.com/v1.0/me/getMemberGroups"
	}
	// App+user tokens (normal delegated user auth) use /me action endpoint
	// https://graph.microsoft.com/v1.0/me/getMemberGroups
	return "https://graph.microsoft.com/v1.0/me/getMemberGroups"
}
