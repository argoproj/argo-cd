package apiclient

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parseHeaders(t *testing.T) {
	t.Run("Header parsed successfully", func(t *testing.T) {
		headerString := []string{"foo:", "foo1:bar1", "foo2:bar2:bar2"}
		headers, err := parseHeaders(headerString)
		require.NoError(t, err)
		assert.Empty(t, headers.Get("foo"))
		assert.Equal(t, "bar1", headers.Get("foo1"))
		assert.Equal(t, "bar2:bar2", headers.Get("foo2"))
	})

	t.Run("Header parsed error", func(t *testing.T) {
		headerString := []string{"foo"}
		_, err := parseHeaders(headerString)
		assert.ErrorContains(t, err, "additional headers must be colon(:)-separated: foo")
	})
}

func Test_parseGRPCHeaders(t *testing.T) {
	t.Run("Header parsed successfully", func(t *testing.T) {
		headerStrings := []string{"origin: https://foo.bar", "content-length: 123"}
		headers, err := parseGRPCHeaders(headerStrings)
		require.NoError(t, err)
		assert.Equal(t, []string{" https://foo.bar"}, headers.Get("origin"))
		assert.Equal(t, []string{" 123"}, headers.Get("content-length"))
	})

	t.Run("Header parsed error", func(t *testing.T) {
		headerString := []string{"foo"}
		_, err := parseGRPCHeaders(headerString)
		assert.ErrorContains(t, err, "additional headers must be colon(:)-separated: foo")
	})
}

func Test_NewClientTLS(t *testing.T) {
	t.Run("Test transport using TLS", func(t *testing.T) {
		opts := &ClientOptions{
			ServerAddr: "example.com:1234",
			AuthToken:  "dummy-token",
			GRPCWeb:    true,
		}
		cl, err := NewClient(opts)
		require.NoError(t, err)
		cli := cl.(*client)
		transport := cli.httpClient.Transport
		tr, ok := transport.(*http.Transport)
		require.True(t, ok, "expected http.Transport for non-plaintext client")
		require.NotNil(t, tr.TLSClientConfig, "TLSClientConfig should be configured when PlainText is false")
	})
	t.Run("Test client use PROXY from environment", func(t *testing.T) {
		opts := &ClientOptions{
			ServerAddr: "example.com:1234",
			AuthToken:  "dummy-token",
			GRPCWeb:    true,
		}
		t.Setenv("ALL_PROXY", "socks5://127.0.0.1:1080")
		cl, err := NewClient(opts)
		require.NoError(t, err)
		tr := cl.(*client).httpClient.Transport.(*http.Transport)
		require.NotNil(t, tr.DialContext, "DialContext should be configured when ALL_PROXY is set")
	})
	t.Run("Test Plaintext still works", func(t *testing.T) {
		opts := &ClientOptions{
			ServerAddr: "example.com:1234",
			AuthToken:  "dummy-token",
			GRPCWeb:    true,
			PlainText:  true,
		}
		cl, err := NewClient(opts)
		require.NoError(t, err)
		cli := cl.(*client)
		transport := cli.httpClient.Transport
		require.Nil(t, transport, "Transport should be nil for plaintext client")
	})
}
