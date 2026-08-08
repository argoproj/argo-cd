package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	utilnet "k8s.io/apimachinery/pkg/util/net"

	"github.com/argoproj/argo-cd/v3/common"
)

func TestCookieMaxLength(t *testing.T) {
	t.Parallel()
	cookies, err := MakeCookieMetadata("foo", "bar")
	require.NoError(t, err)
	assert.Equal(t, "foo=bar", cookies[0])

	// keys will be of format foo, foo-1, foo-2 ..
	cookies, err = MakeCookieMetadata("foo", strings.Repeat("_", (maxCookieLength-5)*maxCookieNumber))
	require.EqualError(t, err, "the authentication token is 81760 characters long and requires 21 cookies but the max number of cookies is 20. Contact your Argo CD administrator to increase the max number of cookies")
	assert.Empty(t, cookies)
}

func TestCookieWithAttributes(t *testing.T) {
	t.Parallel()
	flags := []string{"SameSite=lax", "httpOnly"}

	cookies, err := MakeCookieMetadata("foo", "bar", flags...)
	require.NoError(t, err)
	assert.Equal(t, "foo=bar; SameSite=lax; httpOnly", cookies[0])
}

func TestSplitCookie(t *testing.T) {
	t.Parallel()
	cookieValue := strings.Repeat("_", (maxCookieLength-6)*4)
	cookies, err := MakeCookieMetadata("foo", cookieValue)
	require.NoError(t, err)
	assert.Len(t, cookies, 4)
	assert.Len(t, strings.Split(cookies[0], "="), 2)
	token := strings.Split(cookies[0], "=")[1]
	assert.Len(t, strings.Split(token, ":"), 2)
	assert.Equal(t, "4", strings.Split(token, ":")[0])

	cookies = append(cookies, "bar=this-entry-should-be-filtered")
	var cookieList []*http.Cookie
	for _, cookie := range cookies {
		parts := strings.Split(cookie, "=")
		cookieList = append(cookieList, &http.Cookie{Name: parts[0], Value: parts[1]})
	}
	token, err = JoinCookies("foo", cookieList)
	require.NoError(t, err)
	assert.Equal(t, cookieValue, token)
}

// mockResponseWriter is a mock implementation of http.ResponseWriter.
// It captures added headers for verification in tests.
type mockResponseWriter struct {
	header http.Header
}

func (m *mockResponseWriter) Header() http.Header {
	if m.header == nil {
		m.header = make(http.Header)
	}
	return m.header
}
func (m *mockResponseWriter) Write([]byte) (int, error) { return 0, nil }
func (m *mockResponseWriter) WriteHeader(_ int)         {}

func TestSetTokenCookie(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		token           string
		baseHRef        string
		isSecure        bool
		expectedCookies []string // Expected Set-Cookie header values
	}{
		{
			name:     "Insecure cookie",
			token:    "insecure-token",
			baseHRef: "",
			isSecure: false,
			expectedCookies: []string{
				fmt.Sprintf("%s=%s; path=/; SameSite=lax; httpOnly", common.AuthCookieName, "insecure-token"),
			},
		},
		{
			name:     "Secure cookie",
			token:    "secure-token",
			baseHRef: "",
			isSecure: true,
			expectedCookies: []string{
				fmt.Sprintf("%s=%s; path=/; SameSite=lax; httpOnly; Secure", common.AuthCookieName, "secure-token"),
			},
		},
		{
			name:     "Insecure cookie with baseHRef",
			token:    "token-with-path",
			baseHRef: "/app",
			isSecure: false,
			expectedCookies: []string{
				fmt.Sprintf("%s=%s; path=/app; SameSite=lax; httpOnly", common.AuthCookieName, "token-with-path"),
			},
		},
		{
			name:     "Secure cookie with baseHRef",
			token:    "secure-token-with-path",
			baseHRef: "app/",
			isSecure: true,
			expectedCookies: []string{
				fmt.Sprintf("%s=%s; path=/app; SameSite=lax; httpOnly; Secure", common.AuthCookieName, "secure-token-with-path"),
			},
		},
		{
			name:     "Unsecured cookie, baseHRef with multiple segments and mixed slashes",
			token:    "complex-path-token",
			baseHRef: "///api/v1/auth///",
			isSecure: false,
			expectedCookies: []string{
				fmt.Sprintf("%s=%s; path=/api/v1/auth; SameSite=lax; httpOnly", common.AuthCookieName, "complex-path-token"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := &mockResponseWriter{}

			err := SetTokenCookie(tt.token, tt.baseHRef, tt.isSecure, w)
			if err != nil {
				t.Fatalf("%s: Unexpected error: %v", tt.name, err)
			}

			setCookieHeaders := w.Header()["Set-Cookie"]

			if len(setCookieHeaders) != len(tt.expectedCookies) {
				t.Errorf("Mistmatch in Set-Cookie header length: %s\nExpected: %d\nGot: %d",
					tt.name, len(tt.expectedCookies), len(setCookieHeaders))
				return
			}

			if len(tt.expectedCookies) > 0 && setCookieHeaders[0] != tt.expectedCookies[0] {
				t.Errorf("Mismatch in Set-Cookie header: %s\nExpected: %s\nGot:      %s",
					tt.name, tt.expectedCookies[0], setCookieHeaders[0])
			}
		})
	}
}

