package dex

import (
	"fmt"

	"github.com/ghodss/yaml"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util/settings"
)

func GenerateDexConfigYAML(settings *settings.ArgoCDSettings) ([]byte, error) {
	if !settings.IsDexConfigured() {
		return nil, nil
	}
	redirectURL, err := settings.RedirectURL()
	if err != nil {
		return nil, fmt.Errorf("failed to infer redirect url from config: %v", err)
	}
	var dexCfg map[string]interface{}
	err = yaml.Unmarshal([]byte(settings.DexConfig), &dexCfg)
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
	dexCfg["telemetry"] = map[string]interface{}{
		"http": "0.0.0.0:5558",
	}
	dexCfg["oauth2"] = map[string]interface{}{
		"skipApprovalScreen": true,
	}

	argoCDStaticClient := map[string]interface{}{
		"id":     common.ArgoCDClientAppID,
		"name":   common.ArgoCDClientAppName,
		"secret": settings.DexOAuth2ClientSecret(),
		"redirectURIs": []string{
			redirectURL,
		},
	}
	argoCDCLIStaticClient := map[string]interface{}{
		"id":     common.ArgoCDCLIClientAppID,
		"name":   common.ArgoCDCLIClientAppName,
		"public": true,
		"redirectURIs": []string{
			"http://localhost",
			"http://localhost:8085/auth/callback",
		},
	}

	staticClients, ok := dexCfg["staticClients"].([]interface{})
	if ok {
		dexCfg["staticClients"] = append([]interface{}{argoCDStaticClient, argoCDCLIStaticClient}, staticClients...)
	} else {
		dexCfg["staticClients"] = []interface{}{argoCDStaticClient, argoCDCLIStaticClient}
	}

	dexRedirectURL, err := settings.DexRedirectURL()
	if err != nil {
		return nil, err
	}
	connectors, ok := dexCfg["connectors"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("malformed Dex configuration found")
	}
	for i, connectorIf := range connectors {
		connector, ok := connectorIf.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("malformed Dex configuration found")
		}
		connectorType := connector["type"].(string)
		if !needsRedirectURI(connectorType) {
			continue
		}
		connectorCfg, ok := connector["config"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("malformed Dex configuration found")
		}
		connectorCfg["redirectURI"] = dexRedirectURL
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
			newObj[k] = settings.ReplaceStringSecret(val, secretValues)
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
			newObj[i] = settings.ReplaceStringSecret(val, secretValues)
		default:
			newObj[i] = val
		}
	}
	return newObj
}

// needsRedirectURI returns whether or not the given connector type needs a redirectURI
// Update this list as necessary, as new connectors are added
// https://dexidp.io/docs/connectors/
func needsRedirectURI(connectorType string) bool {
	switch connectorType {
	case "oidc", "saml", "microsoft", "linkedin", "gitlab", "github", "bitbucket-cloud", "openshift":
		return true
	}
	return false
}
