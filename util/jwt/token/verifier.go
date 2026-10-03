package token

import (
	"context"

	jwtgo "github.com/golang-jwt/jwt/v5"

	"github.com/argoproj/argo-cd/v3/util/settings"
)

// Verifier defines the contract to invoke JWT token verification logic
type Verifier interface {
	// Verify the given JWT token string and return the claims if valid
	Verify(ctx context.Context, tokenString string, argoSettings *settings.ArgoCDSettings) (jwtgo.Claims, error)
}
