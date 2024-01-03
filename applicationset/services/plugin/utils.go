package plugin

import (
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/v2/common"
)

// ParseSecretKey retrieves secret appSetName if different from common ArgoCDSecretName.
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
