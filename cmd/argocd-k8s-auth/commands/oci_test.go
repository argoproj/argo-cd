package commands

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockOCIKeyProvider struct {
	privateKey *rsa.PrivateKey
	keyID      string
	err        error
}

func (m *mockOCIKeyProvider) PrivateRSAKey() (*rsa.PrivateKey, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.privateKey, nil
}

func (m *mockOCIKeyProvider) KeyID() (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.keyID, nil
}

func TestGetSignedOCIRequestWithProvider(t *testing.T) {
	t.Parallel()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	t.Run("signs cluster_request URL with authorization and date query params", func(t *testing.T) {
		t.Parallel()
		provider := &mockOCIKeyProvider{
			privateKey: privateKey,
			keyID:      "ST$mock-rpst-token",
		}

		clusterID := "ocid1.cluster.oc1.me-riyadh-1.aaaaaaaawf5ax575c2ebepszi44myapoavo3jtm6kyxficsbecbujce5go5q"
		signedURL, err := getSignedOCIRequestWithProvider(t.Context(), clusterID, "me-riyadh-1", provider)
		require.NoError(t, err)
		require.NotEmpty(t, signedURL)

		parsedURL, err := url.Parse(signedURL)
		require.NoError(t, err)
		assert.Equal(t, "https", parsedURL.Scheme)
		assert.Equal(t, "containerengine.me-riyadh-1.oraclecloud.com", parsedURL.Host)
		assert.Equal(t, "/cluster_request/"+clusterID, parsedURL.Path)
		assert.NotEmpty(t, parsedURL.Query().Get("date"))

		authHeader := parsedURL.Query().Get("authorization")
		assert.Contains(t, authHeader, `keyId="ST$mock-rpst-token"`)
		assert.Contains(t, authHeader, `headers="date (request-target) host"`)
		assert.Contains(t, authHeader, `algorithm="rsa-sha256"`)
	})

	t.Run("returns error when key provider fails", func(t *testing.T) {
		t.Parallel()
		provider := &mockOCIKeyProvider{
			err: errors.New("failed to get security token"),
		}

		signedURL, err := getSignedOCIRequestWithProvider(t.Context(), "ocid1.cluster.test", "me-riyadh-1", provider)
		require.Error(t, err)
		assert.Empty(t, signedURL)
		assert.Contains(t, err.Error(), "error signing OCI cluster request")
	})
}

func TestGetSignedOCIRequestWithRetry(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("returns error immediately when cluster ID is empty", func(t *testing.T) {
		t.Parallel()
		calls := 0
		signed, err := getSignedOCIRequestWithRetry(ctx, time.Second, time.Millisecond, "", "me-riyadh-1", func(_ context.Context, _, _ string) (string, error) {
			calls++
			return "token", nil
		})
		require.Error(t, err)
		assert.Empty(t, signed)
		assert.Equal(t, 0, calls)
	})

	t.Run("will return signed request on first attempt", func(t *testing.T) {
		t.Parallel()
		calls := 0
		signed, err := getSignedOCIRequestWithRetry(ctx, time.Second, time.Millisecond, "cluster-id", "me-riyadh-1", func(_ context.Context, _, _ string) (string, error) {
			calls++
			return "token", nil
		})
		require.NoError(t, err)
		assert.Equal(t, "token", signed)
		assert.Equal(t, 1, calls)
	})

	t.Run("will return signed request on third attempt", func(t *testing.T) {
		t.Parallel()
		calls := 0
		signed, err := getSignedOCIRequestWithRetry(ctx, time.Second, time.Millisecond, "cluster-id", "me-riyadh-1", func(_ context.Context, _, _ string) (string, error) {
			calls++
			if calls < 3 {
				return "", errors.New("some error")
			}
			return "token", nil
		})
		require.NoError(t, err)
		assert.Equal(t, "token", signed)
		assert.Equal(t, 3, calls)
	})

	t.Run("will return error on timeout", func(t *testing.T) {
		t.Parallel()
		signed, err := getSignedOCIRequestWithRetry(ctx, 20*time.Millisecond, 5*time.Millisecond, "cluster-id", "me-riyadh-1", func(_ context.Context, _, _ string) (string, error) {
			return "", errors.New("some error")
		})
		require.Error(t, err)
		assert.Empty(t, signed)
	})
}
