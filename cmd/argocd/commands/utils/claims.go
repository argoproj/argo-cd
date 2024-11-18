package utils

import (
	"github.com/golang-jwt/jwt/v4"
)

const (
	federatedClaimsKey = "federated_claims"
	userIDKey          = "user_id"
	subKey             = "sub"
)

// GetUserIdentifier returns a consistent user identifier, checking federated_claims.user_id when Dex is in use
func GetUserIdentifier(claims jwt.MapClaims) string {
	if federatedClaims, ok := claims[federatedClaimsKey].(map[string]interface{}); ok {
		if userID, exists := federatedClaims[userIDKey].(string); exists && userID != "" {
			return userID
		}
	}
	// Fallback to sub
	if sub, ok := claims[subKey].(string); ok && sub != "" {
		return sub
	}
	return ""
}
