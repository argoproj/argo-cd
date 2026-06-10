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
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/server/settings/oidc"
	"github.com/argoproj/argo-cd/v3/util"
	"github.com/argoproj/argo-cd/v3/util/cache"
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
			URL:                       "https://argocd.example.com",
			AdditionalURLs:            []string{"https://localhost:8080", "https://other.argocd.example.com"},
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
		app, err := NewClientApp(cdSettings, dexTestServer.URL, &dex.DexTLSConfig{StrictValidation: false}, "https://argocd.example.com", cache.NewInMemoryCache(24*time.Hour))
		require.NoError(t, err)

		t.Run("should accept login redirecting on the main domain", func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/auth/login", http.NoBody)

			req.URL.RawQuery = url.Values{
				"return_url": []string{"https://argocd.example.com/applications"},
			}.Encode()

			w := httptest.NewRecorder()

			app.HandleLogin(w, req)

			assert.Equal(t, http.StatusSeeOther, w.Code)
			location, err := url.Parse(w.Header().Get("Location"))
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("%s://%s", location.Scheme, location.Host), oidcTestServer.URL)
			assert.Equal(t, "/auth", location.Path)
			assert.Equal(t, "https://argocd.example.com/auth/callback", location.Query().Get("redirect_uri"))
		})

		t.Run("should accept login redirecting on the alternative domains", func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://localhost:8080/auth/login", http.NoBody)

			req.URL.RawQuery = url.Values{
				"return_url": []string{"https://localhost:8080/applications"},
			}.Encode()

			w := httptest.NewRecorder()

			app.HandleLogin(w, req)

			assert.Equal(t, http.StatusSeeOther, w.Code)
			location, err := url.Parse(w.Header().Get("Location"))
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("%s://%s", location.Scheme, location.Host), oidcTestServer.URL)
			assert.Equal(t, "/auth", location.Path)
			assert.Equal(t, "https://localhost:8080/auth/callback", location.Query().Get("redirect_uri"))
		})

		t.Run("should accept login redirecting on the alternative domains", func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://other.argocd.example.com/auth/login", http.NoBody)

			req.URL.RawQuery = url.Values{
				"return_url": []string{"https://other.argocd.example.com/applications"},
			}.Encode()

			w := httptest.NewRecorder()

			app.HandleLogin(w, req)

			assert.Equal(t, http.StatusSeeOther, w.Code)
			location, err := url.Parse(w.Header().Get("Location"))
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("%s://%s", location.Scheme, location.Host), oidcTestServer.URL)
			assert.Equal(t, "/auth", location.Path)
			assert.Equal(t, "https://other.argocd.example.com/auth/callback", location.Query().Get("redirect_uri"))
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

func TestSetGroupsFromUserInfo(t *testing.T) {
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

			// prepoluate cache to predict what the GetUserInfo function will return to the SetGroupsFromUserInfo function (without having to mock the userinfo response)
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
				encCacheClaims, err := crypto.Encrypt([]byte(cacheClaims), encryptionKey)
				require.NoError(t, err, "failed encrypting dummy access token")
				err = a.clientCache.Set(&cache.Item{
					Key:    FormatUserInfoResponseCacheKey("randomUser"),
					Object: encCacheClaims,
				})
				require.NoError(t, err, "failed setting item to in-memory cache")
			}

			receivedClaims, err := a.SetGroupsFromUserInfo(t.Context(), tt.inputClaims, "argocd")
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
			oidcTokenCache: NewOidcTokenCache("", (&oauth2.Token{}).WithExtra(map[string]any{"id_token": "simple"})),
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
			oidcTokenCache: NewOidcTokenCache("", (&oauth2.Token{}).WithExtra(map[string]any{"id_token": "simple"})),
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

func makeTestClientApp(t *testing.T, signature []byte) *ClientApp {
	t.Helper()
	sig := signature
	if sig == nil {
		var err error
		sig, err = util.MakeSignature(32)
		require.NoError(t, err)
	}
	cdSettings := &settings.ArgoCDSettings{
		ServerSignature: sig,
		OIDCConfigRAW: `
issuer: http://localhost:9999
clientID: test-client
clientSecret: test-secret`,
	}
	a, err := NewClientApp(cdSettings, "", nil, "/", cache.NewInMemoryCache(24*time.Hour))
	require.NoError(t, err)
	return a
}

