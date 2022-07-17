package stream

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/util/io/files"
	log "github.com/sirupsen/logrus"
)

type StreamSender interface {
	Send(*applicationpkg.ApplicationManifestQueryWithFilesWrapper) error
}

func SendFile(ctx context.Context, sender StreamSender, file *os.File) error {
	reader := bufio.NewReader(file)
	chunk := make([]byte, 1024)
	for {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("client stream context error: %w", err)
			}
		}
		n, err := reader.Read(chunk)
		if n > 0 {
			fr := &applicationpkg.ApplicationManifestQueryWithFilesWrapper{
				Part: &applicationpkg.ApplicationManifestQueryWithFilesWrapper_Chunk{
					Chunk: &applicationpkg.FileChunk{
						Chunk: chunk[:n],
					},
				},
			}
			if e := sender.Send(fr); e != nil {
				return fmt.Errorf("error sending stream: %w", err)
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("buffer reader error: %w", err)
		}
	}
	return nil
}

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

// compressFiles will create a tgz file with all contents of appPath
// directory excluding globs in the excluded array. Returns the file
// alongside its sha256 hash to be used as checksum. It is the
// responsibility of the caller to close the file.
func CompressFiles(appPath string, excluded []string) (*os.File, string, error) {
	appName := filepath.Base(appPath)
	tempDir, err := files.CreateTempDir(os.TempDir())
	if err != nil {
		return nil, "", fmt.Errorf("error creating tempDir for compressing files: %s", err)
	}
	tgzFile, err := os.CreateTemp(tempDir, appName)
	if err != nil {
		return nil, "", fmt.Errorf("error creating app temp tgz file: %w", err)
	}
	hasher := sha256.New()
	err = files.Tgz(appPath, excluded, tgzFile, hasher)
	if err != nil {
		CloseAndDelete(tgzFile)
		return nil, "", fmt.Errorf("error creating app tgz file: %w", err)
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))
	hasher.Reset()

	// reposition the offset to the beginning of the file for proper reads
	_, err = tgzFile.Seek(0, io.SeekStart)
	if err != nil {
		CloseAndDelete(tgzFile)
		return nil, "", fmt.Errorf("error processing tgz file: %w", err)
	}
	return tgzFile, checksum, nil
}
