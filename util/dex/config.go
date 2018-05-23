package dex

import (
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util/settings"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
)

func GenerateDexConfigYAML(settings *settings.ArgoCDSettings) ([]byte, error) {
	if !settings.IsSSOConfigured() {
		return nil, nil
	}
	var dexCfg map[string]interface{}
	err := yaml.Unmarshal([]byte(settings.DexConfig), &dexCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal dex.config from configmap: %v", err)
	}
	dexCfg["issuer"] = settings.IssuerURL()
	dexCfg["storage"] = map[string]interface{}{
		"type": "memory",
	}
	dexCfg["web"] = map[string]interface{}{
		"http": "0.0.0.0:5556",
	}
	dexCfg["grpc"] = map[string]interface{}{
		"addr": "0.0.0.0:5557",
	}
	dexCfg["oauth2"] = map[string]interface{}{
		"skipApprovalScreen": true,
	}
	dexCfg["staticClients"] = []map[string]interface{}{
		{
			"id":     common.ArgoCDClientAppID,
			"name":   common.ArgoCDClientAppName,
			"secret": settings.OAuth2ClientSecret(),
			"redirectURIs": []string{
				settings.RedirectURL(),
			},
		},
		{
			"id":     common.ArgoCDCLIClientAppID,
			"name":   common.ArgoCDCLIClientAppName,
			"public": true,
			"redirectURIs": []string{
				"http://localhost",
			},
		},
	}
	connectors := dexCfg["connectors"].([]interface{})
	for i, connectorIf := range connectors {
		connector := connectorIf.(map[string]interface{})
		connectorType := connector["type"].(string)
		if !needsRedirectURI(connectorType) {
			continue
		}
		connectorCfg := connector["config"].(map[string]interface{})
		connectorCfg["redirectURI"] = settings.URL + "/api/dex/callback"
		connector["config"] = connectorCfg
		connectors[i] = connector
	}
	dexCfg["connectors"] = connectors
	dexCfg = replaceMapSecrets(dexCfg, settings.Secrets)
	return yaml.Marshal(dexCfg)
}

// replaceMapSecrets takes a json object and recursively looks for any secret key references in the
// object and replaces the value with the secret value
func replaceMapSecrets(obj map[string]interface{}, secretValues map[string]string) map[string]interface{} {
	newObj := make(map[string]interface{})
	for k, v := range obj {
		switch val := v.(type) {
		case map[string]interface{}:
			newObj[k] = replaceMapSecrets(val, secretValues)
		case []interface{}:
			newObj[k] = replaceListSecrets(val, secretValues)
		case string:
			newObj[k] = replaceStringSecret(val, secretValues)
		default:
			newObj[k] = val
		}
	}
	return newObj
}

func replaceListSecrets(obj []interface{}, secretValues map[string]string) []interface{} {
	newObj := make([]interface{}, len(obj))
	for i, v := range obj {
		switch val := v.(type) {
		case map[string]interface{}:
			newObj[i] = replaceMapSecrets(val, secretValues)
		case []interface{}:
			newObj[i] = replaceListSecrets(val, secretValues)
		case string:
			newObj[i] = replaceStringSecret(val, secretValues)
		default:
			newObj[i] = val
		}
	}
	return newObj
}

func replaceStringSecret(val string, secretValues map[string]string) string {
	if val == "" || !strings.HasPrefix(val, "$") {
		return val
	}
	secretKey := val[1:]
	secretVal, ok := secretValues[secretKey]
	if !ok {
		log.Warnf("config referenced '%s', but key does not exist in secret", val)
		return val
	}
	return secretVal
}

// needsRedirectURI returns whether or not the given connector type needs a redirectURI
// Update this list as necessary, as new connectors are added
// https://github.com/coreos/dex/tree/master/Documentation/connectors
func needsRedirectURI(connectorType string) bool {
	switch connectorType {
	case "oidc", "saml", "microsoft", "linkedin", "gitlab", "github":
		return true
	}
	return false
}