func TestCreateCompactSessionToken(t *testing.T) {
	sig, err := util.MakeSignature(32)
	require.NoError(t, err)
	a := makeTestClientApp(t, sig)

	t.Run("compact token contains essential claims", func(t *testing.T) {
		oidcClaims := jwt.MapClaims{
			"iss":   "https://idp.example.com",
			"sub":   "user123",
			"email": "user@example.com",
			"name":  "Test User",
			"sid":   "session-abc",
			"exp":   float64(time.Now().Add(time.Hour).Unix()),
			"groups": []string{
				"group1", "group2", "group3",
			},
		}

		compactToken, err := a.createCompactSessionToken(oidcClaims)
		require.NoError(t, err)
		assert.NotEmpty(t, compactToken)

		// Parse without verification to inspect claims
		parser := jwt.NewParser(jwt.WithoutClaimsValidation())
		var got jwt.MapClaims
		_, _, err = parser.ParseUnverified(compactToken, &got)
		require.NoError(t, err)

		assert.Equal(t, "argocd", got["iss"])
		assert.Equal(t, "user123", got["sub"])
		assert.Equal(t, "user@example.com", got["email"])
		assert.Equal(t, "Test User", got["name"])
		assert.Equal(t, "session-abc", got["sid"])
		assert.Nil(t, got["groups"], "groups must be stripped from compact token")

		fc, ok := got["federated_claims"].(map[string]any)
		require.True(t, ok, "federated_claims must be present")
		assert.Equal(t, "https://idp.example.com", fc["connector_id"])
	})

	t.Run("compact token is smaller than original when many groups", func(t *testing.T) {
		groups := make([]string, 200)
		for i := range groups {
			groups[i] = fmt.Sprintf("group-with-a-somewhat-long-name-%d", i)
		}
		oidcClaims := jwt.MapClaims{
			"iss":    "https://idp.example.com",
			"sub":    "user123",
			"email":  "user@example.com",
			"exp":    float64(time.Now().Add(time.Hour).Unix()),
			"groups": groups,
		}

		// Simulate what the raw OIDC token would look like (just approximate via claims JSON)
		rawClaimsJSON, err := json.Marshal(oidcClaims)
		require.NoError(t, err)

		compactToken, err := a.createCompactSessionToken(oidcClaims)
		require.NoError(t, err)

		assert.Less(t, len(compactToken), len(rawClaimsJSON),
			"compact token (%d bytes) should be smaller than raw claims (%d bytes)",
			len(compactToken), len(rawClaimsJSON))
	})

	t.Run("existing federated_claims are preserved including user_id", func(t *testing.T) {
		oidcClaims := jwt.MapClaims{
			"iss": "https://dex.example.com",
			"sub": "Cgd1c2VyMTIzEgZnaXRodWI",
			"exp": float64(time.Now().Add(time.Hour).Unix()),
			"federated_claims": map[string]any{
				"connector_id": "github",
				"user_id":      "user123",
			},
		}
		compactToken, err := a.createCompactSessionToken(oidcClaims)
		require.NoError(t, err)

		parser := jwt.NewParser(jwt.WithoutClaimsValidation())
		var got jwt.MapClaims
		_, _, err = parser.ParseUnverified(compactToken, &got)
		require.NoError(t, err)

		fc, ok := got["federated_claims"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "github", fc["connector_id"])
		assert.Equal(t, "user123", fc["user_id"])
	})

	t.Run("compact token is verifiable with ArgoCD server signature", func(t *testing.T) {
		oidcClaims := jwt.MapClaims{
			"iss": "https://idp.example.com",
			"sub": "user123",
			"exp": float64(time.Now().Add(time.Hour).Unix()),
		}
		compactToken, err := a.createCompactSessionToken(oidcClaims)
		require.NoError(t, err)

		var got jwt.MapClaims
		parsed, err := jwt.ParseWithClaims(compactToken, &got, func(*jwt.Token) (any, error) {
			return sig, nil
		})
		require.NoError(t, err)
		assert.True(t, parsed.Valid)
	})
}

