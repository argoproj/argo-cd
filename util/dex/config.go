package dex

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util/settings"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// ArgoCDClientAppName is name of the Oauth client app used when registering our web app to dex
	ArgoCDClientAppName = "ArgoCD"
	// ArgoCDClientAppID is the Oauth client ID we will use when registering our app to dex
	ArgoCDClientAppID = "argo-cd"
	// ArgoCDCLIClientAppName is name of the Oauth client app used when registering our CLI to dex
	ArgoCDCLIClientAppName = "ArgoCD CLI"
	// ArgoCDCLIClientAppID is the Oauth client ID we will use when registering our CLI to dex
	ArgoCDCLIClientAppID = "argo-cd-cli"
)

func GenerateDexConfigYAML(kubeClientset kubernetes.Interface, namespace string) ([]byte, error) {
	settingsMgr := settings.NewSettingsManager(kubeClientset, namespace)
	settings, err := settingsMgr.GetSettings()
	if err != nil {
		return nil, err
	}
	if !settings.IsSSOConfigured() {
		return nil, nil
	}
	var dexCfg map[string]interface{}
	err = yaml.Unmarshal([]byte(settings.DexConfig), &dexCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal dex.config from configmap: %v", err)
	}
	dexCfg["issuer"] = settings.URL + DexAPIEndpoint
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
			"id":     ArgoCDClientAppID,
			"name":   ArgoCDClientAppName,
			"secret": formulateOAuthClientSecret(settings.ServerSignature),
			"redirectURIs": []string{
				settings.URL + CallbackEndpoint,
			},
		},
		{
			"id":     ArgoCDCLIClientAppID,
			"name":   ArgoCDCLIClientAppName,
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

	secretValues, err := getSecretValues(kubeClientset, namespace)
	if err != nil {
		return nil, err
	}
	dexCfg = replaceMapSecrets(dexCfg, secretValues)
	return yaml.Marshal(dexCfg)
}

// formulateOAuthClientSecret calculates an arbitrary, but predictable OAuth2 client secret string
// derived some seed input (typically the server secret). This is called by the dex startup wrapper
// (argocd-util rundex), as well as the API server, such that they both independently come to the
// same conclusion of what the OAuth2 shared client secret should be.
func formulateOAuthClientSecret(in []byte) string {
	h := sha256.New()
	_, err := h.Write(in)
	if err != nil {
		panic(err)
	}
	sha := h.Sum(nil)
	return base64.URLEncoding.EncodeToString(sha)[:40]
}

// getSecretValues is a convenience to get the ArgoCD secret data as a map[string]string
func getSecretValues(kubeClientset kubernetes.Interface, namespace string) (map[string]string, error) {
	sec, err := kubeClientset.CoreV1().Secrets(namespace).Get(common.ArgoCDSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	secretValues := make(map[string]string, len(sec.Data))
	for k, v := range sec.Data {
		secretValues[k] = string(v)
	}
	return secretValues, nil
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
