package utils

import (
	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

const testConfigFilePath = "../testdata/local.config"

func TestNewPrompt_PromptsEnabled_True(t *testing.T) {
	testConfig := `contexts:
- name: localhost:8080
  server: localhost:8080
  user: localhost:8080
current-context: localhost:8080
servers:
- plain-text: true
  server: localhost:8080
users:
- auth-token: vErrYS3c3tReFRe$hToken
  name: localhost:8080
prompts-enabled: true`

	err := os.WriteFile(testConfigFilePath, []byte(testConfig), os.ModePerm)
	require.NoError(t, err)

	defer os.Remove(testConfigFilePath)

	err = os.Chmod(testConfigFilePath, 0o600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)

	clientOpts := apiclient.ClientOptions{
		ConfigPath: testConfigFilePath,
	}

	prompt, err := NewPrompt(&clientOpts)
	require.NoError(t, err)

	assert.True(t, prompt.enabled)
}

func TestNewPrompt_PromptsEnabled_False(t *testing.T) {
	testConfig := `contexts:
- name: localhost:8080
  server: localhost:8080
  user: localhost:8080
current-context: localhost:8080
servers:
- plain-text: true
  server: localhost:8080
users:
- auth-token: vErrYS3c3tReFRe$hToken
  name: localhost:8080
prompts-enabled: false`

	err := os.WriteFile(testConfigFilePath, []byte(testConfig), os.ModePerm)
	require.NoError(t, err)

	defer os.Remove(testConfigFilePath)

	err = os.Chmod(testConfigFilePath, 0o600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)

	clientOpts := apiclient.ClientOptions{
		ConfigPath: testConfigFilePath,
	}

	prompt, err := NewPrompt(&clientOpts)
	require.NoError(t, err)

	assert.False(t, prompt.enabled)
}

func TestNewPrompt_PromptsEnabled_Unspecified(t *testing.T) {
	testConfig := `contexts:
- name: localhost:8080
  server: localhost:8080
  user: localhost:8080
current-context: localhost:8080
servers:
- plain-text: true
  server: localhost:8080
users:
- auth-token: vErrYS3c3tReFRe$hToken
  name: localhost:8080`

	err := os.WriteFile(testConfigFilePath, []byte(testConfig), os.ModePerm)
	require.NoError(t, err)

	defer os.Remove(testConfigFilePath)

	err = os.Chmod(testConfigFilePath, 0o600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)

	clientOpts := apiclient.ClientOptions{
		ConfigPath: testConfigFilePath,
	}

	prompt, err := NewPrompt(&clientOpts)
	require.NoError(t, err)

	assert.False(t, prompt.enabled)
}
