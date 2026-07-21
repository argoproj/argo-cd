package v2

import (
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	gocache "github.com/patrickmn/go-cache"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity/v2/repository"
)

const (
	// EnvTokenCacheTTL is the environment variable component CLIs use as the
	// default for their token cache TTL flag. Named here so all components
	// agree on it; the value is parsed at the CLI edge, not in this package.
	EnvTokenCacheTTL = "ARGOCD_WORKLOAD_IDENTITY_TOKEN_CACHE_TTL"
	// EnvTokenCacheMaxTTL is the environment variable component CLIs use as
	// the default for their token cache max TTL flag.
	EnvTokenCacheMaxTTL = "ARGOCD_WORKLOAD_IDENTITY_TOKEN_CACHE_MAX_TTL"

	// DefaultTokenCacheTTL is the TTL used when the authenticator did not
	// report a token expiry. 40 minutes is safely below the shortest token
	// lifetime issued by the supported providers (one hour for GCP, Azure and
	// K8s tokens).
	DefaultTokenCacheTTL = 40 * time.Minute
	// DefaultTokenCacheMaxTTL bounds how long a credential is served from the
	// cache even when the token itself lives longer (ECR tokens last twelve
	// hours). The cache key covers only repository configuration, so identity
	// changes that happen outside it — a rotated role-arn/client-id annotation
	// on the project service account, a server-side revocation — take effect
	// only once the entry expires; this cap limits that window.
	DefaultTokenCacheMaxTTL = time.Hour
)

var (
	defaultCacheTTL = DefaultTokenCacheTTL
	maxCacheTTL     = DefaultTokenCacheMaxTTL

	// expiryMargin is subtracted from a reported token expiry so cached
	// credentials are never handed out moments before they stop working.
	expiryMargin = 5 * time.Minute
)

// SetTokenCacheTTLs configures the credential cache TTLs: defaultTTL applies
// to credentials whose expiry is unknown (0 disables caching those entirely),
// maxTTL caps every entry regardless of token lifetime. Values flow from the
// component CLI flags/environment; call once at process startup, before any
// credential resolution.
func SetTokenCacheTTLs(defaultTTL, maxTTL time.Duration) {
	defaultCacheTTL = defaultTTL
	maxCacheTTL = maxTTL
}

// CredentialCache is an in-memory cache of resolved repository credentials,
// keyed by a hash of everything that influences credential resolution. It lets
// ResolveCredentials skip the identity-token and repository-token exchanges
// while a previously issued token is still valid.
type CredentialCache struct {
	cache *gocache.Cache
}

// sharedCredentialCache is process-wide so the short-lived Resolver instances
// created per credential lookup all hit the same cache.
var sharedCredentialCache = NewCredentialCache()

// NewCredentialCache creates an empty credential cache.
func NewCredentialCache() *CredentialCache {
	// Every Set supplies an explicit per-entry TTL, so no cache-wide default
	// expiration applies; the hour is the janitor's cleanup interval.
	return &CredentialCache{cache: gocache.New(gocache.NoExpiration, time.Hour)}
}

// Get returns cached credentials for the key, if present and not expired.
func (c *CredentialCache) Get(key string) (*repository.Credentials, bool) {
	entry, ok := c.cache.Get(key)
	if !ok {
		return nil, false
	}
	creds, ok := entry.(*repository.Credentials)
	return creds, ok
}

// Set caches credentials for the key. The TTL is derived from the credential
// expiry when known (minus a safety margin), otherwise the default TTL is
// used; either way it is capped at maxCacheTTL. Credentials that expire
// within the safety margin are not cached.
func (c *CredentialCache) Set(key string, creds *repository.Credentials) {
	ttl := defaultCacheTTL
	if creds.ExpiresAt != nil {
		ttl = time.Until(*creds.ExpiresAt) - expiryMargin
	}
	ttl = min(ttl, maxCacheTTL)
	if ttl <= 0 {
		return
	}
	c.cache.Set(key, creds, ttl)
}

// credentialCacheKey builds a cache key from every repository field that
// influences the resolved credentials. Keys and values are hashed as separate
// NUL-separated parts so adjacent fields cannot collide, and the result is
// hashed so no sensitive configuration is stored in a decodable form.
func credentialCacheKey(repo *v1alpha1.Repository) string {
	parts := []string{
		repo.WorkloadIdentityProvider,
		repo.Project,
		repo.Repo,
		// Type selects the authenticator for some providers (Azure uses
		// passthrough for git and the ACR exchange for everything else), so
		// repos differing only in type must not share credentials.
		repo.Type,
		repo.WorkloadIdentityTokenURL,
		repo.WorkloadIdentityAudience,
		repo.WorkloadIdentityUsername,
		repo.WorkloadIdentityAuthHost,
		repo.WorkloadIdentityMethod,
		repo.WorkloadIdentityPathTemplate,
		repo.WorkloadIdentityBodyTemplate,
		repo.WorkloadIdentityAuthType,
		repo.WorkloadIdentityResponseTokenField,
		repo.WorkloadIdentityResponseUsernameField,
		strconv.FormatBool(repo.Insecure),
	}
	for _, k := range slices.Sorted(maps.Keys(repo.WorkloadIdentityParams)) {
		parts = append(parts, k, repo.WorkloadIdentityParams[k])
	}

	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}
