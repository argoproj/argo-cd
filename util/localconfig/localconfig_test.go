//go:build !windows

package localconfig

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/argoproj/argo-cd/v2/util/config"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestGetUsername(t *testing.T) {
	assert.Equal(t, "admin", GetUsername("admin:login"))
	assert.Equal(t, "admin", GetUsername("admin"))
	assert.Equal(t, "", GetUsername(""))
}

func TestFilePermission(t *testing.T) {
	dirPath := "testfolder/"

	err := os.MkdirAll(path.Dir(dirPath), 0o700)
	require.NoError(t, err, "Could not create argocd folder with 0700 permission: %v", err)

	t.Cleanup(func() {
		err := os.RemoveAll(dirPath)
		require.NoError(t, err, "Could not remove directory")
	})

	for _, c := range []struct {
		name          string
		testfile      string
		perm          os.FileMode
		expectedError error
	}{
		{
			name:          "Test config file with permission 0700",
			testfile:      ".config_0700",
			perm:          0o700,
			expectedError: errors.New("config file has incorrect permission flags:-rwx------.change the file permission either to 0400 or 0600."),
		},
		{
			name:          "Test config file with permission 0777",
			testfile:      ".config_0777",
			perm:          0o777,
			expectedError: errors.New("config file has incorrect permission flags:-rwxrwxrwx.change the file permission either to 0400 or 0600."),
		},
		{
			name:          "Test config file with permission 0600",
			testfile:      ".config_0600",
			perm:          0o600,
			expectedError: nil,
		},
		{
			name:          "Test config file with permission 0400",
			testfile:      ".config_0400",
			perm:          0o400,
			expectedError: nil,
		},
		{
			name:          "Test config file with permission 0300",
			testfile:      ".config_0300",
			perm:          0o300,
			expectedError: errors.New("config file has incorrect permission flags:--wx------.change the file permission either to 0400 or 0600."),
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			filePath := filepath.Join(dirPath, c.testfile)

			f, err := os.Create(filePath)
			require.NoError(t, err, "Could not write  create config file: %v", err)
			defer func() {
				assert.NoError(t, f.Close())
			}()

			err = f.Chmod(c.perm)
			require.NoError(t, err, "Could not change the file permission to %s: %v", c.perm, err)

			fi, err := os.Stat(filePath)
			require.NoError(t, err, "Could not access the fileinfo: %v", err)

			if err := getFilePermission(fi); err != nil {
				assert.EqualError(t, err, c.expectedError.Error())
			} else {
				require.NoError(t, c.expectedError)
			}
		})
	}
}

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

func loadOpts(t *testing.T, opts string) {
	t.Helper()
	t.Setenv("ARGOCD_OPTS", opts)
	assert.NoError(t, config.LoadFlags())
}

func TestGetPromptsEnabled_useCLIOpts_false_localConfigPromptsEnabled_true(t *testing.T) {
	// Write the test config file
	err := os.WriteFile(testConfigFilePath, []byte(testConfig+"\nprompts-enabled: true"), os.ModePerm)
	require.NoError(t, err)

	defer os.Remove(testConfigFilePath)

	err = os.Chmod(testConfigFilePath, 0o600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)

	loadOpts(t, "--config "+testConfigFilePath)

	assert.True(t, GetPromptsEnabled(false))
}

func TestGetPromptsEnabled_useCLIOpts_false_localConfigPromptsEnabled_false(t *testing.T) {
	// Write the test config file
	err := os.WriteFile(testConfigFilePath, []byte(testConfig+"\nprompts-enabled: false"), os.ModePerm)
	require.NoError(t, err)

	defer os.Remove(testConfigFilePath)

	err = os.Chmod(testConfigFilePath, 0o600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)

	loadOpts(t, "--config "+testConfigFilePath)

	assert.False(t, GetPromptsEnabled(false))
}

func TestGetPromptsEnabled_useCLIOpts_true_forcePromptsEnabled_default(t *testing.T) {
	// Write the test config file
	err := os.WriteFile(testConfigFilePath, []byte(testConfig+"\nprompts-enabled: false"), os.ModePerm)
	require.NoError(t, err)

	defer os.Remove(testConfigFilePath)

	err = os.Chmod(testConfigFilePath, 0o600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)

	loadOpts(t, "--config "+testConfigFilePath+" --prompts-enabled")

	assert.True(t, GetPromptsEnabled(true))
}

func TestGetPromptsEnabled_useCLIOpts_true_forcePromptsEnabled_true(t *testing.T) {
	// Write the test config file
	err := os.WriteFile(testConfigFilePath, []byte(testConfig+"\nprompts-enabled: false"), os.ModePerm)
	require.NoError(t, err)

	defer os.Remove(testConfigFilePath)

	err = os.Chmod(testConfigFilePath, 0o600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)

	loadOpts(t, "--config "+testConfigFilePath+" --prompts-enabled=true")

	assert.True(t, GetPromptsEnabled(true))
}

func TestGetPromptsEnabled_useCLIOpts_true_forcePromptsEnabled_false(t *testing.T) {
	// Write the test config file
	err := os.WriteFile(testConfigFilePath, []byte(testConfig+"\nprompts-enabled: true"), os.ModePerm)
	require.NoError(t, err)

	defer os.Remove(testConfigFilePath)

	err = os.Chmod(testConfigFilePath, 0o600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)

	loadOpts(t, "--config "+testConfigFilePath+" --prompts-enabled=false")

	assert.False(t, GetPromptsEnabled(true))
}
