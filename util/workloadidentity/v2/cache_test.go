package v2

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity/v2/identity/mocks"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity/v2/repository"
	repomocks "github.com/argoproj/argo-cd/v3/util/workloadidentity/v2/repository/mocks"
)

// newTestResolver returns a Resolver with its own credential cache so tests
// don't interfere with each other through the process-wide shared cache.
func newTestResolver(clientset kubernetes.Interface, namespace string) *Resolver {
	r := NewResolver(clientset, namespace)
	r.credCache = NewCredentialCache()
	return r
}

func TestResolveCredentials_CacheHitSkipsTokenExchange(t *testing.T) {
	resolver := newTestResolver(fake.NewSimpleClientset(), "argocd")

	provider := mocks.NewProvider(t)
	provider.EXPECT().GetToken(mock.Anything, mock.Anything, mock.Anything).
		Return(&repository.Token{Type: repository.TokenTypeBearer, Token: "test-token"}, nil).
		Once()

	repoAuth := repomocks.NewAuthenticator(t)
	repoAuth.EXPECT().Authenticate(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&repository.Credentials{Username: "user", Password: "pass"}, nil).
		Once()

	repo := &v1alpha1.Repository{
		Repo:                     "https://cache-hit.example.com",
		Project:                  "default",
		WorkloadIdentityProvider: "gcp",
	}

	first, err := resolver.ResolveCredentials(context.Background(), provider, repoAuth, repo)
	require.NoError(t, err)

	// Second resolution must come from the cache: the mocks are limited to a
	// single call each, so any further token exchange would fail the test.
	second, err := resolver.ResolveCredentials(context.Background(), provider, repoAuth, repo)
	require.NoError(t, err)
	assert.Equal(t, first, second)
}

func TestResolveCredentials_ErrorsAreNotCached(t *testing.T) {
	resolver := newTestResolver(fake.NewSimpleClientset(), "argocd")

	provider := mocks.NewProvider(t)
	provider.EXPECT().GetToken(mock.Anything, mock.Anything, mock.Anything).
		Return(&repository.Token{Type: repository.TokenTypeBearer, Token: "test-token"}, nil).
		Twice()

	repoAuth := repomocks.NewAuthenticator(t)
	repoAuth.EXPECT().Authenticate(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, assert.AnError).
		Once()
	repoAuth.EXPECT().Authenticate(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&repository.Credentials{Username: "user", Password: "pass"}, nil).
		Once()

	repo := &v1alpha1.Repository{
		Repo:                     "https://error-not-cached.example.com",
		Project:                  "default",
		WorkloadIdentityProvider: "gcp",
	}

	_, err := resolver.ResolveCredentials(context.Background(), provider, repoAuth, repo)
	require.Error(t, err)

	creds, err := resolver.ResolveCredentials(context.Background(), provider, repoAuth, repo)
	require.NoError(t, err)
	assert.Equal(t, "user", creds.Username)
}

func TestCredentialCache_ExpiredCredentialsNotCached(t *testing.T) {
	c := NewCredentialCache()

	// Expiry within the safety margin: must not be cached.
	soon := time.Now().Add(time.Minute)
	c.Set("key", &repository.Credentials{Username: "user", Password: "pass", ExpiresAt: &soon})
	_, ok := c.Get("key")
	assert.False(t, ok)

	// Expiry beyond the safety margin: cached.
	later := time.Now().Add(time.Hour)
	c.Set("key", &repository.Credentials{Username: "user", Password: "pass", ExpiresAt: &later})
	creds, ok := c.Get("key")
	require.True(t, ok)
	assert.Equal(t, "user", creds.Username)
}

func TestCredentialCache_TTLCappedAtMax(t *testing.T) {
	c := NewCredentialCache()

	// A long-lived token (ECR tokens last twelve hours) must not be served
	// longer than maxCacheTTL, so identity rotation and revocation are
	// bounded by the cap rather than the token lifetime.
	farOut := time.Now().Add(12 * time.Hour)
	c.Set("key", &repository.Credentials{Username: "user", Password: "pass", ExpiresAt: &farOut})

	items := c.cache.Items()
	require.Len(t, items, 1)
	for _, item := range items {
		expiration := time.Unix(0, item.Expiration)
		assert.WithinDuration(t, time.Now().Add(maxCacheTTL), expiration, time.Minute)
	}
}

func TestCredentialCacheKey(t *testing.T) {
	base := func() *v1alpha1.Repository {
		return &v1alpha1.Repository{
			Repo:                     "https://example.com/repo",
			Project:                  "default",
			WorkloadIdentityProvider: "aws",
			WorkloadIdentityParams:   map[string]string{"a": "1", "b": "2"},
		}
	}

	first := credentialCacheKey(base())
	second := credentialCacheKey(base())
	assert.Equal(t, first, second, "identical config must produce identical keys")

	mutations := map[string]func(r *v1alpha1.Repository){
		"repo URL":       func(r *v1alpha1.Repository) { r.Repo = "https://other.example.com/repo" },
		"project":        func(r *v1alpha1.Repository) { r.Project = "other" },
		"repo type":      func(r *v1alpha1.Repository) { r.Type = "git" },
		"provider":       func(r *v1alpha1.Repository) { r.WorkloadIdentityProvider = "gcp" },
		"audience":       func(r *v1alpha1.Repository) { r.WorkloadIdentityAudience = "aud" },
		"token URL":      func(r *v1alpha1.Repository) { r.WorkloadIdentityTokenURL = "https://sts.example.com" },
		"username":       func(r *v1alpha1.Repository) { r.WorkloadIdentityUsername = "robot" },
		"auth host":      func(r *v1alpha1.Repository) { r.WorkloadIdentityAuthHost = "auth.example.com" },
		"method":         func(r *v1alpha1.Repository) { r.WorkloadIdentityMethod = "POST" },
		"path template":  func(r *v1alpha1.Repository) { r.WorkloadIdentityPathTemplate = "/token" },
		"body template":  func(r *v1alpha1.Repository) { r.WorkloadIdentityBodyTemplate = "grant_type=x" },
		"auth type":      func(r *v1alpha1.Repository) { r.WorkloadIdentityAuthType = "basic" },
		"token field":    func(r *v1alpha1.Repository) { r.WorkloadIdentityResponseTokenField = "token" },
		"username field": func(r *v1alpha1.Repository) { r.WorkloadIdentityResponseUsernameField = "username" },
		"params":         func(r *v1alpha1.Repository) { r.WorkloadIdentityParams["a"] = "changed" },
		"insecure":       func(r *v1alpha1.Repository) { r.Insecure = true },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			mutated := base()
			mutate(mutated)
			assert.NotEqual(t, credentialCacheKey(base()), credentialCacheKey(mutated),
				"changing %s must change the cache key", name)
		})
	}

	t.Run("ambiguous params must not collide", func(t *testing.T) {
		a := base()
		a.WorkloadIdentityParams = map[string]string{"scope": "read=true"}
		b := base()
		b.WorkloadIdentityParams = map[string]string{"scope=read": "true"}
		assert.NotEqual(t, credentialCacheKey(a), credentialCacheKey(b))
	})
}
