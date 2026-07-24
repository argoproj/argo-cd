package dex

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/common"
	utillog "github.com/argoproj/argo-cd/v3/util/log"
	"github.com/argoproj/argo-cd/v3/util/settings"
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
    hostName: github.acme.example.com
    clientID: abcdefghijklmnopqrst
    clientSecret: $dex.acme.clientSecret
    orgs:
    - name: your-github-org
`

var goodDexConfigWithStorageTypeKubernetes = `
storage:
  type: kubernetes
  config:
    inCluster: true
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
    hostName: github.acme.example.com
    clientID: abcdefghijklmnopqrst
    clientSecret: $dex.acme.clientSecret
    orgs:
    - name: your-github-org
`

var goodDexConfigWithStorageTypeEtcd = `
storage:
  type: etcd
  config:
    endpoints:
      - http://localhost:2379
    namespace: my-etcd-namespace
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
    hostName: github.acme.example.com
    clientID: abcdefghijklmnopqrst
    clientSecret: $dex.acme.clientSecret
    orgs:
    - name: your-github-org
`

var goodDexConfigWithStorageTypePostgres = `
storage:
  type: postgres
  config:
    host: localhost
    port: 5432
    database: dex_db
    user: dex
    password: 66964843358242dbaaa7778d8477c288
    ssl:
      mode: verify-ca
      caFile: /etc/dex/postgres.ca
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
    hostName: github.acme.example.com
    clientID: abcdefghijklmnopqrst
    clientSecret: $dex.acme.clientSecret
    orgs:
    - name: your-github-org
`

var goodDexConfigLDAPWithDollarSign = `
connectors:
- type: ldap
  id: ldap
  name: OpenLDAP
  config:
    host: localhost:389
    insecureNoSSL: true
    bindDN: cn=admin,dc=example,dc=org
    bindPW: $dex.ldap.bindPW
    userSearch:
      baseDN: ou=People,dc=example,dc=org
      filter: "(objectClass=inetOrgPerson)"
      username: mail
      idAttr: DN
      emailAttr: mail
      nameAttr: cn
`

var goodDexConfigPasswordWithDollarSign = `
connectors:
- type: ldap
  id: ldap
  name: OpenLDAP
  config:
    host: localhost:389
    insecureNoSSL: true
    bindDN: cn=admin,dc=example,dc=org
    bindPW: "test$test"
    userSearch:
      baseDN: ou=People,dc=example,dc=org
      filter: "(objectClass=inetOrgPerson)"
      username: mail
      idAttr: DN
      emailAttr: mail
      nameAttr: cn
`

var goodDexConfigPlainPasswordMultipleDollarSigns = `
connectors:
- type: ldap
  id: ldap
  name: OpenLDAP
  config:
    host: localhost:389
    insecureNoSSL: true
    bindDN: cn=admin,dc=example,dc=org
    bindPW: "a$b$c$d"
    userSearch:
      baseDN: ou=People,dc=example,dc=org
      filter: "(objectClass=inetOrgPerson)"
      username: mail
      idAttr: DN
      emailAttr: mail
      nameAttr: cn
`

var goodDexConfigPlainPassword = `
connectors:
- type: ldap
  id: ldap
  name: OpenLDAP
  config:
    host: localhost:389
    insecureNoSSL: true
    bindDN: cn=admin,dc=example,dc=org
    bindPW: "plainpassword"
    userSearch:
      baseDN: ou=People,dc=example,dc=org
      filter: "(objectClass=inetOrgPerson)"
      username: mail
      idAttr: DN
      emailAttr: mail
      nameAttr: cn
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
    hostName: github.acme.example.com
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

var goodDexConfigWithLogger = `
logger:
  level: debug
  other: value
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
    hostName: github.acme.example.com
    clientID: abcdefghijklmnopqrst
    clientSecret: $dex.acme.clientSecret
    orgs:
    - name: your-github-org
`

var goodDexConfigOIDCWithDollarSign = `
connectors:
- type: oidc
  id: oidc
  name: Some OIDC Provider
  config:
    issuer: https://accounts.example.com
    clientID: argo-cd
    clientSecret: $dex.oidc.clientSecret
    redirectURI: http://localhost/callback
    scopes:
    - openid
    - profile
    - email
`

