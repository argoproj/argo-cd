package utils

import (
	"fmt"

	"github.com/golang-jwt/jwt/v4"
)

// ArgoClaims defines the claims structure based on Dex's documented claims
type ArgoClaims struct {
	jwt.RegisteredClaims
	Email         string   `json:"email,omitempty"`
	EmailVerified bool     `json:"email_verified,omitempty"`
	Name          string   `json:"name,omitempty"`
	Groups        []string `json:"groups,omitempty"`
	// As per Dex docs, federated_claims has a specific structure
	FederatedClaims *FederatedClaims `json:"federated_claims,omitempty"`
}

// FederatedClaims represents the structure documented by Dex
type FederatedClaims struct {
	ConnectorID string `json:"connector_id"`
	UserID      string `json:"user_id"`
}

// GetFederatedClaims extracts federated claims from jwt.MapClaims
func GetFederatedClaims(claims jwt.MapClaims) *FederatedClaims {
	if federated, ok := claims["federated_claims"].(map[string]interface{}); ok {
		return &FederatedClaims{
			ConnectorID: fmt.Sprint(federated["connector_id"]),
			UserID:      fmt.Sprint(federated["user_id"]),
		}
	}
	return nil
}

// GetUserIdentifier returns a consistent user identifier, checking federated_claims.user_id when Dex is in use
func GetUserIdentifier(claims *ArgoClaims) string {
	// Check federated claims first
	if claims.FederatedClaims != nil && claims.FederatedClaims.UserID != "" {
		return claims.FederatedClaims.UserID
	}
	// Fallback to sub
	if claims.Subject != "" {
		return claims.Subject
	}
	return ""
}
