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

// ClaimSource represents a distributed claim source as defined in OIDC spec
type ClaimSource struct {
	Endpoint    string `json:"endpoint"`
	AccessToken string `json:"access_token,omitempty"`
}

// HasDistributedClaims checks if the JWT contains distributed claims
func HasDistributedClaims(claims jwtgo.MapClaims) bool {
	claimNames := claims["_claim_names"]
	claimSources := claims["_claim_sources"]
	
	return claimNames != nil && claimSources != nil
}

// GetClaimSources extracts the claim sources from distributed claims
func GetClaimSources(claims jwtgo.MapClaims) (map[string]ClaimSource, error) {
	if !HasDistributedClaims(claims) {
		return nil, nil
	}

	sourcesRaw, ok := claims["_claim_sources"]
	if !ok {
		return nil, fmt.Errorf("_claim_sources not found in claims")
	}

	sourcesMap, ok := sourcesRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("_claim_sources is not a map, got type %T", sourcesRaw)
	}

	sources := make(map[string]ClaimSource)
	for sourceName, sourceDataRaw := range sourcesMap {
		sourceDataMap, ok := sourceDataRaw.(map[string]any)
		if !ok {
			// Log warning but continue processing other sources
			fmt.Printf("Warning: claim source %s data is not a map, got type %T\n", sourceName, sourceDataRaw)
			continue
		}

		source := ClaimSource{}
		if endpoint, ok := sourceDataMap["endpoint"].(string); ok {
			source.Endpoint = endpoint
		}
		if accessToken, ok := sourceDataMap["access_token"].(string); ok {
			source.AccessToken = accessToken
		}

		if source.Endpoint != "" {
			sources[sourceName] = source
		} else {
			// Log warning for sources without endpoints
			fmt.Printf("Warning: claim source %s has no endpoint\n", sourceName)
		}
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("no valid claim sources found")
	}

	return sources, nil
}

// GetDistributedClaimNames extracts the mapping of claim names to sources
func GetDistributedClaimNames(claims jwtgo.MapClaims) (map[string]string, error) {
	if !HasDistributedClaims(claims) {
		return nil, nil
	}

	claimNamesRaw, ok := claims["_claim_names"]
	if !ok {
		return nil, fmt.Errorf("_claim_names not found in claims")
	}

	claimNamesMap, ok := claimNamesRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("_claim_names is not a map")
	}

	claimNames := make(map[string]string)
	for claimName, sourceNameRaw := range claimNamesMap {
		if sourceName, ok := sourceNameRaw.(string); ok {
			claimNames[claimName] = sourceName
		}
	}

	return claimNames, nil
}