var goodDexConfigGitHubWithDollarSign = `
connectors:
- type: github
  id: github
  name: GitHub
  config:
    clientID: aabbccddeeff00112233
    clientSecret: $dex.github.clientSecret
    orgs:
    - name: your-github-org
`

var goodDexConfigOIDCWithEnvVarReference = `
connectors:
- type: oidc
  id: authentik
  name: Authentik
  config:
    issuer: https://sso.example.com/
    clientID: $AUTHENTIK_CLIENT_ID
    clientSecret: $AUTHENTIK_CLIENT_SECRET
    redirectURI: http://localhost/callback
    scopes:
    - openid
    - profile
    - email
`

var goodSecrets = map[string]string{
	"dex.github.clientSecret": "foobar",
	"dex.acme.clientSecret":   "barfoo",
}

var goodSecretswithCRLF = map[string]string{
	"dex.github.clientSecret": "foobar\n\r",
	"dex.acme.clientSecret":   "barfoo\n\r",
}

func Test_GenerateDexConfigYAMLStorage(t *testing.T) {
	validateKubernetesStorage := func(t *testing.T, storage map[string]any) {
		t.Helper()

		storageConfig, ok := storage["config"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, true, storageConfig["inCluster"])
	}

	validateEtcdStorage := func(t *testing.T, storage map[string]any) {
		t.Helper()

		storageConfig, ok := storage["config"].(map[string]any)
		require.True(t, ok)

		assert.Equal(t, "my-etcd-namespace", storageConfig["namespace"])

		endpoints, ok := storageConfig["endpoints"].([]any)
		require.True(t, ok)
		require.NotEmpty(t, endpoints)

		assert.Equal(t, "http://localhost:2379", endpoints[0])
	}

	validatePostgresStorage := func(t *testing.T, storage map[string]any) {
		t.Helper()

		storageConfig, ok := storage["config"].(map[string]any)
		require.True(t, ok)

		assert.Equal(t, "localhost", storageConfig["host"])
		assert.InDelta(t, 5432.0, storageConfig["port"], 0.0)
		assert.Equal(t, "dex_db", storageConfig["database"])
		assert.Equal(t, "dex", storageConfig["user"])
		assert.Equal(t, "66964843358242dbaaa7778d8477c288", storageConfig["password"])

		ssl, ok := storageConfig["ssl"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "verify-ca", ssl["mode"])
		assert.Equal(t, "/etc/dex/postgres.ca", ssl["caFile"])
	}

	tests := []struct {
		name     string
		settings settings.ArgoCDSettings
		wantType string
		validate func(t *testing.T, storage map[string]any)
	}{
		{
			name: "kubernetes storage type, config",
			settings: settings.ArgoCDSettings{
				URL:       "http://localhost",
				DexConfig: goodDexConfigWithStorageTypeKubernetes,
				Secrets:   goodSecrets,
			},
			wantType: "kubernetes",
			validate: validateKubernetesStorage,
		},
		{
			name: "default storage type is memory when not specified",
			settings: settings.ArgoCDSettings{
				URL:       "http://localhost",
				DexConfig: goodDexConfig,
			},
			wantType: "memory",
		},
		{
			name: "etcd storage type, config",
			settings: settings.ArgoCDSettings{
				URL:       "http://localhost",
				DexConfig: goodDexConfigWithStorageTypeEtcd,
				Secrets:   goodSecrets,
			},
			wantType: "etcd",
			validate: validateEtcdStorage,
		},
		{
			name: "postgres storage type, config",
			settings: settings.ArgoCDSettings{
				URL:       "http://localhost",
				DexConfig: goodDexConfigWithStorageTypePostgres,
				Secrets:   goodSecrets,
			},
			wantType: "postgres",
			validate: validatePostgresStorage,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := GenerateDexConfigYAML(&tc.settings, false)
			require.NoError(t, err)
			require.NotNil(t, config)

			var dexCfg map[string]any
			err = yaml.Unmarshal(config, &dexCfg)
			require.NoError(t, err)

			storage, ok := dexCfg["storage"].(map[string]any)
			require.True(t, ok)

			assert.Equal(t, tc.wantType, storage["type"])

			if tc.validate != nil {
				tc.validate(t, storage)
			}
		})
	}
}

