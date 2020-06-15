package oidc

import (
	"encoding/json"
	"io/ioutil"
	"net/http/httptest"
	"net/url"
	"testing"

	gooidc "github.com/coreos/go-oidc"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"

	"github.com/argoproj/argo-cd/server/settings/oidc"
)

func TestInferGrantType(t *testing.T) {
	for _, path := range []string{"dex", "okta", "auth0", "onelogin"} {
		t.Run(path, func(t *testing.T) {
			rawConfig, err := ioutil.ReadFile("testdata/" + path + ".json")
			assert.NoError(t, err)
			var config OIDCConfiguration
			err = json.Unmarshal(rawConfig, &config)
			assert.NoError(t, err)
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
	assert.Len(t, opts, 0)

	requestedClaims["groups"] = &oidc.Claim{Essential: true}
	opts = AppendClaimsAuthenticationRequestParameter(opts, requestedClaims)
	assert.Len(t, opts, 1)

	authCodeURL, err := url.Parse(oauth2Config.AuthCodeURL("TEST", opts...))
	assert.NoError(t, err)

	values, err := url.ParseQuery(authCodeURL.RawQuery)
	assert.NoError(t, err)

	assert.Equal(t, "{\"id_token\":{\"groups\":{\"essential\":true}}}", values.Get("claims"))
}

type fakeProvider struct {
}

func (p *fakeProvider) Endpoint() (*oauth2.Endpoint, error) {
	return &oauth2.Endpoint{}, nil
}

func (p *fakeProvider) ParseConfig() (*OIDCConfiguration, error) {
	return nil, nil
}

func (p *fakeProvider) Verify(_, _ string) (*gooidc.IDToken, error) {
	return nil, nil
}

func TestHandleCallback(t *testing.T) {
	app := ClientApp{provider: &fakeProvider{}}

	req := httptest.NewRequest("GET", "http://example.com/foo", nil)
	req.Form = url.Values{
		"error":             []string{"login-failed"},
		"error_description": []string{"<script>alert('hello')</script>"},
	}
	w := httptest.NewRecorder()

	app.HandleCallback(w, req)

	assert.Equal(t, "login-failed: &lt;script&gt;alert(&#39;hello&#39;)&lt;/script&gt;\n", w.Body.String())
}
