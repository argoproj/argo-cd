package webhook

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testHarborSecret = "my-harbor-secret"

func TestHarborParser_CanHandle(t *testing.T) {
	tests := []struct {
		name       string
		secret     string
		authHeader string
		want       bool
	}{
		{
			name:       "matching secret",
			secret:     testHarborSecret,
			authHeader: testHarborSecret,
			want:       true,
		},
		{
			name:       "wrong secret",
			secret:     testHarborSecret,
			authHeader: "wrong-secret",
			want:       false,
		},
		{
			name:       "empty auth header",
			secret:     testHarborSecret,
			authHeader: "",
			want:       false,
		},
		{
			name:       "no secret configured",
			secret:     "",
			authHeader: testHarborSecret,
			want:       false,
		},
		{
			name:   "no secret configured, no auth header",
			secret: "",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := newHarborParser(tt.secret)
			req := httptest.NewRequestWithContext(t.Context(), "POST", "/api/webhook",
				strings.NewReader("{}"))
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			assert.Equal(t, tt.want, parser.CanHandle(req))
		})
	}
}

func TestHarborParser_Parse(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		expectErr  bool
		expectSkip bool
		expected   *RegistryEvent
	}{
		{
			name: "valid push artifact event",
			body: `{
				"type": "PUSH_ARTIFACT",
				"occur_at": 1680501893,
				"operator": "admin",
				"event_data": {
					"resources": [
						{
							"digest": "sha256:abc123",
							"tag": "v1.2.3",
							"resource_url": "harbor.example.com/myproject/mychart:v1.2.3"
						}
					],
					"repository": {
						"name": "mychart",
						"namespace": "myproject",
						"repo_full_name": "myproject/mychart",
						"repo_type": "private"
					}
				}
			}`,
			expected: &RegistryEvent{
				RegistryURL: "harbor.example.com",
				Repository:  "myproject/mychart",
				Tag:         "v1.2.3",
			},
		},
		{
			name: "non-push event is skipped",
			body: `{
				"type": "PULL_ARTIFACT",
				"occur_at": 1680501893,
				"operator": "admin",
				"event_data": {
					"resources": [
						{
							"digest": "sha256:abc123",
							"tag": "v1.0.0",
							"resource_url": "harbor.example.com/project/chart:v1.0.0"
						}
					],
					"repository": {
						"name": "chart",
						"namespace": "project",
						"repo_full_name": "project/chart"
					}
				}
			}`,
			expectSkip: true,
		},
		{
			name: "all resources lack tags",
			body: `{
				"type": "PUSH_ARTIFACT",
				"occur_at": 1680501893,
				"operator": "harbor-jobservice",
				"event_data": {
					"resources": [
						{
							"digest": "sha256:deadbeef",
							"tag": "",
							"resource_url": "harbor.example.com/project/repo@sha256:deadbeef"
						}
					],
					"repository": {
						"name": "repo",
						"namespace": "project",
						"repo_full_name": "project/repo"
					}
				}
			}`,
			expectSkip: true,
		},
		{
			name: "repo_full_name absent, falls back to namespace/name",
			body: `{
				"type": "PUSH_ARTIFACT",
				"occur_at": 1680501893,
				"operator": "admin",
				"event_data": {
					"resources": [
						{
							"digest": "sha256:abc123",
							"tag": "2.0.0",
							"resource_url": "registry.corp.com/team/app:2.0.0"
						}
					],
					"repository": {
						"name": "app",
						"namespace": "team",
						"repo_full_name": ""
					}
				}
			}`,
			expected: &RegistryEvent{
				RegistryURL: "registry.corp.com",
				Repository:  "team/app",
				Tag:         "2.0.0",
			},
		},
		{
			name:      "invalid JSON",
			body:      `{invalid json}`,
			expectErr: true,
		},
		{
			name: "repo_full_name, namespace, and name all absent",
			body: `{
				"type": "PUSH_ARTIFACT",
				"occur_at": 1680501893,
				"operator": "admin",
				"event_data": {
					"resources": [
						{
							"digest": "sha256:abc123",
							"tag": "v1.0.0",
							"resource_url": "registry.corp.com/project/app:v1.0.0"
						}
					],
					"repository": {
						"name": "",
						"namespace": "",
						"repo_full_name": ""
					}
				}
			}`,
			expectErr: true,
		},
	}

	parser := newHarborParser(testHarborSecret)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), "POST", "/api/webhook",
				strings.NewReader(tt.body))
			result, err := parser.Parse(req)

			if tt.expectErr {
				require.Error(t, err)
				require.Nil(t, result)
				return
			}

			if tt.expectSkip {
				require.NoError(t, err)
				require.Nil(t, result)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestParseHarborRegistryURL(t *testing.T) {
	tests := []struct {
		name        string
		resourceURL string
		want        string
		wantErr     bool
	}{
		{
			name:        "standard URL with tag",
			resourceURL: "harbor.example.com/project/repo:v1.0.0",
			want:        "harbor.example.com",
		},
		{
			name:        "URL with digest",
			resourceURL: "harbor.example.com/project/repo@sha256:abc123",
			want:        "harbor.example.com",
		},
		{
			name:        "URL with scheme",
			resourceURL: "https://harbor.example.com/project/repo:v1.0.0",
			want:        "harbor.example.com",
		},
		{
			name:        "localhost URL",
			resourceURL: "localhost/namespace/name:tag",
			want:        "localhost",
		},
		{
			name:        "hostname only, no slash",
			resourceURL: "harbor.example.com",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHarborRegistryURL(tt.resourceURL)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
