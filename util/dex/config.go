package dex

import (
	"fmt"

	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

func GenerateDexConfigYAML(argocdSettings *settings.ArgoCDSettings, disableTls bool) ([]byte, error) {
	if !argocdSettings.IsDexConfigured() {
		return nil, nil
	}
	redirectURL, err := argocdSettings.RedirectURL()
	if err != nil {
		return nil, fmt.Errorf("failed to infer redirect url from config: %v", err)
	}
	var dexCfg map[string]interface{}
	err = yaml.Unmarshal([]byte(argocdSettings.DexConfig), &dexCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal dex.config from configmap: %v", err)
	}
	dexCfg["issuer"] = argocdSettings.IssuerURL()
	dexCfg["storage"] = map[string]interface{}{
		"type": "memory",
	}
	if disableTls {
		dexCfg["web"] = map[string]interface{}{
			"http": "0.0.0.0:5556",
		}
	} else {
		dexCfg["web"] = map[string]interface{}{
			"https":   "0.0.0.0:5556",
			"tlsCert": "/tmp/tls.crt",
			"tlsKey":  "/tmp/tls.key",
		}
	}

	dexCfg["grpc"] = map[string]interface{}{
		"addr": "0.0.0.0:5557",
	}
	dexCfg["telemetry"] = map[string]interface{}{
		"http": "0.0.0.0:5558",
	}

	if oauth2Cfg, found := dexCfg["oauth2"].(map[string]interface{}); found {
		if _, found := oauth2Cfg["skipApprovalScreen"].(bool); !found {
			oauth2Cfg["skipApprovalScreen"] = true
		}
	} else {
		dexCfg["oauth2"] = map[string]interface{}{
			"skipApprovalScreen": true,
		}
	}

	argoCDStaticClient := map[string]interface{}{
		"id":     common.ArgoCDClientAppID,
		"name":   common.ArgoCDClientAppName,
		"secret": argocdSettings.DexOAuth2ClientSecret(),
		"redirectURIs": []string{
			redirectURL,
		},
	}
	argoCDPKCEStaticClient := map[string]interface{}{
		"id":   "argo-cd-pkce",
		"name": "Argo CD PKCE",
		"redirectURIs": []string{
			"http://localhost:4000/pkce/verify",
		},
		"public": true,
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
		dexCfg["staticClients"] = append([]interface{}{argoCDStaticClient, argoCDCLIStaticClient, argoCDPKCEStaticClient}, staticClients...)
	} else {
		dexCfg["staticClients"] = []interface{}{argoCDStaticClient, argoCDCLIStaticClient, argoCDPKCEStaticClient}
	}

	dexRedirectURL, err := argocdSettings.DexRedirectURL()
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
	dexCfg = settings.ReplaceMapSecrets(dexCfg, argocdSettings.Secrets)
	return yaml.Marshal(dexCfg)
}

// needsRedirectURI returns whether or not the given connector type needs a redirectURI
// Update this list as necessary, as new connectors are added
// https://dexidp.io/docs/connectors/
func needsRedirectURI(connectorType string) bool {
	switch connectorType {
	case "oidc", "saml", "microsoft", "linkedin", "gitlab", "github", "bitbucket-cloud", "openshift", "gitea", "google", "oauth":
		return true
	}
	return false
}
