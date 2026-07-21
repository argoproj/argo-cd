package repository

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ACRAuthenticator exchanges Azure credentials for ACR authorization tokens.
// It delegates to HTTPTemplateAuthenticator with ACR-specific configuration.
type ACRAuthenticator struct {
	http *HTTPTemplateAuthenticator
}

// NewACRAuthenticator creates a new ACR authenticator
func NewACRAuthenticator() *ACRAuthenticator {
	return &ACRAuthenticator{
		http: NewHTTPTemplateAuthenticator(),
	}
}

// Authenticate exchanges Azure credentials for ACR authorization tokens
func (a *ACRAuthenticator) Authenticate(ctx context.Context, token *Token, repoURL string, _ *Config) (*Credentials, error) {
	// ACR token exchange configuration
	acrConfig := &Config{
		Method:             "POST",
		PathTemplate:       "/oauth2/exchange",
		BodyTemplate:       "grant_type=access_token&service={{ .registry }}&access_token={{ .token }}",
		AuthType:           "none",
		ResponseTokenField: "refresh_token",
		Username:           "00000000-0000-0000-0000-000000000000",
	}
	creds, err := a.http.Authenticate(ctx, token, repoURL, acrConfig)
	if err != nil {
		return nil, err
	}
	// The exchange response carries no expires_in, but the refresh token is a
	// JWT whose exp claim reflects the registry's configured token lifetime
	// (three hours by default), so derive the credential expiry from it.
	if creds.ExpiresAt == nil {
		creds.ExpiresAt = jwtExpiry(creds.Password)
	}
	return creds, nil
}

// jwtExpiry returns the exp claim of a JWT without verifying its signature,
// or nil when the value is not a JWT or carries no expiry. The signature is
// deliberately not checked: the token was just issued to us over TLS and the
// expiry is only used as a caching hint.
func jwtExpiry(token string) *time.Time {
	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(token, claims); err != nil {
		return nil
	}
	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return nil
	}
	return &exp.Time
}

// Ensure ACRAuthenticator implements Authenticator
var _ Authenticator = (*ACRAuthenticator)(nil)
