package cache

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTokenCredential implements azcore.TokenCredential for tests. It returns
// a canned token and records the scopes it was called with so tests can
// assert on them.
type fakeTokenCredential struct {
	token      string
	err        error
	calls      int
	lastScopes []string
}

func (f *fakeTokenCredential) GetToken(_ context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	f.calls++
	f.lastScopes = opts.Scopes
	if f.err != nil {
		return azcore.AccessToken{}, f.err
	}
	return azcore.AccessToken{
		Token:     f.token,
		ExpiresOn: time.Now().Add(time.Hour),
	}, nil
}

// makeJWT constructs a minimally-valid unsigned JWT carrying the supplied
// claims payload. Real Entra ID tokens are signed; we don't validate
// signatures in extractPrincipalID, so a stub is sufficient.
func makeJWT(t *testing.T, payloadJSON string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(payloadJSON))
	return header + "." + payload + ".sig"
}

func TestExtractPrincipalID(t *testing.T) {
	t.Parallel()

	t.Run("prefers oid over sub", func(t *testing.T) {
		t.Parallel()
		tok := makeJWT(t, `{"oid":"00000000-0000-0000-0000-000000000001","sub":"sub-value"}`)
		got, err := extractPrincipalID(tok)
		require.NoError(t, err)
		assert.Equal(t, "00000000-0000-0000-0000-000000000001", got)
	})

	t.Run("falls back to sub", func(t *testing.T) {
		t.Parallel()
		tok := makeJWT(t, `{"sub":"sub-value"}`)
		got, err := extractPrincipalID(tok)
		require.NoError(t, err)
		assert.Equal(t, "sub-value", got)
	})

	t.Run("returns empty when neither claim present", func(t *testing.T) {
		t.Parallel()
		tok := makeJWT(t, `{"aud":"redis"}`)
		got, err := extractPrincipalID(tok)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("rejects malformed token", func(t *testing.T) {
		t.Parallel()
		_, err := extractPrincipalID("not.a-jwt")
		require.Error(t, err)
	})

	t.Run("rejects single-segment token", func(t *testing.T) {
		t.Parallel()
		_, err := extractPrincipalID("onlyone")
		require.Error(t, err)
	})

	t.Run("rejects undecodable payload", func(t *testing.T) {
		t.Parallel()
		_, err := extractPrincipalID("aaa.&&&.sig")
		require.Error(t, err)
	})
}

func TestAzureCredentialsProvider_StaticUsername(t *testing.T) {
	t.Parallel()

	cred := &fakeTokenCredential{token: makeJWT(t, `{"oid":"derived"}`)}
	provider := azureCredentialsProvider(cred, "explicit-oid", "")

	user, pass, err := provider(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "explicit-oid", user, "explicit username must take precedence over oid claim")
	assert.Equal(t, cred.token, pass)
	assert.Equal(t, []string{DefaultAzureCacheForRedisScope}, cred.lastScopes,
		"default scope must be used when none configured")
}

func TestAzureCredentialsProvider_DerivesUsernameFromToken(t *testing.T) {
	t.Parallel()

	cred := &fakeTokenCredential{token: makeJWT(t, `{"oid":"derived-oid"}`)}
	provider := azureCredentialsProvider(cred, "", "")

	user, pass, err := provider(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "derived-oid", user)
	assert.Equal(t, cred.token, pass)
}

func TestAzureCredentialsProvider_CustomScope(t *testing.T) {
	t.Parallel()

	cred := &fakeTokenCredential{token: makeJWT(t, `{"oid":"x"}`)}
	provider := azureCredentialsProvider(cred, "", "https://example.invalid/.default")

	_, _, err := provider(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"https://example.invalid/.default"}, cred.lastScopes)
}

func TestAzureCredentialsProvider_PropagatesTokenError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("federated token unavailable")
	cred := &fakeTokenCredential{err: wantErr}
	provider := azureCredentialsProvider(cred, "", "")

	_, _, err := provider(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "federated token unavailable",
		"wrapped error must include the underlying credential failure")
}

func TestAzureCredentialsProvider_NoUsernameAvailable(t *testing.T) {
	t.Parallel()

	cred := &fakeTokenCredential{token: makeJWT(t, `{"aud":"redis"}`)}
	provider := azureCredentialsProvider(cred, "", "")

	_, _, err := provider(context.Background())
	require.ErrorIs(t, err, errNoUsername)
}

func TestAzureCredentialsProvider_RefreshesTokenPerCall(t *testing.T) {
	// go-redis invokes CredentialsProviderContext on every new connection.
	// The provider therefore *must* call GetToken on each invocation —
	// caching is the credential implementation's responsibility, not ours.
	t.Parallel()

	cred := &fakeTokenCredential{token: makeJWT(t, `{"oid":"x"}`)}
	provider := azureCredentialsProvider(cred, "", "")

	for range 3 {
		_, _, err := provider(context.Background())
		require.NoError(t, err)
	}
	assert.Equal(t, 3, cred.calls)
}
