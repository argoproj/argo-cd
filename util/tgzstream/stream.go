package tgzstream

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/util/io/files"
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

// CompressFiles will create a tgz file with all contents of appPath
// directory excluding globs in the excluded array. Returns the file
// alongside its sha256 hash to be used as checksum. It is the
// responsibility of the caller to close the file.
func CompressFiles(appPath string, included []string, excluded []string) (*os.File, int, string, error) {
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
	filesWritten, err := files.Tgz(appPath, included, excluded, tgzFile, hasher)
	if err != nil {
		CloseAndDelete(tgzFile)
		return nil, 0, "", fmt.Errorf("error creating app tgz file: %w", err)
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))
	hasher.Reset()

	// reposition the offset to the beginning of the file for proper reads
	_, err = tgzFile.Seek(0, io.SeekStart)
	if err != nil {
		CloseAndDelete(tgzFile)
		return nil, 0, "", fmt.Errorf("error processing tgz file: %w", err)
	}
	return tgzFile, filesWritten, checksum, nil
}
