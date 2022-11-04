package image

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	argoexec "github.com/argoproj/pkg/exec"

	"github.com/argoproj/argo-cd/v2/image-updater/kube"
	"github.com/argoproj/argo-cd/v2/image-updater/log"
)

type CredentialSourceType int

const (
	CredentialSourceUnknown    CredentialSourceType = 0
	CredentialSourcePullSecret CredentialSourceType = 1
	CredentialSourceSecret     CredentialSourceType = 2
	CredentialSourceEnv        CredentialSourceType = 3
	CredentialSourceExt        CredentialSourceType = 4
)

type CredentialSource struct {
	Type            CredentialSourceType
	Registry        string
	SecretNamespace string
	SecretName      string
	SecretField     string
	EnvName         string
	ScriptPath      string
}

type Credential struct {
	Username string
	Password string
}

const pullSecretField = ".dockerconfigjson"

// gcr.io=secret:foo/bar#baz
// gcr.io=pullsecret:foo/bar
// gcr.io=env:FOOBAR

func ParseCredentialSource(credentialSource string, requirePrefix bool) (*CredentialSource, error) {
	src := CredentialSource{}
	var secretDef string
	tokens := strings.SplitN(credentialSource, "=", 2)
	if len(tokens) != 2 || tokens[0] == "" || tokens[1] == "" {
		if requirePrefix {
			return nil, fmt.Errorf("invalid credential spec: %s", credentialSource)
		}
		secretDef = credentialSource
	} else {
		src.Registry = tokens[0]
		secretDef = tokens[1]
	}

	tokens = strings.Split(secretDef, ":")
	if len(tokens) != 2 || tokens[0] == "" || tokens[1] == "" {
		return nil, fmt.Errorf("invalid credential spec: %s", credentialSource)
	}

	var err error
	switch strings.ToLower(tokens[0]) {
	case "secret":
		err = src.parseSecretDefinition(tokens[1])
		src.Type = CredentialSourceSecret
	case "pullsecret":
		err = src.parsePullSecretDefinition(tokens[1])
		src.Type = CredentialSourcePullSecret
	case "env":
		err = src.parseEnvDefinition(tokens[1])
		src.Type = CredentialSourceEnv
	case "ext":
		err = src.parseExtDefinition(tokens[1])
		src.Type = CredentialSourceExt
	default:
		err = fmt.Errorf("unknown credential source: %s", tokens[0])
	}

	if err != nil {
		return nil, err
	}

	return &src, nil
}

// FetchCredentials fetches the credentials for a given registry according to
// the credential source.
func (src *CredentialSource) FetchCredentials(registryURL string, kubeclient *kube.KubernetesClient) (*Credential, error) {
	var creds Credential
	log.Tracef("Fetching credentials for registry %s", registryURL)
	switch src.Type {
	case CredentialSourceEnv:
		credEnv := os.Getenv(src.EnvName)
		if credEnv == "" {
			return nil, fmt.Errorf("could not fetch credentials: env '%s' is not set", src.EnvName)
		}
		tokens := strings.SplitN(credEnv, ":", 2)
		if len(tokens) != 2 || tokens[0] == "" || tokens[1] == "" {
			return nil, fmt.Errorf("could not fetch credentials: value of %s is malformed", src.EnvName)
		}
		creds.Username = tokens[0]
		creds.Password = tokens[1]
		return &creds, nil
	case CredentialSourceSecret:
		if kubeclient == nil {
			return nil, fmt.Errorf("could not fetch credentials: no Kubernetes client given")
		}
		data, err := kubeclient.GetSecretField(src.SecretNamespace, src.SecretName, src.SecretField)
		if err != nil {
			return nil, fmt.Errorf("could not fetch secret '%s' from namespace '%s' (field: '%s'): %v", src.SecretName, src.SecretNamespace, src.SecretField, err)
		}
		tokens := strings.SplitN(data, ":", 2)
		if len(tokens) != 2 {
			return nil, fmt.Errorf("invalid credentials in secret '%s' from namespace '%s' (field '%s')", src.SecretName, src.SecretNamespace, src.SecretField)
		}
		creds.Username = tokens[0]
		creds.Password = tokens[1]
		return &creds, nil
	case CredentialSourcePullSecret:
		if kubeclient == nil {
			return nil, fmt.Errorf("could not fetch credentials: no Kubernetes client given")
		}
		src.SecretField = pullSecretField
		data, err := kubeclient.GetSecretField(src.SecretNamespace, src.SecretName, src.SecretField)
		if err != nil {
			return nil, fmt.Errorf("could not fetch secret '%s' from namespace '%s' (field: '%s'): %v", src.SecretName, src.SecretNamespace, src.SecretField, err)
		}
		creds.Username, creds.Password, err = parseDockerConfigJson(registryURL, data)
		if err != nil {
			return nil, err
		}
		return &creds, nil
	case CredentialSourceExt:
		if !strings.HasPrefix(src.ScriptPath, "/") {
			return nil, fmt.Errorf("path to script must be absolute, but is '%s'", src.ScriptPath)
		}
		_, err := os.Stat(src.ScriptPath)
		if err != nil {
			return nil, fmt.Errorf("could not stat %s: %v", src.ScriptPath, err)
		}
		cmd := exec.Command(src.ScriptPath)
		out, err := argoexec.RunCommandExt(cmd, argoexec.CmdOpts{Timeout: 10 * time.Second})
		if err != nil {
			return nil, fmt.Errorf("error executing %s: %v", src.ScriptPath, err)
		}
		tokens := strings.SplitN(out, ":", 2)
		if len(tokens) != 2 {
			return nil, fmt.Errorf("invalid script output, must be single line with syntax <username>:<password>")
		}
		creds.Username = tokens[0]
		creds.Password = tokens[1]
		return &creds, nil

	default:
		return nil, fmt.Errorf("unknown credential type")
	}
}

