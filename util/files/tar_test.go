package files_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/argoproj/argo-cd/v2/util/files"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTgz(t *testing.T) {

	t.Run("will tgz folder successfully", func(t *testing.T) {
		// given
		exclusions := []string{}
		hasher := sha256.New()
		expectedHash := "5fc2bc3cd06dd65a28afb387460c730f7ecd9c04a5a5fbc98104ee1aac2cfff2"
		appDir := getTestAppDir(t)

		// when
		err := files.Tgz(appDir, exclusions, hasher)

		// then
		assert.NoError(t, err)
		assert.Equal(t, expectedHash, hex.EncodeToString(hasher.Sum(nil)))
	})
	t.Run("will exclude files from the exclusion list", func(t *testing.T) {
		// given
		exclusions := []string{"README.md"}
		hasher := sha256.New()
		expectedHash := "ac9ff2154b5200abdf3d7b0261072d3951a76c1334b1db072873a75bfcd8c291"
		appDir := getTestAppDir(t)

		// when
		err := files.Tgz(appDir, exclusions, hasher)

		// then
		assert.NoError(t, err)
		assert.Equal(t, expectedHash, hex.EncodeToString(hasher.Sum(nil)))
	})
	t.Run("will exclude directories from the exclusion list", func(t *testing.T) {
		// given
		exclusions := []string{"README.md", "applicationset/latest"}
		hasher := sha256.New()
		expectedHash := "978c0b70392d55b5adde1a233165b48fc611216e273e5d9fdce7f665b9962099"
		appDir := getTestAppDir(t)

		// when
		err := files.Tgz(appDir, exclusions, hasher)

		// then
		assert.NoError(t, err)
		assert.Equal(t, expectedHash, hex.EncodeToString(hasher.Sum(nil)))
	})
}

func TestUntgz(t *testing.T) {
	createTmpDir := func(t *testing.T) string {
		t.Helper()
		tmpDir, err := ioutil.TempDir(getTestDir(t), "")
		if err != nil {
			t.Fatalf("error creating tmpDir: %s", err)
		}
		return tmpDir
	}
	deleteTmpDir := func(t *testing.T, dirname string) {
		t.Helper()
		err := os.RemoveAll(dirname)
		if err != nil {
			t.Errorf("error removing tmpDir: %s", err)
		}
	}
	createTmpFile := func(t *testing.T, basedir string) *os.File {
		t.Helper()
		f, err := ioutil.TempFile(basedir, "")
		if err != nil {
			t.Fatalf("error creating tmpFile in %q: %s", basedir, err)
		}
		return f
	}
	t.Run("will untgz successfully", func(t *testing.T) {
		// given
		tmpDir := createTmpDir(t)
		defer deleteTmpDir(t, tmpDir)
		tgzFile := createTmpFile(t, tmpDir)
		defer tgzFile.Close()

		err := files.Tgz(getTestAppDir(t), []string{}, tgzFile)
		if err != nil {
			t.Fatalf("error during Tgz: %s", err)
		}
		tgzFile.Seek(0, io.SeekStart)

		// when
		err = files.Untgz(tmpDir, tgzFile)

		// then
		require.NoError(t, err)
		filesInfo, err := ioutil.ReadDir(tmpDir)
		require.NoError(t, err)
		assert.Equal(t, 3, len(filesInfo))
	})

}

// getTestAppDir will return the full path of the app dir under
// the 'testdata' folder.
func getTestAppDir(t *testing.T) string {
	return filepath.Join(getTestDataDir(t), "app")
}

// getTestDataDir will return the full path of hte testdata dir
// under the running test folder.
func getTestDataDir(t *testing.T) string {
	return filepath.Join(getTestDir(t), "testdata")
}

// getTestDir will return the full path for the current test
// file.
func getTestDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("error retrieving directory for the running test")
	}
	return filepath.Dir(filename)
}
