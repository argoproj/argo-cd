package oidc

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/server/settings/oidc"
	"github.com/argoproj/argo-cd/v3/util"
	"github.com/argoproj/argo-cd/v3/util/cache"
	cachemocks "github.com/argoproj/argo-cd/v3/util/cache/mocks"
	"github.com/argoproj/argo-cd/v3/util/crypto"
	"github.com/argoproj/argo-cd/v3/util/dex"
	jwtutil "github.com/argoproj/argo-cd/v3/util/jwt"
	"github.com/argoproj/argo-cd/v3/util/settings"
	"github.com/argoproj/argo-cd/v3/util/test"
)

func setupAzureIdentity(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	tokenFilePath := filepath.Join(tempDir, "token.txt")
	tempFile, err := os.Create(tokenFilePath)
	require.NoError(t, err)
	_, err = tempFile.WriteString("serviceAccountToken")
	require.NoError(t, err)
	t.Setenv("AZURE_FEDERATED_TOKEN_FILE", tokenFilePath)
}

func TestGetDomainHint(t *testing.T) {
	t.Run("Returns domain hint when OIDC config is set", func(t *testing.T) {
		settings := &settings.ArgoCDSettings{
			OIDCConfigRAW: `
name: Test OIDC
issuer: https://example.com
clientID: test-client
clientSecret: test-secret
domainHint: example.com
`,
		}
		domainHint := getDomainHint(settings)
		assert.Equal(t, "example.com", domainHint)
	})

	t.Run("Returns empty string when domain hint is not set", func(t *testing.T) {
		settings := &settings.ArgoCDSettings{
			OIDCConfigRAW: `
name: Test OIDC
issuer: https://example.com
clientID: test-client
clientSecret: test-secret
`,
		}
		domainHint := getDomainHint(settings)
		assert.Empty(t, domainHint)
	})

	t.Run("Returns empty string when OIDC config is nil", func(t *testing.T) {
		settings := &settings.ArgoCDSettings{
			OIDCConfigRAW: "",
		}
		domainHint := getDomainHint(settings)
		assert.Empty(t, domainHint)
	})

	t.Run("Returns empty string when YAML is malformed", func(t *testing.T) {
		settings := &settings.ArgoCDSettings{
			OIDCConfigRAW: `{this is not valid yaml at all]`,
		}
		domainHint := getDomainHint(settings)
		assert.Empty(t, domainHint)
	})

	t.Run("Trims whitespaces from domain hint", func(t *testing.T) {
		settings := &settings.ArgoCDSettings{
			OIDCConfigRAW: `
name: Test OIDC
issuer: https://example.com
clientID: test-client
clientSecret: test-secret
domainHint: "  example.com  "
`,
		}
		domainHint := getDomainHint(settings)
		assert.Equal(t, "example.com", domainHint)
	})
}

func TestInferGrantType(t *testing.T) {
	for _, path := range []string{"dex", "okta", "auth0", "onelogin"} {
		t.Run(path, func(t *testing.T) {
			rawConfig, err := os.ReadFile("testdata/" + path + ".json")
			require.NoError(t, err)
			var config OIDCConfiguration
			err = json.Unmarshal(rawConfig, &config)
			require.NoError(t, err)
			grantType := InferGrantType(&config)
			assert.Equal(t, GrantTypeAuthorizationCode, grantType)

			var noCodeResponseTypes []string
			for _, supportedResponseType := range config.ResponseTypesSupported {
				if supportedResponseType != ResponseTypeCode {
					noCodeResponseTypes = append(noCodeResponseTypes, supportedResponseType)
				}
			}

			config.ResponseTypesSupported = noCodeResponseTypes
			grantType = InferGrantType(&config)
			assert.Equal(t, GrantTypeImplicit, grantType)
		})
	}
}

func TestIDTokenClaims(t *testing.T) {
	oauth2Config := &oauth2.Config{
		ClientID:     "DUMMY_OIDC_PROVIDER",
		ClientSecret: "0987654321",
		Endpoint:     oauth2.Endpoint{AuthURL: "https://argocd-dev.onelogin.com/oidc/auth", TokenURL: "https://argocd-dev.onelogin.com/oidc/token"},
		Scopes:       []string{"oidc", "profile", "groups"},
		RedirectURL:  "https://argocd-dev.io/redirect_url",
	}

	var opts []oauth2.AuthCodeOption
	requestedClaims := make(map[string]*oidc.Claim)

	opts = AppendClaimsAuthenticationRequestParameter(opts, requestedClaims)
	assert.Empty(t, opts)

	requestedClaims["groups"] = &oidc.Claim{Essential: true}
	opts = AppendClaimsAuthenticationRequestParameter(opts, requestedClaims)
	assert.Len(t, opts, 1)

	authCodeURL, err := url.Parse(oauth2Config.AuthCodeURL("TEST", opts...))
	require.NoError(t, err)

	values, err := url.ParseQuery(authCodeURL.RawQuery)
	require.NoError(t, err)

	assert.JSONEq(t, "{\"id_token\":{\"groups\":{\"essential\":true}}}", values.Get("claims"))
}

