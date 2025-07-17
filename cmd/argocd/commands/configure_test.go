package commands

import (
	"os"
	"testing"

	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/util/localconfig"
)

func TestNewConfigureCommand_PromptsEnabled_DefaultTrue(t *testing.T) {
	// Write the test config file
	err := os.WriteFile(testConfigFilePath, []byte(testConfig), os.ModePerm)
	require.NoError(t, err)

	defer os.Remove(testConfigFilePath)

	err = os.Chmod(testConfigFilePath, 0o600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)

	localConfig, err := localconfig.ReadLocalConfig(testConfigFilePath)
	require.NoError(t, err)
	assert.False(t, localConfig.PromptsEnabled)

	// Set `PromptsEnabled` to `true` using `argocd configure --prompts-enabled`
	cmd := NewConfigureCommand(&argocdclient.ClientOptions{ConfigPath: testConfigFilePath})
	cmd.SetArgs([]string{"--prompts-enabled"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Read the test config file
	localConfig, err = localconfig.ReadLocalConfig(testConfigFilePath)
	require.NoError(t, err)

	assert.True(t, localConfig.PromptsEnabled)
}

func TestNewConfigureCommand_PromptsEnabled_True(t *testing.T) {
	// Write the test config file
	err := os.WriteFile(testConfigFilePath, []byte(testConfig), os.ModePerm)
	require.NoError(t, err)

	defer os.Remove(testConfigFilePath)

	err = os.Chmod(testConfigFilePath, 0o600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)

	localConfig, err := localconfig.ReadLocalConfig(testConfigFilePath)
	require.NoError(t, err)
	assert.False(t, localConfig.PromptsEnabled)

	// Set `PromptsEnabled` to `true` using `argocd configure --prompts-enabled=true`
	cmd := NewConfigureCommand(&argocdclient.ClientOptions{ConfigPath: testConfigFilePath})
	cmd.SetArgs([]string{"--prompts-enabled=true"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Read the test config file
	localConfig, err = localconfig.ReadLocalConfig(testConfigFilePath)
	require.NoError(t, err)

	assert.True(t, localConfig.PromptsEnabled)
}

func TestNewConfigureCommand_PromptsEnabled_False(t *testing.T) {
	// Write the test config file
	err := os.WriteFile(testConfigFilePath, []byte(testConfig), os.ModePerm)
	require.NoError(t, err)

	defer os.Remove(testConfigFilePath)

	err = os.Chmod(testConfigFilePath, 0o600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)

	localConfig, err := localconfig.ReadLocalConfig(testConfigFilePath)
	require.NoError(t, err)
	assert.False(t, localConfig.PromptsEnabled)

	// Set `PromptsEnabled` to `false` using `argocd configure --prompts-enabled=false`
	cmd := NewConfigureCommand(&argocdclient.ClientOptions{ConfigPath: testConfigFilePath})
	cmd.SetArgs([]string{"--prompts-enabled=false"})

	err = cmd.Execute()
	require.NoError(t, err)

	// Read the test config file
	localConfig, err = localconfig.ReadLocalConfig(testConfigFilePath)
	require.NoError(t, err)

	assert.False(t, localConfig.PromptsEnabled)
}