func TestGetGroupsFromCachedOIDCToken(t *testing.T) {
	sig, err := util.MakeSignature(32)
	require.NoError(t, err)
	a := makeTestClientApp(t, sig)

	encKey, err := a.settings.GetServerEncryptionKey()
	require.NoError(t, err)

	cacheToken := func(sub, sid string, idTokenClaims jwt.MapClaims) {
		t.Helper()
		idTokenStr, err := jwt.NewWithClaims(jwt.SigningMethodHS256, idTokenClaims).SignedString(sig)
		require.NoError(t, err)
		oidcCache := NewOidcTokenCache("http://localhost/callback", (&oauth2.Token{}).WithExtra(map[string]any{"id_token": idTokenStr}))
		raw, err := json.Marshal(oidcCache)
		require.NoError(t, err)
		enc, err := crypto.Encrypt(raw, encKey)
		require.NoError(t, err)
		err = a.clientCache.Set(&cache.Item{Key: formatOidcTokenCacheKey(sub, sid), Object: enc})
		require.NoError(t, err)
	}

	t.Run("returns groups from cached OIDC token", func(t *testing.T) {
		cacheToken("user1", "sess1", jwt.MapClaims{
			"sub":    "user1",
			"groups": []any{"admins", "developers"},
			"exp":    float64(time.Now().Add(time.Hour).Unix()),
		})
		groups, err := a.getGroupsFromCachedOIDCToken(t.Context(), "user1", "sess1")
		require.NoError(t, err)
		assert.Equal(t, []any{"admins", "developers"}, groups)
	})

	t.Run("returns nil on cache miss", func(t *testing.T) {
		groups, err := a.getGroupsFromCachedOIDCToken(t.Context(), "unknown", "unknown")
		require.NoError(t, err)
		assert.Nil(t, groups)
	})

	t.Run("returns nil when no groups in cached token", func(t *testing.T) {
		cacheToken("user2", "sess2", jwt.MapClaims{
			"sub": "user2",
			"exp": float64(time.Now().Add(time.Hour).Unix()),
		})
		groups, err := a.getGroupsFromCachedOIDCToken(t.Context(), "user2", "sess2")
		require.NoError(t, err)
		assert.Nil(t, groups)
	})
}

func TestSetGroupsFromUserInfo_CompactOIDCToken(t *testing.T) {
	sig, err := util.MakeSignature(32)
	require.NoError(t, err)

	cdSettings := &settings.ArgoCDSettings{
		ServerSignature: sig,
		OIDCConfigRAW: `
issuer: http://localhost:63231
enableUserInfoGroups: true
userInfoPath: /`,
	}
	a, err := NewClientApp(cdSettings, "", nil, "/", cache.NewInMemoryCache(24*time.Hour))
	require.NoError(t, err)

	encKey, err := cdSettings.GetServerEncryptionKey()
	require.NoError(t, err)

	// Cache the full OIDC token (with groups) to simulate what HandleCallback stores
	cacheOIDCToken := func(sub, sid string, groups []any) {
		t.Helper()
		idTokenClaims := jwt.MapClaims{
			"sub":    sub,
			"groups": groups,
			"exp":    float64(time.Now().Add(time.Hour).Unix()),
		}
		idTokenStr, err := jwt.NewWithClaims(jwt.SigningMethodHS256, idTokenClaims).SignedString(sig)
		require.NoError(t, err)
		oidcCache := NewOidcTokenCache("http://localhost/cb", (&oauth2.Token{}).WithExtra(map[string]any{"id_token": idTokenStr}))
		raw, err := json.Marshal(oidcCache)
		require.NoError(t, err)
		enc, err := crypto.Encrypt(raw, encKey)
		require.NoError(t, err)
		err = a.clientCache.Set(&cache.Item{Key: formatOidcTokenCacheKey(sub, sid), Object: enc})
		require.NoError(t, err)
	}

	t.Run("groups reconstructed from Redis cache for compact OIDC token", func(t *testing.T) {
		cacheOIDCToken("user1", "sess1", []any{"eng", "admin"})

		// Simulate what Parse() returns for a compact token
		compactClaims := jwt.MapClaims{
			"iss": "argocd",
			"sub": "user1",
			"sid": "sess1",
			"federated_claims": map[string]any{
				"connector_id": "https://idp.example.com",
			},
		}
		got, err := a.SetGroupsFromUserInfo(t.Context(), compactClaims, "argocd")
		require.NoError(t, err)
		assert.Equal(t, []any{"eng", "admin"}, got["groups"])
	})

	t.Run("regular ArgoCD local-account token is not affected", func(t *testing.T) {
		// Local account token has no federated_claims
		localClaims := jwt.MapClaims{
			"iss": "argocd",
			"sub": "admin",
		}
		got, err := a.SetGroupsFromUserInfo(t.Context(), localClaims, "argocd")
		require.NoError(t, err)
		assert.Nil(t, got["groups"], "local account tokens must not have groups injected")
	})

	t.Run("cache miss falls through to UserInfo endpoint error", func(t *testing.T) {
		compactClaims := jwt.MapClaims{
			"iss": "argocd",
			"sub": "missing-user",
			"sid": "missing-sess",
			"federated_claims": map[string]any{
				"connector_id": "https://idp.example.com",
			},
		}
		// No access token cached, so GetUserInfo will fail
		_, err := a.SetGroupsFromUserInfo(t.Context(), compactClaims, "argocd")
		// Expect an error from the UserInfo endpoint since we have no access token
		require.Error(t, err)
	})
}

