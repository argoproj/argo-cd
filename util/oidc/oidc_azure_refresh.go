package oidc

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"golang.org/x/oauth2"
)

const clientAssertionType = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"

var errAzureRefreshTokenMissing = errors.New("refresh token is not set")

// azureRefreshTokenSource redeems a refresh token using a federated service
// account token as the client assertion.
//
// The mutex makes the source safe for concurrent use and ensures that a
// rotated refresh token is installed before another refresh begins.
type azureRefreshTokenSource struct {
	mu sync.Mutex

	ctx                context.Context
	conf               *oauth2.Config
	getClientAssertion func(context.Context) (string, error)
	refreshToken       string
}

func (s *azureRefreshTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.refreshToken == "" {
		return nil, errAzureRefreshTokenMissing
	}

	clientAssertion, err := s.getClientAssertion(s.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate client assertion: %w", err)
	}

	// Exchange applies AuthCodeOptions after its defaults. This allows the
	// grant type to be changed while retaining oauth2's token response parsing
	// and error handling.
	token, err := s.conf.Exchange(
		s.ctx,
		"",
		oauth2.SetAuthURLParam("grant_type", "refresh_token"),
		oauth2.SetAuthURLParam("refresh_token", s.refreshToken),
		oauth2.SetAuthURLParam("client_assertion_type", clientAssertionType),
		oauth2.SetAuthURLParam("client_assertion", clientAssertion),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token using Azure workload identity: %w", err)
	}

	// OAuth servers may rotate the refresh token. If no replacement is
	// returned, preserve the previous one so it is neither lost from the cache
	// nor omitted from the next refresh request.
	if token.RefreshToken == "" {
		token.RefreshToken = s.refreshToken
	} else {
		s.refreshToken = token.RefreshToken
	}

	return token, nil
}