func Test_GenerateDexConfig(t *testing.T) {
	t.Run("Empty settings", func(t *testing.T) {
		s := settings.ArgoCDSettings{}
		config, err := GenerateDexConfigYAML(&s, false)
		require.NoError(t, err)
		assert.Nil(t, config)
	})

	t.Run("Invalid URL", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       invalidURL,
			DexConfig: goodDexConfig,
		}
		config, err := GenerateDexConfigYAML(&s, false)
		require.Error(t, err)
		assert.Nil(t, config)
	})

	t.Run("No URL set", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "",
			DexConfig: "invalidyaml",
		}
		config, err := GenerateDexConfigYAML(&s, false)
		require.NoError(t, err)
		assert.Nil(t, config)
	})

	t.Run("Invalid YAML", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "http://localhost",
			DexConfig: "invalidyaml",
		}
		config, err := GenerateDexConfigYAML(&s, false)
		require.NoError(t, err)
		assert.Nil(t, config)
	})

	t.Run("Valid YAML but incorrect Dex config", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "http://localhost",
			DexConfig: malformedDexConfig,
		}
		config, err := GenerateDexConfigYAML(&s, false)
		require.Error(t, err)
		assert.Nil(t, config)
	})

	t.Run("Valid YAML but incorrect Dex config", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "http://localhost",
			DexConfig: badDexConfig,
		}
		config, err := GenerateDexConfigYAML(&s, false)
		require.Error(t, err)
		assert.Nil(t, config)
	})

	t.Run("Valid YAML and correct Dex config", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "http://localhost",
			DexConfig: goodDexConfig,
		}
		config, err := GenerateDexConfigYAML(&s, false)
		require.NoError(t, err)
		assert.NotNil(t, config)
	})

	t.Run("Secret dereference", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "http://localhost",
			DexConfig: goodDexConfig,
			Secrets:   goodSecrets,
		}
		config, err := GenerateDexConfigYAML(&s, false)
		require.NoError(t, err)
		assert.NotNil(t, config)
		var dexCfg map[string]any
		err = yaml.Unmarshal(config, &dexCfg)
		if err != nil {
			panic(err.Error())
		}
		connectors, ok := dexCfg["connectors"].([]any)
		assert.True(t, ok)
		for i, connectorsIf := range connectors {
			config := connectorsIf.(map[string]any)["config"].(map[string]any)
			switch i {
			case 0:
				assert.Equal(t, "foobar", config["clientSecret"])
			case 1:
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
		config, err := GenerateDexConfigYAML(&s, false)
		require.NoError(t, err)
		assert.NotNil(t, config)
		var dexCfg map[string]any
		err = yaml.Unmarshal(config, &dexCfg)
		if err != nil {
			panic(err.Error())
		}
		connectors, ok := dexCfg["connectors"].([]any)
		assert.True(t, ok)
		for i, connectorsIf := range connectors {
			config := connectorsIf.(map[string]any)["config"].(map[string]any)
			switch i {
			case 0:
				assert.Equal(t, "foobar", config["clientSecret"])
			case 1:
				assert.Equal(t, "barfoo", config["clientSecret"])
			}
		}
	})

	t.Run("Logging level", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "http://localhost",
			DexConfig: goodDexConfig,
		}
		t.Setenv(common.EnvLogLevel, log.WarnLevel.String())
		t.Setenv(common.EnvLogFormat, utillog.JsonFormat)

		config, err := GenerateDexConfigYAML(&s, false)
		require.NoError(t, err)
		assert.NotNil(t, config)
		var dexCfg map[string]any
		err = yaml.Unmarshal(config, &dexCfg)
		if err != nil {
			panic(err.Error())
		}
		loggerCfg, ok := dexCfg["logger"].(map[string]any)
		assert.True(t, ok)

		level, ok := loggerCfg["level"].(string)
		assert.True(t, ok)
		assert.Equal(t, "WARN", level)

		format, ok := loggerCfg["format"].(string)
		assert.True(t, ok)
		assert.Equal(t, "json", format)
	})

	t.Run("Logging level with config", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "http://localhost",
			DexConfig: goodDexConfigWithLogger,
		}
		t.Setenv(common.EnvLogLevel, log.WarnLevel.String())
		t.Setenv(common.EnvLogFormat, utillog.JsonFormat)

		config, err := GenerateDexConfigYAML(&s, false)
		require.NoError(t, err)
		assert.NotNil(t, config)
		var dexCfg map[string]any
		err = yaml.Unmarshal(config, &dexCfg)
		if err != nil {
			panic(err.Error())
		}
		loggerCfg, ok := dexCfg["logger"].(map[string]any)
		assert.True(t, ok)

		level, ok := loggerCfg["level"].(string)
		assert.True(t, ok)
		assert.Equal(t, "debug", level)

		format, ok := loggerCfg["format"].(string)
		assert.True(t, ok)
		assert.Equal(t, "json", format)

		_, ok = loggerCfg["other"].(string)
		assert.True(t, ok)
	})

	t.Run("Redirect config", func(t *testing.T) {
		types := []string{"oidc", "saml", "microsoft", "linkedin", "gitlab", "github", "bitbucket-cloud", "openshift", "gitea", "google", "oauth"}
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
		config, err := GenerateDexConfigYAML(&s, false)
		require.NoError(t, err)
		assert.NotNil(t, config)
		var dexCfg map[string]any
		err = yaml.Unmarshal(config, &dexCfg)
		if err != nil {
			panic(err.Error())
		}
		clients, ok := dexCfg["staticClients"].([]any)
		assert.True(t, ok)
		assert.Len(t, clients, 4)

		customClient := clients[3].(map[string]any)
		assert.Equal(t, "argo-workflow", customClient["id"].(string))
		assert.Len(t, customClient["redirectURIs"].([]any), 1)
	})
	t.Run("Custom static clients secret dereference with trailing CRLF", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "http://localhost",
			DexConfig: customStaticClientDexConfig,
			Secrets:   goodSecretswithCRLF,
		}
		config, err := GenerateDexConfigYAML(&s, false)
		require.NoError(t, err)
		assert.NotNil(t, config)
		var dexCfg map[string]any
		err = yaml.Unmarshal(config, &dexCfg)
		if err != nil {
			panic(err.Error())
		}
		clients, ok := dexCfg["staticClients"].([]any)
		assert.True(t, ok)
		assert.Len(t, clients, 4)

		customClient := clients[3].(map[string]any)
		assert.Equal(t, "barfoo", customClient["secret"])
	})
	t.Run("Override dex oauth2 configuration", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "http://localhost",
			DexConfig: goodDexConfigWithOauthOverrides,
		}
		config, err := GenerateDexConfigYAML(&s, false)
		require.NoError(t, err)
		assert.NotNil(t, config)
		var dexCfg map[string]any
		err = yaml.Unmarshal(config, &dexCfg)
		if err != nil {
			panic(err.Error())
		}
		oauth2Config, ok := dexCfg["oauth2"].(map[string]any)
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
		config, err := GenerateDexConfigYAML(&s, false)
		require.NoError(t, err)
		assert.NotNil(t, config)
		var dexCfg map[string]any
		err = yaml.Unmarshal(config, &dexCfg)
		if err != nil {
			panic(err.Error())
		}
		oauth2Config, ok := dexCfg["oauth2"].(map[string]any)
		assert.True(t, ok)
		pwConn, ok := oauth2Config["passwordConnector"].(string)
		assert.True(t, ok)
		assert.Equal(t, "ldap", pwConn)

		skipApprScr, ok := oauth2Config["skipApprovalScreen"].(bool)
		assert.True(t, ok)
		assert.False(t, skipApprScr)
	})
}

