//go:build !windows

package localconfig

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/argoproj/argo-cd/v2/util/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetToken_Exist(t *testing.T) {
	err := os.WriteFile(test.TestConfigFilePath, []byte(test.TestConfig), os.ModePerm)
	assert.NoError(t, err)
	defer os.Remove(test.TestConfigFilePath)

	err = os.Chmod(test.TestConfigFilePath, 0600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)
	localConfig, err := ReadLocalConfig(test.TestConfigFilePath)
	assert.NoError(t, err)

	token := localConfig.GetToken(localConfig.CurrentContext)

	assert.Equal(t, "vErrYS3c3tReFRe$hToken", token)
}

func TestGetToken_Not_Exist(t *testing.T) {
	err := os.WriteFile(test.TestConfigFilePath, []byte(test.TestConfig), os.ModePerm)
	assert.NoError(t, err)
	defer os.Remove(test.TestConfigFilePath)

	err = os.Chmod(test.TestConfigFilePath, 0600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)
	localConfig, err := ReadLocalConfig(test.TestConfigFilePath)
	assert.NoError(t, err)

	// serverName does exist in TestConfig
	token := localConfig.GetToken("localhost")

	assert.Equal(t, "", token)
}

func TestRemoveToken_Exist(t *testing.T) {
	err := os.WriteFile(test.TestConfigFilePath, []byte(test.TestConfig), os.ModePerm)
	assert.NoError(t, err)
	defer os.Remove(test.TestConfigFilePath)

	err = os.Chmod(test.TestConfigFilePath, 0600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)
	localConfig, err := ReadLocalConfig(test.TestConfigFilePath)
	assert.NoError(t, err)

	removed := localConfig.RemoveToken(localConfig.CurrentContext)

	assert.Equal(t, true, removed)
}

func TestRemoveToken_Not_Exist(t *testing.T) {
	err := os.WriteFile(test.TestConfigFilePath, []byte(test.TestConfig), os.ModePerm)
	assert.NoError(t, err)
	defer os.Remove(test.TestConfigFilePath)

	err = os.Chmod(test.TestConfigFilePath, 0600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)
	localConfig, err := ReadLocalConfig(test.TestConfigFilePath)
	assert.NoError(t, err)

	// serverName does exist in TestConfig
	removed := localConfig.RemoveToken("localhost")

	assert.Equal(t, false, removed)
}

func TestValidateLocalConfig(t *testing.T) {
	err := os.WriteFile(test.TestConfigFilePath, []byte(test.TestConfig), os.ModePerm)
	assert.NoError(t, err)
	defer os.Remove(test.TestConfigFilePath)

	err = os.Chmod(test.TestConfigFilePath, 0600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)
	localConfig, err := ReadLocalConfig(test.TestConfigFilePath)
	assert.NoError(t, err)

	err = ValidateLocalConfig(*localConfig)

	assert.Equal(t, nil, err)
}

func TestWriteLocalConfig(t *testing.T) {
	err := os.WriteFile(test.TestConfigFilePath, []byte(test.TestConfig), os.ModePerm)
	assert.NoError(t, err)
	defer os.Remove(test.TestConfigFilePath)

	err = os.Chmod(test.TestConfigFilePath, 0600)
	require.NoError(t, err, "Could not change the file permission to 0600 %v", err)
	localConfig, err := ReadLocalConfig(test.TestConfigFilePath)
	assert.NoError(t, err)

	err = WriteLocalConfig(*localConfig, test.WriteConfigFilePath)
	defer os.Remove(test.WriteConfigFilePath)

	assert.Equal(t, nil, err)
}

func TestGetUsername(t *testing.T) {
	assert.Equal(t, "admin", GetUsername("admin:login"))
	assert.Equal(t, "admin", GetUsername("admin"))
	assert.Equal(t, "", GetUsername(""))
}

func TestFilePermission(t *testing.T) {
	dirPath := "testfolder/"

	err := os.MkdirAll(path.Dir(dirPath), 0700)
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
			perm:          0700,
			expectedError: fmt.Errorf("config file has incorrect permission flags:-rwx------.change the file permission either to 0400 or 0600."),
		},
		{
			name:          "Test config file with permission 0777",
			testfile:      ".config_0777",
			perm:          0777,
			expectedError: fmt.Errorf("config file has incorrect permission flags:-rwxrwxrwx.change the file permission either to 0400 or 0600."),
		},
		{
			name:          "Test config file with permission 0600",
			testfile:      ".config_0600",
			perm:          0600,
			expectedError: nil,
		},
		{
			name:          "Test config file with permission 0400",
			testfile:      ".config_0400",
			perm:          0400,
			expectedError: nil,
		},
		{
			name:          "Test config file with permission 0300",
			testfile:      ".config_0300",
			perm:          0300,
			expectedError: fmt.Errorf("config file has incorrect permission flags:--wx------.change the file permission either to 0400 or 0600."),
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
				require.Nil(t, c.expectedError)
			}
		})
	}
}
