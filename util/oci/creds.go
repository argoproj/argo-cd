package oci

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	gocache "github.com/patrickmn/go-cache"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// gcpAccessTokenUsername is the fixed username Google Artifact Registry (and other GCP OCI
// registries) expects when authenticating with a short-lived OAuth2 access token.
const gcpAccessTokenUsername = "oauth2accesstoken"

// cloudPlatformScope is the OAuth2 scope required to mint access tokens for GCP registries.
const cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

// googleCloudTokenSource is an in-memory cache of oauth2.TokenSource keyed by the GCP credentials
// hash. The TokenSource refreshes expired tokens itself and never expires, so it is reused
// across OCI clients rather than rebuilt on every request.
var googleCloudTokenSource = gocache.New(gocache.NoExpiration, 0)

// NewCredentialFunc returns the oras credential resolver for the given creds. GCP credentials (a
// service account key or a workload identity federation config) take precedence and yield
// short-lived OAuth2 access tokens fetched (and cached) on demand; otherwise the static
// username/password scoped to registry is used.
func NewCredentialFunc(registry string, creds Creds) auth.CredentialFunc {
	if creds.GCPServiceAccountKey != "" {
		return gcpCredential([]byte(creds.GCPServiceAccountKey))
	}
	return auth.StaticCredential(registry, auth.Credential{
		Username: creds.Username,
		Password: creds.Password,
	})
}

// gcpCredential resolves credentials by exchanging GCP credentials for a short-lived OAuth2 access
// token. The underlying token source is cached in googleCloudTokenSource and refreshed
// automatically, so tokens are reused across calls.
func gcpCredential(jsonKey []byte) auth.CredentialFunc {
	return func(ctx context.Context, _ string) (auth.Credential, error) {
		ts, err := gcpTokenSource(ctx, jsonKey)
		if err != nil {
			return auth.Credential{}, err
		}
		token, err := ts.Token()
		if err != nil {
			return auth.Credential{}, fmt.Errorf("failed to fetch GCP OAuth access token: %w", err)
		}
		return auth.Credential{Username: gcpAccessTokenUsername, Password: token.AccessToken}, nil
	}
}

func gcpTokenSource(ctx context.Context, jsonKey []byte) (oauth2.TokenSource, error) {
	h := sha256.Sum256(jsonKey)
	key := hex.EncodeToString(h[:])

	if ts, found := googleCloudTokenSource.Get(key); found {
		return ts.(oauth2.TokenSource), nil
	}

	// Detect the credential type from the JSON so this works for service account keys as well as
	// workload identity federation (external_account) and other types. CredentialsFromJSONWithType
	// is used over the deprecated CredentialsFromJSON.
	creds, err := google.CredentialsFromJSONWithType(ctx, jsonKey, gcpCredentialType(jsonKey), cloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GCP credentials: %w", err)
	}

	// The TokenSource refreshes expired tokens itself, so it can be reused indefinitely.
	googleCloudTokenSource.Set(key, creds.TokenSource, gocache.NoExpiration)
	return creds.TokenSource, nil
}

// gcpCredentialType peeks at the "type" field and returns the matching google.CredentialsType,
// defaulting to a service account key when the type is absent or unrecognized.
func gcpCredentialType(jsonKey []byte) google.CredentialsType {
	var f struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(jsonKey, &f); err != nil {
		return google.ServiceAccount
	}
	switch f.Type {
	case "external_account":
		return google.ExternalAccount
	case "impersonated_service_account":
		return google.ImpersonatedServiceAccount
	case "authorized_user":
		return google.AuthorizedUser
	default:
		return google.ServiceAccount
	}
}
