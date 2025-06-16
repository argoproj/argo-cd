package oidc

import (
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
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

type fakeProvider struct{}

func (p *fakeProvider) Endpoint() (*oauth2.Endpoint, error) {
	return &oauth2.Endpoint{}, nil
}

func (p *fakeProvider) ParseConfig() (*OIDCConfiguration, error) {
	return nil, nil
}

func (p *fakeProvider) Verify(_ string, _ *settings.ArgoCDSettings) (*gooidc.IDToken, error) {
	return nil, nil
}

func TestHandleCallback(t *testing.T) {
	app := ClientApp{provider: &fakeProvider{}, settings: &settings.ArgoCDSettings{}}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/foo", http.NoBody)
	req.Form = url.Values{
		"error":             []string{"login-failed"},
		"error_description": []string{"<script>alert('hello')</script>"},
	}
	w := httptest.NewRecorder()

	app.HandleCallback(w, req)

	assert.Equal(t, "login-failed: &lt;script&gt;alert(&#39;hello&#39;)&lt;/script&gt;\n", w.Body.String())
}

func TestClientApp_HandleLogin(t *testing.T) {
	oidcTestServer := test.GetOIDCTestServer(t)
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

		req := httptest.NewRequest(http.MethodGet, "https://argocd.example.com/auth/login", http.NoBody)

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

		req := httptest.NewRequest(http.MethodGet, "https://argocd.example.com/auth/login", http.NoBody)

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

		req := httptest.NewRequest(http.MethodGet, "https://argocd.example.com/auth/login", http.NoBody)
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

		req := httptest.NewRequest(http.MethodGet, "https://argocd.example.com/auth/login", http.NoBody)
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

		req := httptest.NewRequest(http.MethodGet, "https://argocd.example.com/auth/login", http.NoBody)
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
			req := httptest.NewRequest(http.MethodGet, "https://argocd.example.com/auth/login", http.NoBody)

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
			req := httptest.NewRequest(http.MethodGet, "https://localhost:8080/auth/login", http.NoBody)

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
			req := httptest.NewRequest(http.MethodGet, "https://other.argocd.example.com/auth/login", http.NoBody)

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
			req := httptest.NewRequest(http.MethodGet, "https://not-argocd.example.com/auth/login", http.NoBody)

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

	oidcTestServer := test.GetOIDCTestServer(t)
	t.Cleanup(oidcTestServer.Close)

	cdSettings := &settings.ArgoCDSettings{
		URL: "https://argocd.example.com",
		OIDCConfigRAW: fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]`, oidcTestServer.URL),
		OIDCTLSInsecureSkipVerify: true,
	}
	// The base href (the last argument for NewClientApp) is what HandleLogin will fall back to when no explicit
	// redirect URL is given.
	app, err := NewClientApp(cdSettings, "", nil, "/", cache.NewInMemoryCache(24*time.Hour))
	require.NoError(t, err)

	w := httptest.NewRecorder()

	req := httptest.NewRequest(http.MethodGet, "https://argocd.example.com/auth/login", http.NoBody)

	app.HandleLogin(w, req)

	redirectURL, err := w.Result().Location()
	require.NoError(t, err)

	state := redirectURL.Query()["state"]

	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("https://argocd.example.com/auth/callback?state=%s&code=abc", state), http.NoBody)
	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}

	w = httptest.NewRecorder()

	app.HandleCallback(w, req)

	assert.NotContains(t, w.Body.String(), ErrInvalidRedirectURL.Error())
}

func TestClientApp_HandleCallback(t *testing.T) {
	oidcTestServer := test.GetOIDCTestServer(t)
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

		req := httptest.NewRequest(http.MethodGet, "https://argocd.example.com/auth/callback", http.NoBody)

		w := httptest.NewRecorder()

		app.HandleCallback(w, req)

		if !strings.Contains(w.Body.String(), "certificate signed by unknown authority") && !strings.Contains(w.Body.String(), "certificate is not trusted") {
			t.Fatal("did not receive expected certificate verification failure error")
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

		req := httptest.NewRequest(http.MethodGet, "https://argocd.example.com/auth/callback", http.NoBody)

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
		for i := 0; i < numGoroutines; i++ {
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
		for i := 0; i < numGoroutines; i++ {
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

		req := httptest.NewRequest(http.MethodGet, "https://argocd.example.com/auth/callback", http.NoBody)
		req.Form = url.Values{
			"code":  {"abc"},
			"state": {"123"},
		}
		w := httptest.NewRecorder()

		app.HandleCallback(w, req)

		if !strings.Contains(w.Body.String(), "certificate signed by unknown authority") && !strings.Contains(w.Body.String(), "certificate is not trusted") {
			t.Fatal("did not receive expected certificate verification failure error")
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
	state, err := app.generateAppState(expectedReturnURL, generateResponse)
	require.NoError(t, err)

	t.Run("VerifyAppState_Successful", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		for _, cookie := range generateResponse.Result().Cookies() {
			req.AddCookie(cookie)
		}

		returnURL, err := app.verifyAppState(req, httptest.NewRecorder(), state)
		require.NoError(t, err)
		assert.Equal(t, expectedReturnURL, returnURL)
	})

	t.Run("VerifyAppState_Failed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		for _, cookie := range generateResponse.Result().Cookies() {
			req.AddCookie(cookie)
		}

		_, err := app.verifyAppState(req, httptest.NewRecorder(), "wrong state")
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
		state, err := app.generateAppState(expectedReturnURL, generateResponse)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		for _, cookie := range generateResponse.Result().Cookies() {
			req.AddCookie(cookie)
		}

		returnURL, err := app.verifyAppState(req, httptest.NewRecorder(), state)
		require.ErrorIs(t, err, ErrInvalidRedirectURL)
		assert.Empty(t, returnURL)
	})

	t.Run("valid return URL succeeds", func(t *testing.T) {
		expectedReturnURL := "https://argocd.example.com/some/path"
		generateResponse := httptest.NewRecorder()
		state, err := app.generateAppState(expectedReturnURL, generateResponse)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		for _, cookie := range generateResponse.Result().Cookies() {
			req.AddCookie(cookie)
		}

		returnURL, err := app.verifyAppState(req, httptest.NewRecorder(), state)
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

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	encrypted, err := crypto.Encrypt([]byte("123"), key)
	require.NoError(t, err)
	app, err := NewClientApp(cdSettings, "", nil, "/argo-cd", cache.NewInMemoryCache(24*time.Hour))
	require.NoError(t, err)

	req.AddCookie(&http.Cookie{Name: common.StateCookieName, Value: hex.EncodeToString(encrypted)})
	returnURL, err := app.verifyAppState(req, httptest.NewRecorder(), "123")
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
		idpHandler func(w http.ResponseWriter, r *http.Request)
		idpClaims  jwt.MapClaims // as per specification sub and exp are REQUIRED fields
		cache      cache.CacheClient
		cacheItems []struct { // items to put in cache before execution
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
					key:         formatUserInfoResponseCacheKey("randomUser"),
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
					key:     formatAccessTokenCacheKey("randomUser"),
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
					key:         formatUserInfoResponseCacheKey("randomUser"),
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
					key:     formatAccessTokenCacheKey("randomUser"),
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
					key:         formatUserInfoResponseCacheKey("randomUser"),
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
					key:     formatAccessTokenCacheKey("randomUser"),
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
					key:         formatUserInfoResponseCacheKey("randomUser"),
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
					key:             formatUserInfoResponseCacheKey("randomUser"),
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
					key:     formatAccessTokenCacheKey("randomUser"),
					value:   "FakeAccessToken",
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

			got, unauthenticated, err := a.GetUserInfo(tt.idpClaims, ts.URL, tt.userInfoPath)
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
