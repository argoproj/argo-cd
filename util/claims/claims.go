package claims

import (
	"github.com/golang-jwt/jwt/v5"
)

// GetUserIdentifier returns a consistent user identifier, checking federated_claims.user_id when Dex is in use
func GetUserIdentifier(c jwt.MapClaims) string {
	if c == nil {
		return ""
	}

	// Fallback to sub if federated_claims.user_id is not set.
	fallback, err := c.GetSubject()
	if err != nil {
		fallback = ""
	}

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