func TestHandleCallback_CompactTokenInCookie(t *testing.T) {
	// Build a large group list that would bust the 4 KB single-cookie limit if stored raw.
	largeGroups := make([]string, 150)
	for i := range largeGroups {
		largeGroups[i] = fmt.Sprintf("team-%03d-engineering-department", i)
	}

	oidcTestServer := test.GetOIDCTestServerWithGroups(t, largeGroups)
	t.Cleanup(oidcTestServer.Close)

	sig, err := util.MakeSignature(32)
	require.NoError(t, err)
	cdSettings := &settings.ArgoCDSettings{
		URL:             "https://argocd.example.com",
		ServerSignature: sig,
		OIDCConfigRAW: fmt.Sprintf(`
name: Test
issuer: %s
clientID: test-client-id
clientSecret: test-client-secret
requestedScopes: ["oidc"]`, oidcTestServer.URL),
		OIDCTLSInsecureSkipVerify: true,
	}
	app, err := NewClientApp(cdSettings, "", nil, "/", cache.NewInMemoryCache(24*time.Hour))
	require.NoError(t, err)

	// Drive the login → callback flow
	w := httptest.NewRecorder()
	loginReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/auth/login", http.NoBody)
	app.HandleLogin(w, loginReq)

	redirectURL, err := w.Result().Location()
	require.NoError(t, err)
	state := redirectURL.Query().Get("state")

	callbackReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		fmt.Sprintf("https://argocd.example.com/auth/callback?state=%s&code=abc", state), http.NoBody)
	for _, c := range w.Result().Cookies() {
		callbackReq.AddCookie(c)
	}

	w = httptest.NewRecorder()
	app.HandleCallback(w, callbackReq)
	require.Equal(t, http.StatusSeeOther, w.Code, "callback must redirect: %s", w.Body.String())

	// Find the argocd.token cookie
	var tokenCookie string
	for _, c := range w.Result().Cookies() {
		if strings.HasPrefix(c.Name, common.AuthCookieName) {
			tokenCookie = c.Value
			break
		}
	}
	require.NotEmpty(t, tokenCookie, "argocd.token cookie must be set")

	// The cookie value should be a single cookie (no chunking prefix) or at most a few chunks —
	// the key assertion is that it's an ArgoCD-signed token, not the raw OIDC ID token.
	// We strip the chunk-count prefix if present.
	rawToken := tokenCookie
	if before, after, ok := strings.Cut(tokenCookie, ":"); ok {
		if _, convErr := fmt.Sscanf(before, "%d", new(int)); convErr == nil {
			rawToken = after
		}
	}

	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	var claims jwt.MapClaims
	_, _, err = parser.ParseUnverified(rawToken, &claims)
	require.NoError(t, err)

	assert.Equal(t, "argocd", claims["iss"], "compact token must be ArgoCD-signed")
	assert.Nil(t, claims["groups"], "groups must not appear in the cookie token")
	_, hasFC := claims["federated_claims"]
	assert.True(t, hasFC, "federated_claims must be present to mark OIDC origin")
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