func Test_GenerateDexConfigYAML(t *testing.T) {
	type testData struct {
		name      string
		secrets   map[string]string
		dexConfig string
		field     string
		expected  string
	}

	tt := []testData{
		{
			name:      "LDAP bindPW with dollar sign is escaped for Dex env expansion",
			secrets:   map[string]string{"dex.ldap.bindPW": "test$test"},
			dexConfig: goodDexConfigLDAPWithDollarSign,
			expected:  "test$$test",
			field:     "bindPW",
		},
		{
			name:      "LDAP bindPW without dollar sign is unaffected",
			secrets:   map[string]string{"dex.ldap.bindPW": "plainpassword"},
			dexConfig: goodDexConfigLDAPWithDollarSign,
			expected:  "plainpassword",
			field:     "bindPW",
		},
		{
			name:      "LDAP bindPW with multiple dollar signs all escaped",
			secrets:   map[string]string{"dex.ldap.bindPW": "a$b$c$d"},
			dexConfig: goodDexConfigLDAPWithDollarSign,
			expected:  "a$$b$$c$$d",
			field:     "bindPW",
		},
		{
			name:      "literal with multiple dollar signs in bindPW (no secret reference) is escaped",
			secrets:   map[string]string{},
			dexConfig: goodDexConfigPlainPasswordMultipleDollarSigns,
			expected:  "a$$b$$c$$d",
			field:     "bindPW",
		},
		{
			name:      "literal dollar sign in bindPW (no secret reference) is escaped",
			secrets:   map[string]string{},
			dexConfig: goodDexConfigPasswordWithDollarSign,
			expected:  "test$$test",
			field:     "bindPW",
		},
		{
			name:      "literal plain password in bindPW (no secret reference) is unaffected",
			secrets:   map[string]string{},
			dexConfig: goodDexConfigPlainPassword,
			expected:  "plainpassword",
			field:     "bindPW",
		},
		{
			name:      "OIDC clientSecret with dollar sign is escaped",
			secrets:   map[string]string{"dex.oidc.clientSecret": "oidc$secret"},
			dexConfig: goodDexConfigOIDCWithDollarSign,
			field:     "clientSecret",
			expected:  "oidc$$secret",
		},
		{
			name:      "GitHub clientSecret with dollar sign is escaped",
			secrets:   map[string]string{"dex.github.clientSecret": "gh_$token"},
			dexConfig: goodDexConfigGitHubWithDollarSign,
			field:     "clientSecret",
			expected:  "gh_$$token",
		},
		{
			name:      "password with special characters but no dollar sign is not modified",
			secrets:   map[string]string{"dex.ldap.bindPW": `p@ssw0rd!#%^&*()[]{};:,.<>?/~` + "`" + `\n`},
			dexConfig: goodDexConfigLDAPWithDollarSign,
			field:     "bindPW",
			expected:  `p@ssw0rd!#%^&*()[]{};:,.<>?/~` + "`" + `\n`,
		},
		{
			name:      "password with ${VAR} form dollar sign is also escaped",
			secrets:   map[string]string{"dex.ldap.bindPW": "pass${WORD}end"},
			dexConfig: goodDexConfigLDAPWithDollarSign,
			field:     "bindPW",
			expected:  "pass$${WORD}end",
		},
		{
			name:      "password with unicode characters is not modified",
			secrets:   map[string]string{"dex.ldap.bindPW": "pässwörð£€"},
			dexConfig: goodDexConfigLDAPWithDollarSign,
			field:     "bindPW",
			expected:  "pässwörð£€",
		},
		{
			name:      "unresolved environment variable reference is NOT escaped (allows Dex env expansion)",
			secrets:   map[string]string{},
			dexConfig: goodDexConfigOIDCWithEnvVarReference,
			field:     "clientID",
			expected:  "$AUTHENTIK_CLIENT_ID",
		},
		{
			name:      "unresolved clientSecret environment variable reference is NOT escaped",
			secrets:   map[string]string{},
			dexConfig: goodDexConfigOIDCWithEnvVarReference,
			field:     "clientSecret",
			expected:  "$AUTHENTIK_CLIENT_SECRET",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			config, err := GenerateDexConfigYAML(argoCDSettings(tc.dexConfig, tc.secrets), false)
			require.NoError(t, err)
			require.NotNil(t, config)

			var dexCfg map[string]any
			require.NoError(t, yaml.Unmarshal(config, &dexCfg))

			connectors := dexCfg["connectors"].([]any)
			connCfg := connectors[0].(map[string]any)["config"].(map[string]any)
			assert.Equal(t, tc.expected, connCfg[tc.field])
		})
	}

	t.Run("top-level issuer is NOT escaped even if it contained a dollar sign", func(t *testing.T) {
		config, err := GenerateDexConfigYAML(argoCDSettings(goodDexConfigLDAPWithDollarSign,
			map[string]string{"dex.ldap.bindPW": "test$test"}), false)
		require.NoError(t, err)
		var dexCfg map[string]any
		require.NoError(t, yaml.Unmarshal(config, &dexCfg))
		assert.NotContains(t, dexCfg["issuer"].(string), "$$")
	})
}

