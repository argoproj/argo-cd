package oidc

import (
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

type cacheItem struct {
	key     string
	value   string
	encrypt bool
}

type expectedCacheItem struct {
	key             string
	value           string
	expectEncrypted bool
	expectError     bool
}

var claimsBare = jwt.MapClaims{
	"sub": "user123",
	"exp": float64(time.Now().Add(5 * time.Minute).Unix()),
	"scp": "User.Read profile email",
}

var claimsWithOverage = jwt.MapClaims{
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
	"scp": "User.Read profile email",
}

var claimsWithOverageAndMultipleKeys = jwt.MapClaims{
	"sub": "user123",
	"exp": float64(time.Now().Add(5 * time.Minute).Unix()),
	"_claim_sources": map[string]any{
		"src1": map[string]any{
			"endpoint": "https://graph.windows.net/tenant-id/users/user-id/getMemberObjects",
		},
		"src2": map[string]any{
			"endpoint": "https://graph.windows.net/tenant-id/users/user-id/getMemberObjects",
		},
	},
	"_claim_names": map[string]any{
		"groups": "src1",
		"roles":  "src2",
	},
	"scp": "User.Read profile email",
}

func TestHasGroupsOverageClaim(t *testing.T) {
	tests := []struct {
		name     string
		claims   jwt.MapClaims
		expected bool
	}{
		{
			name:     "no overage claim",
			claims:   claimsBare,
			expected: false,
		},
		{
			name:     "with overage claim",
			claims:   claimsWithOverage,
			expected: true,
		},
		{
			name:     "with overage claim and multiple keys",
			claims:   claimsWithOverageAndMultipleKeys,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasOverflow := hasGroupsOverageClaim(tt.claims)
			assert.Equal(t, tt.expected, hasOverflow)
		})
	}
}

