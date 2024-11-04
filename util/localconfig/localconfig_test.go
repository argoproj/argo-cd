//go:build !windows

package localconfig

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

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
			expectedError: fmt.Errorf("config file has incorrect permission flags:-rwx------.change the file permission either to 0400 or 0600."),
		},
		{
			name:          "Test config file with permission 0777",
			testfile:      ".config_0777",
			perm:          0o777,
			expectedError: fmt.Errorf("config file has incorrect permission flags:-rwxrwxrwx.change the file permission either to 0400 or 0600."),
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
				require.NoError(t, c.expectedError)
			}
		})
	}
}
