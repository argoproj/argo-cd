package dex

import (
	"testing"

	"github.com/stretchr/testify/assert"

	// "github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util/settings"
)

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
			URL:       ":://localhost/foo/bar",
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

	t.Run("Valid YAML and correct Dex config", func(t *testing.T) {
		s := settings.ArgoCDSettings{
			URL:       "http://localhost",
			DexConfig: goodDexConfig,
			Secrets:   goodSecrets,
		}
		config, err := GenerateDexConfigYAML(&s)
		assert.NoError(t, err)
		assert.NotNil(t, config)
	})

}
