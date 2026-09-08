package files_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
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
	t.Run("resolves destination path symlinks before inbound checks", func(t *testing.T) {
		// Models macOS where extract roots under /var resolve to /private/var.
		// Without resolving dstPath first, in-bounds archive symlinks fail Inbound
		// because EvalSymlinks on the link target rewrites /var to /private/var.

		// given
		tmpDir := createTmpDir(t)
		defer deleteTmpDir(t, tmpDir)

		realDest := filepath.Join(tmpDir, "real-dest")
		require.NoError(t, os.MkdirAll(realDest, 0o755))

		linkDest := filepath.Join(tmpDir, "link-dest")
		require.NoError(t, os.Symlink(realDest, linkDest))

		tgzFile := createTgz(t, getTestAppDir(t), tmpDir)
		defer tgzFile.Close()

		// when
		err := files.Untgz(linkDest, tgzFile, math.MaxInt64, false)

		// then
		require.NoError(t, err)
		names := readFiles(t, realDest)
		assert.Len(t, names, 8)
		assert.Contains(t, names, "README.md")
		assert.Contains(t, names, "applicationset/latest/kustomization.yaml")
		assert.Contains(t, names, "applicationset/stable/kustomization.yaml")
		assert.Contains(t, names, "applicationset/readme-symlink")
		assert.Equal(t, "../README.md", names["applicationset/readme-symlink"])
	})
	t.Run("resolves symlinks for non-existent final component", func(t *testing.T) {
		// given

		// tmpdir/link -> tmpdir/dest
		tmpDir := createTmpDir(t)
		defer deleteTmpDir(t, tmpDir)

		realDest := filepath.Join(tmpDir, "dest")
		require.NoError(t, os.MkdirAll(realDest, 0o755))

		linkDest := filepath.Join(tmpDir, "link")
		require.NoError(t, os.Symlink(realDest, linkDest))

		realTarget := filepath.Join(realDest, "non-existent")
		linkTarget := filepath.Join(linkDest, "non-existent")

		tgzFile := createTgz(t, getTestAppDir(t), tmpDir)
		defer tgzFile.Close()

		// when
		err := files.Untgz(linkTarget, tgzFile, math.MaxInt64, false)

		// then
		require.NoError(t, err)

		names := readFiles(t, realTarget)
		assert.Len(t, names, 8)
		assert.Contains(t, names, "README.md")
		assert.Contains(t, names, "applicationset/latest/kustomization.yaml")
		assert.Contains(t, names, "applicationset/stable/kustomization.yaml")
		assert.Contains(t, names, "applicationset/readme-symlink")
		assert.Equal(t, "../README.md", names["applicationset/readme-symlink"])

		names = readFiles(t, linkTarget)
		assert.Len(t, names, 8)
		assert.Contains(t, names, "README.md")
		assert.Contains(t, names, "applicationset/latest/kustomization.yaml")
		assert.Contains(t, names, "applicationset/stable/kustomization.yaml")
		assert.Contains(t, names, "applicationset/readme-symlink")
		assert.Equal(t, "../README.md", names["applicationset/readme-symlink"])
	})
	t.Run("will fail if not absolute dstPath", func(t *testing.T) {
		// given
		dummyTgz := prepareCraftedTgz(t)
		relativePath := "./relative/path"

		// when
		err := files.Untgz(relativePath, bytes.NewReader(dummyTgz), math.MaxInt64, false)

		// then
		assert.ErrorContains(t, err, "dstPath points to a relative path")
	})
	t.Run("will protect against zip-slip in names", func(t *testing.T) {
		names := []string{
			"../outside",
			"../../outside",
			"foo/../../outside",
		}

		for _, name := range names {
			t.Run(name, func(t *testing.T) {
				tmpDir := createTmpDir(t)
				defer deleteTmpDir(t, tmpDir)
				destDir := filepath.Join(tmpDir, "untgz")

				tar := prepareCraftedTgz(t, func(tw *tar.Writer) {
					writeTarFile(t, tw, name, "evil")
				})
				err := files.Untgz(destDir, bytes.NewReader(tar), math.MaxInt64, false)
				require.Error(t, err)
				// when run with tarinsecurepath=0 (Makefile does this), the error message is "insecure file path" from tar reader Next()
				// when run without (e.g. directly go test) the error is from the os.Root API
				assert.True(t,
					strings.Contains(err.Error(), "insecure file path") ||
						strings.Contains(err.Error(), "path escapes from parent"),
					"unexpected error: %s", err.Error())
			})
		}
	})
	t.Run("allows for symlink to non existing target file", func(t *testing.T) {
		// given
		tmpDir := createTmpDir(t)
		defer deleteTmpDir(t, tmpDir)
		destDir := filepath.Join(tmpDir, "untgz")
		require.NoError(t, os.MkdirAll(destDir, 0o755))

		tar := prepareCraftedTgz(t, func(tw *tar.Writer) {
			writeTarSymlink(t, tw, "link.txt", "non-existent.txt")
		})

		// when
		err := files.Untgz(destDir, bytes.NewReader(tar), math.MaxInt64, false)
		require.NoError(t, err)

		// then
		names := readFiles(t, destDir)
		assert.Len(t, names, 2)
		assert.Equal(t, "non-existent.txt", names["link.txt"])
	})
	t.Run("allows for symlink to file first", func(t *testing.T) {
		// given
		tmpDir := createTmpDir(t)
		defer deleteTmpDir(t, tmpDir)
		destDir := filepath.Join(tmpDir, "untgz")
		require.NoError(t, os.MkdirAll(destDir, 0o755))

		tar := prepareCraftedTgz(t, func(tw *tar.Writer) {
			// first write the symlink, then the file so when reading
			// there is the link entry first
			writeTarSymlink(t, tw, "link.txt", "future-data.txt")
			writeTarFile(t, tw, "future-data.txt", "content")
		})

		// when
		err := files.Untgz(destDir, bytes.NewReader(tar), math.MaxInt64, false)
		require.NoError(t, err)

		// then
		names := readFiles(t, destDir)
		assert.Len(t, names, 3)
		assert.Equal(t, "future-data.txt", names["link.txt"])
		assert.Empty(t, names["future-data.txt"])
		content, err := os.ReadFile(filepath.Join(destDir, "link.txt"))
		require.NoError(t, err)
		assert.Equal(t, "content", string(content))
	})
	t.Run("will protect against symlink chain escape", func(t *testing.T) {
		// given
		tmpDir := createTmpDir(t)
		defer deleteTmpDir(t, tmpDir)
		destDir := filepath.Join(tmpDir, "untgz")
		require.NoError(t, os.MkdirAll(destDir, 0o755))

		tar := prepareCraftedTgz(t, func(tw *tar.Writer) {
			writeTarSymlink(t, tw, "link", "link2")
			writeTarSymlink(t, tw, "link2", "../../../outside")
		})

		// when
		err := files.Untgz(destDir, bytes.NewReader(tar), math.MaxInt64, false)
		assert.ErrorContains(t, err, "illegal filepath in symlink")
	})
	t.Run("rewrites absolute linkname into in-bounds relative target", func(t *testing.T) {
		// given
		tmpDir := createTmpDir(t)
		defer deleteTmpDir(t, tmpDir)
		destDir := filepath.Join(tmpDir, "untgz")
		require.NoError(t, os.MkdirAll(destDir, 0o755))

		asboluteOutsidePath := filepath.Join(tmpDir, "outside")

		tar := prepareCraftedTgz(t, func(tw *tar.Writer) {
			writeTarSymlink(t, tw, "link", asboluteOutsidePath)
		})

		// when
		err := files.Untgz(destDir, bytes.NewReader(tar), math.MaxInt64, false)
		// then
		require.NoError(t, err)
		names := readFiles(t, destDir)
		assert.Len(t, names, 2)
		assert.Equal(t, strings.TrimPrefix(asboluteOutsidePath, "/"), names["link"])
	})
	t.Run("has correct file content", func(t *testing.T) {
		// given
		tmpDir := createTmpDir(t)
		defer deleteTmpDir(t, tmpDir)
		destDir := filepath.Join(tmpDir, "untgz")

		tar := prepareCraftedTgz(t, func(tw *tar.Writer) {
			writeTarFile(t, tw, "file", "content")
		})

		// when
		err := files.Untgz(destDir, bytes.NewReader(tar), math.MaxInt64, false)
		require.NoError(t, err)

		// then
		content, err := os.ReadFile(filepath.Join(destDir, "file"))
		require.NoError(t, err)
		assert.Equal(t, "content", string(content))
	})
	t.Run("creates nested directories", func(t *testing.T) {
		// given
		tmpDir := createTmpDir(t)
		defer deleteTmpDir(t, tmpDir)
		destDir := filepath.Join(tmpDir, "untgz")

		tar := prepareCraftedTgz(t, func(tw *tar.Writer) {
			writeTarDir(t, tw, "dir")
			writeTarDir(t, tw, "dir2/nested")
		})

		// when
		err := files.Untgz(destDir, bytes.NewReader(tar), math.MaxInt64, false)
		require.NoError(t, err)

		// then
		names := readFiles(t, destDir)
		assert.Len(t, names, 4)
		assert.Empty(t, names["dir"])
		assert.Empty(t, names["dir2"])
		assert.Empty(t, names["dir2/nested"])
		for _, name := range []string{"dir", "dir2", "dir2/nested"} {
			stat, err := os.Stat(filepath.Join(destDir, name))
			require.NoError(t, err)
			assert.True(t, stat.IsDir())
		}
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

func prepareCraftedTgz(t *testing.T, entries ...func(*tar.Writer)) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	for _, writeEntry := range entries {
		writeEntry(tw)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gzw.Close())
	return buf.Bytes()
}

func writeTarFile(t *testing.T, tw *tar.Writer, name, content string) {
	t.Helper()

	header := &tar.Header{
		Name:     name,
		Mode:     0o666,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}
	require.NoError(t, tw.WriteHeader(header))
	_, err := tw.Write([]byte(content))
	require.NoError(t, err)
}

func writeTarSymlink(t *testing.T, tw *tar.Writer, name, target string) {
	t.Helper()
	header := &tar.Header{
		Name:     name,
		Mode:     0o777,
		Typeflag: tar.TypeSymlink,
		Linkname: target,
	}
	require.NoError(t, tw.WriteHeader(header))
}

func writeTarDir(t *testing.T, tw *tar.Writer, name string) {
	t.Helper()
	header := &tar.Header{
		Name:     name,
		Mode:     0o755,
		Typeflag: tar.TypeDir,
	}
	require.NoError(t, tw.WriteHeader(header))
}
