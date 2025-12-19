package apiclient

import (
	"net/http"
	"net/http/httptest"
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

func TestHTTPClient_ProxyAuthToken(t *testing.T) {
	t.Run("Proxy-Authorization header is sent when ProxyAuthToken is provided", func(t *testing.T) {
		var receivedHeaders http.Header
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedHeaders = r.Header.Clone()
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		c := &client{
			ProxyAuthToken: "test-proxy-token",
			UserAgent:      "test-agent",
		}

		httpClient, err := c.HTTPClient()
		require.NoError(t, err)

		req, err := http.NewRequest("GET", server.URL, nil)
		require.NoError(t, err)

		_, err = httpClient.Do(req)
		require.NoError(t, err)

		assert.Equal(t, "Bearer test-proxy-token", receivedHeaders.Get("Proxy-Authorization"))
	})

	t.Run("Proxy-Authorization header is not sent when ProxyAuthToken is empty", func(t *testing.T) {
		var receivedHeaders http.Header
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedHeaders = r.Header.Clone()
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		c := &client{
			ProxyAuthToken: "",
			UserAgent:      "test-agent",
		}

		httpClient, err := c.HTTPClient()
		require.NoError(t, err)

		req, err := http.NewRequest("GET", server.URL, nil)
		require.NoError(t, err)

		_, err = httpClient.Do(req)
		require.NoError(t, err)

		assert.Empty(t, receivedHeaders.Get("Proxy-Authorization"))
	})

	t.Run("User-Agent header is sent when UserAgent is provided", func(t *testing.T) {
		var receivedHeaders http.Header
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedHeaders = r.Header.Clone()
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		c := &client{
			ProxyAuthToken: "proxy-token",
			UserAgent:      "test-agent/1.0",
		}

		httpClient, err := c.HTTPClient()
		require.NoError(t, err)

		req, err := http.NewRequest("GET", server.URL, nil)
		require.NoError(t, err)

		_, err = httpClient.Do(req)
		require.NoError(t, err)

		assert.Equal(t, "Bearer proxy-token", receivedHeaders.Get("Proxy-Authorization"))
		assert.Equal(t, "test-agent/1.0", receivedHeaders.Get("User-Agent"))
	})
}
