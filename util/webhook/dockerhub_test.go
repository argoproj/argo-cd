package webhook

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDockerhubParser_Parse(t *testing.T) {
	const secret = "parse-test-secret"
	parser := newDockerHubParser(secret)
	tests := []struct {
		name       string
		method     string
		body       string
		expectErr  bool
		expectSkip bool
		expected   *RegistryEvent
	}{
		{
			name:   "valid push event with repo_name",
			method: http.MethodPost,
			body: `{
				"repository": {
					"name": "repo",
					"namespace": "user",
					"repo_name": "user/repo",
					"status": "Active"
				},
				"push_data": {
					"tag": "1.0.0"
				}
			}`,
			expected: &RegistryEvent{
				RegistryURL: "docker.io",
				Repository:  "user/repo",
				Tag:         "1.0.0",
			},
		},
		{
			name:   "assemble repository from namespace and name",
			method: http.MethodPost,
			body: `{
				"repository": {
					"name": "repo",
					"namespace": "user"
				},
				"push_data": {
					"tag": "2.0.0"
				}
			}`,
			expected: &RegistryEvent{
				RegistryURL: "docker.io",
				Repository:  "user/repo",
				Tag:         "2.0.0",
			},
		},
		{
			name:   "official image is canonicalized to library namespace",
			method: http.MethodPost,
			body: `{
				"repository": {
					"name": "nginx"
				},
				"push_data": {
					"tag": "latest"
				}
			}`,
			expected: &RegistryEvent{
				RegistryURL: "docker.io",
				Repository:  "library/nginx",
				Tag:         "latest",
			},
		},
		{
			name:   "helm chart push is parsed like any other artifact",
			method: http.MethodPost,
			body: `{
				"repository": {
					"repo_name": "user/my-chart"
				},
				"push_data": {
					"tag": "1.2.3"
				}
			}`,
			expected: &RegistryEvent{
				RegistryURL: "docker.io",
				Repository:  "user/my-chart",
				Tag:         "1.2.3",
			},
		},
		{
			name:   "missing repository",
			method: http.MethodPost,
			body: `{
				"push_data": {
					"tag": "1.0.0"
				}
			}`,
			expectSkip: true,
		},
		{
			name:   "missing tag",
			method: http.MethodPost,
			body: `{
				"repository": {
					"repo_name": "user/repo"
				}
			}`,
			expectSkip: true,
		},
		{
			name:      "invalid json",
			method:    http.MethodPost,
			body:      `{invalid}`,
			expectErr: true,
		},
		{
			name:      "non-POST method",
			method:    http.MethodGet,
			body:      `{}`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), tt.method, "/?secret="+secret, strings.NewReader(tt.body))
			event, err := parser.Parse(req)

			if tt.expectErr {
				require.Error(t, err)
				require.Nil(t, event)
				return
			}

			if tt.expectSkip {
				require.NoError(t, err)
				require.Nil(t, event)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, event)
		})
	}
}

func TestDockerhubParser_CanHandle(t *testing.T) {
	tests := []struct {
		name     string
		secret   string
		query    string
		expected bool
	}{
		{"dockerhub type", "configured-secret", "type=dockerhub", true},
		{"ghcr type", "configured-secret", "type=ghcr", false},
		{"empty type", "configured-secret", "type=", false},
		{"missing type", "configured-secret", "", false},
		// With no secret configured the parser must claim nothing, so the
		// endpoint is never reachable without authentication.
		{"no secret configured disables the parser", "", "type=dockerhub", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newDockerHubParser(tt.secret)
			target := "/api/webhook"
			if tt.query != "" {
				target += "?" + tt.query
			}
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, target, http.NoBody)
			assert.Equal(t, tt.expected, p.CanHandle(req))
		})
	}
}

func TestDockerHubPushEvent(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandlerWithDockerHubSecret("correct-secret", []string{})

	payload, err := os.ReadFile("testdata/dockerhub-push-event.json")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook?type=dockerhub&secret=correct-secret", io.NopCloser(bytes.NewReader(payload)))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	h.Shutdown()

	assert.Equal(t, http.StatusOK, w.Code)
	assertLogContains(t, hook, "Received registry webhook event")
}

