package files_test

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
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
	type fixture struct {
		file *os.File
	}
	setup := func(t *testing.T) *fixture {
		t.Helper()
		testDir := getTestDataDir(t)
		f, err := ioutil.TempFile(testDir, "")
		require.NoError(t, err)
		return &fixture{
			file: f,
		}
	}
	teardown := func(f *fixture) {
		f.file.Close()
		os.Remove(f.file.Name())
	}
	prepareRead := func(f *fixture) {
		_, err := f.file.Seek(0, io.SeekStart)
		require.NoError(t, err)
	}

	t.Run("will tgz folder successfully", func(t *testing.T) {
		// given
		t.Parallel()
		exclusions := []string{}
		f := setup(t)
		defer teardown(f)

		// when
		err := files.Tgz(getTestAppDir(t), exclusions, f.file)

		// then
		assert.NoError(t, err)
		prepareRead(f)
		files, err := read(f.file)
		require.NoError(t, err)
		assert.Equal(t, 3, len(files))
		assert.Contains(t, files, "README.md")
		assert.Contains(t, files, "applicationset/latest/kustomization.yaml")
		assert.Contains(t, files, "applicationset/stable/kustomization.yaml")
	})
	t.Run("will exclude files from the exclusion list", func(t *testing.T) {
		// given
		t.Parallel()
		exclusions := []string{"README.md"}
		f := setup(t)
		defer teardown(f)

		// when
		err := files.Tgz(getTestAppDir(t), exclusions, f.file)

		// then
		assert.NoError(t, err)
		prepareRead(f)
		files, err := read(f.file)
		require.NoError(t, err)
		assert.Equal(t, 2, len(files))
		assert.Contains(t, files, "applicationset/latest/kustomization.yaml")
		assert.Contains(t, files, "applicationset/stable/kustomization.yaml")
	})
	t.Run("will exclude directories from the exclusion list", func(t *testing.T) {
		// given
		t.Parallel()
		exclusions := []string{"README.md", "applicationset/latest"}
		f := setup(t)
		defer teardown(f)

		// when
		err := files.Tgz(getTestAppDir(t), exclusions, f.file)

		// then
		assert.NoError(t, err)
		prepareRead(f)
		files, err := read(f.file)
		require.NoError(t, err)
		assert.Equal(t, 1, len(files))
		assert.Contains(t, files, "applicationset/stable/kustomization.yaml")
	})
}

func TestUntgz(t *testing.T) {
	createTmpDir := func(t *testing.T) string {
		t.Helper()
		tmpDir, err := ioutil.TempDir(getTestDataDir(t), "")
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
	createTgz := func(t *testing.T, destdir string) *os.File {
		t.Helper()
		f, err := ioutil.TempFile(destdir, "")
		if err != nil {
			t.Fatalf("error creating tmpFile in %q: %s", destdir, err)
		}
		err = files.Tgz(getTestAppDir(t), []string{}, f)
		if err != nil {
			t.Fatalf("error during Tgz: %s", err)
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			t.Fatalf("seek error: %s", err)
		}
		return f
	}
	readFiles := func(t *testing.T, basedir string) []string {
		t.Helper()
		names := []string{}
		err := filepath.Walk(basedir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.Mode().IsRegular() {
				relativePath := files.RelativePath(path, basedir)
				names = append(names, relativePath)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("error reading files: %s", err)
		}
		return names
	}
	t.Run("will untgz successfully", func(t *testing.T) {
		// given
		tmpDir := createTmpDir(t)
		defer deleteTmpDir(t, tmpDir)
		tgzFile := createTgz(t, tmpDir)
		defer tgzFile.Close()

		destDir := filepath.Join(tmpDir, "untgz1")

		// when
		err := files.Untgz(destDir, tgzFile)

		// then
		require.NoError(t, err)
		names := readFiles(t, destDir)
		assert.Equal(t, 3, len(names))
		assert.Contains(t, names, "README.md")
		assert.Contains(t, names, "applicationset/stable/kustomization.yaml")
	})

}

func read(tgz *os.File) ([]string, error) {
	files := []string{}
	gzr, err := gzip.NewReader(tgz)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("error while iterating on tar reader: %w", err)
		}
		if header == nil {
			continue
		}
		files = append(files, header.Name)
	}
	return files, nil
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