// TestRoundTripper just copy request headers to the resposne.
type TestRoundTripper struct{}

func (rt TestRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := http.Response{}
	resp.Header = http.Header{}
	for k, vs := range req.Header {
		for _, v := range vs {
			resp.Header.Add(k, v)
		}
	}
	return &resp, nil
}

func TestTransportWithHeader(t *testing.T) {
	t.Parallel()
	client := &http.Client{}
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "/foo", http.NoBody)
	req.Header.Set("Bar", "req_1")
	req.Header.Set("Foo", "req_1")

	// No default headers.
	client.Transport = &TransportWithHeader{
		RoundTripper: &TestRoundTripper{},
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.Header{
		"Bar": []string{"req_1"},
		"Foo": []string{"req_1"},
	}, resp.Header)

	// with default headers.
	client.Transport = &TransportWithHeader{
		RoundTripper: &TestRoundTripper{},
		Header: http.Header{
			"Foo": []string{"default_1", "default_2"},
		},
	}
	resp, err = client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.Header{
		"Bar": []string{"req_1"},
		"Foo": []string{"default_1", "default_2", "req_1"},
	}, resp.Header)
}

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestIsLongRunningRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		url              string
		serverPathPrefix string
		headers          http.Header
		expected         bool
	}{
		{name: "standard request", url: "https://kubernetes.example/api/v1/pods"},
		{name: "list request", url: "https://kubernetes.example/api/v1/pods?limit=500&resourceVersion=123"},
		{name: "watch request", url: "https://kubernetes.example/api/v1/pods?watch=true", expected: true},
		{name: "watch alternate boolean", url: "https://kubernetes.example/api/v1/pods?watch=1", expected: true},
		{name: "watch false", url: "https://kubernetes.example/api/v1/pods?watch=false"},
		{name: "watch true after false", url: "https://kubernetes.example/api/v1/pods?watch=false&watch=true", expected: true},
		{name: "followed pod log", url: "https://kubernetes.example/api/v1/namespaces/default/pods/pod/log?follow=true", expected: true},
		{name: "pod log without follow", url: "https://kubernetes.example/api/v1/namespaces/default/pods/pod/log"},
		{name: "upgrade header", url: "https://kubernetes.example/api/v1/pods", headers: http.Header{"Upgrade": {"websocket"}}, expected: true},
		{name: "connection upgrade token", url: "https://kubernetes.example/api/v1/pods", headers: http.Header{"Connection": {"keep-alive, Upgrade"}}, expected: true},
		{name: "connection substring is not token", url: "https://kubernetes.example/api/v1/pods", headers: http.Header{"Connection": {"notupgrade"}}},
		{name: "pod exec", url: "https://kubernetes.example/api/v1/namespaces/default/pods/pod/exec", expected: true},
		{name: "pod attach", url: "https://kubernetes.example/api/v1/namespaces/default/pods/pod/attach", expected: true},
		{name: "pod portforward", url: "https://kubernetes.example/api/v1/namespaces/default/pods/pod/portforward", expected: true},
		{name: "execution is not exec", url: "https://kubernetes.example/api/v1/namespaces/default/pods/pod/execution"},
		{name: "pod named exec", url: "https://kubernetes.example/api/v1/namespaces/default/pods/exec"},
		{name: "service proxy", url: "https://kubernetes.example/api/v1/namespaces/default/services/service/proxy/path", expected: true},
		{name: "proxying is not proxy", url: "https://kubernetes.example/api/v1/namespaces/default/services/service/proxying"},
		{name: "service named proxy", url: "https://kubernetes.example/api/v1/namespaces/default/services/proxy"},
		{name: "node proxy", url: "https://kubernetes.example/api/v1/nodes/node/proxy", expected: true},
		{name: "proxy prefix regular request", url: "https://kubernetes.example/proxy/k8s/api/v1/secrets", serverPathPrefix: "/proxy/k8s"},
		{name: "proxy prefix list request with encoded selector", url: "https://kubernetes.example/proxy/k8s/api/v1/secrets?limit=500&labelSelector=app%3Dargocd", serverPathPrefix: "/proxy/k8s"},
		{name: "proxy prefix pod exec", url: "https://kubernetes.example/proxy/k8s/api/v1/namespaces/default/pods/pod/exec", serverPathPrefix: "/proxy/k8s", expected: true},
		{name: "server prefix resembling service proxy", url: "https://kubernetes.example/services/gateway/proxy/urn:cluster/api/v1/secrets", serverPathPrefix: "/services/gateway/proxy/urn:cluster"},
		{name: "service proxy after server prefix", url: "https://kubernetes.example/services/gateway/proxy/urn:cluster/api/v1/namespaces/default/services/service/proxy/path", serverPathPrefix: "/services/gateway/proxy/urn:cluster", expected: true},
		{name: "complex proxy subpath regular request", url: "https://k8s-proxy.example/proxy/k8s/namespaces/namespace-id/api/v1/pods", serverPathPrefix: "/proxy/k8s/namespaces/namespace-id"},
		{name: "complex proxy subpath watch request", url: "https://k8s-proxy.example/proxy/k8s/namespaces/namespace-id/api/v1/pods?watch=true", serverPathPrefix: "/proxy/k8s/namespaces/namespace-id", expected: true},
		{name: "complex proxy subpath pod exec", url: "https://k8s-proxy.example/proxy/k8s/namespaces/namespace-id/api/v1/namespaces/default/pods/pod/exec", serverPathPrefix: "/proxy/k8s/namespaces/namespace-id", expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, tt.url, http.NoBody)
			require.NoError(t, err)
			req.Header = tt.headers
			assert.Equal(t, tt.expected, isLongRunningRequest(req, tt.serverPathPrefix))
		})
	}

	assert.False(t, isLongRunningRequest(nil, ""))
	assert.False(t, isLongRunningRequest(&http.Request{}, ""))
}

func TestStripServerPathPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		requestPath      string
		serverPathPrefix string
		expected         string
	}{
		{name: "empty prefix", requestPath: "/api/v1/pods", expected: "/api/v1/pods"},
		{name: "matching prefix", requestPath: "/proxy/urn:cluster/api/v1/pods", serverPathPrefix: "/proxy/urn:cluster", expected: "/api/v1/pods"},
		{name: "exact prefix", requestPath: "/proxy/urn:cluster", serverPathPrefix: "/proxy/urn:cluster", expected: "/"},
		{name: "prefix must end at segment boundary", requestPath: "/proxy/urn:cluster-other/api/v1/pods", serverPathPrefix: "/proxy/urn:cluster", expected: "/proxy/urn:cluster-other/api/v1/pods"},
		{name: "different prefix", requestPath: "/other/api/v1/pods", serverPathPrefix: "/proxy/urn:cluster", expected: "/other/api/v1/pods"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, stripServerPathPrefix(tt.requestPath, tt.serverPathPrefix))
		})
	}
}

func TestTimeoutForNonLongRunningRequestsSetsExpectedDeadline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		url              string
		serverPathPrefix string
		expectsDeadline  bool
	}{
		{name: "regular request", url: "https://kubernetes.example/api/v1/pods", expectsDeadline: true},
		{name: "regular request behind server path prefix", url: "https://kubernetes.example/services/gateway/proxy/urn:cluster/api/v1/pods", serverPathPrefix: "/services/gateway/proxy/urn:cluster/", expectsDeadline: true},
		{name: "watch request", url: "https://kubernetes.example/api/v1/pods?watch=true"},
		{name: "followed pod log", url: "https://kubernetes.example/api/v1/namespaces/default/pods/pod/log?follow=true"},
		{name: "pod exec", url: "https://kubernetes.example/api/v1/namespaces/default/pods/pod/exec"},
		{name: "service proxy behind server path prefix", url: "https://kubernetes.example/services/gateway/proxy/urn:cluster/api/v1/namespaces/default/services/service/proxy/path", serverPathPrefix: "/services/gateway/proxy/urn:cluster"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var requestContext context.Context
			inner := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				requestContext = req.Context()
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("ok")),
				}, nil
			})
			wrapped := WithTimeoutForNonLongRunningRequests(time.Minute, tt.serverPathPrefix)(inner)
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, tt.url, http.NoBody)
			require.NoError(t, err)

			resp, err := wrapped.RoundTrip(req)
			require.NoError(t, err)
			_, hasDeadline := requestContext.Deadline()
			assert.Equal(t, tt.expectsDeadline, hasDeadline)
			require.NoError(t, resp.Body.Close())
		})
	}
}

