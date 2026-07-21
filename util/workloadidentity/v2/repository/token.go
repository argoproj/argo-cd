package repository

import "time"

// TokenType indicates the format of the identity token
type TokenType string

const (
	// TokenTypeBearer is a bearer token (JWT, OAuth access token)
	TokenTypeBearer TokenType = "bearer"
	// TokenTypeAWS is AWS credentials for SigV4 signing
	TokenTypeAWS TokenType = "aws"
)

// Token represents a token from an identity provider
type Token struct {
	// Type indicates the token format
	Type TokenType

	// Token holds the bearer token value (for TokenTypeBearer)
	Token string

	// Username is the recommended username to use with this token
	// For passthrough auth, this is used directly (e.g., "oauth2accesstoken" for GCP)
	// May be empty if the authenticator should use its own default
	Username string

	// ExpiresAt is the time at which the token stops being valid, when the
	// identity provider knows it (TokenRequest expiration, OAuth expires_in,
	// GCP impersonation expireTime). Authenticators that use the token
	// directly as the credential (passthrough) propagate it to
	// Credentials.ExpiresAt; exchange-based authenticators ignore it since
	// the exchanged token has its own lifetime.
	ExpiresAt *time.Time

	// AWSCredentials holds AWS credentials (for TokenTypeAWS)
	AWSCredentials *AWSCredentials
}

// AWSCredentials holds AWS temporary credentials
type AWSCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Region          string
	Expiration      *time.Time
}
