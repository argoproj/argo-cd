package plugin

import (
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/v2/common"
	log "github.com/sirupsen/logrus"
)

// ReplaceStringSecret checks if given string is a secret key reference ( starts with $ ) and returns corresponding value from provided map
func ReplaceStringSecret(val string, secretValues map[string]string) string {
	if val == "" || !strings.HasPrefix(val, "$") {
		return val
	}
	secretKey := val[1:]
	secretVal, ok := secretValues[secretKey]
	if !ok {
		log.Warnf("config referenced '%s', but key does not exist in secret", val)
		return val
	}

	return strings.TrimSpace(secretVal)
}

// Retrieve secret name if different from common ArgoCDSecretName.
func ParseSecretKey(key string) (secretName string, tokenKey string) {
	if strings.Contains(key, ":") {
		parts := strings.Split(key, ":")
		secretName = parts[0][1:]
		tokenKey = fmt.Sprintf("$%s", parts[1])
	} else {
		secretName = common.ArgoCDSecretName
		tokenKey = key
	}
	return secretName, tokenKey
}