func argoCDSettings(dexConfig string, secrets map[string]string) *settings.ArgoCDSettings {
	return &settings.ArgoCDSettings{
		URL:       "http://localhost",
		DexConfig: dexConfig,
		Secrets:   secrets,
	}
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
		server := httptest.NewServer(http.HandlerFunc(NewDexHTTPReverseProxy(fakeDex.URL, "/", nil)))
		fmt.Printf("Fake API Server listening on %s\n", server.URL)
		defer server.Close()
		target, _ := url.Parse(fakeDex.URL)
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, http.NoBody)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		assert.NotNil(t, resp)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, host, target.Host)
		fmt.Printf("%s\n", resp.Status)
		require.NoError(t, resp.Body.Close())
	})

	t.Run("Bad case", func(t *testing.T) {
		fakeDex := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
			rw.WriteHeader(http.StatusInternalServerError)
		}))
		defer fakeDex.Close()
		fmt.Printf("Fake Dex listening on %s\n", fakeDex.URL)
		server := httptest.NewServer(http.HandlerFunc(NewDexHTTPReverseProxy(fakeDex.URL, "/", nil)))
		fmt.Printf("Fake API Server listening on %s\n", server.URL)
		defer server.Close()
		client := &http.Client{
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, http.NoBody)
		require.NoError(t, err)
		resp, err := client.Do(req)
		assert.NotNil(t, resp)
		require.NoError(t, err)
		assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
		location, _ := resp.Location()
		fmt.Printf("%s %s\n", resp.Status, location.RequestURI())
		assert.True(t, strings.HasPrefix(location.RequestURI(), "/login?has_sso_error=true"))
		assert.NoError(t, resp.Body.Close())
	})

	t.Run("Invalid URL for Dex reverse proxy", func(t *testing.T) {
		// Can't test for now, since it would call exit
		t.Skip()
		f := NewDexHTTPReverseProxy(invalidURL, "/", nil)
		assert.Nil(t, f)
	})

	t.Run("Round Tripper", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
			assert.Equal(t, "/", req.URL.String())
		}))
		defer server.Close()
		rt := NewDexRewriteURLRoundTripper(server.URL, http.DefaultTransport)
		assert.NotNil(t, rt)
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "/", bytes.NewBuffer([]byte("")))
		require.NoError(t, err)
		_, err = rt.RoundTrip(req)
		require.NoError(t, err)
		target, _ := url.Parse(server.URL)
		assert.Equal(t, req.Host, target.Host)
		require.NoError(t, req.Body.Close())
	})
}

