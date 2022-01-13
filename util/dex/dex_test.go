package dex

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"

	// "github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/v2/util/settings"
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
    clientSecret: abcdefghijklmnopqrst\n\r
    orgs:
    - name: your-github-org
staticClients:
- id: argo-workflow
  name: Argo Workflow
  redirectURIs:
  - https://argo/oauth2/callback
  secret:  $dex.acme.clientSecret  
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

var goodDexConfigWithOauthOverrides = `
oauth2:
  passwordConnector: ldap
connectors:
- type: ldap
  name: OpenLDAP
  id: ldap
  config:
    host: localhost:389
    insecureNoSSL: true
    bindDN: cn=admin,dc=example,dc=org
    bindPW: admin
    usernamePrompt: Email Address
    userSearch:
      baseDN: ou=People,dc=example,dc=org
      filter: "(objectClass=person)"
      username: mail
      idAttr: DN
      emailAttr: mail
      nameAttr: cn
    groupSearch:
      baseDN: ou=Groups,dc=example,dc=org
      filter: "(objectClass=groupOfNames)"
      nameAttr: cn
`
var goodDexConfigWithEnabledApprovalScreen = `
oauth2:
  passwordConnector: ldap
  skipApprovalScreen: false
connectors:
- type: ldap
  name: OpenLDAP
  id: ldap
  config:
    host: localhost:389
    insecureNoSSL: true
    bindDN: cn=admin,dc=example,dc=org
    bindPW: admin
    usernamePrompt: Email Address
    userSearch:
      baseDN: ou=People,dc=example,dc=org
      filter: "(objectClass=person)"
      username: mail
      idAttr: DN
      emailAttr: mail
      nameAttr: cn
    groupSearch:
      baseDN: ou=Groups,dc=example,dc=org
      filter: "(objectClass=groupOfNames)"
      nameAttr: cn
`

var goodSecrets = map[string]string{
	"dex.github.clientSecret": "foobar",
	"dex.acme.clientSecret":   "barfoo",
}

var goodSecretswithCRLF = map[string]string{
	"dex.github.clientSecret": "foobar\n\r",
	"dex.acme.clientSecret":   "barfoo\n\r",
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

	t.Run("Secret dereference with extra white space", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "http://localhost",
			DexConfig: goodDexConfig,
			Secrets:   goodSecretswithCRLF,
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
			Secrets:   goodSecretswithCRLF,
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
	t.Run("Custom static clients secret dereference with trailing CRLF", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "http://localhost",
			DexConfig: customStaticClientDexConfig,
			Secrets:   goodSecretswithCRLF,
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
		assert.Equal(t, "barfoo", customClient["secret"])
	})
	t.Run("Override dex oauth2 configuration", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "http://localhost",
			DexConfig: goodDexConfigWithOauthOverrides,
		}
		config, err := GenerateDexConfigYAML(&s)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		var dexCfg map[string]interface{}
		err = yaml.Unmarshal(config, &dexCfg)
		if err != nil {
			panic(err.Error())
		}
		oauth2Config, ok := dexCfg["oauth2"].(map[string]interface{})
		assert.True(t, ok)
		pwConn, ok := oauth2Config["passwordConnector"].(string)
		assert.True(t, ok)
		assert.Equal(t, "ldap", pwConn)

		skipApprScr, ok := oauth2Config["skipApprovalScreen"].(bool)
		assert.True(t, ok)
		assert.True(t, skipApprScr)
	})
	t.Run("Override dex oauth2 with enabled ApprovalScreen", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "http://localhost",
			DexConfig: goodDexConfigWithEnabledApprovalScreen,
		}
		config, err := GenerateDexConfigYAML(&s)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		var dexCfg map[string]interface{}
		err = yaml.Unmarshal(config, &dexCfg)
		if err != nil {
			panic(err.Error())
		}
		oauth2Config, ok := dexCfg["oauth2"].(map[string]interface{})
		assert.True(t, ok)
		pwConn, ok := oauth2Config["passwordConnector"].(string)
		assert.True(t, ok)
		assert.Equal(t, "ldap", pwConn)

		skipApprScr, ok := oauth2Config["skipApprovalScreen"].(bool)
		assert.True(t, ok)
		assert.False(t, skipApprScr)
	})
}

func Test_DexReverseProxy(t *testing.T) {
	t.Run("Good case", func(t *testing.T) {
		var host string
		fakeDex := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			host = req.Host
			rw.WriteHeader(http.StatusOK)
		}))
		defer fakeDex.Close()
		fmt.Printf("Fake Dex listening on %s\n", fakeDex.URL)
		server := httptest.NewServer(http.HandlerFunc(NewDexHTTPReverseProxy(fakeDex.URL, "/")))
		fmt.Printf("Fake API Server listening on %s\n", server.URL)
		defer server.Close()
		target, _ := url.Parse(fakeDex.URL)
		resp, err := http.Get(server.URL)
		assert.NotNil(t, resp)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, host, target.Host)
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
