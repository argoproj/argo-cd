package repository

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rewriteTransport redirects every request to the test server, since the ACR
// authenticator always targets https://<registry from repo URL>.
type rewriteTransport struct {
	server *httptest.Server
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	target, err := url.Parse(t.server.URL)
	if err != nil {
		return nil, err
	}
	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	return t.server.Client().Transport.RoundTrip(req)
}

func newACRExchangeServer(t *testing.T, refreshToken string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/oauth2/exchange", r.URL.Path)
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "access_token", r.PostForm.Get("grant_type"))
		assert.Equal(t, "myregistry.azurecr.io", r.PostForm.Get("service"))
		assert.Equal(t, "azure-ad-token", r.PostForm.Get("access_token"))
		require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"refresh_token": refreshToken}))
	}))
}

func TestACRAuthenticator_SetsExpiryFromRefreshTokenJWT(t *testing.T) {
	// ACR refresh tokens are JWTs; their exp claim (three hours by default)
	// determines how long the credentials may be cached.
	expiry := time.Now().Add(3 * time.Hour).Truncate(time.Second)
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": expiry.Unix(),
	}).SignedString([]byte("test-secret"))
	require.NoError(t, err)

	server := newACRExchangeServer(t, refreshToken)
	defer server.Close()

	a := NewACRAuthenticator()
	a.http.HTTPClient = &http.Client{Transport: rewriteTransport{server}}

	creds, err := a.Authenticate(context.Background(),
		&Token{Type: TokenTypeBearer, Token: "azure-ad-token"},
		"myregistry.azurecr.io/myrepo", nil)
	require.NoError(t, err)
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", creds.Username)
	assert.Equal(t, refreshToken, creds.Password)
	require.NotNil(t, creds.ExpiresAt)
	assert.Equal(t, expiry.UTC(), creds.ExpiresAt.UTC())
}

func TestACRAuthenticator_NonJWTRefreshTokenHasNoExpiry(t *testing.T) {
	server := newACRExchangeServer(t, "opaque-refresh-token")
	defer server.Close()

	a := NewACRAuthenticator()
	a.http.HTTPClient = &http.Client{Transport: rewriteTransport{server}}

	creds, err := a.Authenticate(context.Background(),
		&Token{Type: TokenTypeBearer, Token: "azure-ad-token"},
		"myregistry.azurecr.io/myrepo", nil)
	require.NoError(t, err)
	assert.Equal(t, "opaque-refresh-token", creds.Password)
	assert.Nil(t, creds.ExpiresAt)
}

func TestJWTExpiry(t *testing.T) {
	t.Run("JWT without exp claim", func(t *testing.T) {
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "someone",
		}).SignedString([]byte("test-secret"))
		require.NoError(t, err)
		assert.Nil(t, jwtExpiry(token))
	})

	t.Run("not a JWT", func(t *testing.T) {
		assert.Nil(t, jwtExpiry("not-a-jwt"))
	})

	t.Run("empty", func(t *testing.T) {
		assert.Nil(t, jwtExpiry(""))
	})
}
