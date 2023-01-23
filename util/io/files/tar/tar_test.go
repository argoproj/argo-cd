package tar

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/test"
	"github.com/argoproj/argo-cd/v2/util/io/files"
)

func TestTgz(t *testing.T) {
	type fixture struct {
		file *os.File
	}
	setup := func(t *testing.T) *fixture {
		t.Helper()
		testDir := getTestDataDir(t)
		f, err := os.CreateTemp(testDir, "")
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
		filesWritten, err := Tgz(getTestAppDir(t), nil, exclusions, f.file)

		// then
		assert.Equal(t, 3, filesWritten)
		assert.NoError(t, err)
		prepareRead(f)
		files, err := read(f.file)
		require.NoError(t, err)
		assert.Equal(t, 8, len(files))
		assert.Contains(t, files, "README.md")
		assert.Contains(t, files, "applicationset/latest/kustomization.yaml")
		assert.Contains(t, files, "applicationset/stable/kustomization.yaml")
		assert.Contains(t, files, "applicationset/readme-symlink")
		assert.Equal(t, files["applicationset/readme-symlink"], "../README.md")
	})
	t.Run("will exclude files from the exclusion list", func(t *testing.T) {
		// given
		t.Parallel()
		exclusions := []string{"README.md"}
		f := setup(t)
		defer teardown(f)

		// when
		filesWritten, err := Tgz(getTestAppDir(t), nil, exclusions, f.file)

		// then
		assert.Equal(t, 2, filesWritten)
		assert.NoError(t, err)
		prepareRead(f)
		files, err := read(f.file)
		require.NoError(t, err)
		assert.Equal(t, 7, len(files))
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
		filesWritten, err := Tgz(getTestAppDir(t), nil, exclusions, f.file)

		// then
		assert.Equal(t, 1, filesWritten)
		assert.NoError(t, err)
		prepareRead(f)
		files, err := read(f.file)
		require.NoError(t, err)
		assert.Equal(t, 5, len(files))
		assert.Contains(t, files, "applicationset/stable/kustomization.yaml")
	})
}

func TestUntgz(t *testing.T) {
	createTmpDir := func(t *testing.T) string {
		t.Helper()
		tmpDir, err := os.MkdirTemp(getTestDataDir(t), "")
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
	createTgz := func(t *testing.T, fromDir, destDir string) *os.File {
		t.Helper()
		f, err := os.CreateTemp(destDir, "")
		if err != nil {
			t.Fatalf("error creating tmpFile in %q: %s", destDir, err)
		}
		_, err = Tgz(fromDir, nil, nil, f)
		if err != nil {
			t.Fatalf("error during Tgz: %s", err)
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			t.Fatalf("seek error: %s", err)
		}
		return f
	}
	readFiles := func(t *testing.T, basedir string) map[string]string {
		t.Helper()
		names := make(map[string]string)
		err := filepath.Walk(basedir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			link := ""
			if files.IsSymlink(info) {
				link, err = os.Readlink(path)
				if err != nil {
					return err
				}
			}
			relativePath, err := files.RelativePath(path, basedir)
			require.NoError(t, err)
			names[relativePath] = link
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
		tgzFile := createTgz(t, getTestAppDir(t), tmpDir)
		defer tgzFile.Close()

		destDir := filepath.Join(tmpDir, "untgz1")

		// when
		err := Untgz(destDir, tgzFile, math.MaxInt64)

		// then
		require.NoError(t, err)
		names := readFiles(t, destDir)
		assert.Equal(t, 8, len(names))
		assert.Contains(t, names, "README.md")
		assert.Contains(t, names, "applicationset/latest/kustomization.yaml")
		assert.Contains(t, names, "applicationset/stable/kustomization.yaml")
		assert.Contains(t, names, "applicationset/readme-symlink")
		assert.Equal(t, filepath.Join(destDir, "README.md"), names["applicationset/readme-symlink"])
	})
	t.Run("will protect against symlink exploit", func(t *testing.T) {
		// given
		tmpDir := createTmpDir(t)
		defer deleteTmpDir(t, tmpDir)
		tgzFile := createTgz(t, filepath.Join(getTestDataDir(t), "symlink-exploit"), tmpDir)

		defer tgzFile.Close()

		destDir := filepath.Join(tmpDir, "untgz2")
		err := os.MkdirAll(destDir, 0755)
		require.NoError(t, err)

		// when
		err = Untgz(destDir, tgzFile, math.MaxInt64)

		// then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "out of bounds symlink found")
	})
}

// read returns a map with the filename as key. In case
// the file is a symlink, the value will be populated with
// the target file pointed by the symlink.
func read(tgz *os.File) (map[string]string, error) {
	files := make(map[string]string)
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
		files[header.Name] = header.Linkname
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
