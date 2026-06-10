package repository

import (
	"context"
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
	return a.http.Authenticate(ctx, token, repoURL, acrConfig)
}

// Ensure ACRAuthenticator implements Authenticator
var _ Authenticator = (*ACRAuthenticator)(nil)
