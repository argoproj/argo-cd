package cmp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	pluginclient "github.com/argoproj/argo-cd/v2/cmpserver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/files"
)

// StreamSender defines the contract to send App files over stream
type StreamSender interface {
	Send(*pluginclient.AppStreamRequest) error
}

// StreamReceiver defines the contract for receiving Application's files
// over gRPC stream
type StreamReceiver interface {
	Recv() (*pluginclient.AppStreamRequest, error)
}

// ReceiveApplicationStream will receive the Application's files and the env entries
// over the gRPC stream. Will return the path where the files are saved
// and the env entries if no error.
func ReceiveApplicationStream(ctx context.Context, receiver StreamReceiver) (string, []*pluginclient.EnvEntry, error) {
	header, err := receiver.Recv()
	if err != nil {
		return "", nil, fmt.Errorf("error receiving stream header: %s", err)
	}
	if header == nil || header.GetMetadata() == nil {
		return "", nil, fmt.Errorf("error getting stream metadata: metadata is nil")
	}
	metadata := header.GetMetadata()
	workDir, err := ioutil.TempDir(os.TempDir(), metadata.GetAppName())
	if err != nil {
		return "", nil, fmt.Errorf("error creating workDir: %s", err)
	}

	tgzFile, err := receiveFile(ctx, receiver, metadata.GetChecksum(), workDir)
	if err != nil {
		return "", nil, fmt.Errorf("error receiving file: %s", err)
	}
	err = files.Untgz(workDir, tgzFile)
	if err != nil {
		return "", nil, fmt.Errorf("error decompressing tgz file: %s", err)
	}
	err = os.Remove(tgzFile.Name())
	if err != nil {
		log.Warnf("error removing the tgz file %q: %s", tgzFile.Name(), err)
	}
	return workDir, metadata.GetEnv(), nil
}

// SendApplicationStream will compress the files under the given appPath and send
// them using the plugin stream sender.
func SendApplicationStream(ctx context.Context, appPath string, sender StreamSender, env []string) error {

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
	err = sendFile(ctx, sender, tgz)
	if err != nil {
		return fmt.Errorf("error sending app files to cmp-server: %s", err)
	}
	return nil
}

// sendFile will send the file over the gRPC stream using a
// buffer.
func sendFile(ctx context.Context, sender StreamSender, file *os.File) error {
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

// receiveFile will receive the file from the gRPC stream and save it in the dst folder.
// Returns error if checksum doesn't match the one provided in the fileMetadata.
// It is responsibility of the caller to close the returned file.
func receiveFile(ctx context.Context, receiver StreamReceiver, checksum, dst string) (*os.File, error) {
	fileBuffer := bytes.Buffer{}
	hasher := sha256.New()
	for {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("stream context error: %s", err)
			}
		}
		req, err := receiver.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("stream Recv error: %s", err)
		}
		f := req.GetFile()
		if f == nil {
			return nil, fmt.Errorf("stream request file is nil")
		}
		_, err = fileBuffer.Write(f.Chunk)
		if err != nil {
			return nil, fmt.Errorf("error writing file buffer: %s", err)
		}
		_, err = hasher.Write(f.Chunk)
		if err != nil {
			return nil, fmt.Errorf("error writing hasher: %s", err)
		}
	}
	if hex.EncodeToString(hasher.Sum(nil)) != checksum {
		return nil, fmt.Errorf("file checksum validation error")
	}

	tgzFile, err := ioutil.TempFile(dst, "")
	if err != nil {
		return nil, fmt.Errorf("error creating tgz file: %s", err)
	}
	_, err = fileBuffer.WriteTo(tgzFile)
	if err != nil {
		return nil, fmt.Errorf("error writing tgz file: %s", err)
	}
	_, err = tgzFile.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("tgz seek error: %s", err)
	}
	return tgzFile, nil
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
