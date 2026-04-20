package services

import (
	"crypto/tls"
	"net/http"
	"testing"
	"time"

	bitbucketv1 "github.com/gfleury/go-bitbucket-v1"
	"github.com/stretchr/testify/require"
)

func TestSetupBitbucketClient(t *testing.T) {
	ctx := t.Context()
	cfg := &bitbucketv1.Configuration{}

	// Act
	client := SetupBitbucketClient(ctx, cfg, "", false, nil)

	// Assert
	require.NotNil(t, client, "expected client to be created")
	require.NotNil(t, cfg.HTTPClient, "expected HTTPClient to be set")

	// The transport should be a clone of DefaultTransport
	tr, ok := cfg.HTTPClient.Transport.(*http.Transport)
	require.True(t, ok, "expected HTTPClient.Transport to be *http.Transport")
	require.NotSame(t, http.DefaultTransport, tr, "transport should be a clone, not the global DefaultTransport")

	// Ensure TLSClientConfig is set
	require.IsType(t, &tls.Config{}, tr.TLSClientConfig)

	// Defaults from http.DefaultTransport.Clone() should be preserved
	require.Greater(t, tr.IdleConnTimeout, time.Duration(0), "IdleConnTimeout should be non-zero")
	require.Positive(t, tr.MaxIdleConns, "MaxIdleConns should be non-zero")
	require.Greater(t, tr.TLSHandshakeTimeout, time.Duration(0), "TLSHandshakeTimeout should be non-zero")
}
