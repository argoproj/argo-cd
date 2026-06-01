package apiclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	ctx := context.Background()
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
	ctx := context.Background()
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
				ctx := context.Background()
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
	ctx := context.Background()
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