func TestTimeoutForNonLongRunningRequestsTimesOutRoundTrip(t *testing.T) {
	t.Parallel()

	inner := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
	wrapped := WithTimeoutForNonLongRunningRequests(10*time.Millisecond, "")(inner)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://kubernetes.example/api/v1/pods", http.NoBody)
	require.NoError(t, err)

	_, err = wrapped.RoundTrip(req)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.NoError(t, req.Context().Err())
}

type contextBlockingBody struct {
	ctx context.Context
}

func (b *contextBlockingBody) Read(_ []byte) (int, error) {
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}

func (b *contextBlockingBody) Close() error {
	return nil
}

func TestTimeoutForNonLongRunningRequestsTimesOutResponseBodyRead(t *testing.T) {
	t.Parallel()

	inner := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       &contextBlockingBody{ctx: req.Context()},
		}, nil
	})
	wrapped := WithTimeoutForNonLongRunningRequests(10*time.Millisecond, "")(inner)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://kubernetes.example/api/v1/pods", http.NoBody)
	require.NoError(t, err)

	resp, err := wrapped.RoundTrip(req)
	require.NoError(t, err)
	_, err = io.ReadAll(resp.Body)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.NoError(t, resp.Body.Close())
}

func TestTimeoutForNonLongRunningRequestsCancelsContextAfterBodyCompletion(t *testing.T) {
	t.Parallel()

	for _, closeBody := range []bool{false, true} {
		name := "EOF"
		if closeBody {
			name = "close"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var requestContext context.Context
			inner := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				requestContext = req.Context()
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("ok")),
				}, nil
			})
			wrapped := WithTimeoutForNonLongRunningRequests(time.Minute, "")(inner)
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://kubernetes.example/api/v1/pods", http.NoBody)
			require.NoError(t, err)

			resp, err := wrapped.RoundTrip(req)
			require.NoError(t, err)
			if closeBody {
				require.NoError(t, resp.Body.Close())
			} else {
				_, err = io.ReadAll(resp.Body)
				require.NoError(t, err)
			}
			assert.ErrorIs(t, requestContext.Err(), context.Canceled)
		})
	}
}

func TestTimeoutForNonLongRunningRequestsIsTransparentWrapper(t *testing.T) {
	t.Parallel()

	inner := &TestRoundTripper{}
	wrapped := WithTimeoutForNonLongRunningRequests(time.Minute, "")(inner)
	wrapper, ok := wrapped.(utilnet.RoundTripperWrapper)
	require.True(t, ok)
	assert.Same(t, inner, wrapper.WrappedRoundTripper())
}

func TestTimeoutForNonLongRunningRequestsUsesDefaultTransportWhenInnerIsNil(t *testing.T) {
	t.Parallel()

	wrapped := WithTimeoutForNonLongRunningRequests(time.Minute, "")(nil)
	wrapper, ok := wrapped.(utilnet.RoundTripperWrapper)
	require.True(t, ok)
	assert.Same(t, http.DefaultTransport, wrapper.WrappedRoundTripper())
}

func BenchmarkIsLongRunningRequest(b *testing.B) {
	tests := []struct {
		name             string
		url              string
		serverPathPrefix string
	}{
		{name: "no query", url: "https://kubernetes.example/api/v1/pods"},
		{name: "list query", url: "https://kubernetes.example/api/v1/pods?limit=500&resourceVersion=123"},
		{name: "watch query", url: "https://kubernetes.example/api/v1/pods?watch=true&resourceVersion=123"},
		{name: "proxy prefix no query", url: "https://kubernetes.example/proxy/k8s/api/v1/secrets", serverPathPrefix: "/proxy/k8s"},
		{name: "proxy prefix list query", url: "https://kubernetes.example/proxy/k8s/api/v1/secrets?limit=500&resourceVersion=123", serverPathPrefix: "/proxy/k8s"},
		{name: "proxy prefix pod exec", url: "https://kubernetes.example/proxy/k8s/api/v1/namespaces/default/pods/pod/exec", serverPathPrefix: "/proxy/k8s"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			req, err := http.NewRequestWithContext(b.Context(), http.MethodGet, tt.url, http.NoBody)
			require.NoError(b, err)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				isLongRunningRequest(req, tt.serverPathPrefix)
			}
		})
	}
}
