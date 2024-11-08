package utils

import (
	"github.com/golang-jwt/jwt/v4"
)

// GetUserIdentifier returns a consistent user identifier, checking federated_claims.user_id when Dex is in use
func GetUserIdentifier(claims jwt.MapClaims) string {
	if federatedClaims, ok := claims["federated_claims"].(map[string]interface{}); ok {
		if userID, exists := federatedClaims["user_id"].(string); exists && userID != "" {
			return userID
		}
	}
	// Fallback to sub
	if sub, ok := claims["sub"].(string); ok && sub != "" {
		return sub
	}
	return ""
}
