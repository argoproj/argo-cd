package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/argoproj/argo-cd/v2/common"
	log "github.com/sirupsen/logrus"
)

type authTokenProvider struct {
	filePath string
}

func (a *authTokenProvider) getAuthToken() string {
	path := fmt.Sprintf("%s/%s", strings.TrimRight(a.filePath, "/"), common.PluginAuthSecretName)
	content, err := os.ReadFile(path)
	if err != nil {
		log.Errorf("No authentication secret present at %s", path)
		return ""
	}
	return strings.TrimSpace(string(content))
}