func TestHandleLogin_IncludesDomainHint(t *testing.T) {
	oidcTestServer := test.GetOIDCTestServer(t, nil)
	t.Cleanup(oidcTestServer.Close)

	cdSettings := &settings.ArgoCDSettings{
		URL:                       "https://argocd.example.com",
		OIDCTLSInsecureSkipVerify: true,
		OIDCConfigRAW: fmt.Sprintf(`
name: Test
issuer: %s
clientID: test-client-id
clientSecret: test-client-secret
domainHint: example.com
requestedScopes: ["openid", "profile", "email", "groups"]`, oidcTestServer.URL),
	}
	app, err := NewClientApp(cdSettings, "", nil, "https://argocd.example.com", cache.NewInMemoryCache(24*time.Hour))
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/auth/login", http.NoBody)
	w := httptest.NewRecorder()
	app.HandleLogin(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	location := w.Header().Get("Location")
	assert.Contains(t, location, "domain_hint=example.com")
}

type fakeProvider struct {
	EndpointError bool
}

func (p *fakeProvider) Endpoint() (*oauth2.Endpoint, error) {
	if p.EndpointError {
		return nil, errors.New("fake provider endpoint error")
	}
	return &oauth2.Endpoint{}, nil
}

func (p *fakeProvider) ParseConfig() (*OIDCConfiguration, error) {
	return nil, nil
}

func (p *fakeProvider) Verify(_ context.Context, _ string, _ *settings.ArgoCDSettings) (*gooidc.IDToken, error) {
	return nil, nil
}

func TestHandleCallback(t *testing.T) {
	app := ClientApp{provider: &fakeProvider{}, settings: &settings.ArgoCDSettings{}}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.com/foo", http.NoBody)
	req.Form = url.Values{
		"error":             []string{"login-failed"},
		"error_description": []string{"<script>alert('hello')</script>"},
	}
	w := httptest.NewRecorder()

	app.HandleCallback(w, req)

	assert.Equal(t, "login-failed: &lt;script&gt;alert(&#39;hello&#39;)&lt;/script&gt;\n", w.Body.String())
}

func TestClientApp_HandleLogin(t *testing.T) {
	oidcTestServer := test.GetOIDCTestServer(t, nil)
	t.Cleanup(oidcTestServer.Close)

	dexTestServer := test.GetDexTestServer(t)
	t.Cleanup(dexTestServer.Close)

	t.Run("oidc certificate checking during login should toggle on config", func(t *testing.T) {
		cdSettings := &settings.ArgoCDSettings{
			URL: "https://argocd.example.com",
			OIDCConfigRAW: fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]`, oidcTestServer.URL),
		}
		app, err := NewClientApp(cdSettings, dexTestServer.URL, nil, "https://argocd.example.com", cache.NewInMemoryCache(24*time.Hour))
		require.NoError(t, err)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/auth/login", http.NoBody)

		w := httptest.NewRecorder()

		app.HandleLogin(w, req)

		assert.Contains(t, w.Body.String(), "certificate signed by unknown authority")

		cdSettings.OIDCTLSInsecureSkipVerify = true

		app, err = NewClientApp(cdSettings, dexTestServer.URL, nil, "https://argocd.example.com", cache.NewInMemoryCache(24*time.Hour))
		require.NoError(t, err)

		w = httptest.NewRecorder()

		app.HandleLogin(w, req)

		assert.NotContains(t, w.Body.String(), "certificate is not trusted")
		assert.NotContains(t, w.Body.String(), "certificate signed by unknown authority")
	})

	t.Run("dex certificate checking during login should toggle on config", func(t *testing.T) {
		cdSettings := &settings.ArgoCDSettings{
			URL: "https://argocd.example.com",
			DexConfig: `connectors:
- type: github
  name: GitHub
  config:
    clientID: aabbccddeeff00112233
    clientSecret: aabbccddeeff00112233`,
		}
		cert, err := tls.X509KeyPair(test.Cert, test.PrivateKey)
		require.NoError(t, err)
		cdSettings.Certificate = &cert
		app, err := NewClientApp(cdSettings, dexTestServer.URL, nil, "https://argocd.example.com", cache.NewInMemoryCache(24*time.Hour))
		require.NoError(t, err)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/auth/login", http.NoBody)

		w := httptest.NewRecorder()

		app.HandleLogin(w, req)

		if !strings.Contains(w.Body.String(), "certificate signed by unknown authority") && !strings.Contains(w.Body.String(), "certificate is not trusted") {
			t.Fatal("did not receive expected certificate verification failure error")
		}

		app, err = NewClientApp(cdSettings, dexTestServer.URL, &dex.DexTLSConfig{StrictValidation: false}, "https://argocd.example.com", cache.NewInMemoryCache(24*time.Hour))
		require.NoError(t, err)

		w = httptest.NewRecorder()

		app.HandleLogin(w, req)

		assert.NotContains(t, w.Body.String(), "certificate is not trusted")
		assert.NotContains(t, w.Body.String(), "certificate signed by unknown authority")
	})

	t.Run("OIDC auth", func(t *testing.T) {
		cdSettings := &settings.ArgoCDSettings{
			URL:                       "https://argocd.example.com",
			OIDCTLSInsecureSkipVerify: true,
		}
		oidcConfig := settings.OIDCConfig{
			Name:         "Test",
			Issuer:       oidcTestServer.URL,
			ClientID:     "xxx",
			ClientSecret: "yyy",
		}
		oidcConfigRaw, err := yaml.Marshal(oidcConfig)
		require.NoError(t, err)
		cdSettings.OIDCConfigRAW = string(oidcConfigRaw)

		app, err := NewClientApp(cdSettings, dexTestServer.URL, &dex.DexTLSConfig{StrictValidation: false}, "https://argocd.example.com", cache.NewInMemoryCache(24*time.Hour))
		require.NoError(t, err)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/auth/login", http.NoBody)
		w := httptest.NewRecorder()
		app.HandleLogin(w, req)

		assert.Equal(t, http.StatusSeeOther, w.Code)
		location, err := url.Parse(w.Header().Get("Location"))
		require.NoError(t, err)
		values, err := url.ParseQuery(location.RawQuery)
		require.NoError(t, err)
		assert.Equal(t, []string{"openid", "profile", "email", "groups"}, strings.Split(values.Get("scope"), " "))
		assert.Equal(t, "xxx", values.Get("client_id"))
		assert.Equal(t, "code", values.Get("response_type"))
	})

	t.Run("OIDC auth with custom scopes", func(t *testing.T) {
		cdSettings := &settings.ArgoCDSettings{
			URL:                       "https://argocd.example.com",
			OIDCTLSInsecureSkipVerify: true,
		}
		oidcConfig := settings.OIDCConfig{
			Name:            "Test",
			Issuer:          oidcTestServer.URL,
			ClientID:        "xxx",
			ClientSecret:    "yyy",
			RequestedScopes: []string{"oidc"},
		}
		oidcConfigRaw, err := yaml.Marshal(oidcConfig)
		require.NoError(t, err)
		cdSettings.OIDCConfigRAW = string(oidcConfigRaw)

		app, err := NewClientApp(cdSettings, dexTestServer.URL, &dex.DexTLSConfig{StrictValidation: false}, "https://argocd.example.com", cache.NewInMemoryCache(24*time.Hour))
		require.NoError(t, err)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/auth/login", http.NoBody)
		w := httptest.NewRecorder()
		app.HandleLogin(w, req)

		assert.Equal(t, http.StatusSeeOther, w.Code)
		location, err := url.Parse(w.Header().Get("Location"))
		require.NoError(t, err)
		values, err := url.ParseQuery(location.RawQuery)
		require.NoError(t, err)
		assert.Equal(t, []string{"oidc"}, strings.Split(values.Get("scope"), " "))
		assert.Equal(t, "xxx", values.Get("client_id"))
		assert.Equal(t, "code", values.Get("response_type"))
	})

	t.Run("Dex auth", func(t *testing.T) {
		cdSettings := &settings.ArgoCDSettings{
			URL: dexTestServer.URL,
		}
		dexConfig := map[string]any{
			"connectors": []map[string]any{
				{
					"type": "github",
					"name": "GitHub",
					"config": map[string]any{
						"clientId":     "aabbccddeeff00112233",
						"clientSecret": "aabbccddeeff00112233",
					},
				},
			},
		}
		dexConfigRaw, err := yaml.Marshal(dexConfig)
		require.NoError(t, err)
		cdSettings.DexConfig = string(dexConfigRaw)

		app, err := NewClientApp(cdSettings, dexTestServer.URL, &dex.DexTLSConfig{StrictValidation: false}, "https://argocd.example.com", cache.NewInMemoryCache(24*time.Hour))
		require.NoError(t, err)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/auth/login", http.NoBody)
		w := httptest.NewRecorder()
		app.HandleLogin(w, req)

		assert.Equal(t, http.StatusSeeOther, w.Code)
		location, err := url.Parse(w.Header().Get("Location"))
		require.NoError(t, err)
		values, err := url.ParseQuery(location.RawQuery)
		require.NoError(t, err)
		assert.Equal(t, []string{"openid", "profile", "email", "groups", common.DexFederatedScope}, strings.Split(values.Get("scope"), " "))
		assert.Equal(t, common.ArgoCDClientAppID, values.Get("client_id"))
		assert.Equal(t, "code", values.Get("response_type"))
	})

	t.Run("with additional base URL", func(t *testing.T) {
		cdSettings := &settings.ArgoCDSettings{
			URL:                       "https://argocd.example.com/argocd",
			AdditionalURLs:            []string{"https://localhost:8080/argocd", "https://other.argocd.example.com/argocd"},
			OIDCTLSInsecureSkipVerify: true,
			DexConfig: `connectors:
			- type: github
			  name: GitHub
			  config:
			    clientID: aabbccddeeff00112233
			    clientSecret: aabbccddeeff00112233`,
			OIDCConfigRAW: fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]`, oidcTestServer.URL),
		}
		cert, err := tls.X509KeyPair(test.Cert, test.PrivateKey)
		require.NoError(t, err)
		cdSettings.Certificate = &cert
		app, err := NewClientApp(cdSettings, dexTestServer.URL, &dex.DexTLSConfig{StrictValidation: false}, "/argocd", cache.NewInMemoryCache(24*time.Hour))
		require.NoError(t, err)

		t.Run("should accept login redirecting on the main domain", func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/argocd/auth/login", http.NoBody)

			req.URL.RawQuery = url.Values{
				"return_url": []string{"https://argocd.example.com/argocd/applications"},
			}.Encode()

			w := httptest.NewRecorder()

			app.HandleLogin(w, req)

			assert.Equal(t, http.StatusSeeOther, w.Code)
			location, err := url.Parse(w.Header().Get("Location"))
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("%s://%s", location.Scheme, location.Host), oidcTestServer.URL)
			assert.Equal(t, "/auth", location.Path)
			assert.Equal(t, "https://argocd.example.com/argocd/auth/callback", location.Query().Get("redirect_uri"))
		})

		t.Run("should accept login redirecting on the alternative domains", func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://localhost:8080/argocd/auth/login", http.NoBody)

			req.URL.RawQuery = url.Values{
				"return_url": []string{"https://localhost:8080/argocd/applications"},
			}.Encode()

			w := httptest.NewRecorder()

			app.HandleLogin(w, req)

			assert.Equal(t, http.StatusSeeOther, w.Code)
			location, err := url.Parse(w.Header().Get("Location"))
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("%s://%s", location.Scheme, location.Host), oidcTestServer.URL)
			assert.Equal(t, "/auth", location.Path)
			assert.Equal(t, "https://localhost:8080/argocd/auth/callback", location.Query().Get("redirect_uri"))
		})

		t.Run("should accept login redirecting on the alternative domains", func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://other.argocd.example.com/argocd/auth/login", http.NoBody)

			req.URL.RawQuery = url.Values{
				"return_url": []string{"https://other.argocd.example.com/argocd/applications"},
			}.Encode()

			w := httptest.NewRecorder()

			app.HandleLogin(w, req)

			assert.Equal(t, http.StatusSeeOther, w.Code)
			location, err := url.Parse(w.Header().Get("Location"))
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("%s://%s", location.Scheme, location.Host), oidcTestServer.URL)
			assert.Equal(t, "/auth", location.Path)
			assert.Equal(t, "https://other.argocd.example.com/argocd/auth/callback", location.Query().Get("redirect_uri"))
		})

		t.Run("should preserve alternative domain after root path stripping", func(t *testing.T) {
			requestURL := "https://localhost:8080/argocd/auth/login?" + url.Values{
				"return_url": []string{"https://localhost:8080/argocd/applications"},
			}.Encode()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, requestURL, http.NoBody)
			w := httptest.NewRecorder()
			handler := http.StripPrefix("/argocd", http.HandlerFunc(app.HandleLogin))

			handler.ServeHTTP(w, req)

			require.Equal(t, http.StatusSeeOther, w.Code)
			location, err := url.Parse(w.Header().Get("Location"))
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("%s://%s", location.Scheme, location.Host), oidcTestServer.URL)
			assert.Equal(t, "/auth", location.Path)
			assert.Equal(t, "https://localhost:8080/argocd/auth/callback", location.Query().Get("redirect_uri"))
		})

		t.Run("should deny login redirecting on the alternative domains", func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://not-argocd.example.com/auth/login", http.NoBody)

			req.URL.RawQuery = url.Values{
				"return_url": []string{"https://not-argocd.example.com/applications"},
			}.Encode()

			w := httptest.NewRecorder()

			app.HandleLogin(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Empty(t, w.Header().Get("Location"))
		})
	})
	t.Run("with multiple dex connectors", func(t *testing.T) {
		tests := []struct {
			cdSettings   *settings.ArgoCDSettings
			name         string
			expectConnID bool
		}{
			{
				name: "set connector_id if specified",
				cdSettings: &settings.ArgoCDSettings{
					URL:                dexTestServer.URL,
					DexConfig:          "connectors: [{id: github}, {id: gitlab}]",
					DexAuthConnectorID: "github",
				},
				expectConnID: true,
			},
			{
				name: "omit connector_id if empty",
				cdSettings: &settings.ArgoCDSettings{
					URL:                dexTestServer.URL,
					DexConfig:          "connectors: [{id: github}, {id: gitlab}]",
					DexAuthConnectorID: "",
				},
				expectConnID: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				app, err := NewClientApp(tt.cdSettings, dexTestServer.URL, &dex.DexTLSConfig{StrictValidation: false}, "https://argocd.example.com", cache.NewInMemoryCache(24*time.Hour))
				require.NoError(t, err)

				req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/auth/login", http.NoBody)
				w := httptest.NewRecorder()
				app.HandleLogin(w, req)

				assert.Equal(t, http.StatusSeeOther, w.Code)
				location, err := url.Parse(w.Header().Get("Location"))
				require.NoError(t, err)
				values, err := url.ParseQuery(location.RawQuery)
				require.NoError(t, err)
				assert.Equal(t, tt.expectConnID, values.Has("connector_id"))
				if tt.expectConnID {
					assert.Equal(t, tt.cdSettings.DexAuthConnectorID, values.Get("connector_id"))
				}
			})
		}
	})
}

func Test_Login_Flow(t *testing.T) {
	// Show that SSO login works when no redirect URL is provided, and we fall back to the configured base href for the
	// Argo CD instance.

	oidcTestServer := test.GetOIDCTestServer(t, nil)
	t.Cleanup(oidcTestServer.Close)

	cdSettings := &settings.ArgoCDSettings{
		URL: "https://argocd.example.com",
		OIDCConfigRAW: fmt.Sprintf(`
name: Test
issuer: %s
clientID: test-client-id
clientSecret: test-client-secret
requestedScopes: ["oidc"]`, oidcTestServer.URL),
		OIDCTLSInsecureSkipVerify: true,
	}
	// The base href (the last argument for NewClientApp) is what HandleLogin will fall back to when no explicit
	// redirect URL is given.
	app, err := NewClientApp(cdSettings, "", nil, "/", cache.NewInMemoryCache(24*time.Hour))
	require.NoError(t, err)

	w := httptest.NewRecorder()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/auth/login", http.NoBody)

	app.HandleLogin(w, req)

	redirectURL, err := w.Result().Location()
	require.NoError(t, err)

	state := redirectURL.Query().Get("state")

	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("https://argocd.example.com/auth/callback?state=%s&code=abc", state), http.NoBody)
	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}

	w = httptest.NewRecorder()

	app.HandleCallback(w, req)

	assert.Equal(t, 303, w.Code)
	assert.NotContains(t, w.Body.String(), ErrInvalidRedirectURL.Error())
}

