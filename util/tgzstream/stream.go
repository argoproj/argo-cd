package tgzstream

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
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

// CloseAndDeleteTempFile closes and deletes a file created in a dedicated
// temporary directory, then removes that directory. The directory is removed
// only when it is a direct UUID-named child of os.TempDir().
func CloseAndDeleteTempFile(f *os.File) {
	if f == nil {
		return
	}
	name := f.Name()
	CloseAndDelete(f)
	tempDir := filepath.Dir(name)
	if !isOwnedTempDir(tempDir) {
		log.Warnf("refusing to remove unowned temporary directory %q for file %q", tempDir, name)
		return
	}
	if err := os.Remove(tempDir); err != nil {
		log.Warnf("error removing temporary directory for file %q: %s", name, err)
	}
}

func isOwnedTempDir(tempDir string) bool {
	relPath, err := filepath.Rel(os.TempDir(), tempDir)
	if err != nil || filepath.Dir(relPath) != "." {
		return false
	}
	_, err = uuid.Parse(relPath)
	return err == nil
}

// CompressFiles will create a tgz file with all contents of appPath
// directory excluding globs in the excluded array. Returns the file
// alongside its sha256 hash to be used as checksum. It is the
// responsibility of the caller to close and delete the file using
// CloseAndDeleteTempFile.
func CompressFiles(appPath string, included []string, excluded []string) (*os.File, int, string, error) {
	appName := filepath.Base(appPath)
	tempDir, err := files.CreateTempDir(os.TempDir())
	if err != nil {
		return nil, 0, "", fmt.Errorf("error creating tempDir for compressing files: %w", err)
	}
	tgzFile, err := os.CreateTemp(tempDir, appName)
	if err != nil {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			log.Warnf("error removing temporary directory %q: %s", tempDir, removeErr)
		}
		return nil, 0, "", fmt.Errorf("error creating app temp tgz file: %w", err)
	}
	hasher := sha256.New()
	filesWritten, err := files.Tgz(appPath, included, excluded, tgzFile, hasher)
	if err != nil {
		CloseAndDeleteTempFile(tgzFile)
		return nil, 0, "", fmt.Errorf("error creating app tgz file: %w", err)
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))
	hasher.Reset()

	// reposition the offset to the beginning of the file for proper reads
	_, err = tgzFile.Seek(0, io.SeekStart)
	if err != nil {
		CloseAndDeleteTempFile(tgzFile)
		return nil, 0, "", fmt.Errorf("error processing tgz file: %w", err)
	}
	return tgzFile, filesWritten, checksum, nil
}
