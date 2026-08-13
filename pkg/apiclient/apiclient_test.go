package apiclient

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
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

func TestExecuteRequest_ClosesBodyOnHTTPError(t *testing.T) {
	t.Parallel()
	bodyClosed := &atomic.Bool{}

	// Create a test server that returns HTTP 500 error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Create client with custom httpClient that tracks body closure
	originalTransport := http.DefaultTransport
	customTransport := &testTransport{
		base:       originalTransport,
		bodyClosed: bodyClosed,
	}

	c := &client{
		ServerAddr: server.URL[7:], // Remove "http://"
		PlainText:  true,
		httpClient: &http.Client{
			Transport: customTransport,
		},
		GRPCWebRootPath: "",
	}

	// Execute request that should fail with HTTP 500
	ctx := t.Context()
	md := metadata.New(map[string]string{})
	_, err := c.executeRequest(ctx, "/test.Service/Method", []byte("test"), md)

	// Verify error was returned
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed with status code 500")

	// Give a small delay to ensure Close() was called
	time.Sleep(10 * time.Millisecond)

	// Verify body was closed to prevent connection leak
	assert.True(t, bodyClosed.Load(), "response body should be closed on HTTP error to prevent connection leak")
}

func TestExecuteRequest_ClosesBodyOnGRPCError(t *testing.T) {
	t.Parallel()
	bodyClosed := &atomic.Bool{}

	// Create a test server that returns HTTP 200 but with gRPC error status
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Grpc-Status", "3") // codes.InvalidArgument
		w.Header().Set("Grpc-Message", "invalid argument")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create client with custom httpClient that tracks body closure
	originalTransport := http.DefaultTransport
	customTransport := &testTransport{
		base:       originalTransport,
		bodyClosed: bodyClosed,
	}

	c := &client{
		ServerAddr: server.URL[7:], // Remove "http://"
		PlainText:  true,
		httpClient: &http.Client{
			Transport: customTransport,
		},
		GRPCWebRootPath: "",
	}

	// Execute request that should fail with gRPC error
	ctx := t.Context()
	md := metadata.New(map[string]string{})
	_, err := c.executeRequest(ctx, "/test.Service/Method", []byte("test"), md)

	// Verify gRPC error was returned
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid argument")

	// Give a small delay to ensure Close() was called
	time.Sleep(10 * time.Millisecond)

	// Verify body was closed to prevent connection leak
	assert.True(t, bodyClosed.Load(), "response body should be closed on gRPC error to prevent connection leak")
}

