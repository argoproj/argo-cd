package files_test

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/test"
	"github.com/argoproj/argo-cd/v3/util/io/files"
)

func TestTgz(t *testing.T) {
	t.Parallel()

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
		filesWritten, err := files.Tgz(getTestAppDir(t), nil, exclusions, f.file)

		// then
		assert.Equal(t, 3, filesWritten)
		require.NoError(t, err)
		prepareRead(f)
		files, err := read(f.file)
		require.NoError(t, err)
		assert.Len(t, files, 8)
		assert.Contains(t, files, "README.md")
		assert.Contains(t, files, "applicationset/latest/kustomization.yaml")
		assert.Contains(t, files, "applicationset/stable/kustomization.yaml")
		assert.Contains(t, files, "applicationset/readme-symlink")
		assert.Equal(t, "../README.md", files["applicationset/readme-symlink"])
	})
	t.Run("will exclude files from the exclusion list", func(t *testing.T) {
		// given
		t.Parallel()
		exclusions := []string{"README.md"}
		f := setup(t)
		defer teardown(f)

		// when
		filesWritten, err := files.Tgz(getTestAppDir(t), nil, exclusions, f.file)

		// then
		assert.Equal(t, 2, filesWritten)
		require.NoError(t, err)
		prepareRead(f)
		files, err := read(f.file)
		require.NoError(t, err)
		assert.Len(t, files, 7)
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
		filesWritten, err := files.Tgz(getTestAppDir(t), nil, exclusions, f.file)

		// then
		assert.Equal(t, 1, filesWritten)
		require.NoError(t, err)
		prepareRead(f)
		files, err := read(f.file)
		require.NoError(t, err)
		assert.Len(t, files, 5)
		assert.Contains(t, files, "applicationset/stable/kustomization.yaml")
	})
	t.Run("will match the inclusion list against file names, not paths", func(t *testing.T) {
		// given
		t.Parallel()
		inclusions := []string{"*.yaml"}
		f := setup(t)
		defer teardown(f)

		// when
		filesWritten, err := files.Tgz(getTestAppDir(t), inclusions, nil, f.file)

		// then
		assert.Equal(t, 2, filesWritten)
		require.NoError(t, err)
		prepareRead(f)
		files, err := read(f.file)
		require.NoError(t, err)
		assert.Contains(t, files, "applicationset/latest/kustomization.yaml")
		assert.Contains(t, files, "applicationset/stable/kustomization.yaml")
		assert.NotContains(t, files, "README.md")
	})
}

func TestTgzWithOptions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		opts         files.TarOptions
		filesWritten int
		expected     []string
		unexpected   []string
	}{
		{
			name:         "will include a directory with everything below it",
			opts:         files.TarOptions{IncludePaths: []string{"applicationset/stable"}},
			filesWritten: 1,
			expected:     []string{"applicationset/stable/kustomization.yaml"},
			unexpected:   []string{"README.md", "applicationset/latest", "applicationset/latest/kustomization.yaml"},
		},
		{
			name:         "will include a single file",
			opts:         files.TarOptions{IncludePaths: []string{"README.md"}},
			filesWritten: 1,
			expected:     []string{"README.md"},
			unexpected:   []string{"applicationset", "applicationset/stable/kustomization.yaml"},
		},
		{
			name:         "will include the files matched by a glob",
			opts:         files.TarOptions{IncludePaths: []string{"applicationset/*/kustomization.yaml"}},
			filesWritten: 2,
			expected:     []string{"applicationset/latest/kustomization.yaml", "applicationset/stable/kustomization.yaml"},
			unexpected:   []string{"README.md", "applicationset/readme-symlink"},
		},
		{
			name:         "will include everything below a directory matched by a glob",
			opts:         files.TarOptions{IncludePaths: []string{"applicationset/*"}},
			filesWritten: 2,
			expected: []string{
				"applicationset/latest/kustomization.yaml",
				"applicationset/stable/kustomization.yaml",
				"applicationset/readme-symlink",
			},
			unexpected: []string{"README.md"},
		},
		{
			name: "will not include the target of an included symlink",
			opts: files.TarOptions{IncludePaths: []string{"applicationset"}},
			// The symlink is included, but its target has to be selected on its own.
			filesWritten: 2,
			expected:     []string{"applicationset/readme-symlink"},
			unexpected:   []string{"README.md"},
		},
		{
			name:         "will include a symlink together with its selected target",
			opts:         files.TarOptions{IncludePaths: []string{"applicationset", "README.md"}},
			filesWritten: 3,
			expected:     []string{"applicationset/readme-symlink", "README.md"},
		},
		{
			name:         "will include everything for the archive root",
			opts:         files.TarOptions{IncludePaths: []string{"."}},
			filesWritten: 3,
			expected:     []string{"README.md", "applicationset/latest/kustomization.yaml"},
		},
		{
			name:         "will write no file when no path matches",
			opts:         files.TarOptions{IncludePaths: []string{"does-not-exist"}},
			filesWritten: 0,
			unexpected:   []string{"README.md", "applicationset", "applicationset/stable/kustomization.yaml"},
		},
		{
			name: "will exclude files from an included path",
			opts: files.TarOptions{
				IncludePaths: []string{"applicationset/latest", "applicationset/stable"},
				Exclusions:   []string{"applicationset/latest"},
			},
			filesWritten: 1,
			expected:     []string{"applicationset/stable/kustomization.yaml"},
			unexpected:   []string{"applicationset/latest/kustomization.yaml"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			t.Parallel()
			f, err := os.CreateTemp(getTestDataDir(t), "")
			require.NoError(t, err)
			defer func() {
				f.Close()
				os.Remove(f.Name())
			}()

			// when
			filesWritten, err := files.TgzWithOptions(getTestAppDir(t), tc.opts, f)

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.filesWritten, filesWritten)
			_, err = f.Seek(0, io.SeekStart)
			require.NoError(t, err)
			names, err := read(f)
			require.NoError(t, err)
			for _, name := range tc.expected {
				assert.Contains(t, names, name)
			}
			for _, name := range tc.unexpected {
				assert.NotContains(t, names, name)
			}
		})
	}

	t.Run("will keep the link name of an included symlink", func(t *testing.T) {
		// given
		t.Parallel()
		f, err := os.CreateTemp(getTestDataDir(t), "")
		require.NoError(t, err)
		defer func() {
			f.Close()
			os.Remove(f.Name())
		}()

		// when
		_, err = files.TgzWithOptions(getTestAppDir(t), files.TarOptions{IncludePaths: []string{"applicationset"}}, f)

		// then
		require.NoError(t, err)
		_, err = f.Seek(0, io.SeekStart)
		require.NoError(t, err)
		names, err := read(f)
		require.NoError(t, err)
		assert.Equal(t, "../README.md", names["applicationset/readme-symlink"])
	})
}