func Test_Login_Flow_With_PKCE(t *testing.T) {
	var codeChallenge string

	oidcTestServer := test.GetOIDCTestServer(t, func(r *http.Request) {
		codeVerifier := r.FormValue("code_verifier")
		assert.NotEmpty(t, codeVerifier)
		assert.Equal(t, oauth2.S256ChallengeFromVerifier(codeVerifier), codeChallenge)
	})
	t.Cleanup(oidcTestServer.Close)

	cdSettings := &settings.ArgoCDSettings{
		URL: "https://example.com/argocd",
		OIDCConfigRAW: fmt.Sprintf(`
name: Test
issuer: %s
clientID: test-client-id
clientSecret: test-client-secret
requestedScopes: ["oidc"]
enablePKCEAuthentication: true`, oidcTestServer.URL),
		OIDCTLSInsecureSkipVerify: true,
	}
	app, err := NewClientApp(cdSettings, "", nil, "/", cache.NewInMemoryCache(24*time.Hour))
	require.NoError(t, err)

	w := httptest.NewRecorder()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/argocd/auth/login", http.NoBody)

	app.HandleLogin(w, req)

	redirectURL, err := w.Result().Location()
	require.NoError(t, err)

	codeChallenge = redirectURL.Query().Get("code_challenge")

	assert.NotEmpty(t, codeChallenge)
	assert.Equal(t, "S256", redirectURL.Query().Get("code_challenge_method"))

	state := redirectURL.Query().Get("state")

	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("https://example.com/argocd/auth/callback?state=%s&code=abc", state), http.NoBody)
	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}

	w = httptest.NewRecorder()

	app.HandleCallback(w, req)

	assert.Equal(t, 303, w.Code)
	assert.NotContains(t, w.Body.String(), ErrInvalidRedirectURL.Error())
}

func TestClientApp_HandleCallback(t *testing.T) {
	oidcTestServer := test.GetOIDCTestServer(t, nil)
	t.Cleanup(oidcTestServer.Close)

	dexTestServer := test.GetDexTestServer(t)
	t.Cleanup(dexTestServer.Close)

	t.Run("oidc certificate checking during oidc callback should toggle on config", func(t *testing.T) {
		cdSettings := &settings.ArgoCDSettings{
			URL: "https://argocd.example.com",
			OIDCConfigRAW: fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]`, oidcTestServer.URL),
		}
		app, err := NewClientApp(cdSettings, dexTestServer.URL, nil, "https://argocd.example.com", cache.NewInMemoryCache(24*time.Hour))
		require.NoError(t, err)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/auth/callback", http.NoBody)

		w := httptest.NewRecorder()

		app.HandleCallback(w, req)

		if !strings.Contains(w.Body.String(), "certificate signed by unknown authority") && !strings.Contains(w.Body.String(), "certificate is not trusted") {
			t.Fatalf("did not receive expected certificate verification failure error: %v", w.Code)
		}

		cdSettings.OIDCTLSInsecureSkipVerify = true

		app, err = NewClientApp(cdSettings, dexTestServer.URL, nil, "https://argocd.example.com", cache.NewInMemoryCache(24*time.Hour))
		require.NoError(t, err)

		w = httptest.NewRecorder()

		app.HandleCallback(w, req)

		assert.NotContains(t, w.Body.String(), "certificate is not trusted")
		assert.NotContains(t, w.Body.String(), "certificate signed by unknown authority")
	})

	t.Run("dex certificate checking during oidc callback should toggle on config", func(t *testing.T) {
		cdSettings := &settings.ArgoCDSettings{
			URL: "https://argocd.example.com",
			DexConfig: `connectors:
- type: github
  name: GitHub
  config:
    clientID: aabbccddeeff00112233
    clientSecret: aabbccddeeff00112233`,
		}
		cert, err := tls.X509KeyPair(test.Cert, test.PrivateKey)
		require.NoError(t, err)
		cdSettings.Certificate = &cert
		app, err := NewClientApp(cdSettings, dexTestServer.URL, nil, "https://argocd.example.com", cache.NewInMemoryCache(24*time.Hour))
		require.NoError(t, err)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/auth/callback", http.NoBody)

		w := httptest.NewRecorder()

		app.HandleCallback(w, req)

		if !strings.Contains(w.Body.String(), "certificate signed by unknown authority") && !strings.Contains(w.Body.String(), "certificate is not trusted") {
			t.Fatal("did not receive expected certificate verification failure error")
		}

		app, err = NewClientApp(cdSettings, dexTestServer.URL, &dex.DexTLSConfig{StrictValidation: false}, "https://argocd.example.com", cache.NewInMemoryCache(24*time.Hour))
		require.NoError(t, err)

		w = httptest.NewRecorder()

		app.HandleCallback(w, req)

		assert.NotContains(t, w.Body.String(), "certificate is not trusted")
		assert.NotContains(t, w.Body.String(), "certificate signed by unknown authority")
	})
}

func Test_azureApp_getFederatedServiceAccountToken(t *testing.T) {
	app := azureApp{mtx: &sync.RWMutex{}}

	setupAzureIdentity(t)

	t.Run("before the method call assertion should be empty.", func(t *testing.T) {
		assert.Empty(t, app.assertion)
	})

	t.Run("Fetch the token value from the file", func(t *testing.T) {
		_, err := app.getFederatedServiceAccountToken(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "serviceAccountToken", app.assertion)
	})

	t.Run("Workload Identity Not enabled.", func(t *testing.T) {
		t.Setenv("AZURE_FEDERATED_TOKEN_FILE", "")
		_, err := app.getFederatedServiceAccountToken(t.Context())
		assert.ErrorContains(t, err, "AZURE_FEDERATED_TOKEN_FILE env variable not found, make sure workload identity is enabled on the cluster")
	})

	t.Run("Workload Identity invalid file", func(t *testing.T) {
		t.Setenv("AZURE_FEDERATED_TOKEN_FILE", filepath.Join(t.TempDir(), "invalid.txt"))
		_, err := app.getFederatedServiceAccountToken(t.Context())
		assert.ErrorContains(t, err, "AZURE_FEDERATED_TOKEN_FILE specified file does not exist")
	})

	t.Run("Concurrent access to the function", func(t *testing.T) {
		currentExpiryTime := app.expires

		var wg sync.WaitGroup
		numGoroutines := 10
		wg.Add(numGoroutines)
		for range numGoroutines {
			go func() {
				defer wg.Done()
				_, err := app.getFederatedServiceAccountToken(t.Context())
				require.NoError(t, err)
				assert.Equal(t, "serviceAccountToken", app.assertion)
			}()
		}
		wg.Wait()

		// Event with multiple concurrent calls the expiry time should not change untile it passes.
		assert.Equal(t, currentExpiryTime, app.expires)
	})

	t.Run("Concurrent access to the function when the current token expires", func(t *testing.T) {
		var wg sync.WaitGroup
		currentExpiryTime := app.expires
		app.expires = time.Now()
		numGoroutines := 10
		wg.Add(numGoroutines)
		for range numGoroutines {
			go func() {
				defer wg.Done()
				_, err := app.getFederatedServiceAccountToken(t.Context())
				require.NoError(t, err)
				assert.Equal(t, "serviceAccountToken", app.assertion)
			}()
		}
		wg.Wait()

		assert.NotEqual(t, currentExpiryTime, app.expires)
	})
}

func TestClientAppWithAzureWorkloadIdentity_HandleCallback(t *testing.T) {
	tokenRequestAssertions := func(r *http.Request) {
		err := r.ParseForm()
		require.NoError(t, err)

		formData := r.Form
		clientAssertion := formData.Get("client_assertion")
		clientAssertionType := formData.Get("client_assertion_type")
		assert.Equal(t, "serviceAccountToken", clientAssertion)
		assert.Equal(t, "urn:ietf:params:oauth:client-assertion-type:jwt-bearer", clientAssertionType)
	}

	oidcTestServer := test.GetAzureOIDCTestServer(t, tokenRequestAssertions)
	t.Cleanup(oidcTestServer.Close)

	dexTestServer := test.GetDexTestServer(t)
	t.Cleanup(dexTestServer.Close)
	signature, err := util.MakeSignature(32)
	require.NoError(t, err)

	setupAzureIdentity(t)

	t.Run("oidc certificate checking during oidc callback should toggle on config", func(t *testing.T) {
		cdSettings := &settings.ArgoCDSettings{
			URL:             "https://argocd.example.com",
			ServerSignature: signature,
			OIDCConfigRAW: fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
azure:
  useWorkloadIdentity: true
skipAudienceCheckWhenTokenHasNoAudience: true
requestedScopes: ["oidc"]`, oidcTestServer.URL),
		}

		app, err := NewClientApp(cdSettings, dexTestServer.URL, nil, "https://argocd.example.com", cache.NewInMemoryCache(24*time.Hour))
		require.NoError(t, err)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/auth/callback", http.NoBody)
		req.Form = url.Values{
			"code":  {"abc"},
			"state": {"123"},
		}
		w := httptest.NewRecorder()

		app.HandleCallback(w, req)

		if !strings.Contains(w.Body.String(), "certificate signed by unknown authority") && !strings.Contains(w.Body.String(), "certificate is not trusted") {
			t.Fatalf("did not receive expected certificate verification failure error: %v", w.Code)
		}

		cdSettings.OIDCTLSInsecureSkipVerify = true

		app, err = NewClientApp(cdSettings, dexTestServer.URL, nil, "https://argocd.example.com", cache.NewInMemoryCache(24*time.Hour))
		require.NoError(t, err)

		w = httptest.NewRecorder()

		key, err := cdSettings.GetServerEncryptionKey()
		require.NoError(t, err)
		encrypted, _ := crypto.Encrypt([]byte("123"), key)
		req.AddCookie(&http.Cookie{Name: common.StateCookieName, Value: hex.EncodeToString(encrypted)})

		app.HandleCallback(w, req)

		assert.NotContains(t, w.Body.String(), "certificate is not trusted")
		assert.NotContains(t, w.Body.String(), "certificate signed by unknown authority")
	})
}

