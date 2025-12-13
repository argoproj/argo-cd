package commands

import (
	"os"
	"testing"

	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/localconfig"
)

const testConfig = `contexts:
- name: argocd1.example.com:443
  server: argocd1.example.com:443
  user: argocd1.example.com:443
- name: argocd2.example.com:443
  server: argocd2.example.com:443
  user: argocd2.example.com:443
- name: localhost:8080
  server: localhost:8080
  user: localhost:8080
current-context: localhost:8080
servers:
- server: argocd1.example.com:443
- server: argocd2.example.com:443
- plain-text: true
  server: localhost:8080
users:
- auth-token: vErrYS3c3tReFRe$hToken
  name: argocd1.example.com:443
  refresh-token: vErrYS3c3tReFRe$hToken
- auth-token: vErrYS3c3tReFRe$hToken
  name: argocd2.example.com:443
  refresh-token: vErrYS3c3tReFRe$hToken
- auth-token: vErrYS3c3tReFRe$hToken
  name: localhost:8080`

const testConfigFilePath = "./testdata/local.config"

func TestContextList(t *testing.T) {
	// Write the test config file
	err := os.WriteFile(testConfigFilePath, []byte(testConfig), os.ModePerm)
	require.NoError(t, err)
	defer os.Remove(testConfigFilePath)

	err = os.Chmod(testConfigFilePath, 0o600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)

	expected := "CURRENT  NAME                     SERVER\n         argocd1.example.com:443  argocd1.example.com:443\n         argocd2.example.com:443  argocd2.example.com:443\n*        localhost:8080           localhost:8080\n"
	output, err := captureOutput(func() error {
		printArgoCDContexts(testConfigFilePath)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, expected, output)
}

func TestContextUseUndefinedContext(t *testing.T) {
	// Write the test config file
	err := os.WriteFile(testConfigFilePath, []byte(testConfig), os.ModePerm)
	require.NoError(t, err)
	defer os.Remove(testConfigFilePath)

	err = os.Chmod(testConfigFilePath, 0o600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)

	output, err := captureOutput(func() error {
		clientOpts := &argocdclient.ClientOptions{ConfigPath: testConfigFilePath}

		useCmd := NewContextUseCommand(clientOpts)
		useCmd.SetArgs([]string{"undefined-context:8081"})
		err = useCmd.Execute()

		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)
	expected := "Context 'undefined-context:8081' undefined\n"
	assert.Equal(t, expected, output)
}

func TestContextUseCurrentContext(t *testing.T) {
	// Write the test config file
	err := os.WriteFile(testConfigFilePath, []byte(testConfig), os.ModePerm)
	require.NoError(t, err)
	defer os.Remove(testConfigFilePath)

	err = os.Chmod(testConfigFilePath, 0o600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)
	localConfig, err := localconfig.ReadLocalConfig(testConfigFilePath)
	require.NoError(t, err)
	// Assert current context is localhost
	assert.Equal(t, "localhost:8080", localConfig.CurrentContext)

	output, err := captureOutput(func() error {
		clientOpts := &argocdclient.ClientOptions{ConfigPath: testConfigFilePath}

		useCmd := NewContextUseCommand(clientOpts)
		useCmd.SetArgs([]string{"localhost:8080"})
		err = useCmd.Execute()

		require.NoError(t, err)
		assert.Equal(t, "localhost:8080", localConfig.CurrentContext)
		return nil
	})
	require.NoError(t, err)
	expected := "Already at context 'localhost:8080'\n"
	assert.Equal(t, expected, output)
}

func TestContextUseDifferentContext(t *testing.T) {
	// Write the test config file
	err := os.WriteFile(testConfigFilePath, []byte(testConfig), os.ModePerm)
	require.NoError(t, err)
	defer os.Remove(testConfigFilePath)

	err = os.Chmod(testConfigFilePath, 0o600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)
	localConfig, err := localconfig.ReadLocalConfig(testConfigFilePath)
	require.NoError(t, err)
	// Assert current context is localhost
	assert.Equal(t, "localhost:8080", localConfig.CurrentContext)

	output, err := captureOutput(func() error {
		clientOpts := &argocdclient.ClientOptions{ConfigPath: testConfigFilePath}

		useCmd := NewContextUseCommand(clientOpts)
		useCmd.SetArgs([]string{"argocd1.example.com:443"})
		err = useCmd.Execute()
		require.NoError(t, err)

		localConfig, err = localconfig.ReadLocalConfig(testConfigFilePath)
		require.NoError(t, err)

		assert.Equal(t, "argocd1.example.com:443", localConfig.CurrentContext)
		return nil
	})

	require.NoError(t, err)
	expected := "Switched to context 'argocd1.example.com:443'\n"
	assert.Equal(t, expected, output)
}

func TestContextDelete(t *testing.T) {
	// Write the test config file
	err := os.WriteFile(testConfigFilePath, []byte(testConfig), os.ModePerm)
	require.NoError(t, err)
	defer os.Remove(testConfigFilePath)

	err = os.Chmod(testConfigFilePath, 0o600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)
	localConfig, err := localconfig.ReadLocalConfig(testConfigFilePath)
	require.NoError(t, err)
	assert.Equal(t, "localhost:8080", localConfig.CurrentContext)
	assert.Contains(t, localConfig.Contexts, localconfig.ContextRef{Name: "localhost:8080", Server: "localhost:8080", User: "localhost:8080"})

	// Delete a non-current context
	err = deleteContext("argocd1.example.com:443", testConfigFilePath)
	require.NoError(t, err)

	localConfig, err = localconfig.ReadLocalConfig(testConfigFilePath)
	require.NoError(t, err)
	assert.Equal(t, "localhost:8080", localConfig.CurrentContext)
	assert.NotContains(t, localConfig.Contexts, localconfig.ContextRef{Name: "argocd1.example.com:443", Server: "argocd1.example.com:443", User: "argocd1.example.com:443"})
	assert.NotContains(t, localConfig.Servers, localconfig.Server{Server: "argocd1.example.com:443"})
	assert.NotContains(t, localConfig.Users, localconfig.User{AuthToken: "vErrYS3c3tReFRe$hToken", Name: "argocd1.example.com:443"})
	assert.Contains(t, localConfig.Contexts, localconfig.ContextRef{Name: "argocd2.example.com:443", Server: "argocd2.example.com:443", User: "argocd2.example.com:443"})
	assert.Contains(t, localConfig.Contexts, localconfig.ContextRef{Name: "localhost:8080", Server: "localhost:8080", User: "localhost:8080"})

	// Delete the current context
	err = deleteContext("localhost:8080", testConfigFilePath)
	require.NoError(t, err)

	localConfig, err = localconfig.ReadLocalConfig(testConfigFilePath)
	require.NoError(t, err)
	assert.Empty(t, localConfig.CurrentContext)
	assert.NotContains(t, localConfig.Contexts, localconfig.ContextRef{Name: "localhost:8080", Server: "localhost:8080", User: "localhost:8080"})
	assert.NotContains(t, localConfig.Servers, localconfig.Server{PlainText: true, Server: "localhost:8080"})
	assert.NotContains(t, localConfig.Users, localconfig.User{AuthToken: "vErrYS3c3tReFRe$hToken", Name: "localhost:8080"})
	assert.Contains(t, localConfig.Contexts, localconfig.ContextRef{Name: "argocd2.example.com:443", Server: "argocd2.example.com:443", User: "argocd2.example.com:443"})
}
