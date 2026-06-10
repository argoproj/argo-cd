package repository

import (
	"context"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
)

// PassthroughAuthenticator uses the identity token directly as credentials.
// It passes the token as the password with a configured username.
// This is the simplest authenticator - no exchange, just format the credentials.
//
// Username priority (first non-empty wins):
//  1. config.Username (from repository secret)
//  2. token.Username (from identity provider)
type PassthroughAuthenticator struct{}

// NewPassthroughAuthenticator creates a new passthrough authenticator
func NewPassthroughAuthenticator() *PassthroughAuthenticator {
	return &PassthroughAuthenticator{}
}

// Authenticate returns credentials using the token as password
func (a *PassthroughAuthenticator) Authenticate(ctx context.Context, token *Token, repoURL string, config *Config) (*Credentials, error) {
	if token.Type != TokenTypeBearer {
		return nil, fmt.Errorf("passthrough authenticator requires a bearer token, got %s", token.Type)
	}

	if token.Token == "" {
		return nil, errors.New("empty bearer token")
	}

	// Username priority: config overrides token's recommended username
	username := token.Username
	if config != nil && config.Username != "" {
		username = config.Username
	}

	if username == "" {
		return nil, errors.New("passthrough authenticator requires a username (set in config or from identity provider)")
	}

	log.WithField("username", username).Info("Passthrough: using bearer token as password")

	return &Credentials{
		Username: username,
		Password: token.Token,
	}, nil
}

// Ensure PassthroughAuthenticator implements Authenticator
var _ Authenticator = (*PassthroughAuthenticator)(nil)