func TestIsValidRedirect(t *testing.T) {
	tests := []struct {
		name        string
		valid       bool
		redirectURL string
		allowedURLs []string
	}{
		{
			name:        "Single allowed valid URL",
			valid:       true,
			redirectURL: "https://localhost:4000",
			allowedURLs: []string{"https://localhost:4000/"},
		},
		{
			name:        "Empty URL",
			valid:       true,
			redirectURL: "",
			allowedURLs: []string{"https://localhost:4000/"},
		},
		{
			name:        "Trailing single slash and empty suffix are handled the same",
			valid:       true,
			redirectURL: "https://localhost:4000/",
			allowedURLs: []string{"https://localhost:4000"},
		},
		{
			name:        "Multiple valid URLs with one allowed",
			valid:       true,
			redirectURL: "https://localhost:4000",
			allowedURLs: []string{"https://wherever:4000", "https://localhost:4000"},
		},
		{
			name:        "Multiple valid URLs with none allowed",
			valid:       false,
			redirectURL: "https://localhost:4000",
			allowedURLs: []string{"https://wherever:4000", "https://invalid:4000"},
		},
		{
			name:        "Invalid redirect URL because path prefix does not match",
			valid:       false,
			redirectURL: "https://localhost:4000/applications",
			allowedURLs: []string{"https://localhost:4000/argocd"},
		},
		{
			name:        "Valid redirect URL because prefix matches",
			valid:       true,
			redirectURL: "https://localhost:4000/argocd/applications",
			allowedURLs: []string{"https://localhost:4000/argocd"},
		},
		{
			name:        "Invalid redirect URL because resolved path does not match prefix",
			valid:       false,
			redirectURL: "https://localhost:4000/argocd/../applications",
			allowedURLs: []string{"https://localhost:4000/argocd"},
		},
		{
			name:        "Invalid redirect URL because scheme mismatch",
			valid:       false,
			redirectURL: "http://localhost:4000",
			allowedURLs: []string{"https://localhost:4000"},
		},
		{
			name:        "Invalid redirect URL because port mismatch",
			valid:       false,
			redirectURL: "https://localhost",
			allowedURLs: []string{"https://localhost:80"},
		},
		{
			name:        "Invalid redirect URL because of CRLF in path",
			valid:       false,
			redirectURL: "https://localhost:80/argocd\r\n",
			allowedURLs: []string{"https://localhost:80/argocd\r\n"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := isValidRedirectURL(tt.redirectURL, tt.allowedURLs)
			assert.Equal(t, res, tt.valid)
		})
	}
}

func TestGenerateAppState(t *testing.T) {
	signature, err := util.MakeSignature(32)
	require.NoError(t, err)
	expectedReturnURL := "http://argocd.example.com/"
	app, err := NewClientApp(&settings.ArgoCDSettings{ServerSignature: signature, URL: expectedReturnURL}, "", nil, "", cache.NewInMemoryCache(24*time.Hour))
	require.NoError(t, err)
	generateResponse := httptest.NewRecorder()
	expectedPKCEVerifier := oauth2.GenerateVerifier()
	state, err := app.generateAppState(expectedReturnURL, expectedPKCEVerifier, generateResponse)
	require.NoError(t, err)

	t.Run("VerifyAppState_Successful", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
		for _, cookie := range generateResponse.Result().Cookies() {
			req.AddCookie(cookie)
		}

		returnURL, pkceVerifier, err := app.verifyAppState(req, httptest.NewRecorder(), state)
		require.NoError(t, err)
		assert.Equal(t, expectedReturnURL, returnURL)
		assert.Equal(t, expectedPKCEVerifier, pkceVerifier)
	})

	t.Run("VerifyAppState_Failed", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
		for _, cookie := range generateResponse.Result().Cookies() {
			req.AddCookie(cookie)
		}

		_, _, err := app.verifyAppState(req, httptest.NewRecorder(), "wrong state")
		require.Error(t, err)
	})
}

func TestGenerateAppState_XSS(t *testing.T) {
	signature, err := util.MakeSignature(32)
	require.NoError(t, err)
	app, err := NewClientApp(
		&settings.ArgoCDSettings{
			// Only return URLs starting with this base should be allowed.
			URL:             "https://argocd.example.com",
			ServerSignature: signature,
		},
		"", nil, "", cache.NewInMemoryCache(24*time.Hour),
	)
	require.NoError(t, err)

	t.Run("XSS fails", func(t *testing.T) {
		// This attack assumes the attacker has compromised the server's secret key. We use `generateAppState` here for
		// convenience, but an attacker with access to the server secret could write their own code to generate the
		// malicious cookie.

		expectedReturnURL := "javascript: alert('hi')"
		generateResponse := httptest.NewRecorder()
		state, err := app.generateAppState(expectedReturnURL, "", generateResponse)
		require.NoError(t, err)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
		for _, cookie := range generateResponse.Result().Cookies() {
			req.AddCookie(cookie)
		}

		returnURL, _, err := app.verifyAppState(req, httptest.NewRecorder(), state)
		require.ErrorIs(t, err, ErrInvalidRedirectURL)
		assert.Empty(t, returnURL)
	})

	t.Run("valid return URL succeeds", func(t *testing.T) {
		expectedReturnURL := "https://argocd.example.com/some/path"
		generateResponse := httptest.NewRecorder()
		state, err := app.generateAppState(expectedReturnURL, "", generateResponse)
		require.NoError(t, err)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
		for _, cookie := range generateResponse.Result().Cookies() {
			req.AddCookie(cookie)
		}

		returnURL, _, err := app.verifyAppState(req, httptest.NewRecorder(), state)
		require.NoError(t, err)
		assert.Equal(t, expectedReturnURL, returnURL)
	})
}

func TestGenerateAppState_NoReturnURL(t *testing.T) {
	signature, err := util.MakeSignature(32)
	require.NoError(t, err)
	cdSettings := &settings.ArgoCDSettings{ServerSignature: signature}
	key, err := cdSettings.GetServerEncryptionKey()
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	encrypted, err := crypto.Encrypt([]byte("123"), key)
	require.NoError(t, err)
	app, err := NewClientApp(cdSettings, "", nil, "/argo-cd", cache.NewInMemoryCache(24*time.Hour))
	require.NoError(t, err)

	req.AddCookie(&http.Cookie{Name: common.StateCookieName, Value: hex.EncodeToString(encrypted)})
	returnURL, _, err := app.verifyAppState(req, httptest.NewRecorder(), "123")
	require.NoError(t, err)
	assert.Equal(t, "/argo-cd", returnURL)
}

func TestGetUserInfoCacheExpiration(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		configuredTTL string
		userInfoTTL   time.Duration
		expectedTTL   time.Duration
	}{
		{name: "missing expiry with shorter limit", configuredTTL: "5m", expectedTTL: 5 * time.Minute},
		{name: "missing expiry without limit", expectedTTL: time.Hour},
		{name: "missing expiry with longer limit", configuredTTL: "2h", expectedTTL: time.Hour},
		{name: "response expiry with shorter limit", configuredTTL: "5m", userInfoTTL: 30 * time.Minute, expectedTTL: 5 * time.Minute},
		{name: "response expiry with longer limit", configuredTTL: "2h", userInfoTTL: 30 * time.Minute, expectedTTL: 30 * time.Minute},
		{name: "response expiry without limit", userInfoTTL: 30 * time.Minute, expectedTTL: 30 * time.Minute},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			userInfo := jwt.MapClaims{"sub": "randomUser", "groups": []string{"developers"}}
			if testCase.userInfoTTL > 0 {
				userInfo["exp"] = time.Now().Add(testCase.userInfoTTL).Unix()
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				assert.NoError(t, json.NewEncoder(w).Encode(userInfo))
			}))
			defer server.Close()

			signature, err := util.MakeSignature(32)
			require.NoError(t, err)
			cdSettings := &settings.ArgoCDSettings{
				ServerSignature: signature,
				OIDCConfigRAW:   fmt.Sprintf("userInfoCacheExpiration: %q", testCase.configuredTTL),
			}
			baseCache := cache.NewInMemoryCache(24 * time.Hour)
			app, err := NewClientApp(cdSettings, "", nil, "/argo-cd", baseCache)
			require.NoError(t, err)
			require.NoError(t, app.SetValueInEncryptedCache(t.Context(), FormatAccessTokenCacheKey("randomUser"), []byte("access-token"), time.Hour))

			clientCache := &cachemocks.MockCacheClient{BaseCache: baseCache}
			clientCache.On("Get", FormatUserInfoResponseCacheKey("randomUser"), mock.Anything).Return(cache.ErrCacheMiss).Once()
			clientCache.On("Get", FormatAccessTokenCacheKey("randomUser"), mock.Anything).Return(nil).Once()
			clientCache.On("Set", mock.Anything).Run(func(args mock.Arguments) {
				item := args.Get(0).(*cache.Item)
				assert.Equal(t, FormatUserInfoResponseCacheKey("randomUser"), item.Key)
				assert.LessOrEqual(t, item.CacheActionOpts.Expiration, testCase.expectedTTL)
				assert.Greater(t, item.CacheActionOpts.Expiration, testCase.expectedTTL-5*time.Second)
			}).Return(nil).Once()
			app.clientCache = clientCache

			claims := jwt.MapClaims{"sub": "randomUser", "exp": float64(time.Now().Add(time.Hour).Unix())}
			_, unauthorized, err := app.GetUserInfo(t.Context(), claims, server.URL, "", "/userinfo")
			require.NoError(t, err)
			assert.False(t, unauthorized)
			clientCache.AssertExpectations(t)
		})
	}
}

