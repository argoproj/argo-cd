package commands

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/util/localconfig"
)

func TestLogout(t *testing.T) {

	// Write the test config file
	err := os.WriteFile(testConfigFilePath, []byte(testConfig), os.ModePerm)
	assert.NoError(t, err)
	defer os.Remove(testConfigFilePath)

	err = os.Chmod(testConfigFilePath, 0600)
	require.NoError(t, err)

	localConfig, err := localconfig.ReadLocalConfig(testConfigFilePath)
	assert.NoError(t, err)
	assert.Equal(t, localConfig.CurrentContext, "localhost:8080")
	assert.Contains(t, localConfig.Contexts, localconfig.ContextRef{Name: "localhost:8080", Server: "localhost:8080", User: "localhost:8080"})

	command := NewLogoutCommand(&argocdclient.ClientOptions{ConfigPath: testConfigFilePath})
	command.Run(nil, []string{"localhost:8080"})

	localConfig, err = localconfig.ReadLocalConfig(testConfigFilePath)
	assert.NoError(t, err)
	assert.Equal(t, localConfig.CurrentContext, "localhost:8080")
	assert.NotContains(t, localConfig.Users, localconfig.User{AuthToken: "vErrYS3c3tReFRe$hToken", Name: "localhost:8080"})
	assert.Contains(t, localConfig.Contexts, localconfig.ContextRef{Name: "argocd1.example.com:443", Server: "argocd1.example.com:443", User: "argocd1.example.com:443"})
	assert.Contains(t, localConfig.Contexts, localconfig.ContextRef{Name: "argocd2.example.com:443", Server: "argocd2.example.com:443", User: "argocd2.example.com:443"})
	assert.Contains(t, localConfig.Contexts, localconfig.ContextRef{Name: "localhost:8080", Server: "localhost:8080", User: "localhost:8080"})
}
