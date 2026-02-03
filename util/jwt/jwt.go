package jwt

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	jwtgo "github.com/golang-jwt/jwt/v5"
)

// MapClaims converts jwtgo.Claims (which might be a struct like jwtgo.RegisteredClaims or a custom struct embedding it)
// into jwtgo.MapClaims for easier field access, especially for custom claims.
func MapClaims(claims jwtgo.Claims) (jwtgo.MapClaims, error) {
	// If it's already MapClaims, return it directly.
	if mapClaims, ok := claims.(jwtgo.MapClaims); ok {
		return mapClaims, nil
	}
	// If it's *MapClaims (less common but possible), dereference and return.
	if mapClaimsPtr, ok := claims.(*jwtgo.MapClaims); ok {
		return *mapClaimsPtr, nil
	}

	// Otherwise, marshal the claims struct to JSON and unmarshal back into MapClaims.
	// This handles RegisteredClaims and custom structs embedding RegisteredClaims.
	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal claims to JSON: %w", err)
	}
	var mapClaims jwtgo.MapClaims
	err = json.Unmarshal(claimsBytes, &mapClaims)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal claims JSON to MapClaims: %w", err)
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

// GetScopeValues extracts the values of specified scopes (claim names) from the claims map.
// It handles cases where the claim value is a single string or a slice of strings/interfaces.
func GetScopeValues(claims jwtgo.MapClaims, scopes []string) []string {
	values := make([]string, 0)
	for _, scope := range scopes {
		scopeIf, ok := claims[scope]
		if !ok {
			continue
		}

		switch v := scopeIf.(type) {
		case string:
			values = append(values, v)
		case []string:
			values = append(values, v...)
		case []any: // Handle JSON arrays which often unmarshal to []interface{}
			for _, item := range v {
				if strVal, ok := item.(string); ok {
					values = append(values, strVal)
				}
			}
			// Could add handling for other types like []float64 if needed
		}
	}
	return values
}

// IssuedAtTime returns the issued at ("iat") claim as a time.Time pointer.
// Returns nil, nil if the claim is not present.
// Returns nil, error if the claim is present but invalid.
func IssuedAtTime(m jwtgo.MapClaims) (*time.Time, error) {
	claim, err := m.GetIssuedAt()
	if err != nil {
		// Check if the error is specifically because the claim is missing
		if _, ok := m["iat"]; !ok {
			return nil, nil // Claim is missing, return nil, nil as per test expectation
		}
		// Otherwise, the claim exists but is invalid
		return nil, fmt.Errorf("failed to get 'iat' claim: %w", err)
	}
	if claim == nil {
		// This case might occur if GetIssuedAt returns nil without error (unlikely but safe to handle)
		return nil, nil
	}
	t := claim.Time
	return &t, nil
}

// ExpirationTime returns the expiration ("exp") claim as a time.Time pointer.
// Returns nil, nil if the claim is not present.
// Returns nil, error if the claim is present but invalid.
func ExpirationTime(m jwtgo.MapClaims) (*time.Time, error) {
	claim, err := m.GetExpirationTime()
	if err != nil {
		return nil, fmt.Errorf("failed to get 'exp' claim: %w", err)
	}
	if claim == nil {
		// This case might occur if GetExpirationTime returns nil without error
		return nil, nil
	}
	t := claim.Time
	return &t, nil
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

// GetGroups retrieves group information from claims using specified scope names.
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
