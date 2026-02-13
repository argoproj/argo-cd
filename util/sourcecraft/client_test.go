package sourcecraft

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		options []ClientOption
		wantErr bool
	}{
		{
			name:    "ValidClientWithoutOptions",
			url:     "https://api.example.com",
			options: nil,
			wantErr: false,
		},
		{
			name:    "ValidClientWithToken",
			url:     "https://api.example.com/",
			options: []ClientOption{SetToken("test-token")},
			wantErr: false,
		},
		{
			name:    "ValidClientWithHTTPClient",
			url:     "https://api.example.com",
			options: []ClientOption{SetHTTPClient(&http.Client{Timeout: 10 * time.Second})},
			wantErr: false,
		},
		{
			name: "ValidClientWithMultipleOptions",
			url:  "https://api.example.com",
			options: []ClientOption{
				SetToken("test-token"),
				SetHTTPClient(&http.Client{Timeout: 5 * time.Second}),
			},
			wantErr: false,
		},
		{
			name:    "URLWithTrailingSlash",
			url:     "https://api.example.com/v1/",
			options: nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.url, tt.options...)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, client)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, client)
				assert.NotNil(t, client.client)
				// Verify URL has trailing slash removed
				assert.Equal(t, strings.TrimSuffix(tt.url, "/"), client.url)
			}
		})
	}
}

func TestClient_SetHTTPClient(t *testing.T) {
	client, err := NewClient("https://api.example.com")
	require.NoError(t, err)

	customClient := &http.Client{Timeout: 30 * time.Second}
	client.SetHTTPClient(customClient)

	assert.Equal(t, customClient, client.client)
}

func TestClient_SetToken(t *testing.T) {
	token := "test-access-token"
	client, err := NewClient("https://api.example.com", SetToken(token))
	require.NoError(t, err)

	assert.Equal(t, token, client.accessToken)
}

func TestWithHTTPClient(t *testing.T) {
	tests := []struct {
		name     string
		insecure bool
	}{
		{
			name:     "SecureHTTPClient",
			insecure: false,
		},
		{
			name:     "InsecureHTTPClient",
			insecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient("https://api.example.com", WithHTTPClient(tt.insecure))
			require.NoError(t, err)
			require.NotNil(t, client)
			require.NotNil(t, client.client)

			if tt.insecure {
				// Verify insecure client has cookie jar
				assert.NotNil(t, client.client.Jar, "insecure client should have cookie jar")

				// Verify insecure client has custom transport with TLS config
				assert.NotNil(t, client.client.Transport, "insecure client should have custom transport")

				transport, ok := client.client.Transport.(*http.Transport)
				require.True(t, ok, "transport should be *http.Transport")
				require.NotNil(t, transport.TLSClientConfig, "TLS config should be set")
				assert.True(t, transport.TLSClientConfig.InsecureSkipVerify, "InsecureSkipVerify should be true")
			} else {
				// Verify secure client uses default settings
				assert.Nil(t, client.client.Jar, "secure client should not have cookie jar")
				assert.Nil(t, client.client.Transport, "secure client should use default transport")
			}
		})
	}
}

func TestWithHTTPClient_Integration(t *testing.T) {
	// Test that the client with WithHTTPClient can make actual requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"status":"ok"}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	tests := []struct {
		name     string
		insecure bool
	}{
		{
			name:     "SecureClientMakesRequest",
			insecure: false,
		},
		{
			name:     "InsecureClientMakesRequest",
			insecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(server.URL, WithHTTPClient(tt.insecure))
			require.NoError(t, err)

			resp, err := client.doRequest(context.Background(), "GET", "/test", nil, nil)
			require.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

func TestClient_doRequest(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		token          string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		checkRequest   func(t *testing.T, r *http.Request)
	}{
		{
			name:   "SuccessfulGETRequest",
			method: "GET",
			path:   "/test",
			token:  "",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"status":"ok"}`))
				require.NoError(t, err)
			},
			wantErr: false,
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/test", r.URL.Path)
			},
		},
		{
			name:   "RequestWithToken",
			method: "GET",
			path:   "/protected",
			token:  "secret-token",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr: false,
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				assert.Equal(t, "Bearer secret-token", r.Header.Get("Authorization"))
			},
		},
		{
			name:   "POSTRequest",
			method: "POST",
			path:   "/create",
			token:  "",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusCreated)
			},
			wantErr: false,
			checkRequest: func(t *testing.T, r *http.Request) {
				t.Helper()
				assert.Equal(t, "POST", r.Method)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedRequest *http.Request
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequest = r
				tt.serverResponse(w, r)
			}))
			defer server.Close()

			client, err := NewClient(server.URL, SetToken(tt.token))
			require.NoError(t, err)

			resp, err := client.doRequest(context.Background(), tt.method, tt.path, nil, nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, resp)
				if tt.checkRequest != nil {
					tt.checkRequest(t, capturedRequest)
				}
			}
		})
	}
}

func TestClient_getResponse(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		wantData       string
	}{
		{
			name: "SuccessfulResponse",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"message":"success"}`))
				require.NoError(t, err)
			},
			wantErr:  false,
			wantData: `{"message":"success"}`,
		},
		{
			name: "ErrorResponse400",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, err := w.Write([]byte(`{"message":"bad request"}`))
				require.NoError(t, err)
			},
			wantErr: true,
		},
		{
			name: "ErrorResponse404",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, err := w.Write([]byte(`{"message":"not found"}`))
				require.NoError(t, err)
			},
			wantErr: true,
		},
		{
			name: "ErrorResponse500",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, err := w.Write([]byte(`{"message":"internal server error"}`))
				require.NoError(t, err)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			client, err := NewClient(server.URL)
			require.NoError(t, err)

			data, resp, err := client.getResponse(context.Background(), "GET", "/test", nil, nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tt.wantData, string(data))
			}
		})
	}
}