func TestDockerHubPushEvent_SecretInAuthorizationHeader(t *testing.T) {
	hook := test.NewGlobal()
	h := NewMockHandlerWithDockerHubSecret("correct-secret", []string{})

	payload, err := os.ReadFile("testdata/dockerhub-push-event.json")
	require.NoError(t, err)

	// No secret in the URL: a proxy moved it into the Authorization header.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook?type=dockerhub", io.NopCloser(bytes.NewReader(payload)))
	req.Header.Set("Authorization", "correct-secret")
	w := httptest.NewRecorder()
	h.Handler(w, req)
	h.Shutdown()

	assert.Equal(t, http.StatusOK, w.Code)
	assertLogContains(t, hook, "Received registry webhook event")
}

// With no secret configured the parser claims nothing, so the request falls through
// to "Unknown webhook event" rather than reaching an unauthenticated endpoint.
func TestDockerHubPushEvent_DisabledWithoutSecret(t *testing.T) {
	h := NewMockHandlerWithDockerHubSecret("", []string{})

	payload, err := os.ReadFile("testdata/dockerhub-push-event.json")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook?type=dockerhub", io.NopCloser(bytes.NewReader(payload)))
	w := httptest.NewRecorder()
	h.Handler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDockerHubPushEvent_Unauthorized(t *testing.T) {
	h := NewMockHandlerWithDockerHubSecret("correct-secret", []string{})

	payload, err := os.ReadFile("testdata/dockerhub-push-event.json")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook?type=dockerhub&secret=wrong-secret", io.NopCloser(bytes.NewReader(payload)))
	w := httptest.NewRecorder()
	h.Handler(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDockerhubParser_validateSecret(t *testing.T) {
	const secret = "my-secret"

	tests := []struct {
		name           string
		configured     string
		providedSecret string
		providedHeader string
		expectError    bool
		expectSentinel bool
	}{
		{
			name:           "valid secret in query parameter",
			configured:     secret,
			providedSecret: secret,
		},
		{
			name:           "valid secret in Authorization header",
			configured:     secret,
			providedHeader: secret,
		},
		{
			name:           "header takes precedence over query parameter",
			configured:     secret,
			providedHeader: secret,
			providedSecret: "wrong-secret",
		},
		{
			name:           "bad header is not rescued by a good query parameter",
			configured:     secret,
			providedHeader: "wrong-secret",
			providedSecret: secret,
			expectError:    true,
			expectSentinel: true,
		},
		{
			name:           "invalid secret",
			configured:     secret,
			providedSecret: "wrong-secret",
			expectError:    true,
			expectSentinel: true,
		},
		{
			name:           "invalid secret in Authorization header",
			configured:     secret,
			providedHeader: "wrong-secret",
			expectError:    true,
			expectSentinel: true,
		},
		{
			name:           "missing secret",
			configured:     secret,
			expectError:    true,
			expectSentinel: true,
		},
		{
			// Unreachable in practice (CanHandle refuses first), but validateSecret
			// must fail closed rather than wave the request through.
			name:           "no secret configured fails closed",
			configured:     "",
			providedSecret: "anything",
			expectError:    true,
			expectSentinel: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := newDockerHubParser(tt.configured)

			target := "/"
			if tt.providedSecret != "" {
				target = "/?secret=" + tt.providedSecret
			}
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, target, http.NoBody)
			if tt.providedHeader != "" {
				req.Header.Set("Authorization", tt.providedHeader)
			}

			err := parser.validateSecret(req)

			if tt.expectError {
				require.Error(t, err)
				if tt.expectSentinel {
					// Docker Hub cannot sign payloads, so a failure here is a
					// pre-shared secret mismatch, never an HMAC failure.
					require.ErrorIs(t, err, ErrSecretVerificationFailed)
					require.NotErrorIs(t, err, ErrHMACVerificationFailed)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
