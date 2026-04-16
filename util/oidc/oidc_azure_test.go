package oidc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util"
	"github.com/argoproj/argo-cd/v3/util/cache"
	"github.com/argoproj/argo-cd/v3/util/crypto"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

func createTestJWT(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("test-secret"))
	require.NoError(t, err)
	return tokenString
}

func TestHasAzureGroupsClaimOverflow(t *testing.T) {
	tests := []struct {
		name     string
		claims   jwt.MapClaims
		expected bool
	}{
		{
			name: "no overage indicators",
			claims: jwt.MapClaims{
				"sub": "user123",
			},
			expected: false,
		},
		{
			name: "with overage claim",
			claims: jwt.MapClaims{
				"sub": "user123",
				"_claim_sources": map[string]any{
					"src1": map[string]any{
						"endpoint": "https://graph.windows.net/tenant-id/users/user-id/getMemberObjects",
					},
				},
				"_claim_names": map[string]any{
					"groups": "src1",
				},
			},
			expected: true,
		},
		{
			name: "with _claim_names but no groups key",
			claims: jwt.MapClaims{
				"sub": "user123",
				"_claim_sources": map[string]any{
					"src1": map[string]any{},
				},
				"_claim_names": map[string]any{
					"roles": "src1",
				},
			},
			expected: false,
		},
		{
			name: "with _claim_names but missing _claim_sources",
			claims: jwt.MapClaims{
				"sub": "user123",
				"_claim_names": map[string]any{
					"groups": "src1",
				},
			},
			expected: false,
		},
		{
			name: "with nil _claim_names",
			claims: jwt.MapClaims{
				"sub":            "user123",
				"_claim_names":   nil,
				"_claim_sources": map[string]any{},
			},
			expected: false,
		},
		{
			name: "with multiple overage keys including groups",
			claims: jwt.MapClaims{
				"sub": "user123",
				"_claim_sources": map[string]any{
					"src1": map[string]any{},
					"src2": map[string]any{},
				},
				"_claim_names": map[string]any{
					"groups": "src1",
					"roles":  "src2",
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasAzureGroupsClaimOverflow(tt.claims)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasUserReadScope(t *testing.T) {
	tests := []struct {
		name        string
		accessToken string
		expected    bool
	}{
		{
			name:        "valid token with User.Read scope",
			accessToken: createTestJWT(t, jwt.MapClaims{"scp": "User.Read profile email"}),
			expected:    true,
		},
		{
			name:        "valid token without User.Read scope",
			accessToken: createTestJWT(t, jwt.MapClaims{"scp": "profile email"}),
			expected:    false,
		},
		{
			name:        "case insensitive User.Read",
			accessToken: createTestJWT(t, jwt.MapClaims{"scp": "user.read profile"}),
			expected:    true,
		},
		{
			name:        "token without scp claim",
			accessToken: createTestJWT(t, jwt.MapClaims{"sub": "user123"}),
			expected:    false,
		},
		{
			name:        "invalid token",
			accessToken: "invalid.jwt.token",
			expected:    false,
		},
		{
			name:        "User.Read in array format",
			accessToken: createTestJWT(t, jwt.MapClaims{"scp": []string{"User.Read", "profile"}}),
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasUserReadScope(tt.accessToken)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetUserGroupsFromAzureOverageClaim(t *testing.T) {
	overageClaims := jwt.MapClaims{
		"sub": "user123",
		"exp": float64(time.Now().Add(5 * time.Minute).Unix()),
		"_claim_sources": map[string]any{
			"src1": map[string]any{
				"endpoint": "https://graph.windows.net/tenant-id/users/user-id/getMemberObjects",
			},
		},
		"_claim_names": map[string]any{
			"groups": "src1",
		},
	}

	tests := []struct {
		name           string
		claims         jwt.MapClaims
		idpHandler     func(w http.ResponseWriter, r *http.Request)
		cacheItems     map[string]string // key -> value to pre-populate (will be encrypted)
		expectedGroups []string
		expectError    bool
	}{
		{
			name:   "no overage claim returns nil",
			claims: jwt.MapClaims{"sub": "user123", "exp": float64(time.Now().Add(5 * time.Minute).Unix())},
			idpHandler: func(_ http.ResponseWriter, _ *http.Request) {
				t.Fatal("Graph API should not be called when no overage claim")
			},
			expectedGroups: nil,
			expectError:    false,
		},
		{
			name:   "successful Graph API call",
			claims: overageClaims,
			idpHandler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				err := json.NewEncoder(w).Encode(map[string]any{
					"value": []string{"group-id-1", "group-id-2"},
				})
				require.NoError(t, err)
			},
			cacheItems: map[string]string{
				FormatAccessTokenCacheKey("user123"): createTestJWT(t, jwt.MapClaims{"scp": "User.Read profile email"}),
			},
			expectedGroups: []string{"group-id-1", "group-id-2"},
			expectError:    false,
		},
		{
			name:   "no access token in cache",
			claims: overageClaims,
			idpHandler: func(_ http.ResponseWriter, _ *http.Request) {
				t.Fatal("Graph API should not be called without access token")
			},
			cacheItems:     nil,
			expectedGroups: nil,
			expectError:    true,
		},
		{
			name:   "access token missing User.Read scope",
			claims: overageClaims,
			idpHandler: func(_ http.ResponseWriter, _ *http.Request) {
				t.Fatal("Graph API should not be called without User.Read scope")
			},
			cacheItems: map[string]string{
				FormatAccessTokenCacheKey("user123"): createTestJWT(t, jwt.MapClaims{"scp": "profile email"}),
			},
			expectedGroups: nil,
			expectError:    true,
		},
		{
			name:   "Graph API returns error",
			claims: overageClaims,
			idpHandler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
			cacheItems: map[string]string{
				FormatAccessTokenCacheKey("user123"): createTestJWT(t, jwt.MapClaims{"scp": "User.Read profile email"}),
			},
			expectedGroups: nil,
			expectError:    true,
		},
		{
			name:   "Graph API rate limited",
			claims: overageClaims,
			idpHandler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusTooManyRequests)
			},
			cacheItems: map[string]string{
				FormatAccessTokenCacheKey("user123"): createTestJWT(t, jwt.MapClaims{"scp": "User.Read profile email"}),
			},
			expectedGroups: nil,
			expectError:    true,
		},
		{
			name:   "cached groups returned without Graph API call",
			claims: overageClaims,
			idpHandler: func(_ http.ResponseWriter, _ *http.Request) {
				t.Fatal("Graph API should not be called when groups are cached")
			},
			cacheItems: map[string]string{
				FormatAccessTokenCacheKey("user123"):                createTestJWT(t, jwt.MapClaims{"scp": "User.Read profile email"}),
				FormatAzureGroupsOverageResponseCacheKey("user123"): `["cached-group-1","cached-group-2"]`,
			},
			expectedGroups: []string{"cached-group-1", "cached-group-2"},
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(tt.idpHandler))
			defer ts.Close()

			signature, err := util.MakeSignature(32)
			require.NoError(t, err)
			cdSettings := &settings.ArgoCDSettings{
				ServerSignature: signature,
				OIDCConfigRAW:   `{"azure": {"enableUserGroupOverageClaim": true}}`,
			}
			encryptionKey, err := cdSettings.GetServerEncryptionKey()
			require.NoError(t, err)

			testCache := cache.NewInMemoryCache(24 * time.Hour)
			a, err := NewClientApp(cdSettings, "", nil, "/argo-cd", testCache)
			require.NoError(t, err)

			for key, value := range tt.cacheItems {
				encValue, err := crypto.Encrypt([]byte(value), encryptionKey)
				require.NoError(t, err)
				err = a.clientCache.Set(&cache.Item{
					Key:    key,
					Object: encValue,
				})
				require.NoError(t, err)
			}

			groups, err := a.GetUserGroupsFromAzureOverageClaim(t.Context(), tt.claims, ts.URL)
			assert.Equal(t, tt.expectedGroups, groups)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestFormatAzureGroupsOverageResponseCacheKey(t *testing.T) {
	key := FormatAzureGroupsOverageResponseCacheKey("user123")
	assert.Equal(t, "azure_groups_overage_response_user123", key)
}
