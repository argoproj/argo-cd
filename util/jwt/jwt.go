package jwt

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	jwtgo "github.com/dgrijalva/jwt-go/v4"
)

// MapClaims converts a jwt.Claims to a MapClaims
func MapClaims(claims jwtgo.Claims) (jwtgo.MapClaims, error) {
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
		case []interface{}:
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

func GetID(m jwtgo.MapClaims) (string, error) {
	if jtiIf, ok := m["jti"]; ok {
		if jti, ok := jtiIf.(string); ok {
			return jti, nil
		}
	}
	return "", fmt.Errorf("jti '%v' is not a string", m["jti"])
}

// IssuedAt returns the issued at as an int64
func IssuedAt(m jwtgo.MapClaims) (int64, error) {
	iatField, ok := m["iat"]
	if !ok {
		return 0, errors.New("token does not have iat claim")
	}
	switch iat := iatField.(type) {
	case float64:
		return int64(iat), nil
	case json.Number:
		return iat.Int64()
	case int64:
		return iat, nil
	default:
		return 0, fmt.Errorf("iat '%v' is not a number", iat)
	}
}

// IssuedAtTime returns the issued at as a time.Time
func IssuedAtTime(m jwtgo.MapClaims) (time.Time, error) {
	iat, err := IssuedAt(m)
	return time.Unix(iat, 0), err
}

func Claims(in interface{}) jwtgo.Claims {
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
