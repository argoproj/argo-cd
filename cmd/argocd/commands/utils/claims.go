package utils

import (
	"github.com/golang-jwt/jwt/v4"
)

// GetUserIdentifier returns a consistent user identifier, checking federated_claims.user_id when Dex is in use
func GetUserIdentifier(claims jwt.MapClaims) string {
	// Check for federated_claims.user_id if Dex is used
	if federatedClaims, ok := claims["federated_claims"].(map[string]interface{}); ok {
		if userID, exists := federatedClaims["user_id"].(string); exists {
			return userID
		}
	}

	// Fallback to sub
	if sub, ok := claims["sub"].(string); ok {
		return sub
	}
	return ""

}
