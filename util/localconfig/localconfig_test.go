package localconfig

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/stretchr/testify/assert"

	"testing"
)

func TestGetUsername(t *testing.T) {
	assert.Equal(t, "admin", GetUsername("admin:login"))
	assert.Equal(t, "admin", GetUsername("admin"))
	assert.Equal(t, "", GetUsername(""))
}

func TestFilePermission(t *testing.T) {
	dirPath := "testfolder/"
	fpath := filepath.Join(dirPath, ".config")

	if err := os.MkdirAll(path.Dir(dirPath), 0700); err != nil {
		t.Fatalf("Could not create argocd folder with 0700 permission: %v", err)
	}

	f, err := os.Create(fpath)

	if err != nil {
		f.Close()
		t.Fatalf("Could not write  create config file: %v", err)
	}
	fi, err := os.Stat(fpath)

	if err != nil {
		t.Fatalf("Could not access the fileinfo: %v", err)
	}
	for _, c := range []struct {
		name          string
		perm          os.FileMode
		expectedError error
	}{
		{
			name:          "Test config file with permission 0700",
			perm:          0700,
			expectedError: fmt.Errorf("config file has incorrect permission flags: -rwx------  change the permission to 0700  -rw-------"),
		},
		{
			name:          "Test config file with permission 0777",
			perm:          0777,
			expectedError: fmt.Errorf("config file has incorrect permission flags: -rwxrwxrwx  change the permission to 0777  -rw-------"),
		},
		{
			name: "Test config file with permission 0600",
			perm: 0600,
		},
	} {
		t.Run(c.name, func(t *testing.T) {

			if err = f.Chmod(c.perm); err != nil {
				t.Fatalf("Could not change the file permission to %s: %v", c.perm, err)
			}

			if err := GetFilePermission(fi); err != nil {
				assert.EqualError(t, err, c.expectedError.Error())
			} else {
				if fi.Mode().Perm().String() != "-rw-------" {
					t.Fatalf("file %v Permission mismatch source (-rw------) vs destination(%v)", fpath, fi.Mode().Perm())
				}
			}
		})

	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dirPath); err != nil {
			t.Error("Could not remove directory")
		}
	})
}
