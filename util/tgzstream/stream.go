package tgzstream

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/util/io/files"
)

func CloseAndDelete(f *os.File) {
	if f == nil {
		return
	}
	if err := f.Close(); err != nil {
		log.Warnf("error closing file %q: %s", f.Name(), err)
	}
	if err := os.Remove(f.Name()); err != nil {
		log.Warnf("error removing file %q: %s", f.Name(), err)
	}
}

// ExtraFile describes a file to inject into the tarball at a specific path
type ExtraFile struct {
	DstPath string
	SrcPath string
}

// CompressFiles will create a tgz file with all contents of appPath
// directory excluding globs in the excluded array. Returns the file
// alongside its sha256 hash to be used as checksum. It is the
// responsibility of the caller to close the file.
func CompressFiles(appPath string, included []string, excluded []string, extraFiles []ExtraFile) (*os.File, int, string, error) {
	for _, ef := range extraFiles {
		if filepath.IsAbs(ef.DstPath) {
			return nil, 0, "", fmt.Errorf("extra file DstPath must be relative, got %q", ef.DstPath)
		}
		if strings.Contains(ef.DstPath, "..") {
			return nil, 0, "", fmt.Errorf("extra file DstPath must not contain '..', got %q", ef.DstPath)
		}
		fi, err := os.Stat(ef.SrcPath)
		if err != nil {
			return nil, 0, "", fmt.Errorf("extra file %q: %w", ef.SrcPath, err)
		}
		if fi.IsDir() {
			return nil, 0, "", fmt.Errorf("extra file %q is a directory; only regular files are allowed", ef.SrcPath)
		}
	}

	appName := filepath.Base(appPath)
	tempDir, err := files.CreateTempDir(os.TempDir())
	if err != nil {
		return nil, 0, "", fmt.Errorf("error creating tempDir for compressing files: %w", err)
	}
	tgzFile, err := os.CreateTemp(tempDir, appName)
	if err != nil {
		return nil, 0, "", fmt.Errorf("error creating app temp tgz file: %w", err)
	}

	hasher := sha256.New()
	gzw := gzip.NewWriter(io.MultiWriter(tgzFile, hasher))
	tw := tar.NewWriter(gzw)

	filesWritten, err := files.WriteDirToTar(tw, appPath, included, excluded)
	if err != nil {
		CloseAndDelete(tgzFile)
		return nil, 0, "", fmt.Errorf("error creating app tgz file: %w", err)
	}

	for _, ef := range extraFiles {
		if err := appendFileToTar(tw, ef.SrcPath, ef.DstPath); err != nil {
			CloseAndDelete(tgzFile)
			return nil, 0, "", fmt.Errorf("error appending extra file %q: %w", ef.SrcPath, err)
		}
	}

	if err := tw.Close(); err != nil {
		CloseAndDelete(tgzFile)
		return nil, 0, "", fmt.Errorf("error closing tar writer: %w", err)
	}
	if err := gzw.Close(); err != nil {
		CloseAndDelete(tgzFile)
		return nil, 0, "", fmt.Errorf("error closing gzip writer: %w", err)
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))

	// reposition the offset to the beginning of the file for proper reads
	if _, err = tgzFile.Seek(0, io.SeekStart); err != nil {
		CloseAndDelete(tgzFile)
		return nil, 0, "", fmt.Errorf("error processing tgz file: %w", err)
	}
	return tgzFile, filesWritten, checksum, nil
}

func appendFileToTar(tw *tar.Writer, srcPath, dstPath string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	hdr := &tar.Header{
		Name:    dstPath,
		Mode:    0o644, // rw-r--r--
		Size:    fi.Size(),
		ModTime: fi.ModTime(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = io.Copy(tw, f)
	return err
}
