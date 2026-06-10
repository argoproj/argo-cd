package repository

import (
	"context"
)

// Credentials holds resolved username and password for repository access
type Credentials struct {
	Username string
	Password string
}

// Config holds registry-specific configuration
type Config struct {
	// Username for credentials (e.g., "oauth2accesstoken" for GCR, "$oauthtoken" for Quay)
	// Default: "$oauthtoken"
	Username string

	// Insecure skips TLS certificate verification
	Insecure bool

	// AuthHost overrides the registry host for the auth request URL.
	// Use this when the auth endpoint is on a different host than the registry.
	// e.g., for octo-sts: registry is ghcr.io but auth is at octo-sts.dev
	AuthHost string

	// Method is the HTTP method for template-based auth (GET or POST)
	// Default: GET
	Method string

	// PathTemplate is a URL path template with placeholders
	// e.g., "/v2/auth?service={registry}&scope={scope}"
	// Available variables: {token}, {registry}, {repo}, plus custom Params
	PathTemplate string

	// BodyTemplate is the request body template for POST requests
	// Can be form-urlencoded or JSON (auto-detected by leading '{')
	// e.g., "grant_type=access_token&access_token={token}"
	BodyTemplate string

	// AuthType specifies how to send the identity token
	// - "bearer": Authorization: Bearer {token} (default)
	// - "basic": Basic auth with Username:{token}
	// - "none": Token only sent via template placeholders
	AuthType string

	// Params are custom parameters for template substitution
	// e.g., {"policy": "argocd", "provider": "my-oidc"}
	Params map[string]string

	// ResponseTokenField is the JSON field containing the token in the response
	// Default: tries "access_token", then "token", then "refresh_token"
	ResponseTokenField string
}

// Authenticator converts identity tokens to registry credentials
type Authenticator interface {
	// Authenticate exchanges an identity token for registry credentials
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - token: The identity token from an IdentityProvider
	//   - repoURL: The repository URL (for extracting registry host, region, etc.)
	//   - config: Authenticator configuration
	//
	// Returns credentials that can be used to access the registry
	Authenticate(ctx context.Context, token *Token, repoURL string, config *Config) (*Credentials, error)
}
