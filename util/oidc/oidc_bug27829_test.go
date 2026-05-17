package oidc

import (
	"testing"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v3/util/settings"
)

// mockProviderWithScopes returns a Provider whose ParseConfig() reports
// specific scopes_supported, letting us control what OfflineAccess() sees.
type mockProviderWithScopes struct {
	fakeProvider
	scopesSupported []string
}

func (m *mockProviderWithScopes) ParseConfig() (*OIDCConfiguration, error) {
	return &OIDCConfiguration{
		ScopesSupported: m.scopesSupported,
	}, nil
}

// TestBug27829_Fix_DexWebFlow_IncludesOfflineAccess verifies the fix:
// getScopes() now calls OfflineAccess() and appends offline_access when
// the provider supports it, matching the CLI path's behavior.
func TestBug27829_Fix_DexWebFlow_IncludesOfflineAccess(t *testing.T) {
	t.Run("Dex configured, provider supports offline_access", func(t *testing.T) {
		cdSettings := &settings.ArgoCDSettings{
			URL: "https://argocd.example.com",
			DexConfig: `connectors:
- type: github
  name: GitHub
  config:
    clientID: aabbccddeeff00112233
    clientSecret: aabbccddeeff00112233`,
		}
		provider := &mockProviderWithScopes{
			scopesSupported: []string{
				"openid", "profile", "email", "groups",
				"offline_access", "federated:id",
			},
		}
		app := &ClientApp{
			settings: cdSettings,
			provider: provider,
		}

		scopes := app.getScopes()

		// FIX VERIFIED: offline_access is now included
		assert.Contains(t, scopes, gooidc.ScopeOfflineAccess,
			"After fix: getScopes() includes offline_access when provider supports it")
		assert.Contains(t, scopes, "openid")
		assert.Contains(t, scopes, "federated:id")
	})

	t.Run("Dex configured, provider does NOT support offline_access", func(t *testing.T) {
		cdSettings := &settings.ArgoCDSettings{
			URL: "https://argocd.example.com",
			DexConfig: `connectors:
- type: github
  name: GitHub
  config:
    clientID: aabbccddeeff00112233
    clientSecret: aabbccddeeff00112233`,
		}
		provider := &mockProviderWithScopes{
			scopesSupported: []string{"openid", "profile", "email"},
		}
		app := &ClientApp{
			settings: cdSettings,
			provider: provider,
		}

		scopes := app.getScopes()

		// Should NOT include offline_access when provider doesn't support it
		assert.NotContains(t, scopes, gooidc.ScopeOfflineAccess,
			"offline_access NOT added when provider doesn't support it")
	})

	t.Run("OIDC config with explicit requestedScopes", func(t *testing.T) {
		cdSettings := &settings.ArgoCDSettings{
			OIDCConfigRAW: `
name: Test
issuer: https://example.com
clientID: test-client
clientSecret: test-secret
requestedScopes:
- openid
- profile
`,
		}
		provider := &mockProviderWithScopes{
			scopesSupported: []string{"openid", "offline_access"},
		}
		app := &ClientApp{
			settings: cdSettings,
			provider: provider,
		}

		scopes := app.getScopes()

		// User's explicit scopes are preserved, offline_access is appended
		assert.Contains(t, scopes, "openid")
		assert.Contains(t, scopes, "profile")
		assert.Contains(t, scopes, gooidc.ScopeOfflineAccess,
			"offline_access appended to user's explicit scopes")
	})
}

// TestBug27829_OfflineAccess_DetectsCorrectly proves the building blocks work.
func TestBug27829_OfflineAccess_DetectsCorrectly(t *testing.T) {
	assert.True(t, OfflineAccess([]string{"openid", "offline_access"}))
	assert.False(t, OfflineAccess([]string{"openid", "profile"}))
	assert.True(t, OfflineAccess(nil))
	assert.True(t, OfflineAccess([]string{}))
}