// Parse a secret definition in form of 'namespace/name#field'
func (src *CredentialSource) parseSecretDefinition(definition string) error {
	tokens := strings.Split(definition, "#")
	if len(tokens) != 2 || tokens[0] == "" || tokens[1] == "" {
		return fmt.Errorf("invalid secret definition: %s", definition)
	}
	src.SecretField = tokens[1]
	tokens = strings.Split(tokens[0], "/")
	if len(tokens) != 2 || tokens[0] == "" || tokens[1] == "" {
		return fmt.Errorf("invalid secret definition: %s", definition)
	}
	src.SecretNamespace = tokens[0]
	src.SecretName = tokens[1]

	return nil
}

// Parse an image pull secret definition in form of 'namespace/name'
func (src *CredentialSource) parsePullSecretDefinition(definition string) error {
	tokens := strings.Split(definition, "/")
	if len(tokens) != 2 || tokens[0] == "" || tokens[1] == "" {
		return fmt.Errorf("invalid secret definition: %s", definition)
	}

	src.SecretNamespace = tokens[0]
	src.SecretName = tokens[1]
	src.SecretField = pullSecretField

	return nil
}

// Parse an environment definition
// nolint:unparam
func (src *CredentialSource) parseEnvDefinition(definition string) error {
	src.EnvName = definition
	return nil
}

// Parse an external script definition
// nolint:unparam
func (src *CredentialSource) parseExtDefinition(definition string) error {
	src.ScriptPath = definition
	return nil
}

// This unmarshals & parses Docker's config.json file, returning username and
// password for given registry URL
func parseDockerConfigJson(registryURL string, jsonSource string) (string, string, error) {
	var dockerConf map[string]interface{}
	err := json.Unmarshal([]byte(jsonSource), &dockerConf)
	if err != nil {
		return "", "", err
	}
	auths, ok := dockerConf["auths"].(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("no credentials in image pull secret")
	}

	var regPrefix string
	if strings.HasPrefix(registryURL, "http://") {
		regPrefix = strings.TrimPrefix(registryURL, "http://")
	} else if strings.HasPrefix(registryURL, "https://") {
		regPrefix = strings.TrimPrefix(registryURL, "https://")
	} else {
		regPrefix = registryURL
	}

	regPrefix = strings.TrimSuffix(regPrefix, "/")

	for registry, authConf := range auths {
		if !strings.HasPrefix(registry, registryURL) && !strings.HasPrefix(registry, regPrefix) {
			log.Tracef("found registry %s in image pull secret, but we want %s (%s) - skipping", registry, registryURL, regPrefix)
			continue
		}
		authEntry, ok := authConf.(map[string]interface{})
		if !ok {
			return "", "", fmt.Errorf("invalid auth entry for registry entry %s ('auths' entry should be map)", registry)
		}
		authString, ok := authEntry["auth"].(string)
		if !ok {
			return "", "", fmt.Errorf("invalid auth token for registry entry %s ('auth' should be string')", registry)
		}
		authToken, err := base64.StdEncoding.DecodeString(authString)
		if err != nil {
			return "", "", fmt.Errorf("could not base64-decode auth data for registry entry %s: %v", registry, err)
		}
		tokens := strings.SplitN(string(authToken), ":", 2)
		if len(tokens) != 2 {
			return "", "", fmt.Errorf("invalid data after base64 decoding auth entry for registry entry %s", registry)
		}

		return tokens[0], tokens[1], nil
	}

	return "", "", fmt.Errorf("no valid auth entry for registry %s found in image pull secret", registryURL)
}
