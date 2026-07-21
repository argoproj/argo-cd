package identity

import (
	"context"

	"github.com/argoproj/argo-cd/v3/util/workloadidentity/v2/repository"
)

// Config holds identity provider configuration
type Config struct {
	// Audience overrides the provider's default audience for token requests
	// If empty, the provider uses its own default (e.g., "sts.amazonaws.com" for AWS,
	// "kubernetes.default.svc" for K8s)
	Audience string

	// TokenURL is a custom token endpoint (overrides provider default)
	TokenURL string

	// Insecure skips TLS certificate verification
	Insecure bool
}

// TokenRequester requests a K8s service account token with the specified audience.
// This is passed to providers so they can request tokens with the audience they need.
type TokenRequester func(ctx context.Context, audience string) (string, error)

// Provider acquires identity tokens from a platform
type Provider interface {
	// GetToken exchanges K8s SA context for a platform identity token
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - sa: The Kubernetes service account (for annotations like role ARN)
	//   - requestToken: Function to request a K8s token with a specific audience
	//   - config: Provider configuration
	//
	// The provider calls requestToken with the audience it needs (e.g., "sts.amazonaws.com").
	// Returns an identity token that can be used by a RepositoryAuthenticator.
	GetToken(ctx context.Context, audience, tokenURL string) (*repository.Token, error)

	DefaultRepositoryAuthenticator() repository.Authenticator
}
