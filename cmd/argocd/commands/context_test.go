package commands

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/util/localconfig"
)

const testConfig = `contexts:
- name: argocd.example.com:443
  server: argocd.example.com:443
  user: argocd.example.com:443
- name: localhost:8080
  server: localhost:8080
  user: localhost:8080
current-context: localhost:8080
servers:
- server: argocd.example.com:443
- plain-text: true
  server: localhost:8080
users:
- auth-token: vErrYS3c3tReFRe$hToken
  name: argocd.example.com:443
  refresh-token: vErrYS3c3tReFRe$hToken
- auth-token: vErrYS3c3tReFRe$hToken
  name: localhost:8080`

const testConfigFilePath = "./testdata/config"

func TestContextDelete(t *testing.T) {

	// Write the test config file
	err := ioutil.WriteFile(testConfigFilePath, []byte(testConfig), os.ModePerm)
	assert.NoError(t, err)

	localConfig, err := localconfig.ReadLocalConfig(testConfigFilePath)
	assert.NoError(t, err)
	assert.Equal(t, localConfig.CurrentContext, "localhost:8080")
	assert.Contains(t, localConfig.Contexts, localconfig.ContextRef{Name: "localhost:8080", Server: "localhost:8080", User: "localhost:8080"})

	err = deleteContext("localhost:8080", testConfigFilePath)
	assert.NoError(t, err)

	localConfig, err = localconfig.ReadLocalConfig(testConfigFilePath)
	assert.NoError(t, err)
	assert.Equal(t, localConfig.CurrentContext, "argocd.example.com:443")
	assert.NotContains(t, localConfig.Contexts, localconfig.ContextRef{Name: "localhost:8080", Server: "localhost:8080", User: "localhost:8080"})
	assert.NotContains(t, localConfig.Servers, localconfig.Server{PlainText: true, Server: "localhost:8080"})
	assert.NotContains(t, localConfig.Users, localconfig.User{AuthToken: "vErrYS3c3tReFRe$hToken", Name: "localhost:8080"})
	assert.Contains(t, localConfig.Contexts, localconfig.ContextRef{Name: "argocd.example.com:443", Server: "argocd.example.com:443", User: "argocd.example.com:443"})

	// Write the file again so that no conflicts are made in git
	err = ioutil.WriteFile(testConfigFilePath, []byte(testConfig), os.ModePerm)
	assert.NoError(t, err)

}
