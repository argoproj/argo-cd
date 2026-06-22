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
	parser := newDockerHubParser("")
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
			req := httptest.NewRequestWithContext(t.Context(), tt.method, "/", strings.NewReader(tt.body))
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
	p := newDockerHubParser("")

	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		{"dockerhub type", "type=dockerhub", true},
		{"ghcr type", "type=ghcr", false},
		{"empty type", "type=", false},
		{"missing type", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
	h := NewMockHandler(nil, []string{})

	payload, err := os.ReadFile("testdata/dockerhub-push-event.json")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/webhook?type=dockerhub", io.NopCloser(bytes.NewReader(payload)))
	w := httptest.NewRecorder()
	h.Handler(w, req)
	h.Shutdown()

	assert.Equal(t, http.StatusOK, w.Code)
	assertLogContains(t, hook, "Received registry webhook event")
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
		expectError    bool
		expectHMAC     bool
	}{
		{
			name:           "valid secret",
			configured:     secret,
			providedSecret: secret,
		},
		{
			name:           "invalid secret",
			configured:     secret,
			providedSecret: "wrong-secret",
			expectError:    true,
			expectHMAC:     true,
		},
		{
			name:        "missing secret",
			configured:  secret,
			expectError: true,
			expectHMAC:  true,
		},
		{
			name:           "no secret configured (skip validation)",
			configured:     "",
			providedSecret: "anything",
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

			err := parser.validateSecret(req)

			if tt.expectError {
				require.Error(t, err)
				if tt.expectHMAC {
					require.ErrorIs(t, err, ErrHMACVerificationFailed)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
