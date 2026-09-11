package http

import (
	"fmt"
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

func TestWithServerSideTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		url             string
		timeout         time.Duration
		expectedTimeout string
	}{
		{
			name:            "regular request",
			url:             "https://kubernetes.example/api/v1/pods",
			timeout:         time.Minute,
			expectedTimeout: "1m0s",
		},
		{
			name:            "preserves existing query parameters",
			url:             "https://kubernetes.example/api/v1/pods?limit=500&resourceVersion=123",
			timeout:         30 * time.Second,
			expectedTimeout: "30s",
		},
		{
			name:            "watch is left for the API server to classify",
			url:             "https://kubernetes.example/api/v1/pods?watch=true",
			timeout:         time.Minute,
			expectedTimeout: "1m0s",
		},
		{
			name:            "preserves explicit request timeout",
			url:             "https://kubernetes.example/api/v1/pods?timeout=5s",
			timeout:         time.Minute,
			expectedTimeout: "5s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var receivedRequest *http.Request
			inner := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				receivedRequest = req
				return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
			})
			wrapped := WithServerSideTimeout(tt.timeout)(inner)
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, tt.url, http.NoBody)
			require.NoError(t, err)
			originalRawQuery := req.URL.RawQuery

			resp, err := wrapped.RoundTrip(req)
			require.NoError(t, err)
			require.NotNil(t, receivedRequest)
			assert.Equal(t, tt.expectedTimeout, receivedRequest.URL.Query().Get("timeout"))
			assert.Equal(t, req.URL.Query().Get("limit"), receivedRequest.URL.Query().Get("limit"))
			assert.Equal(t, req.URL.Query().Get("resourceVersion"), receivedRequest.URL.Query().Get("resourceVersion"))
			_, hasDeadline := receivedRequest.Context().Deadline()
			assert.False(t, hasDeadline)
			assert.Equal(t, originalRawQuery, req.URL.RawQuery, "the original request must not be mutated")
			require.NoError(t, resp.Body.Close())
		})
	}
}

func TestWithServerSideTimeoutIsTransparentWrapper(t *testing.T) {
	t.Parallel()

	inner := &TestRoundTripper{}
	wrapped := WithServerSideTimeout(time.Minute)(inner)
	wrapper, ok := wrapped.(utilnet.RoundTripperWrapper)
	require.True(t, ok)
	assert.Same(t, inner, wrapper.WrappedRoundTripper())
}

func TestWithServerSideTimeoutUsesDefaultTransportWhenInnerIsNil(t *testing.T) {
	t.Parallel()

	wrapped := WithServerSideTimeout(time.Minute)(nil)
	wrapper, ok := wrapped.(utilnet.RoundTripperWrapper)
	require.True(t, ok)
	assert.Same(t, http.DefaultTransport, wrapper.WrappedRoundTripper())
}

func TestWithServerSideTimeoutDisabledReturnsInnerTransport(t *testing.T) {
	t.Parallel()

	inner := &TestRoundTripper{}
	assert.Same(t, inner, WithServerSideTimeout(0)(inner))
}