func TestGetUserInfo(t *testing.T) {
	tests := []struct {
		name                  string
		userInfoPath          string
		expectedOutput        any
		expectError           bool
		expectUnauthenticated bool
		expectedCacheItems    []struct { // items to check in cache after function call
			key             string
			value           string
			expectEncrypted bool
			expectError     bool
		}
		idpHandler         func(w http.ResponseWriter, r *http.Request)
		idpHandlerUserInfo func(w http.ResponseWriter, r *http.Request) // same as idpHandler but listening on userInfoBaseURL instead of issuerURL
		idpClaims          jwt.MapClaims                                // as per specification sub and exp are REQUIRED fields
		cache              cache.CacheClient
		cacheItems         []struct { // items to put in cache before execution
			key     string
			value   string
			encrypt bool
		}
	}{
		{
			name:                  "call UserInfo with wrong userInfoPath",
			userInfoPath:          "/user",
			expectedOutput:        jwt.MapClaims(nil),
			expectError:           true,
			expectUnauthenticated: false,
			expectedCacheItems: []struct {
				key             string
				value           string
				expectEncrypted bool
				expectError     bool
			}{
				{
					key:         FormatUserInfoResponseCacheKey("randomUser"),
					expectError: true,
				},
			},
			idpClaims: jwt.MapClaims{"sub": "randomUser", "exp": float64(time.Now().Add(5 * time.Minute).Unix())},
			idpHandler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			cache: cache.NewInMemoryCache(24 * time.Hour),
			cacheItems: []struct {
				key     string
				value   string
				encrypt bool
			}{
				{
					key:     FormatAccessTokenCacheKey("randomUser"),
					value:   "FakeAccessToken",
					encrypt: true,
				},
			},
		},
		{
			name:                  "call UserInfo with bad accessToken",
			userInfoPath:          "/user-info",
			expectedOutput:        jwt.MapClaims(nil),
			expectError:           false,
			expectUnauthenticated: true,
			expectedCacheItems: []struct {
				key             string
				value           string
				expectEncrypted bool
				expectError     bool
			}{
				{
					key:         FormatUserInfoResponseCacheKey("randomUser"),
					expectError: true,
				},
			},
			idpClaims: jwt.MapClaims{"sub": "randomUser", "exp": float64(time.Now().Add(5 * time.Minute).Unix())},
			idpHandler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			cache: cache.NewInMemoryCache(24 * time.Hour),
			cacheItems: []struct {
				key     string
				value   string
				encrypt bool
			}{
				{
					key:     FormatAccessTokenCacheKey("randomUser"),
					value:   "FakeAccessToken",
					encrypt: true,
				},
			},
		},
		{
			name:                  "call UserInfo with garbage returned",
			userInfoPath:          "/user-info",
			expectedOutput:        jwt.MapClaims(nil),
			expectError:           true,
			expectUnauthenticated: false,
			expectedCacheItems: []struct {
				key             string
				value           string
				expectEncrypted bool
				expectError     bool
			}{
				{
					key:         FormatUserInfoResponseCacheKey("randomUser"),
					expectError: true,
				},
			},
			idpClaims: jwt.MapClaims{"sub": "randomUser", "exp": float64(time.Now().Add(5 * time.Minute).Unix())},
			idpHandler: func(w http.ResponseWriter, _ *http.Request) {
				userInfoBytes := `
			  notevenJsongarbage
				`
				_, err := w.Write([]byte(userInfoBytes))
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusTeapot)
			},
			cache: cache.NewInMemoryCache(24 * time.Hour),
			cacheItems: []struct {
				key     string
				value   string
				encrypt bool
			}{
				{
					key:     FormatAccessTokenCacheKey("randomUser"),
					value:   "FakeAccessToken",
					encrypt: true,
				},
			},
		},
		{
			name:                  "call UserInfo without accessToken in cache",
			userInfoPath:          "/user-info",
			expectedOutput:        jwt.MapClaims(nil),
			expectError:           true,
			expectUnauthenticated: true,
			expectedCacheItems: []struct {
				key             string
				value           string
				expectEncrypted bool
				expectError     bool
			}{
				{
					key:         FormatUserInfoResponseCacheKey("randomUser"),
					expectError: true,
				},
			},
			idpClaims: jwt.MapClaims{"sub": "randomUser", "exp": float64(time.Now().Add(5 * time.Minute).Unix())},
			idpHandler: func(w http.ResponseWriter, _ *http.Request) {
				userInfoBytes := `
				{
					"groups":["githubOrg:engineers"]
				}`
				w.Header().Set("content-type", "application/json")
				_, err := w.Write([]byte(userInfoBytes))
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			},
			cache: cache.NewInMemoryCache(24 * time.Hour),
		},
		{
			name:                  "call UserInfo with valid accessToken in cache",
			userInfoPath:          "/user-info",
			expectedOutput:        jwt.MapClaims{"groups": []any{"githubOrg:engineers"}},
			expectError:           false,
			expectUnauthenticated: false,
			expectedCacheItems: []struct {
				key             string
				value           string
				expectEncrypted bool
				expectError     bool
			}{
				{
					key:             FormatUserInfoResponseCacheKey("randomUser"),
					value:           "{\"groups\":[\"githubOrg:engineers\"]}",
					expectEncrypted: true,
					expectError:     false,
				},
			},
			idpClaims: jwt.MapClaims{"sub": "randomUser", "exp": float64(time.Now().Add(5 * time.Minute).Unix())},
			idpHandler: func(w http.ResponseWriter, _ *http.Request) {
				userInfoBytes := `
				{
					"groups":["githubOrg:engineers"]
				}`
				w.Header().Set("content-type", "application/json")
				_, err := w.Write([]byte(userInfoBytes))
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			},
			cache: cache.NewInMemoryCache(24 * time.Hour),
			cacheItems: []struct {
				key     string
				value   string
				encrypt bool
			}{
				{
					key:     FormatAccessTokenCacheKey("randomUser"),
					value:   "FakeAccessToken",
					encrypt: true,
				},
			},
		},
		{
			name:                  "call UserInfo on separate endpoint",
			userInfoPath:          "/user-info",
			expectedOutput:        jwt.MapClaims{"groups": []any{"githubOrg:developers"}}, // response from separate idpHandlerUserInfo expected
			expectError:           false,
			expectUnauthenticated: false,
			expectedCacheItems: []struct {
				key             string
				value           string
				expectEncrypted bool
				expectError     bool
			}{
				{
					key:             FormatUserInfoResponseCacheKey("randomUser"),
					value:           "{\"groups\":[\"githubOrg:developers\"]}",
					expectEncrypted: true,
					expectError:     false,
				},
			},
			idpClaims: jwt.MapClaims{"sub": "randomUser", "exp": float64(time.Now().Add(5 * time.Minute).Unix())},
			idpHandler: func(w http.ResponseWriter, _ *http.Request) {
				userInfoBytes := `
				{
					"groups":["githubOrg:engineers"]
				}`
				w.Header().Set("content-type", "application/json")
				_, err := w.Write([]byte(userInfoBytes))
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			},
			idpHandlerUserInfo: func(w http.ResponseWriter, _ *http.Request) {
				userInfoBytes := `
				{
					"groups":["githubOrg:developers"]
				}`
				w.Header().Set("content-type", "application/json")
				_, err := w.Write([]byte(userInfoBytes))
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			},
			cache: cache.NewInMemoryCache(24 * time.Hour),
			cacheItems: []struct {
				key     string
				value   string
				encrypt bool
			}{
				{
					key:     FormatAccessTokenCacheKey("randomUser"),
					value:   "FakeAccessToken",
					encrypt: true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tsUserInfo *httptest.Server
			if tt.idpHandlerUserInfo != nil {
				tsUserInfo = httptest.NewServer(http.HandlerFunc(tt.idpHandlerUserInfo))
			} else {
				tsUserInfo = httptest.NewServer(http.HandlerFunc(tt.idpHandler))
			}

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

			got, unauthenticated, err := a.GetUserInfo(t.Context(), tt.idpClaims, ts.URL, tsUserInfo.URL, tt.userInfoPath)
			assert.Equal(t, tt.expectedOutput, got)
			assert.Equal(t, tt.expectUnauthenticated, unauthenticated)
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

func TestSetGroupsClaimFromEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		inputClaims    jwt.MapClaims // function input
		cacheClaims    jwt.MapClaims // userinfo response
		expectedClaims jwt.MapClaims // function output
		expectError    bool
	}{
		{
			name:           "set correct groups from userinfo endpoint", // enriches the JWT claims with information from the userinfo endpoint, default case
			inputClaims:    jwt.MapClaims{"sub": "randomUser", "exp": float64(time.Now().Add(5 * time.Minute).Unix())},
			cacheClaims:    jwt.MapClaims{"sub": "randomUser", "groups": []string{"githubOrg:example"}, "exp": float64(time.Now().Add(5 * time.Minute).Unix())},
			expectedClaims: jwt.MapClaims{"sub": "randomUser", "groups": []any{"githubOrg:example"}, "exp": float64(time.Now().Add(5 * time.Minute).Unix())}, // the groups must be of type any since the response we get was parsed by GetUserInfo and we don't yet know the type of the groups claim
			expectError:    false,
		},
		{
			name:           "return error for wrong userinfo claims returned", // when there's an error in this feature, the claims should be untouched for the rest to still proceed
			inputClaims:    jwt.MapClaims{"sub": "randomUser", "exp": float64(time.Now().Add(5 * time.Minute).Unix())},
			cacheClaims:    jwt.MapClaims{"sub": "wrongUser", "exp": float64(time.Now().Add(5 * time.Minute).Unix())},
			expectedClaims: jwt.MapClaims{"sub": "randomUser", "exp": float64(time.Now().Add(5 * time.Minute).Unix())},
			expectError:    true,
		},
		{
			name:           "override groups already defined in input claims", // this is expected behavior since input claims might have been truncated (HTTP header 4K limit)
			inputClaims:    jwt.MapClaims{"sub": "randomUser", "groups": []string{"groupfromjwt"}, "exp": float64(time.Now().Add(5 * time.Minute).Unix())},
			cacheClaims:    jwt.MapClaims{"sub": "randomUser", "groups": []string{"superusers", "usergroup", "support-group"}, "exp": float64(time.Now().Add(5 * time.Minute).Unix())},
			expectedClaims: jwt.MapClaims{"sub": "randomUser", "groups": []any{"superusers", "usergroup", "support-group"}, "exp": float64(time.Now().Add(5 * time.Minute).Unix())},
			expectError:    false,
		},
		{
			name:           "empty cache and non-rechable userinfo endpoint", // this will try to reach the userinfo endpoint defined in the test and fail
			inputClaims:    jwt.MapClaims{"sub": "randomUser", "groups": []string{"groupfromjwt"}, "exp": float64(time.Now().Add(5 * time.Minute).Unix())},
			cacheClaims:    nil, // the test doesn't set the cache for an empty object
			expectedClaims: jwt.MapClaims{"sub": "randomUser", "groups": []string{"groupfromjwt"}, "exp": float64(time.Now().Add(5 * time.Minute).Unix())},
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// create the ClientApp
			userInfoCache := cache.NewInMemoryCache(24 * time.Hour)
			signature, err := util.MakeSignature(32)
			require.NoError(t, err, "failed creating signature for settings object")
			cdSettings := &settings.ArgoCDSettings{
				ServerSignature: signature,
				OIDCConfigRAW: `
issuer: http://localhost:63231
enableUserInfoGroups: true
userInfoPath: /`,
			}
			a, err := NewClientApp(cdSettings, "", nil, "/argo-cd", userInfoCache)
			require.NoError(t, err, "failed creating clientapp")

			// prepoluate cache to predict what the GetUserInfo function will return to the SetGroupsClaimFromEndpoint function (without having to mock the userinfo response)
			encryptionKey, err := cdSettings.GetServerEncryptionKey()
			require.NoError(t, err, "failed obtaining encryption key from settings")

			// set fake accessToken for function to not return early
			encAccessToken, err := crypto.Encrypt([]byte("123456"), encryptionKey)
			require.NoError(t, err, "failed encrypting dummy access token")
			err = a.clientCache.Set(&cache.Item{
				Key:    FormatAccessTokenCacheKey("randomUser"),
				Object: encAccessToken,
			})
			require.NoError(t, err, "failed setting item to in-memory cache")

			// set cacheClaims to in-memory cache to let GetUserInfo return early with this information (GetUserInfo has a separate test, here we focus on SetUserInfoGroups)
			if tt.cacheClaims != nil {
				cacheClaims, err := json.Marshal(tt.cacheClaims)
				require.NoError(t, err)
				encCacheClaims, err := crypto.Encrypt(cacheClaims, encryptionKey)
				require.NoError(t, err, "failed encrypting dummy access token")
				err = a.clientCache.Set(&cache.Item{
					Key:    FormatUserInfoResponseCacheKey("randomUser"),
					Object: encCacheClaims,
				})
				require.NoError(t, err, "failed setting item to in-memory cache")
			}

			receivedClaims, err := a.SetGroupsClaimFromEndpoint(t.Context(), tt.inputClaims, "argocd")
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expectedClaims, receivedClaims) // check that the claims were successfully enriched with what we expect
		})
	}
}

