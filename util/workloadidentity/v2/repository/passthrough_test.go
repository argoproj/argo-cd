package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPassthroughAuthenticator_Authenticate_WithConfigUsername(t *testing.T) {
	a := NewPassthroughAuthenticator()

	token := &Token{
		Type:  TokenTypeBearer,
		Token: "my-oauth-token",
	}

	config := &Config{
		Username: "config-username",
	}

	creds, err := a.Authenticate(context.Background(), token, "https://registry.example.com/repo", config)

	require.NoError(t, err)
	require.NotNil(t, creds)
	assert.Equal(t, "config-username", creds.Username, "Config username should take precedence")
	assert.Equal(t, "my-oauth-token", creds.Password)
}

func TestPassthroughAuthenticator_Authenticate_WithTokenUsername(t *testing.T) {
	a := NewPassthroughAuthenticator()

	token := &Token{
		Type:     TokenTypeBearer,
		Token:    "my-oauth-token",
		Username: "oauth2accesstoken", // Set by GCP provider
	}

	config := &Config{
		// No username in config - should use token's username
	}

	creds, err := a.Authenticate(context.Background(), token, "https://gcr.io/myproject/myrepo", config)

	require.NoError(t, err)
	require.NotNil(t, creds)
	assert.Equal(t, "oauth2accesstoken", creds.Username, "Should use token's username when config is empty")
	assert.Equal(t, "my-oauth-token", creds.Password)
}

func TestPassthroughAuthenticator_Authenticate_ConfigOverridesToken(t *testing.T) {
	a := NewPassthroughAuthenticator()

	token := &Token{
		Type:     TokenTypeBearer,
		Token:    "my-oauth-token",
		Username: "token-username",
	}

	config := &Config{
		Username: "config-username", // Should override token's username
	}

	creds, err := a.Authenticate(context.Background(), token, "https://registry.example.com/repo", config)

	require.NoError(t, err)
	require.NotNil(t, creds)
	assert.Equal(t, "config-username", creds.Username, "Config username should override token username")
	assert.Equal(t, "my-oauth-token", creds.Password)
}

func TestPassthroughAuthenticator_Authenticate_NilConfig_WithTokenUsername(t *testing.T) {
	a := NewPassthroughAuthenticator()

	token := &Token{
		Type:     TokenTypeBearer,
		Token:    "my-oauth-token",
		Username: "oauth2accesstoken",
	}

	creds, err := a.Authenticate(context.Background(), token, "https://gcr.io/myproject/myrepo", nil)

	require.NoError(t, err)
	require.NotNil(t, creds)
	assert.Equal(t, "oauth2accesstoken", creds.Username)
	assert.Equal(t, "my-oauth-token", creds.Password)
}

func TestPassthroughAuthenticator_Authenticate_MissingUsername(t *testing.T) {
	a := NewPassthroughAuthenticator()

	token := &Token{
		Type:  TokenTypeBearer,
		Token: "my-oauth-token",
		// No Username set
	}

	config := &Config{
		// No Username in config either
	}

	creds, err := a.Authenticate(context.Background(), token, "https://registry.example.com/repo", config)

	require.Error(t, err)
	assert.Nil(t, creds)
	assert.Contains(t, err.Error(), "requires a username")
}

func TestPassthroughAuthenticator_Authenticate_NilConfigNoTokenUsername(t *testing.T) {
	a := NewPassthroughAuthenticator()

	token := &Token{
		Type:  TokenTypeBearer,
		Token: "my-oauth-token",
		// No Username set
	}

	creds, err := a.Authenticate(context.Background(), token, "https://registry.example.com/repo", nil)

	require.Error(t, err)
	assert.Nil(t, creds)
	assert.Contains(t, err.Error(), "requires a username")
}

func TestPassthroughAuthenticator_Authenticate_WrongTokenType(t *testing.T) {
	a := NewPassthroughAuthenticator()

	token := &Token{
		Type: TokenTypeAWS,
		AWSCredentials: &AWSCredentials{
			AccessKeyID: "AKIA...",
		},
	}

	config := &Config{
		Username: "oauth2accesstoken",
	}

	creds, err := a.Authenticate(context.Background(), token, "https://registry.example.com/repo", config)

	require.Error(t, err)
	assert.Nil(t, creds)
	assert.Contains(t, err.Error(), "requires a bearer token")
	assert.Contains(t, err.Error(), "aws")
}

func TestPassthroughAuthenticator_Authenticate_EmptyToken(t *testing.T) {
	a := NewPassthroughAuthenticator()

	token := &Token{
		Type:     TokenTypeBearer,
		Token:    "",
		Username: "oauth2accesstoken",
	}

	config := &Config{}

	creds, err := a.Authenticate(context.Background(), token, "https://registry.example.com/repo", config)

	require.Error(t, err)
	assert.Nil(t, creds)
	assert.Contains(t, err.Error(), "empty bearer token")
}

func TestPassthroughAuthenticator_ImplementsInterface(t *testing.T) {
	var _ Authenticator = (*PassthroughAuthenticator)(nil)
}
