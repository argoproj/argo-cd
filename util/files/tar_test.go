package files_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/test"
	"github.com/argoproj/argo-cd/v2/util/files"
)

func TestTgz(t *testing.T) {

	t.Run("will tgz folder successfully", func(t *testing.T) {
		// given
		exclusions := []string{}
		hasher := sha256.New()
		expectedHash := "58489f8b7ccfea9c9233e0f4beaab4b760d78118d4b597e3e989840b54581e10"
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
		expectedHash := "4bf33604525ed9d52f440ca9f3f7c0b73b456df62c24854386adf8d64f88efbe"
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
		expectedHash := "0f14bfa12a46dfbdbf1897c03ab15cd9fedda437165f3c9c0888b0b995feb2a9"
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

// getTestDataDir will return the full path of the testdata dir
// under the running test folder.
func getTestDataDir(t *testing.T) string {
	return filepath.Join(test.GetTestDir(t), "testdata")
}