func TestUntgz(t *testing.T) {
	createTmpDir := func(t *testing.T) string {
		t.Helper()
		tmpDir, err := os.MkdirTemp(getTestDataDir(t), "")
		require.NoErrorf(t, err, "error creating tmpDir: %s", err)
		return tmpDir
	}
	deleteTmpDir := func(t *testing.T, dirname string) {
		t.Helper()
		assert.NoError(t, os.RemoveAll(dirname), "error removing tmpDir")
	}
	createTgz := func(t *testing.T, fromDir, destDir string) *os.File {
		t.Helper()
		f, err := os.CreateTemp(destDir, "")
		require.NoErrorf(t, err, "error creating tmpFile in %q: %s", destDir, err)
		_, err = files.Tgz(fromDir, nil, nil, f)
		require.NoErrorf(t, err, "error during Tgz: %s", err)
		_, err = f.Seek(0, io.SeekStart)
		require.NoErrorf(t, err, "seek error: %s", err)
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
		require.NoErrorf(t, err, "error reading files: %s", err)
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
		err := files.Untgz(destDir, tgzFile, math.MaxInt64, false)

		// then
		require.NoError(t, err)
		names := readFiles(t, destDir)
		assert.Len(t, names, 8)
		assert.Contains(t, names, "README.md")
		assert.Contains(t, names, "applicationset/latest/kustomization.yaml")
		assert.Contains(t, names, "applicationset/stable/kustomization.yaml")
		assert.Contains(t, names, "applicationset/readme-symlink")
		assert.Equal(t, "../README.md", names["applicationset/readme-symlink"])
	})
	t.Run("will protect against symlink exploit", func(t *testing.T) {
		// given
		tmpDir := createTmpDir(t)
		defer deleteTmpDir(t, tmpDir)
		tgzFile := createTgz(t, filepath.Join(getTestDataDir(t), "symlink-exploit"), tmpDir)

		defer tgzFile.Close()

		destDir := filepath.Join(tmpDir, "untgz2")

		// when
		err := files.Untgz(destDir, tgzFile, math.MaxInt64, false)

		// then
		assert.ErrorContains(t, err, "illegal filepath in symlink")
	})
	t.Run("will protect against symlink exploit when relativizing symlinks", func(t *testing.T) {
		// given
		tmpDir := createTmpDir(t)
		defer deleteTmpDir(t, tmpDir)
		tgzFile := createTgz(t, filepath.Join(getTestDataDir(t), "symlink-exploit"), tmpDir)

		defer tgzFile.Close()

		destDir := filepath.Join(tmpDir, "untgz2")

		// when
		err := files.Untgz(destDir, tgzFile, math.MaxInt64, false)

		// then
		assert.ErrorContains(t, err, "illegal filepath in symlink")
	})

	t.Run("preserves file mode", func(t *testing.T) {
		// given
		tmpDir := createTmpDir(t)
		defer deleteTmpDir(t, tmpDir)

		scriptFileName := "script.sh"
		srcDir := filepath.Join(getTestDataDir(t), "executable")
		srcScriptFileInfo, err := os.Stat(path.Join(srcDir, scriptFileName))
		require.NoError(t, err)

		tgzFile := createTgz(t, srcDir, tmpDir)
		defer tgzFile.Close()

		destDir := filepath.Join(tmpDir, "untgz1")

		// when
		err = files.Untgz(destDir, tgzFile, math.MaxInt64, true)
		require.NoError(t, err)
		// then
		scriptFileInfo, err := os.Stat(path.Join(destDir, scriptFileName))
		require.NoError(t, err)
		assert.Equal(t, srcScriptFileInfo.Mode(), scriptFileInfo.Mode())
	})
	t.Run("relativizes symlinks", func(t *testing.T) {
		// given
		tmpDir := createTmpDir(t)
		defer deleteTmpDir(t, tmpDir)
		tgzFile := createTgz(t, getTestAppDir(t), tmpDir)
		defer tgzFile.Close()

		destDir := filepath.Join(tmpDir, "symlink-relativize")

		// when
		err := files.Untgz(destDir, tgzFile, math.MaxInt64, false)

		// then
		require.NoError(t, err)
		names := readFiles(t, destDir)
		assert.Equal(t, "../README.md", names["applicationset/readme-symlink"])
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
	t.Helper()
	return filepath.Join(getTestDataDir(t), "app")
}

// getTestDataDir will return the full path of the testdata dir
// under the running test folder.
func getTestDataDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(test.GetTestDir(t), "testdata")
}