func Test_GenerateDexConfigYAML_WebTLSMinVersion(t *testing.T) {
	tests := []struct {
		name           string
		dexConfig      string
		expectMin      string
		expectMinFound bool
	}{
		{
			name: "TLS 1.2",
			dexConfig: `
connectors: []
web:
  tlsMinVersion: "1.2"
`,
			expectMin:      "1.2",
			expectMinFound: true,
		},
		{
			name: "TLS 1.3",
			dexConfig: `
connectors: []
web:
  tlsMinVersion: "1.3"
`,
			expectMin:      "1.3",
			expectMinFound: true,
		},
		{
			name: "empty TLS version",
			dexConfig: `
connectors: []
web:
  tlsMinVersion: ""
`,
			expectMinFound: false,
		},
		{
			name: "no web section",
			dexConfig: `
connectors: []
`,
			expectMinFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &settings.ArgoCDSettings{
				URL:       "https://argocd.example.com",
				DexConfig: tt.dexConfig,
			}
			out, err := GenerateDexConfigYAML(settings, false)
			require.NoError(t, err)
			var cfg map[string]any
			require.NoError(t, yaml.Unmarshal(out, &cfg))
			webCfg, ok := cfg["web"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "0.0.0.0:5556", webCfg["https"])
			assert.Equal(t, "/tmp/tls.crt", webCfg["tlsCert"])
			assert.Equal(t, "/tmp/tls.key", webCfg["tlsKey"])
			got, found := webCfg["tlsMinVersion"]
			assert.Equal(t, tt.expectMinFound, found)
			if tt.expectMinFound {
				assert.Equal(t, tt.expectMin, got)
			}
		})
	}
}
