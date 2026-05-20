package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGHCRParser_Parse(t *testing.T) {
	parser := newGHCRParser("")
	tests := []struct {
		name       string
		body       string
		expectErr  bool
		expectSkip bool
		expected   *RegistryEvent
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
			expected: &RegistryEvent{
				RegistryURL: "ghcr.io",
				Repository:  "user/repo",
				Tag:         "1.0.0",
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
			expectSkip: true,
		},
		{
			name: "ignore non-container package",
			body: `{
				"action": "published",
				"package": {
					"name": "repo",
					"package_type": "npm"
				}
			}`,
			expectSkip: true,
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
			expectSkip: true,
		},
		{
			name:      "invalid json",
			body:      `{invalid}`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(tt.body))
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
		expectHMAC  bool
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
			expectHMAC:  true,
		},
		{
			name:        "invalid signature",
			secret:      secret,
			headerSig:   "sha256=deadbeef",
			expectError: true,
			expectHMAC:  true,
		},
		{
			name:   "no secret configured (skip validation)",
			secret: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := newGHCRParser(tt.secret)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", http.NoBody)

			if tt.headerSig != "" {
				req.Header.Set("X-Hub-Signature-256", tt.headerSig)
			}

			err := parser.validateSignature(req, body)

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