func TestGetOidcTokenCacheFromJSON(t *testing.T) {
	tests := []struct {
		name                string
		oidcTokenCache      *OidcTokenCache
		expectErrorContains string
		expectIdToken       string
	}{
		{
			name:                "empty",
			oidcTokenCache:      &OidcTokenCache{},
			expectErrorContains: "empty token",
		},
		{
			name: "empty id token",
			oidcTokenCache: &OidcTokenCache{
				Token: &oauth2.Token{},
			},
			expectIdToken: "",
		},
		{
			name:           "simple",
			oidcTokenCache: NewOidcTokenCache("", (&oauth2.Token{}).WithExtra(map[string]any{"id_token": "simple"}), time.Time{}),
			expectIdToken:  "simple",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenJSON, err := json.Marshal(tt.oidcTokenCache)
			require.NoError(t, err)
			token, err := GetOidcTokenCacheFromJSON(tokenJSON)
			if tt.expectErrorContains != "" {
				assert.ErrorContains(t, err, tt.expectErrorContains)
				return
			}
			require.NoError(t, err)
			if tt.expectIdToken != "" {
				assert.Equal(t, tt.expectIdToken, token.Token.Extra("id_token").(string))
			}
		})
	}
}

func TestClientApp_GetTokenSourceFromCache(t *testing.T) {
	tests := []struct {
		name                string
		oidcTokenCache      *OidcTokenCache
		expectErrorContains string
		provider            Provider
	}{
		{
			name:                "provider error",
			oidcTokenCache:      &OidcTokenCache{},
			expectErrorContains: "fake provider endpoint error",
			provider: &fakeProvider{
				EndpointError: true,
			},
		},
		{
			name:                "empty oidcTokenCache",
			expectErrorContains: "oidcTokenCache is required",
			provider:            &fakeProvider{},
		},
		{
			name:           "simple",
			oidcTokenCache: NewOidcTokenCache("", (&oauth2.Token{}).WithExtra(map[string]any{"id_token": "simple"}), time.Time{}),
			provider:       &fakeProvider{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := ClientApp{provider: tt.provider, settings: &settings.ArgoCDSettings{}}
			tokenSource, err := app.GetTokenSourceFromCache(t.Context(), tt.oidcTokenCache)
			if tt.expectErrorContains != "" {
				assert.ErrorContains(t, err, tt.expectErrorContains)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, tokenSource)
		})
	}
}

func TestClientApp_GetUpdatedOidcTokenFromCache(t *testing.T) {
	tests := []struct {
		name                string
		subject             string
		session             string
		insertIntoCache     bool
		oidcTokenCache      *OidcTokenCache
		expectErrorContains string
		expectTokenNotNil   bool
	}{
		{
			name:                "empty token cache",
			subject:             "alice",
			session:             "111",
			insertIntoCache:     true,
			expectErrorContains: "failed to unmarshal cached oidc token: empty token",
		},
		{
			name:                "no refresh token",
			subject:             "alice",
			session:             "111",
			insertIntoCache:     true,
			oidcTokenCache:      &OidcTokenCache{Token: &oauth2.Token{}},
			expectErrorContains: "failed to refresh token from source: oauth2: token expired and refresh token is not set",
		},
		{
			name:            "cache miss",
			subject:         "",
			session:         "",
			insertIntoCache: false,
		},
		{
			name:            "updated token from cache",
			subject:         "alice",
			session:         "111",
			insertIntoCache: true,
			oidcTokenCache: &OidcTokenCache{Token: &oauth2.Token{
				RefreshToken: "not empty",
			}},
			expectTokenNotNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oidcTestServer := test.GetOIDCTestServer(t, nil)
			t.Cleanup(oidcTestServer.Close)

			cdSettings := &settings.ArgoCDSettings{
				URL: "https://argocd.example.com",
				OIDCConfigRAW: fmt.Sprintf(`
name: Test
issuer: %s
clientID: test-client-id
clientSecret: test-client-secret
requestedScopes: ["oidc"]`, oidcTestServer.URL),
				OIDCTLSInsecureSkipVerify: true,
				UserSessionDuration:       24 * time.Hour,
			}
			app, err := NewClientApp(cdSettings, "", nil, "/", cache.NewInMemoryCache(24*time.Hour))
			require.NoError(t, err)
			if tt.insertIntoCache {
				oidcTokenCacheJSON, err := json.Marshal(tt.oidcTokenCache)
				require.NoError(t, err)
				require.NoError(t, app.SetValueInEncryptedCache(t.Context(), formatOidcTokenCacheKey(tt.subject, tt.session), oidcTokenCacheJSON, time.Minute))
			}
			token, err := app.GetUpdatedOidcTokenFromCache(t.Context(), tt.subject, tt.session)
			if tt.expectErrorContains != "" {
				assert.ErrorContains(t, err, tt.expectErrorContains)
				return
			}
			require.NoError(t, err)
			if tt.expectTokenNotNil {
				assert.NotNil(t, token)
			}
		})
	}
}

func TestSessionRemainingTTL(t *testing.T) {
	const dur = 10 * time.Minute

	t.Run("zero session start returns original full duration", func(t *testing.T) {
		ttl := sessionRemainingTTL(time.Time{}, dur)
		assert.Equal(t, dur, ttl)
	})

	t.Run("active session returns remaining time", func(t *testing.T) {
		sessionStart := time.Now().Add(-4 * time.Minute)
		ttl := sessionRemainingTTL(sessionStart, dur)
		// ~6 min remaining; accept ±5s of test jitter
		assert.InDelta(t, (6 * time.Minute).Seconds(), ttl.Seconds(), 5)
	})

	t.Run("expired session returns zero", func(t *testing.T) {
		sessionStart := time.Now().Add(-11 * time.Minute)
		ttl := sessionRemainingTTL(sessionStart, dur)
		assert.Equal(t, time.Duration(0), ttl)
	})
}

func TestWithIDTokenExpiry(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	idToken := func(t *testing.T, exp time.Time) string {
		t.Helper()
		raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"exp": exp.Unix()}).SignedString([]byte("test"))
		require.NoError(t, err)
		return raw
	}
	tests := []struct {
		name         string
		accessExpiry time.Time
		idToken      func(t *testing.T) any
		expectExpiry time.Time
	}{
		{
			name:         "ID token expires first",
			accessExpiry: now.Add(2 * time.Hour),
			idToken:      func(t *testing.T) any { t.Helper(); return idToken(t, now.Add(2*time.Minute)) },
			expectExpiry: now.Add(2 * time.Minute),
		},
		{
			name:         "access token expires first",
			accessExpiry: now.Add(time.Minute),
			idToken:      func(t *testing.T) any { t.Helper(); return idToken(t, now.Add(time.Hour)) },
			expectExpiry: now.Add(time.Minute),
		},
		{
			name:         "access token without expiry",
			idToken:      func(t *testing.T) any { t.Helper(); return idToken(t, now.Add(2*time.Minute)) },
			expectExpiry: now.Add(2 * time.Minute),
		},
		{
			name:         "no ID token",
			accessExpiry: now.Add(time.Hour),
			idToken:      func(_ *testing.T) any { return nil },
			expectExpiry: now.Add(time.Hour),
		},
		{
			name:         "ID token is not a JWT",
			accessExpiry: now.Add(time.Hour),
			idToken:      func(_ *testing.T) any { return "not-a-jwt" },
			expectExpiry: now.Add(time.Hour),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &oauth2.Token{AccessToken: "access", Expiry: tt.accessExpiry}
			if id := tt.idToken(t); id != nil {
				token = token.WithExtra(map[string]any{"id_token": id})
			}
			got := withIDTokenExpiry(token)
			assert.True(t, tt.expectExpiry.Equal(got.Expiry), "expiry = %v, want %v", got.Expiry, tt.expectExpiry)
			assert.True(t, tt.accessExpiry.Equal(token.Expiry), "the original token must not be modified")
		})
	}
	assert.Nil(t, withIDTokenExpiry(nil))
}

// The ID token can expire before the access token (e.g. GitLab: 2 minutes vs 2 hours).
// A refresh must then happen based on the ID token's exp, not only the access token's expiry.
func TestClientApp_GetUpdatedOidcTokenFromCache_IDTokenExpiry(t *testing.T) {
	idToken := func(t *testing.T, exp time.Time) string {
		t.Helper()
		// Only the exp claim matters here: the cached ID token is read, not verified.
		raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "alice", "exp": exp.Unix()}).SignedString([]byte("test"))
		require.NoError(t, err)
		return raw
	}
	tests := []struct {
		name          string
		cachedIDToken func(t *testing.T) string
		expectRefresh bool
	}{
		{
			name:          "expired ID token is refreshed while the access token is valid",
			cachedIDToken: func(t *testing.T) string { t.Helper(); return idToken(t, time.Now().Add(-time.Minute)) },
			expectRefresh: true,
		},
		{
			name:          "ID token within the refresh threshold is refreshed",
			cachedIDToken: func(t *testing.T) string { t.Helper(); return idToken(t, time.Now().Add(10*time.Second)) },
			expectRefresh: true,
		},
		{
			name:          "valid ID token and access token are reused",
			cachedIDToken: func(t *testing.T) string { t.Helper(); return idToken(t, time.Now().Add(time.Hour)) },
			expectRefresh: false,
		},
		{
			name:          "an ID token that is not a JWT falls back to the access token expiry",
			cachedIDToken: func(_ *testing.T) string { return "not-a-jwt" },
			expectRefresh: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refreshes := 0
			oidcTestServer := test.GetOIDCTestServer(t, func(_ *http.Request) { refreshes++ })
			t.Cleanup(oidcTestServer.Close)

			cdSettings := &settings.ArgoCDSettings{
				URL: "https://argocd.example.com",
				OIDCConfigRAW: fmt.Sprintf(`
name: Test
issuer: %s
clientID: test-client-id
clientSecret: test-client-secret
refreshTokenThreshold: 30s
requestedScopes: ["oidc"]`, oidcTestServer.URL),
				OIDCTLSInsecureSkipVerify: true,
				UserSessionDuration:       24 * time.Hour,
			}
			app, err := NewClientApp(cdSettings, "", nil, "/", cache.NewInMemoryCache(24*time.Hour))
			require.NoError(t, err)

			cachedIDToken := tt.cachedIDToken(t)
			// The access token is valid for another 2 hours.
			cached := (&oauth2.Token{
				AccessToken:  "cached-access-token",
				RefreshToken: "cached-refresh-token",
				Expiry:       time.Now().Add(2 * time.Hour),
			}).WithExtra(map[string]any{"id_token": cachedIDToken})
			oidcTokenCacheJSON, err := json.Marshal(NewOidcTokenCache("", cached, time.Now()))
			require.NoError(t, err)
			require.NoError(t, app.SetValueInEncryptedCache(t.Context(), formatOidcTokenCacheKey("alice", "111"), oidcTokenCacheJSON, time.Hour))

			token, err := app.GetUpdatedOidcTokenFromCache(t.Context(), "alice", "111")
			require.NoError(t, err)
			require.NotNil(t, token)
			if tt.expectRefresh {
				assert.Equal(t, 1, refreshes, "expected the token to be refreshed")
				assert.NotEqual(t, cachedIDToken, token.Extra("id_token"), "expected a new ID token")
			} else {
				assert.Equal(t, 0, refreshes, "expected the cached token to be reused")
				assert.Equal(t, cachedIDToken, token.Extra("id_token"))
			}
		})
	}
}

