package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGHCRParser_Parse(t *testing.T) {
	parser := NewGHCRParser()

	tests := []struct {
		name      string
		body      string
		expectErr bool
		expected  *WebhookRegistryEvent
	}{
		{
			name: "valid container package event",
			body: `{
				"action": "published",
				"package": {
					"name": "repo",
					"package_type": "container",
					"owner": { "login": "user" },
					"package_version": {
						"container_metadata": {
							"tag": {
								"name": "1.0.0",
								"digest": "sha256:abc123"
							}
						}
					}
				}
			}`,
			expected: &WebhookRegistryEvent{
				RegistryURL: "ghcr.io",
				Repository:  "user/repo",
				Tag:         "1.0.0",
				Digest:      "sha256:abc123",
			},
		},
		{
			name: "ignore non-published action",
			body: `{
				"action": "updated",
				"package": {
					"name": "repo",
					"package_type": "container"
				}
			}`,
			expectErr: true,
		},
		{
			name: "ignore non-container package",
			body: `{
				"action": "updated",
				"package": {
					"name": "repo",
					"package_type": "npm"
				}
			}`,
			expectErr: true,
		},
		{
			name: "missing tag",
			body: `{
				"action": "published",
				"package": {
					"name": "repo",
					"package_type": "container",
					"owner": { "login": "user" },
					"package_version": {
						"container_metadata": {
							"tag": { "name": "" }
						}
					}
				}
			}`,
			expectErr: true,
		},
		{
			name:      "invalid json",
			body:      `{invalid}`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := parser.Parse([]byte(tt.body))

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, event)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, event)
		})
	}
}

func TestValidateSignature(t *testing.T) {
	body := []byte(`{"test":"payload"}`)
	secret := "my-secret"

	computeSig := func(secret string, body []byte) string {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		return "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}

	tests := []struct {
		name        string
		secret      string
		headerSig   string
		expectError bool
	}{
		{
			name:      "valid signature",
			secret:    secret,
			headerSig: computeSig(secret, body),
		},
		{
			name:        "missing signature header",
			secret:      secret,
			expectError: true,
		},
		{
			name:        "invalid signature",
			secret:      secret,
			headerSig:   "sha256=deadbeef",
			expectError: true,
		},
		{
			name:   "no secret configured (skip validation)",
			secret: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &WebhookRegistryHandler{
				secret: tt.secret,
			}

			req := httptest.NewRequest(http.MethodPost, "/", nil)

			if tt.headerSig != "" {
				req.Header.Set("X-Hub-Signature-256", tt.headerSig)
			}

			err := handler.validateSignature(req, body)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
