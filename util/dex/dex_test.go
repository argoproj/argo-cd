package dex

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"

	// "github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util/settings"
)

const invalidURL = ":://localhost/foo/bar"

var malformedDexConfig = `
valid:
  yaml: valid
yaml:
  valid
`

var goodDexConfig = `
connectors:
# GitHub example
- type: github
  id: github
  name: GitHub
  config:
    clientID: aabbccddeeff00112233
    clientSecret: $dex.github.clientSecret
    orgs:
    - name: your-github-org

# GitHub enterprise example
- type: github
  id: acme-github
  name: Acme GitHub
  config:
    hostName: github.acme.com
    clientID: abcdefghijklmnopqrst
    clientSecret: $dex.acme.clientSecret
    orgs:
    - name: your-github-org
`
var customStaticClientDexConfig = `
connectors:
# GitHub example
- type: github
  id: github
  name: GitHub
  config:
    clientID: aabbccddeeff00112233
    clientSecret: abcdefghijklmnopqrst
    orgs:
    - name: your-github-org
staticClients:
- id: argo-workflow
  name: Argo Workflow
  redirectURIs:
  - https://argo/oauth2/callback
  secret: abcdefghijklmnopqrst
`
var badDexConfig = `
connectors:
# GitHub example
- type: github
  id: github
  name: GitHub
  config: foo

# GitHub enterprise example
- type: github
  id: acme-github
  name: Acme GitHub
  config:
    hostName: github.acme.com
    clientID: abcdefghijklmnopqrst
    clientSecret: $dex.acme.clientSecret
    orgs:
    - name: your-github-org
`
var goodSecrets = map[string]string{
	"dex.github.clientSecret": "foobar",
	"dex.acme.clientSecret":   "barfoo",
}

func Test_GenerateDexConfig(t *testing.T) {

	t.Run("Empty settings", func(t *testing.T) {
		s := settings.ArgoCDSettings{}
		config, err := GenerateDexConfigYAML(&s)
		assert.NoError(t, err)
		assert.Nil(t, config)
	})

	t.Run("Invalid URL", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       invalidURL,
			DexConfig: goodDexConfig,
		}
		config, err := GenerateDexConfigYAML(&s)
		assert.Error(t, err)
		assert.Nil(t, config)
	})

	t.Run("No URL set", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "",
			DexConfig: "invalidyaml",
		}
		config, err := GenerateDexConfigYAML(&s)
		assert.NoError(t, err)
		assert.Nil(t, config)
	})

	t.Run("Invalid YAML", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "http://localhost",
			DexConfig: "invalidyaml",
		}
		config, err := GenerateDexConfigYAML(&s)
		assert.NoError(t, err)
		assert.Nil(t, config)
	})

	t.Run("Valid YAML but incorrect Dex config", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "http://localhost",
			DexConfig: malformedDexConfig,
		}
		config, err := GenerateDexConfigYAML(&s)
		assert.Error(t, err)
		assert.Nil(t, config)
	})

	t.Run("Valid YAML but incorrect Dex config", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "http://localhost",
			DexConfig: badDexConfig,
		}
		config, err := GenerateDexConfigYAML(&s)
		assert.Error(t, err)
		assert.Nil(t, config)
	})

	t.Run("Valid YAML and correct Dex config", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "http://localhost",
			DexConfig: goodDexConfig,
		}
		config, err := GenerateDexConfigYAML(&s)
		assert.NoError(t, err)
		assert.NotNil(t, config)
	})

	t.Run("Secret dereference", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "http://localhost",
			DexConfig: goodDexConfig,
			Secrets:   goodSecrets,
		}
		config, err := GenerateDexConfigYAML(&s)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		var dexCfg map[string]interface{}
		err = yaml.Unmarshal(config, &dexCfg)
		if err != nil {
			panic(err.Error())
		}
		connectors, ok := dexCfg["connectors"].([]interface{})
		assert.True(t, ok)
		for i, connectorsIf := range connectors {
			config := connectorsIf.(map[string]interface{})["config"].(map[string]interface{})
			if i == 0 {
				assert.Equal(t, "foobar", config["clientSecret"])
			} else if i == 1 {
				assert.Equal(t, "barfoo", config["clientSecret"])
			}
		}
	})

	t.Run("Redirect config", func(t *testing.T) {
		types := []string{"oidc", "saml", "microsoft", "linkedin", "gitlab", "github", "bitbucket-cloud"}
		for _, c := range types {
			assert.True(t, needsRedirectURI(c))
		}
		assert.False(t, needsRedirectURI("invalid"))
	})

	t.Run("Custom static clients", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "http://localhost",
			DexConfig: customStaticClientDexConfig,
		}
		config, err := GenerateDexConfigYAML(&s)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		var dexCfg map[string]interface{}
		err = yaml.Unmarshal(config, &dexCfg)
		if err != nil {
			panic(err.Error())
		}
		clients, ok := dexCfg["staticClients"].([]interface{})
		assert.True(t, ok)
		assert.Equal(t, 3, len(clients))

		customClient := clients[2].(map[string]interface{})
		assert.Equal(t, "argo-workflow", customClient["id"].(string))
		assert.Equal(t, 1, len(customClient["redirectURIs"].([]interface{})))
	})
}

func Test_DexReverseProxy(t *testing.T) {
	t.Run("Good case", func(t *testing.T) {
		fakeDex := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusOK)
		}))
		defer fakeDex.Close()
		fmt.Printf("Fake Dex listening on %s\n", fakeDex.URL)
		server := httptest.NewServer(http.HandlerFunc(NewDexHTTPReverseProxy(fakeDex.URL, "/")))
		fmt.Printf("Fake API Server listening on %s\n", server.URL)
		defer server.Close()
		resp, err := http.Get(server.URL)
		assert.NotNil(t, resp)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		fmt.Printf("%s\n", resp.Status)
	})

	t.Run("Bad case", func(t *testing.T) {
		fakeDex := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusInternalServerError)
		}))
		defer fakeDex.Close()
		fmt.Printf("Fake Dex listening on %s\n", fakeDex.URL)
		server := httptest.NewServer(http.HandlerFunc(NewDexHTTPReverseProxy(fakeDex.URL, "/")))
		fmt.Printf("Fake API Server listening on %s\n", server.URL)
		defer server.Close()
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			}}
		resp, err := client.Get(server.URL)
		assert.NotNil(t, resp)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
		location, _ := resp.Location()
		fmt.Printf("%s %s\n", resp.Status, location.RequestURI())
		assert.True(t, strings.HasPrefix(location.RequestURI(), "/login?sso_error"))
	})

	t.Run("Invalid URL for Dex reverse proxy", func(t *testing.T) {
		// Can't test for now, since it would call exit
		t.Skip()
		f := NewDexHTTPReverseProxy(invalidURL, "/")
		assert.Nil(t, f)
	})

	t.Run("Round Tripper", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			assert.Equal(t, "/", req.URL.String())
		}))
		defer server.Close()
		rt := NewDexRewriteURLRoundTripper(server.URL, http.DefaultTransport)
		assert.NotNil(t, rt)
		req, err := http.NewRequest("GET", "/", bytes.NewBuffer([]byte("")))
		assert.NoError(t, err)
		_, err = rt.RoundTrip(req)
		assert.NoError(t, err)
	})
}