func TestHasUserReadScope(t *testing.T) {
	tests := []struct {
		name        string
		accessToken string
		expected    bool
		description string
	}{
		{
			name:        "valid token with User.Read scope",
			accessToken: createTestJwt(t, jwt.MapClaims{"scp": "User.Read profile email"}),
			expected:    true,
			description: "should find User.Read in space-separated scopes",
		},
		{
			name:        "valid token with User.Read in array",
			accessToken: createTestJwt(t, jwt.MapClaims{"scp": []string{"User.Read", "profile", "email"}}),
			expected:    true,
			description: "should find User.Read in array format",
		},
		{
			name:        "valid token without User.Read scope",
			accessToken: createTestJwt(t, jwt.MapClaims{"scp": "profile email"}),
			expected:    false,
			description: "should not find User.Read when not present",
		},
		{
			name:        "token with case insensitive User.Read",
			accessToken: createTestJwt(t, jwt.MapClaims{"scp": "user.read profile email"}),
			expected:    true,
			description: "should find User.Read case insensitively",
		},
		{
			name:        "token without scp claim",
			accessToken: createTestJwt(t, jwt.MapClaims{"sub": "user123"}),
			expected:    false,
			description: "should return false when scp claim is missing",
		},
		{
			name:        "invalid token",
			accessToken: "invalid.jwt.token",
			expected:    false,
			description: "should return false for invalid token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasUserReadScope(tt.accessToken)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

func TestFetchGroupsFromGraphApi(t *testing.T) {
	tests := []struct {
		name               string
		expectedOutput     any
		expectError        bool
		expectedCacheItems []expectedCacheItem
		idpHandler         func(w http.ResponseWriter, r *http.Request)
		idpClaims          jwt.MapClaims // as per specification sub and exp are REQUIRED fields
		cache              cache.CacheClient
		cacheItems         []cacheItem
	}{
		{
			name:               "call with bad accessToken",
			expectedOutput:     []string(nil),
			expectError:        true,
			expectedCacheItems: []expectedCacheItem{},
			idpClaims:          claimsWithOverage,
			idpHandler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			cache: cache.NewInMemoryCache(24 * time.Hour),
			cacheItems: []cacheItem{
				{
					key:     FormatAccessTokenCacheKey(claimsWithOverage["sub"].(string)),
					value:   "BadAccessToken",
					encrypt: true,
				},
			},
		},
		{
			name:               "call with garbage returned",
			expectedOutput:     []string(nil),
			expectError:        true,
			expectedCacheItems: []expectedCacheItem{},
			idpClaims:          claimsWithOverage,
			idpHandler: func(w http.ResponseWriter, _ *http.Request) {
				garbageBytes := "garbageBytes"
				_, err := w.Write([]byte(garbageBytes))
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusTeapot)
			},
			cache: cache.NewInMemoryCache(24 * time.Hour),
			cacheItems: []cacheItem{
				{
					key:     FormatAccessTokenCacheKey(claimsWithOverage["sub"].(string)),
					value:   createTestJwt(t, claimsWithOverage),
					encrypt: true,
				},
			},
		},
		{
			name:           "call without accessToken in cache",
			expectedOutput: []string(nil),
			expectError:    true,
			expectedCacheItems: []expectedCacheItem{
				{
					key:         FormatAccessTokenCacheKey(claimsWithOverage["sub"].(string)),
					expectError: true,
				},
			},
			idpClaims: claimsWithOverage,
			idpHandler: func(w http.ResponseWriter, _ *http.Request) {
				responseBytes := `
				{
					"groups":["githubOrg:engineers"]
				}`
				w.Header().Set("content-type", "application/json")
				_, err := w.Write([]byte(responseBytes))
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			},
			cache: cache.NewInMemoryCache(24 * time.Hour),
		},
		{
			name:           "call with valid accessToken in cache",
			expectedOutput: []string{"githubOrg:engineers"},
			expectError:    false,
			expectedCacheItems: []expectedCacheItem{
				{
					key:             FormatAzureGroupsOverageResponseCacheKey(claimsWithOverage["sub"].(string)),
					value:           "[\"githubOrg:engineers\"]",
					expectEncrypted: true,
					expectError:     false,
				},
			},
			idpClaims: claimsWithOverage,
			idpHandler: func(w http.ResponseWriter, _ *http.Request) {
				responseBytes := `
				{
					"value":["githubOrg:engineers"]
				}`
				w.Header().Set("content-type", "application/json")
				_, err := w.Write([]byte(responseBytes))
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			},
			cache: cache.NewInMemoryCache(24 * time.Hour),
			cacheItems: []cacheItem{
				{
					key:     FormatAccessTokenCacheKey(claimsWithOverage["sub"].(string)),
					value:   createTestJwt(t, claimsWithOverage),
					encrypt: true,
				},
			},
		},
		{
			name:               "call with valid groups in cache",
			expectedOutput:     []string{"githubOrg:engineers"},
			expectError:        false,
			expectedCacheItems: []expectedCacheItem{},
			idpClaims:          claimsWithOverage,
			idpHandler: func(_ http.ResponseWriter, _ *http.Request) {
				assert.FailNow(t, "IDP handler should not be called when groups are in cache")
			},
			cache: cache.NewInMemoryCache(24 * time.Hour),
			cacheItems: []cacheItem{
				{
					key:     FormatAccessTokenCacheKey(claimsWithOverage["sub"].(string)),
					value:   createTestJwt(t, claimsWithOverage),
					encrypt: true,
				},
				{
					key:     FormatAzureGroupsOverageResponseCacheKey(claimsWithOverage["sub"].(string)),
					value:   "[\"githubOrg:engineers\"]",
					encrypt: true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(tt.idpHandler))
			defer ts.Close()

			signature, err := util.MakeSignature(32)
			require.NoError(t, err)
			cdSettings := &settings.ArgoCDSettings{ServerSignature: signature}
			encryptionKey, err := cdSettings.GetServerEncryptionKey()
			require.NoError(t, err)
			a, _ := NewClientApp(cdSettings, "", nil, "/argo-cd", tt.cache)

			for _, item := range tt.cacheItems {
				var newValue []byte
				newValue = []byte(item.value)
				if item.encrypt {
					newValue, err = crypto.Encrypt([]byte(item.value), encryptionKey)
					require.NoError(t, err)
				}
				err := a.clientCache.Set(&cache.Item{
					Key:    item.key,
					Object: newValue,
				})
				require.NoError(t, err)
			}

			got, err := a.GetUserGroupsFromAzureOverageClaim(tt.idpClaims, ts.URL)
			assert.Equal(t, tt.expectedOutput, got)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			for _, item := range tt.expectedCacheItems {
				var tmpValue []byte
				err := a.clientCache.Get(item.key, &tmpValue)
				if item.expectError {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					if item.expectEncrypted {
						tmpValue, err = crypto.Decrypt(tmpValue, encryptionKey)
						require.NoError(t, err)
					}
					assert.Equal(t, item.value, string(tmpValue))
				}
			}
		})
	}
}

// Helper function to create test JWT tokens
func createTestJwt(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("test-secret"))
	require.NoError(t, err)
	return tokenString
}