// Providers such as GitLab rotate refresh tokens: a refresh token works once. When the UI sends
// several requests at the same moment with an expired ID token, only one may refresh.
func TestClientApp_GetUpdatedOidcTokenFromCache_ConcurrentRefresh(t *testing.T) {
	var refreshes atomic.Int32
	oidcTestServer := test.GetOIDCTestServer(t, func(_ *http.Request) {
		refreshes.Add(1)
		time.Sleep(100 * time.Millisecond) // keep the refresh in flight while the other requests arrive
	})
	t.Cleanup(oidcTestServer.Close)

	cdSettings := &settings.ArgoCDSettings{
		URL: "https://argocd.example.com",
		OIDCConfigRAW: fmt.Sprintf(`
name: Test
issuer: %s
clientID: test-client-id
clientSecret: test-client-secret
requestedScopes: ["oidc"]`, oidcTestServer.URL),
		OIDCTLSInsecureSkipVerify: true,
		UserSessionDuration:       24 * time.Hour,
	}
	app, err := NewClientApp(cdSettings, "", nil, "/", cache.NewInMemoryCache(24*time.Hour))
	require.NoError(t, err)

	expiredIDToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "alice", "exp": time.Now().Add(-time.Minute).Unix()}).SignedString([]byte("test"))
	require.NoError(t, err)
	cached := (&oauth2.Token{
		AccessToken:  "cached-access-token",
		RefreshToken: "cached-refresh-token",
		Expiry:       time.Now().Add(2 * time.Hour),
	}).WithExtra(map[string]any{"id_token": expiredIDToken})
	oidcTokenCacheJSON, err := json.Marshal(NewOidcTokenCache("", cached, time.Now()))
	require.NoError(t, err)
	require.NoError(t, app.SetValueInEncryptedCache(t.Context(), formatOidcTokenCacheKey("alice", "111"), oidcTokenCacheJSON, time.Hour))

	const requests = 10
	var wg sync.WaitGroup
	idTokens := make([]any, requests)
	errs := make([]error, requests)
	for i := range requests {
		wg.Go(func() {
			token, err := app.GetUpdatedOidcTokenFromCache(t.Context(), "alice", "111")
			errs[i] = err
			if token != nil {
				idTokens[i] = token.Extra("id_token")
			}
		})
	}
	wg.Wait()
	require.NoError(t, errors.Join(errs...))

	assert.Equal(t, int32(1), refreshes.Load(), "concurrent requests must share one refresh")
	for i, idToken := range idTokens {
		assert.NotEqual(t, expiredIDToken, idToken, "request %d got the expired ID token", i)
		assert.Equal(t, idTokens[0], idToken, "request %d got a different ID token", i)
	}
}

// newRefreshTestApp returns a ClientApp for oidcTestServer with alice's session cached: an access token
// valid for another 2 hours and an ID token that expired a minute ago.
func newRefreshTestApp(t *testing.T, issuerURL string) (*ClientApp, *oauth2.Token) {
	t.Helper()
	cdSettings := &settings.ArgoCDSettings{
		URL: "https://argocd.example.com",
		OIDCConfigRAW: fmt.Sprintf(`
name: Test
issuer: %s
clientID: test-client-id
clientSecret: test-client-secret
refreshTokenThreshold: 1m
requestedScopes: ["oidc"]`, issuerURL),
		OIDCTLSInsecureSkipVerify: true,
		UserSessionDuration:       24 * time.Hour,
	}
	app, err := NewClientApp(cdSettings, "", nil, "/", cache.NewInMemoryCache(24*time.Hour))
	require.NoError(t, err)
	expiredIDToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "alice", "exp": time.Now().Add(-time.Minute).Unix()}).SignedString([]byte("test"))
	require.NoError(t, err)
	cached := (&oauth2.Token{
		AccessToken:  "cached-access-token",
		RefreshToken: "cached-refresh-token",
		Expiry:       time.Now().Add(2 * time.Hour),
	}).WithExtra(map[string]any{"id_token": expiredIDToken})
	storeSession(t, app, cached)
	return app, cached
}

func storeSession(t *testing.T, app *ClientApp, token *oauth2.Token) {
	t.Helper()
	oidcTokenCacheJSON, err := json.Marshal(NewOidcTokenCache("", token, time.Now()))
	require.NoError(t, err)
	require.NoError(t, app.SetValueInEncryptedCache(t.Context(), formatOidcTokenCacheKey("alice", "111"), oidcTokenCacheJSON, time.Hour))
}

// The entry point the API server calls: the request's ID token has expired while the cached access
// token is still valid, so the returned ID token must be a new one.
func TestClientApp_CheckAndRefreshToken_IDTokenExpiredBeforeAccessToken(t *testing.T) {
	var refreshes atomic.Int32
	oidcTestServer := test.GetOIDCTestServer(t, func(_ *http.Request) { refreshes.Add(1) })
	t.Cleanup(oidcTestServer.Close)
	app, cached := newRefreshTestApp(t, oidcTestServer.URL)

	claims := jwt.MapClaims{"sub": "alice", "sid": "111", "iss": oidcTestServer.URL, "exp": float64(time.Now().Add(-time.Minute).Unix())}
	idToken, err := app.CheckAndRefreshToken(t.Context(), claims, time.Minute)
	require.NoError(t, err)
	assert.NotEmpty(t, idToken)
	assert.NotEqual(t, cached.Extra("id_token"), idToken, "expected a new ID token, not the expired cached one")
	assert.Equal(t, int32(1), refreshes.Load())
}

