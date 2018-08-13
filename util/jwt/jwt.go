package jwt

import (
	"encoding/json"

	jwtgo "github.com/dgrijalva/jwt-go"
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

// GetField extracts a field from the claims as a string
func GetField(claims jwtgo.MapClaims, fieldName string) string {
	if fieldIf, ok := claims[fieldName]; ok {
		if field, ok := fieldIf.(string); ok {
			return field
		}
	}
	return ""
}

// GetGroups extracts the groups from a claims
func GetGroups(claims jwtgo.MapClaims) []string {
	groups := make([]string, 0)
	groupsIf, ok := claims["groups"]
	if !ok {
		return groups
	}
	groupIfList, ok := groupsIf.([]interface{})
	if !ok {
		return groups
	}
	for _, groupIf := range groupIfList {
		group, ok := groupIf.(string)
		if ok {
			groups = append(groups, group)
		}
	}
	return groups
}