func TestExecuteRequest_ConcurrentErrorRequests_NoConnectionLeak(t *testing.T) {
	t.Parallel()
	// This test simulates the scenario from the test script:
	// Multiple concurrent requests that fail should all close their response bodies

	var totalRequests atomic.Int32
	var closedBodies atomic.Int32

	// Create a test server that always returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		totalRequests.Add(1)
		// Alternate between HTTP errors and gRPC errors
		if totalRequests.Load()%2 == 0 {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.Header().Set("Grpc-Status", strconv.Itoa(int(codes.PermissionDenied)))
			w.Header().Set("Grpc-Message", "permission denied")
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	// Create client with custom transport that tracks closures
	customTransport := &testTransport{
		base:       http.DefaultTransport,
		bodyClosed: &atomic.Bool{},
		onClose: func() {
			closedBodies.Add(1)
		},
	}

	c := &client{
		ServerAddr: server.URL[7:],
		PlainText:  true,
		httpClient: &http.Client{
			Transport: customTransport,
		},
		GRPCWebRootPath: "",
	}

	// Simulate concurrent requests like in the test script
	concurrency := 10
	iterations := 5

	var wg sync.WaitGroup
	for range iterations {
		for range concurrency {
			wg.Go(func() {
				ctx := t.Context()
				md := metadata.New(map[string]string{})
				_, err := c.executeRequest(ctx, "/application.ApplicationService/ManagedResources", []byte("test"), md)
				// We expect errors
				assert.Error(t, err)
			})
		}
		wg.Wait()
	}

	// Give time for all Close() calls to complete
	time.Sleep(100 * time.Millisecond)

	// Verify all response bodies were closed
	expectedTotal := int32(concurrency * iterations)
	assert.Equal(t, expectedTotal, totalRequests.Load(), "all requests should have been made")
	assert.Equal(t, expectedTotal, closedBodies.Load(), "all response bodies should be closed to prevent connection leaks")
}

func TestExecuteRequest_SuccessDoesNotCloseBodyPrematurely(t *testing.T) {
	t.Parallel()
	// Verify that successful requests do NOT close the body in executeRequest
	// (caller is responsible for closing in success case)

	bodyClosed := &atomic.Bool{}

	// Create a test server that returns success
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Grpc-Status", "0") // codes.OK
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	customTransport := &testTransport{
		base:       http.DefaultTransport,
		bodyClosed: bodyClosed,
	}

	c := &client{
		ServerAddr: server.URL[7:],
		PlainText:  true,
		httpClient: &http.Client{
			Transport: customTransport,
		},
		GRPCWebRootPath: "",
	}

	// Execute successful request
	ctx := t.Context()
	md := metadata.New(map[string]string{})
	resp, err := c.executeRequest(ctx, "/test.Service/Method", []byte("test"), md)

	// Verify success
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()

	// Verify body was NOT closed by executeRequest (caller's responsibility)
	time.Sleep(10 * time.Millisecond)
	assert.False(t, bodyClosed.Load(), "response body should NOT be closed by executeRequest on success - caller is responsible")
}

// testTransport wraps http.RoundTripper to track body closures
type testTransport struct {
	base       http.RoundTripper
	bodyClosed *atomic.Bool
	onClose    func() // Optional callback for each close
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Wrap the response body to track Close() calls
	resp.Body = &closeTracker{
		ReadCloser: resp.Body,
		closed:     t.bodyClosed,
		onClose:    t.onClose,
	}

	return resp, nil
}

type closeTracker struct {
	io.ReadCloser
	closed  *atomic.Bool
	onClose func()
}

func (c *closeTracker) Close() error {
	c.closed.Store(true)
	if c.onClose != nil {
		c.onClose()
	}
	return c.ReadCloser.Close()
}

// TestNewClient_HttpRetryMax_TLSTransport is a regression test for the bug
// where --http-retry-max was silently ignored on TLS (non-plaintext)
// connections. The retryable client's Transport must survive TLS setup.
func TestNewClient_HttpRetryMax_TLSTransport(t *testing.T) {
	t.Run("retry transport survives on TLS connection", func(t *testing.T) {
		ci, err := NewClient(&ClientOptions{
			ServerAddr:   "argocd.example.com:443",
			HttpRetryMax: 4,
			Insecure:     true, // avoid needing a real CA; still a TLS (non-plaintext) client
			GRPCWeb:      true, // skip the plain-gRPC server probe; matches real --grpc-web usage
		})
		require.NoError(t, err)
		c := ci.(*client)

		rt, ok := c.httpClient.Transport.(*retryablehttp.RoundTripper)
		require.True(t, ok, "expected retryablehttp.RoundTripper, got %T", c.httpClient.Transport)
		assert.Equal(t, 4, rt.Client.RetryMax)

		// TLS config must still be honored via the retry client's inner transport.
		inner, ok := rt.Client.HTTPClient.Transport.(*http.Transport)
		require.True(t, ok, "expected inner *http.Transport, got %T", rt.Client.HTTPClient.Transport)
		require.NotNil(t, inner.TLSClientConfig)
		assert.True(t, inner.TLSClientConfig.InsecureSkipVerify)
	})

	t.Run("no retry transport when HttpRetryMax is zero", func(t *testing.T) {
		ci, err := NewClient(&ClientOptions{
			ServerAddr: "argocd.example.com:443",
			Insecure:   true,
			GRPCWeb:    true,
		})
		require.NoError(t, err)
		c := ci.(*client)

		tr, ok := c.httpClient.Transport.(*http.Transport)
		require.True(t, ok, "expected plain *http.Transport, got %T", c.httpClient.Transport)
		require.NotNil(t, tr.TLSClientConfig)
	})
}

// TestExecuteRequest_RetriesOn502 proves that with HttpRetryMax set, a
// transient 502 is retried rather than returned fatally on the first attempt.
func TestExecuteRequest_RetriesOn502(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) < 3 {
			w.WriteHeader(http.StatusBadGateway) // 502 on first two attempts
			return
		}
		w.Header().Set("Grpc-Status", "0") // OK on the third
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 4
	retryClient.RetryWaitMin = time.Millisecond
	retryClient.RetryWaitMax = 5 * time.Millisecond
	retryClient.Logger = nil

	c := &client{
		ServerAddr: server.URL[7:], // Remove "http://"
		PlainText:  true,
		httpClient: retryClient.StandardClient(),
	}

	ctx := t.Context()
	md := metadata.New(map[string]string{})
	_, err := c.executeRequest(ctx, "/test.Service/Method", []byte("test"), md)
	require.NoError(t, err)
	assert.Equal(t, int32(3), attempts.Load(), "expected two 502 retries before success")
}

// TestNewClient_RetriesOn502_OverTLS is the end-to-end regression test for the
// bug: it wires the client through NewClient (not a hand-built httpClient),
// talks to a real TLS server, and verifies that --http-retry-max actually
// retries a transient 502 over that TLS connection. Against the pre-fix code
// the retry transport is clobbered by TLS setup and this fails on the first 502.
func TestNewClient_RetriesOn502_OverTLS(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) < 3 {
			w.WriteHeader(http.StatusBadGateway) // 502 on first two attempts
			return
		}
		w.Header().Set("Grpc-Status", "0") // OK on the third
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ci, err := NewClient(&ClientOptions{
		ServerAddr:   server.Listener.Addr().String(), // real TLS endpoint
		HttpRetryMax: 4,
		Insecure:     true, // trust the httptest self-signed cert
		GRPCWeb:      true, // skip the plain-gRPC probe; matches real --grpc-web usage
	})
	require.NoError(t, err)
	c := ci.(*client)

	// Speed up backoff so the test doesn't wait seconds between retries.
	rt, ok := c.httpClient.Transport.(*retryablehttp.RoundTripper)
	require.True(t, ok, "expected retryablehttp.RoundTripper, got %T", c.httpClient.Transport)
	rt.Client.RetryWaitMin = time.Millisecond
	rt.Client.RetryWaitMax = 5 * time.Millisecond
	rt.Client.Logger = nil

	ctx := t.Context()
	md := metadata.New(map[string]string{})
	_, err = c.executeRequest(ctx, "/test.Service/Method", []byte("test"), md)
	require.NoError(t, err)
	assert.Equal(t, int32(3), attempts.Load(), "expected two 502 retries before success over TLS")
}