// With several argocd-server replicas, another replica can refresh the session first. A provider that
// rotates refresh tokens then rejects this replica's refresh token with invalid_grant; the refresh must
// use the token the other replica stored instead of failing.
func TestClientApp_GetUpdatedOidcTokenFromCache_RefreshedByAnotherReplica(t *testing.T) {
	prevWait := refreshedElsewhereWait
	refreshedElsewhereWait = 50 * time.Millisecond
	t.Cleanup(func() { refreshedElsewhereWait = prevWait })

	tests := []struct {
		name string
		// otherReplica stores the other replica's refreshed token when this replica's refresh request is
		// sent; nil means no other replica refreshed the session.
		otherReplica  func(t *testing.T, app *ClientApp, refreshed *oauth2.Token)
		expectError   string
		expectRefresh bool
	}{
		{
			name: "stored before this replica's refresh fails",
			otherReplica: func(t *testing.T, app *ClientApp, refreshed *oauth2.Token) {
				t.Helper()
				storeSession(t, app, refreshed)
			},
			expectRefresh: true,
		},
		{
			name: "stored shortly after this replica's refresh fails",
			otherReplica: func(t *testing.T, app *ClientApp, refreshed *oauth2.Token) {
				t.Helper()
				go func() {
					time.Sleep(10 * time.Millisecond)
					storeSession(t, app, refreshed)
				}()
			},
			expectRefresh: true,
		},
		{
			name:        "no other replica refreshed",
			expectError: "invalid_grant",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var app *ClientApp
			refreshedIDToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "alice", "exp": time.Now().Add(2 * time.Minute).Unix()}).SignedString([]byte("test"))
			require.NoError(t, err)
			refreshed := (&oauth2.Token{
				AccessToken:  "access-token-from-other-replica",
				RefreshToken: "refresh-token-from-other-replica",
				Expiry:       time.Now().Add(2 * time.Hour),
			}).WithExtra(map[string]any{"id_token": refreshedIDToken})

			oidcTestServer := test.GetOIDCTestServer(t, nil)
			t.Cleanup(oidcTestServer.Close)
			provider := oidcTestServer.Config.Handler
			oidcTestServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.RequestURI != "/token" {
					provider.ServeHTTP(w, r)
					return
				}
				// The other replica already used the refresh token this replica sends.
				if tt.otherReplica != nil {
					tt.otherReplica(t, app, refreshed)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"The provided authorization grant is invalid"}`))
			})
			app, _ = newRefreshTestApp(t, oidcTestServer.URL)

			token, err := app.GetUpdatedOidcTokenFromCache(t.Context(), "alice", "111")
			if tt.expectError != "" {
				assert.ErrorContains(t, err, tt.expectError)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, token)
			assert.Equal(t, refreshedIDToken, token.Extra("id_token"))
		})
	}
}

func TestClientApp_GetUpdatedOidcTokenFromCache_SessionCeiling(t *testing.T) {
	// 10 minute UserSessionDuration, 9 minutes of current session already passed
	const sessionDuration = 10 * time.Minute
	sessionStart := time.Now().Add(-9 * time.Minute)

	oidcTestServer := test.GetOIDCTestServer(t, nil)
	t.Cleanup(oidcTestServer.Close)

	cdSettings := &settings.ArgoCDSettings{
		URL: "https://argocd.example.com",
		OIDCConfigRAW: fmt.Sprintf(`
name: Test
issuer: %s
clientID: test-client-id
clientSecret: test-client-secret
requestedScopes: ["oidc"]`, oidcTestServer.URL),
		OIDCTLSInsecureSkipVerify: true,
		UserSessionDuration:       sessionDuration,
	}
	app, err := NewClientApp(cdSettings, "", nil, "/", cache.NewInMemoryCache(24*time.Hour))
	require.NoError(t, err)

	sub, sid := "alice", "s1"
	oidcTokenCacheJSON, err := json.Marshal(&OidcTokenCache{
		Token:        &oauth2.Token{RefreshToken: "not empty"},
		SessionStart: sessionStart,
	})
	require.NoError(t, err)
	require.NoError(t, app.SetValueInEncryptedCache(t.Context(), formatOidcTokenCacheKey(sub, sid), oidcTokenCacheJSON, sessionDuration))

	_, err = app.GetUpdatedOidcTokenFromCache(t.Context(), sub, sid)
	require.NoError(t, err)

	// Retrieve the updated entry and confirm SessionStart was not reset to now.
	updatedJSON, err := app.GetValueFromEncryptedCache(t.Context(), formatOidcTokenCacheKey(sub, sid))
	require.NoError(t, err)
	require.NotNil(t, updatedJSON, "cache entry should exist after refresh")

	updatedCache, err := GetOidcTokenCacheFromJSON(updatedJSON)
	require.NoError(t, err)

	// TTL should be less than 1 minute after token refresh
	assert.WithinDuration(t, sessionStart, updatedCache.SessionStart, 5*time.Second,
		"SessionStart must be preserved across token refreshes")
}

func TestClientApp_GetUpdatedOidcTokenFromCache_SessionCeilingExceeded(t *testing.T) {
	const sessionDuration = 10 * time.Minute
	sessionStart := time.Now().Add(-11 * time.Minute)

	oidcTestServer := test.GetOIDCTestServer(t, nil)
	t.Cleanup(oidcTestServer.Close)

	cdSettings := &settings.ArgoCDSettings{
		URL: "https://argocd.example.com",
		OIDCConfigRAW: fmt.Sprintf(`
name: Test
issuer: %s
clientID: test-client-id
clientSecret: test-client-secret
requestedScopes: ["oidc"]`, oidcTestServer.URL),
		OIDCTLSInsecureSkipVerify: true,
		UserSessionDuration:       sessionDuration,
	}
	app, err := NewClientApp(cdSettings, "", nil, "/", cache.NewInMemoryCache(24*time.Hour))
	require.NoError(t, err)

	sub, sid := "alice", "s1"
	cacheKey := formatOidcTokenCacheKey(sub, sid)
	originalJSON, err := json.Marshal(&OidcTokenCache{
		Token:        &oauth2.Token{RefreshToken: "not empty"},
		SessionStart: sessionStart,
	})
	require.NoError(t, err)
	require.NoError(t, app.SetValueInEncryptedCache(t.Context(), cacheKey, originalJSON, time.Minute))

	token, err := app.GetUpdatedOidcTokenFromCache(t.Context(), sub, sid)
	require.NoError(t, err)
	assert.Nil(t, token, "expired session must not yield a refreshed token")

	// The refreshed token must not overwrite the cache entry with the default TTL
	cachedValue, err := app.GetValueFromEncryptedCache(t.Context(), cacheKey)
	require.NoError(t, err)
	assert.JSONEq(t, string(originalJSON), string(cachedValue),
		"expired session must not be re-cached with the cache's default TTL")
}

func TestClientApp_CheckAndGetRefreshToken(t *testing.T) {
	tests := []struct {
		name                  string
		expectErrorContains   string
		expectNewToken        bool
		groupClaims           jwt.MapClaims
		refreshTokenThreshold string
	}{
		{
			name: "no new token",
			groupClaims: jwt.MapClaims{
				"aud":    common.ArgoCDClientAppID,
				"exp":    float64(time.Now().Add(time.Hour).Unix()),
				"sub":    "randomUser",
				"sid":    "1111",
				"iss":    "issuer",
				"groups": "group1",
			},
			expectNewToken:        false,
			refreshTokenThreshold: "1m",
		},
		{
			name: "new token",
			groupClaims: jwt.MapClaims{
				"aud":    common.ArgoCDClientAppID,
				"exp":    float64(time.Now().Add(55 * time.Second).Unix()),
				"sub":    "randomUser",
				"sid":    "1111",
				"iss":    "issuer",
				"groups": "group1",
			},
			expectNewToken:        true,
			refreshTokenThreshold: "1m",
		},
		{
			name: "parse error",
			groupClaims: jwt.MapClaims{
				"aud":    common.ArgoCDClientAppID,
				"exp":    float64(time.Now().Add(time.Minute).Unix()),
				"sub":    "randomUser",
				"sid":    "1111",
				"iss":    "issuer",
				"groups": "group1",
			},
			expectNewToken:        false,
			refreshTokenThreshold: "1xx",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oidcTestServer := test.GetOIDCTestServer(t, nil)
			t.Cleanup(oidcTestServer.Close)

			cdSettings := &settings.ArgoCDSettings{
				URL: "https://argocd.example.com",
				OIDCConfigRAW: fmt.Sprintf(`
name: Test
issuer: %s
clientID: test-client-id
clientSecret: test-client-secret
refreshTokenThreshold: %s
requestedScopes: ["oidc"]`, oidcTestServer.URL, tt.refreshTokenThreshold),
				OIDCTLSInsecureSkipVerify: true,
				UserSessionDuration:       24 * time.Hour,
			}
			// The base href (the last argument for NewClientApp) is what HandleLogin will fall back to when no explicit
			// redirect URL is given.
			app, err := NewClientApp(cdSettings, "", nil, "/", cache.NewInMemoryCache(24*time.Hour))
			require.NoError(t, err)
			oidcTokenCacheJSON, err := json.Marshal(&OidcTokenCache{Token: &oauth2.Token{
				RefreshToken: "not empty",
			}})
			require.NoError(t, err)
			sub := jwtutil.StringField(tt.groupClaims, "sub")
			require.NotEmpty(t, sub)
			sid := jwtutil.StringField(tt.groupClaims, "sid")
			require.NotEmpty(t, sid)
			require.NoError(t, app.SetValueInEncryptedCache(t.Context(), formatOidcTokenCacheKey(sub, sid), oidcTokenCacheJSON, time.Minute))
			token, err := app.CheckAndRefreshToken(t.Context(), tt.groupClaims, cdSettings.RefreshTokenThreshold())
			if tt.expectErrorContains != "" {
				require.ErrorContains(t, err, tt.expectErrorContains)
				return
			}
			require.NoError(t, err)
			if tt.expectNewToken {
				require.NotEmpty(t, token)
			} else {
				require.Empty(t, token)
			}
		})
	}
}

func TestClientApp_getRedirectURIForRequest(t *testing.T) {
	tests := []struct {
		name               string
		req                *http.Request
		expectLogContains  string
		expectedRequestURI string
		expectError        bool
	}{
		{
			name: "empty",
			req: &http.Request{
				URL: &url.URL{},
			},
		},
		{
			name:              "nil URL",
			expectLogContains: "falling back to configured redirect URI",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := ClientApp{provider: &fakeProvider{}, settings: &settings.ArgoCDSettings{}}
			hook := test.LogHook{}
			log.AddHook(&hook)
			t.Cleanup(func() {
				log.StandardLogger().ReplaceHooks(log.LevelHooks{})
			})
			redirectURI := app.getRedirectURIForRequest(tt.req)
			if tt.expectLogContains != "" {
				assert.NotEmpty(t, hook.GetRegexMatchesInEntries(tt.expectLogContains), "expected log")
			} else {
				assert.Empty(t, hook.Entries, "expected log")
			}
			if tt.req == nil {
				return
			}
			expectedRedirectURI, err := app.settings.RedirectURLForRequest(tt.req)
			if tt.expectError {
				assert.Error(t, err)
				return
			}
			assert.Equal(t, expectedRedirectURI, redirectURI, "expected URI")
		})
	}
}

func findLoginEntry(t *testing.T, hook *logtest.Hook) *log.Entry {
	t.Helper()
	for _, e := range hook.AllEntries() {
		if e.Data["login.type"] == "sso" {
			return e
		}
	}
	t.Fatal("expected an sso login log entry")
	return nil
}

func TestHandleCallback_LogsLoginAttempt(t *testing.T) {
	oidcTestServer := test.GetOIDCTestServer(t, nil)
	t.Cleanup(oidcTestServer.Close)

	cdSettings := &settings.ArgoCDSettings{
		URL: "https://argocd.example.com",
		OIDCConfigRAW: fmt.Sprintf(`
name: Test
issuer: %s
clientID: test-client-id
clientSecret: test-client-secret
requestedScopes: ["oidc"]`, oidcTestServer.URL),
		OIDCTLSInsecureSkipVerify: true,
	}

	t.Run("successful login", func(t *testing.T) {
		hook := logtest.NewGlobal()
		t.Cleanup(hook.Reset)
		app, err := NewClientApp(cdSettings, "", nil, "/", cache.NewInMemoryCache(24*time.Hour))
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/auth/login", http.NoBody)
		app.HandleLogin(w, req)
		redirectURL, err := w.Result().Location()
		require.NoError(t, err)
		state := redirectURL.Query().Get("state")

		req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("https://argocd.example.com/auth/callback?state=%s&code=abc", state), http.NoBody)
		for _, cookie := range w.Result().Cookies() {
			req.AddCookie(cookie)
		}
		w = httptest.NewRecorder()
		app.HandleCallback(w, req)
		require.Equal(t, http.StatusSeeOther, w.Code)

		entry := findLoginEntry(t, hook)
		assert.Equal(t, log.InfoLevel, entry.Level)
		assert.Equal(t, "Web login successful", entry.Message)
		// sub of the token issued by the OIDC test server
		assert.Equal(t, "1234567890", entry.Data["username"])
		assert.NotEmpty(t, entry.Data["claims"])
	})

	t.Run("failed login", func(t *testing.T) {
		hook := logtest.NewGlobal()
		t.Cleanup(hook.Reset)
		app, err := NewClientApp(cdSettings, "", nil, "/", cache.NewInMemoryCache(24*time.Hour))
		require.NoError(t, err)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/auth/callback?state=bogus&code=abc", http.NoBody)
		w := httptest.NewRecorder()
		app.HandleCallback(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code)

		entry := findLoginEntry(t, hook)
		assert.Equal(t, log.WarnLevel, entry.Level)
		assert.Equal(t, "Login failed", entry.Message)
		assert.NotEmpty(t, entry.Data["error"])
		assert.NotContains(t, entry.Data, "username")
	})

	t.Run("failed implicit flow login", func(t *testing.T) {
		hook := logtest.NewGlobal()
		t.Cleanup(hook.Reset)
		app, err := NewClientApp(cdSettings, "", nil, "/", cache.NewInMemoryCache(24*time.Hour))
		require.NoError(t, err)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/auth/callback?state=bogus", http.NoBody)
		w := httptest.NewRecorder()
		app.HandleCallback(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code)

		entry := findLoginEntry(t, hook)
		assert.Equal(t, log.WarnLevel, entry.Level)
		assert.Equal(t, "Login failed", entry.Message)
		assert.NotEmpty(t, entry.Data["error"])
	})

	t.Run("failed provider setup", func(t *testing.T) {
		hook := logtest.NewGlobal()
		t.Cleanup(hook.Reset)
		// the OIDC test server uses a self-signed certificate, so provider discovery fails without OIDCTLSInsecureSkipVerify
		strictSettings := *cdSettings
		strictSettings.OIDCTLSInsecureSkipVerify = false
		app, err := NewClientApp(&strictSettings, "", nil, "/", cache.NewInMemoryCache(24*time.Hour))
		require.NoError(t, err)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/auth/callback?state=bogus&code=abc", http.NoBody)
		w := httptest.NewRecorder()
		app.HandleCallback(w, req)
		require.Equal(t, http.StatusInternalServerError, w.Code)

		entry := findLoginEntry(t, hook)
		assert.Equal(t, log.WarnLevel, entry.Level)
		assert.Equal(t, "Login failed", entry.Message)
		assert.NotEmpty(t, entry.Data["error"])
	})
}
