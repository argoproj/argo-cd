package commands

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/argoproj/argo-cd/pkg/apiclient"

	"github.com/argoproj/argo-cd/util/localconfig"
	"github.com/stretchr/testify/assert"
)

func TestLogout(t *testing.T) {

	// Write the test config file
	err := ioutil.WriteFile(testConfigFilePath, []byte(testConfig), os.ModePerm)
	assert.NoError(t, err)

	localConfig, err := localconfig.ReadLocalConfig(testConfigFilePath)
	assert.NoError(t, err)
	assert.Equal(t, localConfig.CurrentContext, "localhost:8080")
	assert.Contains(t, localConfig.Contexts, localconfig.ContextRef{Name: "localhost:8080", Server: "localhost:8080", User: "localhost:8080"})

	command := NewLogoutCommand(&apiclient.ClientOptions{ConfigPath: testConfigFilePath})
	command.Run(nil, []string{"localhost:8080"})

	localConfig, err = localconfig.ReadLocalConfig(testConfigFilePath)
	assert.NoError(t, err)
	assert.Equal(t, localConfig.CurrentContext, "localhost:8080")
	assert.NotContains(t, localConfig.Users, localconfig.User{AuthToken: "vErrYS3c3tReFRe$hToken", Name: "localhost:8080"})
	assert.Contains(t, localConfig.Contexts, localconfig.ContextRef{Name: "argocd.example.com:443", Server: "argocd.example.com:443", User: "argocd.example.com:443"})

	// Write the file again so that no conflicts are made in git
	err = ioutil.WriteFile(testConfigFilePath, []byte(testConfig), os.ModePerm)
	assert.NoError(t, err)

}
