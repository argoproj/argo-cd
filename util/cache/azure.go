package cache

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/util/env"
)

// AzureRedisCredentialsProviderName is the value passed to
// --redis-credentials-provider to select the Microsoft Entra ID workload
// identity backend.
const AzureRedisCredentialsProviderName = "azure"

// DefaultAzureCacheForRedisScope is the OAuth scope used to obtain access
// tokens for Azure Cache for Redis when authenticating via Microsoft Entra ID.
// See: https://learn.microsoft.com/azure/azure-cache-for-redis/cache-azure-active-directory-for-authentication
const DefaultAzureCacheForRedisScope = "https://redis.azure.com/.default"

func init() {
	RegisterRedisCredentialsProvider(&azureCredentialsProviderFactory{})
}

// azureCredentialsProviderFactory wires the Microsoft Entra ID (workload
// identity) Redis authentication flow into the RedisCredentialsProviderFactory
// registry. The factory holds the parsed flag values for its backend so that
// AddFlags and Build share state via the same struct instance.
type azureCredentialsProviderFactory struct {
	clientID string
	scope    string
}

// Name returns the registry key for this factory; matches the value users
// pass to --redis-credentials-provider.
func (a *azureCredentialsProviderFactory) Name() string {
	return AzureRedisCredentialsProviderName
}

// AddFlags registers the Azure-specific knobs. They live alongside the
// generic --redis-credentials-provider switch and only have an effect when
// "azure" is the selected backend.
func (a *azureCredentialsProviderFactory) AddFlags(cmd *cobra.Command, flagPrefix, envPrefix string) {
	cmd.Flags().StringVar(&a.clientID, flagPrefix+"redis-azure-client-id", env.StringFromEnv(envPrefix+"REDIS_AZURE_CLIENT_ID", ""),
		"Optional override for the Microsoft Entra ID application/client ID used to acquire Redis access tokens. Defaults to the AZURE_CLIENT_ID injected by the workload-identity webhook. Only honoured when --redis-credentials-provider=azure.")
	cmd.Flags().StringVar(&a.scope, flagPrefix+"redis-azure-scope", env.StringFromEnv(envPrefix+"REDIS_AZURE_SCOPE", DefaultAzureCacheForRedisScope),
		"OAuth scope to request when acquiring Microsoft Entra ID tokens for Redis. Override only when targeting a non-public Azure cloud (e.g. Azure Government). Only honoured when --redis-credentials-provider=azure.")
}

// Build constructs the credentials closure that will be wired into go-redis.
// loadedUsername (sourced from --redis-username / REDIS_USERNAME / mounted
// secrets) is honoured when set; otherwise the username is derived from the
// access-token's `oid` claim on each call.
func (a *azureCredentialsProviderFactory) Build(loadedUsername string) (RedisCredentialsProvider, error) {
	cred, err := newAzureCredential(a.clientID)
	if err != nil {
		return nil, err
	}
	return azureCredentialsProvider(cred, loadedUsername, a.scope), nil
}

// errNoUsername is returned by azureCredentialsProvider when neither an
// explicit username was configured nor a usable principal identifier could be
// extracted from the issued access token.
var errNoUsername = errors.New("redis: cannot determine Microsoft Entra ID principal username; set --redis-username or grant the workload identity an `oid` claim")

// newAzureCredential returns an azcore.TokenCredential suitable for use inside
// an Argo CD pod that has been federated with a Microsoft Entra ID workload
// identity. When clientID is non-empty, it constructs an explicit
// WorkloadIdentityCredential so the override is honoured regardless of the
// AZURE_CLIENT_ID env var injected by the workload-identity admission webhook.
// Otherwise it falls back to DefaultAzureCredential, which transparently picks
// between the workload identity, managed identity and developer credential
// flows.
//
// This is exposed as a package-level var (rather than a func) so tests can
// substitute a stub credential.
var newAzureCredential = func(clientID string) (azcore.TokenCredential, error) {
	if clientID != "" {
		cred, err := azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
			ClientID: clientID,
		})
		if err != nil {
			return nil, fmt.Errorf("redis: failed to construct Microsoft Entra ID workload identity credential: %w", err)
		}
		return cred, nil
	}
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("redis: failed to construct Microsoft Entra ID credential: %w", err)
	}
	return cred, nil
}

// azureCredentialsProvider returns a function compatible with
// redis.Options.CredentialsProviderContext that resolves to (username,
// access-token) on every new connection.
//
// The username is sourced from `staticUsername` when non-empty (for
// deployments that prefer to plumb the principal's object ID explicitly), and
// otherwise extracted from the `oid` (or `sub`) claim of the issued JWT. The
// password is always a freshly issued access token; azcore credential
// implementations cache and proactively refresh tokens, so the per-call cost
// here is negligible in steady state.
func azureCredentialsProvider(cred azcore.TokenCredential, staticUsername, scope string) func(ctx context.Context) (string, string, error) {
	if scope == "" {
		scope = DefaultAzureCacheForRedisScope
	}
	return func(ctx context.Context) (string, string, error) {
		tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{
			Scopes: []string{scope},
		})
		if err != nil {
			return "", "", fmt.Errorf("redis: failed to acquire Microsoft Entra ID token: %w", err)
		}
		username := staticUsername
		if username == "" {
			username, err = extractPrincipalID(tok.Token)
			if err != nil {
				return "", "", err
			}
		}
		if username == "" {
			return "", "", errNoUsername
		}
		log.WithField("username", username).Debug("redis: acquired Microsoft Entra ID access token")
		return username, tok.Token, nil
	}
}

// extractPrincipalID returns the `oid` claim from the supplied JWT, falling
// back to the `sub` claim when `oid` is absent. It performs no signature
// verification: the caller already trusts the token issuer (it just produced
// it via azidentity); we only need to read the payload to recover the
// principal identifier that Azure Cache for Redis expects as the AUTH
// username.
func extractPrincipalID(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", errors.New("redis: invalid JWT format on access token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Fall back to standard padding in case the token uses RFC 7519
		// padding (rare in practice, but cheap to support).
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return "", fmt.Errorf("redis: failed to decode access token payload: %w", err)
		}
	}
	var claims struct {
		OID string `json:"oid"`
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("redis: failed to parse access token payload: %w", err)
	}
	if claims.OID != "" {
		return claims.OID, nil
	}
	return claims.Sub, nil
}