func TestClient_getParsedResponse(t *testing.T) {
	type TestResponse struct {
		Message string `json:"message"`
		Count   int    `json:"count"`
	}

	tests := []struct {
		name           string
		method         string
		header         http.Header
		body           io.Reader
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		expectedObj    TestResponse
	}{
		{
			name:   "SuccessfulParsing",
			method: "GET",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(TestResponse{Message: "test", Count: 42}))
			},
			wantErr:     false,
			expectedObj: TestResponse{Message: "test", Count: 42},
		},
		{
			name:   "InvalidJSON",
			method: "GET",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{invalid json}`))
				require.NoError(t, err)
			},
			wantErr: true,
		},
		{
			name:   "ErrorResponse",
			method: "GET",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, err := w.Write([]byte(`{"message":"error"}`))
				require.NoError(t, err)
			},
			wantErr: true,
		},
		{
			name:   "ErrorResponseOnPost",
			method: "POST",
			header: http.Header{"key": []string{"value"}},
			body:   strings.NewReader("body"),
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, err := w.Write([]byte(`{"message":"error"}`))
				require.NoError(t, err)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			client, err := NewClient(server.URL)
			require.NoError(t, err)

			var result TestResponse
			resp, err := client.getParsedResponse(context.Background(), tt.method, "/test", tt.header, tt.body, &result)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tt.expectedObj, result)
			}
		})
	}
}

func TestStatusCodeToErr(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		body        string
		wantErr     bool
		errContains string
	}{
		{
			name:       "Success200",
			statusCode: http.StatusOK,
			body:       `{"data":"test"}`,
			wantErr:    false,
		},
		{
			name:       "Success201",
			statusCode: http.StatusCreated,
			body:       `{"data":"created"}`,
			wantErr:    false,
		},
		{
			name:        "Error400WithMessage",
			statusCode:  http.StatusBadRequest,
			body:        `{"message":"invalid input"}`,
			wantErr:     true,
			errContains: "invalid input",
		},
		{
			name:        "Error404WithMessage",
			statusCode:  http.StatusNotFound,
			body:        `{"message":"resource not found"}`,
			wantErr:     true,
			errContains: "resource not found",
		},
		{
			name:        "Error500WithoutMessage",
			statusCode:  http.StatusInternalServerError,
			body:        `{"error":"server error"}`,
			wantErr:     true,
			errContains: "500",
		},
		{
			name:        "ErrorWithInvalidJSON",
			statusCode:  http.StatusBadRequest,
			body:        `invalid json`,
			wantErr:     true,
			errContains: "unknown API error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, err := w.Write([]byte(tt.body))
				require.NoError(t, err)
			}))
			defer server.Close()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, http.NoBody)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			response := newResponse(resp)
			_, err = statusCodeToErr(response)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEscapeValidatePathSegments(t *testing.T) {
	tests := []struct {
		name     string
		segments []*string
		wantErr  bool
		expected []string
	}{
		{
			name:     "ValidSegments",
			segments: []*string{strPtr("org"), strPtr("repo")},
			wantErr:  false,
			expected: []string{"org", "repo"},
		},
		{
			name:     "SegmentsWithSpecialChars",
			segments: []*string{strPtr("org name"), strPtr("repo/name")},
			wantErr:  false,
			expected: []string{"org%20name", "repo%2Fname"},
		},
		{
			name:     "EmptySegment",
			segments: []*string{strPtr("org"), strPtr("")},
			wantErr:  true,
		},
		{
			name:     "NilSegment",
			segments: []*string{strPtr("org"), nil},
			wantErr:  true,
		},
		{
			name:     "AllValidSegments",
			segments: []*string{strPtr("my-org"), strPtr("my-repo"), strPtr("main")},
			wantErr:  false,
			expected: []string{"my-org", "my-repo", "main"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := escapeValidatePathSegments(tt.segments...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				for i, seg := range tt.segments {
					assert.Equal(t, tt.expected[i], *seg)
				}
			}
		})
	}
}

func TestClient_ConcurrentAccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"status":"ok"}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, SetToken("initial-token"))
	require.NoError(t, err)

	// Test concurrent reads and writes
	done := make(chan bool)
	for i := range 10 {
		go func(id int) {
			// Concurrent reads
			_, err := client.doRequest(context.Background(), "GET", "/test", nil, nil)
			assert.NoError(t, err)

			// Concurrent token updates
			client.SetHTTPClient(&http.Client{Timeout: time.Duration(id) * time.Second})

			done <- true
		}(i)
	}

	for range 10 {
		<-done
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err = client.doRequest(ctx, "GET", "/test", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestClient_CustomHeaders(t *testing.T) {
	var capturedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	customHeaders := http.Header{
		"X-Custom-Header": []string{"custom-value"},
		"Content-Type":    []string{"application/json"},
	}

	_, err = client.doRequest(context.Background(), "GET", "/test", customHeaders, nil)
	require.NoError(t, err)
	assert.Equal(t, "custom-value", capturedHeaders.Get("X-Custom-Header"))
	assert.Equal(t, "application/json", capturedHeaders.Get("Content-Type"))
}

// Helper function to create string pointers
func strPtr(s string) *string {
	return &s
}
