package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/argoproj/argo-cd/v3/cmd/util"
	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/errors"
	"github.com/argoproj/argo-cd/v3/util/localconfig"
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

func TestNewConfigureCommand_Outputs(t *testing.T) {
	t.Parallel()

	emptyRegex := "^$"

	tests := []struct {
		name                          string
		args                          []string
		createConfigFile              bool
		readOnlyConfigFile            bool
		corruptConfigFile             bool
		expectedPromptsEnabled        bool
		expectedStdout                string
		expectedStderrRegex           string
		expectedExitCode              int
		expectedExitErrorMessageRegex string
	}{
		{
			name:                          "no args",
			args:                          []string{},
			createConfigFile:              true,
			readOnlyConfigFile:            false,
			expectedPromptsEnabled:        false,
			expectedStdout:                "Successfully updated the following configuration settings:\nprompts-enabled: false\n",
			expectedStderrRegex:           emptyRegex,
			expectedExitCode:              0,
			expectedExitErrorMessageRegex: emptyRegex,
		},
		{
			name:                          "prompts-enabled=true",
			args:                          []string{"--prompts-enabled=true"},
			createConfigFile:              true,
			readOnlyConfigFile:            false,
			expectedPromptsEnabled:        true,
			expectedStdout:                "Successfully updated the following configuration settings:\nprompts-enabled: true\n",
			expectedStderrRegex:           emptyRegex,
			expectedExitCode:              0,
			expectedExitErrorMessageRegex: emptyRegex,
		},
		{
			name:                          "prompts-enabled=false",
			args:                          []string{"--prompts-enabled=false"},
			createConfigFile:              true,
			readOnlyConfigFile:            false,
			expectedPromptsEnabled:        false,
			expectedStdout:                "Successfully updated the following configuration settings:\nprompts-enabled: false\n",
			expectedStderrRegex:           emptyRegex,
			expectedExitCode:              0,
			expectedExitErrorMessageRegex: emptyRegex,
		},
		{
			name:                          "non-existent config file",
			args:                          []string{"--prompts-enabled=true"},
			createConfigFile:              false,
			expectedStdout:                "No local configuration found\n",
			expectedStderrRegex:           emptyRegex,
			expectedExitCode:              1,
			expectedExitErrorMessageRegex: emptyRegex,
		},
		{
			name:                          "read-only config file",
			args:                          []string{"--prompts-enabled=true"},
			createConfigFile:              true,
			readOnlyConfigFile:            true,
			expectedStdout:                "",
			expectedStderrRegex:           emptyRegex,
			expectedExitCode:              errors.ErrorGeneric,
			expectedExitErrorMessageRegex: ".* permission denied",
		},
		{
			name:                          "corrupt config file",
			args:                          []string{"--prompts-enabled=true"},
			createConfigFile:              true,
			corruptConfigFile:             true,
			expectedStdout:                "",
			expectedStderrRegex:           emptyRegex,
			expectedExitCode:              errors.ErrorGeneric,
			expectedExitErrorMessageRegex: ".*failed to parse config file",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfgFilePath := filepath.Join(t.TempDir(), "config")

			if test.createConfigFile {
				if test.corruptConfigFile {
					prepareCorruptTestConfig(t, cfgFilePath)
				} else {
					prepareTestConfig(t, cfgFilePath, test.readOnlyConfigFile)
				}
			}

			cmd := NewConfigureCommand(&argocdclient.ClientOptions{ConfigPath: cfgFilePath})

			stdout, stderr, err := runCmd(t, cmd, test.args...)

			if test.expectedExitCode == 0 {
				require.NoError(t, err)

				localConfig, err := localconfig.ReadLocalConfig(cfgFilePath)
				require.NoError(t, err)
				assert.Equal(t, test.expectedPromptsEnabled, localConfig.PromptsEnabled, "prompts enabled mismatch expected: %t, got: %t", test.expectedPromptsEnabled, localConfig.PromptsEnabled)
			} else {
				var e *util.ExitError
				require.ErrorAs(t, err, &e, "expected to get an ExitError")

				assert.Equal(t, test.expectedExitCode, e.ExitCode(), "exit code mismatch expected: %d, got: %d", test.expectedExitCode, e.ExitCode())
				assert.Regexp(t, test.expectedExitErrorMessageRegex, e.Message(), "ExitError message mismatch expected to match regex: %s on message: %s", test.expectedExitErrorMessageRegex, e.Message())
			}

			assert.Equal(t, test.expectedStdout, stdout, "stdout mismatch")
			assert.Regexp(t, test.expectedStderrRegex, stderr, "stderr mismatch")
		})
	}
}

func prepareTestConfig(t *testing.T, filePath string, readOnly bool) {
	t.Helper()

	err := os.WriteFile(filePath, []byte(testConfig), os.ModePerm)
	require.NoError(t, err)

	mode := os.FileMode(0o600)
	if readOnly {
		mode = 0o400
	}
	err = os.Chmod(filePath, mode)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)

	localConfig, err := localconfig.ReadLocalConfig(filePath)
	require.NoError(t, err)

	assert.False(t, localConfig.PromptsEnabled)
}

func prepareCorruptTestConfig(t *testing.T, filePath string) {
	t.Helper()

	err := os.WriteFile(filePath, []byte("this is not valid yaml: [[["), 0o600)
	require.NoError(t, err)
}
