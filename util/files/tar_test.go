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
		t.Parallel()
		exclusions := []string{}
		hasher := sha256.New()
		expectedHash := "82c1bc4f878b340d541901faaa172386d2ea3c77abf3401ac7fd5571ef920f3f"
		appDir := getTestAppDir(t)

		// when
		err := files.Tgz(appDir, exclusions, hasher)

		// then
		assert.NoError(t, err)
		assert.Equal(t, expectedHash, hex.EncodeToString(hasher.Sum(nil)))
	})
	t.Run("will exclude files from the exclusion list", func(t *testing.T) {
		// given
		t.Parallel()
		exclusions := []string{"README.md"}
		hasher := sha256.New()
		expectedHash := "b4bc670a2ff74250da027c876565d8a14c17c5cfdb017c7ef89bfc1ac0b1f429"
		appDir := getTestAppDir(t)

		// when
		err := files.Tgz(appDir, exclusions, hasher)

		// then
		assert.NoError(t, err)
		assert.Equal(t, expectedHash, hex.EncodeToString(hasher.Sum(nil)))
	})
	t.Run("will exclude directories from the exclusion list", func(t *testing.T) {
		// given
		t.Parallel()
		exclusions := []string{"README.md", "applicationset/latest"}
		hasher := sha256.New()
		expectedHash := "ee898e268b2e7b5ee6c234fabf3fe0a85c186af98e47ed72159374ed5fdb5706"
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
		tmpDir, err := ioutil.TempDir(test.GetTestDir(t), "")
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
		if _, err := tgzFile.Seek(0, io.SeekStart); err != nil {
			t.Fatalf("seek error: %s", err)
		}

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
