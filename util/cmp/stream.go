package cmp

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	pluginclient "github.com/argoproj/argo-cd/v2/cmpserver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/files"
)

// StreamSender defines the contract to send App files over stream
type StreamSender interface {
	Send(*pluginclient.AppStreamRequest) error
}

// SendAppFiles will compress the files under the given appPath and send
// them using the plugin stream sender.
func SendAppFiles(ctx context.Context, appPath string, sender StreamSender, env []string) error {

	// compress all files in appPath in tgz
	tgz, checksum, err := compressFiles(appPath)
	if err != nil {
		return fmt.Errorf("error compressing app files: %s", err)
	}
	defer func() {
		tgz.Close()
		os.Remove(tgz.Name())
	}()

	// send metadata first
	mr := appMetadataRequest(appPath, env, checksum)
	err = sender.Send(mr)
	if err != nil {
		return fmt.Errorf("error sending generate manifest metadata to cmp-server: %s", err)
	}

	// send the compressed file
	err = SendAppFile(ctx, sender, tgz)
	if err != nil {
		return fmt.Errorf("error sending app files to cmp-server: %s", err)
	}
	return nil
}

// sendAppFiles will send the file over the gRPC stream using a
// buffer.
func SendAppFile(ctx context.Context, sender StreamSender, file *os.File) error {
	reader := bufio.NewReader(file)
	chunk := make([]byte, 1024)
	for {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("client stream context error: %s", err)
			}
		}
		n, err := reader.Read(chunk)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading tgz file: %s", err)
		}
		fr := appFileRequest(chunk[:n])
		err = sender.Send(fr)
		if err != nil {
			return fmt.Errorf("error sending tgz chunk: %s", err)
		}
	}
	return nil
}

// compressFiles will create a tgz file with all contents of appPath
// directory excluding the .git folder. Returns the file alongside
// its sha256 hash to be used as checksum. It is the resposibility
// of the caller to close the file.
func compressFiles(appPath string) (*os.File, string, error) {
	excluded := []string{".git"}
	appName := filepath.Base(appPath)
	tgzFile, err := ioutil.TempFile(os.TempDir(), appName)
	if err != nil {
		return nil, "", fmt.Errorf("error creating app temp tgz file: %s", err)
	}
	hasher := sha256.New()
	err = files.Tgz(appPath, excluded, tgzFile, hasher)
	if err != nil {
		tgzFile.Close()
		os.Remove(tgzFile.Name())
		return nil, "", fmt.Errorf("error creating app tgz file: %s", err)
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))

	// reposition the offset to the beginning of the file for proper reads
	_, err = tgzFile.Seek(0, io.SeekStart)
	if err != nil {
		return nil, "", fmt.Errorf("error processing tgz file: %s", err)
	}
	return tgzFile, checksum, nil
}

// appFileRequest build the file payload for the ManifestRequest
func appFileRequest(chunk []byte) *pluginclient.AppStreamRequest {
	return &pluginclient.AppStreamRequest{
		Request: &pluginclient.AppStreamRequest_File{
			File: &pluginclient.File{
				Chunk: chunk,
			},
		},
	}
}

// appMetadataRequest build the metadata payload for the ManifestRequest
func appMetadataRequest(appPath string, env []string, checksum string) *pluginclient.AppStreamRequest {
	return &pluginclient.AppStreamRequest{
		Request: &pluginclient.AppStreamRequest_Metadata{
			Metadata: &pluginclient.ManifestRequestMetadata{
				AppName:  filepath.Base(appPath),
				Env:      toEnvEntry(env),
				Checksum: checksum,
			},
		},
	}
}

func toEnvEntry(envVars []string) []*pluginclient.EnvEntry {
	envEntry := make([]*pluginclient.EnvEntry, 0)
	for _, env := range envVars {
		pair := strings.Split(env, "=")
		if len(pair) != 2 {
			continue
		}
		envEntry = append(envEntry, &pluginclient.EnvEntry{Name: pair[0], Value: pair[1]})
	}
	return envEntry
}
