package claims

import (
	"encoding/json"

	"github.com/golang-jwt/jwt/v5"
)

// ArgoClaims defines the claims structure based on Dex's documented claims
type ArgoClaims struct {
	jwt.RegisteredClaims
	Email         string `json:"email,omitempty"`
	EmailVerified bool   `json:"email_verified,omitempty"`
	Name          string `json:"name,omitempty"`
	// As per Dex docs, federated_claims has a specific structure
	FederatedClaims *FederatedClaims `json:"federated_claims,omitempty"`
}

// FederatedClaims represents the structure documented by Dex
type FederatedClaims struct {
	ConnectorID string `json:"connector_id"`
	UserID      string `json:"user_id"`
}

// MapClaimsToArgoClaims converts a jwt.MapClaims to a ArgoClaims
func MapClaimsToArgoClaims(claims jwt.MapClaims) (*ArgoClaims, error) {
	if claims == nil {
		return &ArgoClaims{}, nil
	}

	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return nil, err
	}

	var argoClaims ArgoClaims
	err = json.Unmarshal(claimsBytes, &argoClaims)
	if err != nil {
		return nil, err
	}
	return &argoClaims, nil
}

// GetUserIdentifier returns a consistent user identifier, checking federated_claims.user_id when Dex is in use
func (c *ArgoClaims) GetUserIdentifier() string {
	// Check federated claims first
	if c.FederatedClaims != nil && c.FederatedClaims.UserID != "" {
		return c.FederatedClaims.UserID
	}
	// Fallback to sub
	if c.Subject != "" {
		return c.Subject
	}
	return ""
}
