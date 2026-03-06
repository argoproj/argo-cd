package tgzstream

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// listTarPaths returns the list of file names stored in the tgz file.
func listTarPaths(t *testing.T, f *os.File) []string {
	t.Helper()
	_, err := f.Seek(0, io.SeekStart)
	require.NoError(t, err)

	gzr, err := gzip.NewReader(f)
	require.NoError(t, err)
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var paths []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		paths = append(paths, hdr.Name)
	}
	return paths
}

func makeDir(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(d, "values.yaml"), []byte("key: val\n"), 0o644))
	return d
}

func makeExtraFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "extra-*.yaml")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

func TestCompressFiles_ExtraFileInjected(t *testing.T) {
	appDir := makeDir(t)
	extraSrc := makeExtraFile(t, "override: true\n")

	extraFiles := []ExtraFile{{DstPath: "_argocd_extra_values/0.yaml", SrcPath: extraSrc}}
	tgzFile, filesWritten, checksum, err := CompressFiles(appDir, nil, nil, extraFiles)
	require.NoError(t, err)
	defer CloseAndDelete(tgzFile)

	assert.Positive(t, filesWritten)
	assert.NotEmpty(t, checksum)

	paths := listTarPaths(t, tgzFile)
	assert.Contains(t, paths, "_argocd_extra_values/0.yaml", "extra file must appear at the expected tar path")
	assert.Contains(t, paths, "values.yaml", "chart file must still be present")
}

func TestCompressFiles_NoExtraFiles(t *testing.T) {
	appDir := makeDir(t)
	tgzFile, filesWritten, checksum, err := CompressFiles(appDir, nil, nil, nil)
	require.NoError(t, err)
	defer CloseAndDelete(tgzFile)

	assert.Equal(t, 1, filesWritten)
	assert.NotEmpty(t, checksum)

	paths := listTarPaths(t, tgzFile)
	assert.Contains(t, paths, "values.yaml")
	assert.NotContains(t, paths, "_argocd_extra_values/0.yaml", "no extra files expected")
}

func TestCompressFiles_AbsoluteDstPathRejected(t *testing.T) {
	appDir := makeDir(t)
	extraSrc := makeExtraFile(t, "x: 1\n")

	_, _, _, err := CompressFiles(appDir, nil, nil, []ExtraFile{{DstPath: "/abs/path.yaml", SrcPath: extraSrc}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be relative")
}

func TestCompressFiles_DotDotInDstPathRejected(t *testing.T) {
	appDir := makeDir(t)
	extraSrc := makeExtraFile(t, "x: 1\n")

	_, _, _, err := CompressFiles(appDir, nil, nil, []ExtraFile{{DstPath: "../escape.yaml", SrcPath: extraSrc}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain '..'")
}

func TestCompressFiles_NonExistentSrcRejected(t *testing.T) {
	appDir := makeDir(t)

	_, _, _, err := CompressFiles(appDir, nil, nil, []ExtraFile{{DstPath: "extra.yaml", SrcPath: "/nonexistent/file.yaml"}})
	require.Error(t, err)
}

func TestCompressFiles_DirectorySrcRejected(t *testing.T) {
	appDir := makeDir(t)
	srcDir := t.TempDir()

	_, _, _, err := CompressFiles(appDir, nil, nil, []ExtraFile{{DstPath: "extra.yaml", SrcPath: srcDir}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is a directory")
}

func TestCompressFiles_MultipleExtraFiles(t *testing.T) {
	appDir := makeDir(t)
	extra0 := makeExtraFile(t, "env: staging\n")
	extra1 := makeExtraFile(t, "env: prod\n")

	extraFiles := []ExtraFile{
		{DstPath: "_argocd_extra_values/0.yaml", SrcPath: extra0},
		{DstPath: "_argocd_extra_values/1.yaml", SrcPath: extra1},
	}
	tgzFile, _, _, err := CompressFiles(appDir, nil, nil, extraFiles)
	require.NoError(t, err)
	defer CloseAndDelete(tgzFile)

	paths := listTarPaths(t, tgzFile)
	assert.Contains(t, paths, "_argocd_extra_values/0.yaml")
	assert.Contains(t, paths, "_argocd_extra_values/1.yaml")
}
