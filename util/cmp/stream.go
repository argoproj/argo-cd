package cmp

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	pluginclient "github.com/argoproj/argo-cd/v2/cmpserver/apiclient"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/io/files"
	"github.com/argoproj/argo-cd/v2/util/tgzstream"
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

// ReceiveRepoStream will receive the repository files and save them
// in destDir. Will return the stream metadata if no error. Metadata
// will be nil in case of errors.
func ReceiveRepoStream(ctx context.Context, receiver StreamReceiver, destDir string, preserveFileMode bool) (*pluginclient.ManifestRequestMetadata, error) {
	header, err := receiver.Recv()
	if err != nil {
		return nil, fmt.Errorf("error receiving stream header: %w", err)
	}
	if header == nil || header.GetMetadata() == nil {
		return nil, fmt.Errorf("error getting stream metadata: metadata is nil")
	}
	metadata := header.GetMetadata()

	tgzFile, err := receiveFile(ctx, receiver, metadata.GetChecksum(), destDir)
	if err != nil {
		return nil, fmt.Errorf("error receiving tgz file: %w", err)
	}
	err = files.Untgz(destDir, tgzFile, math.MaxInt64, preserveFileMode)
	if err != nil {
		return nil, fmt.Errorf("error decompressing tgz file: %w", err)
	}
	err = os.Remove(tgzFile.Name())
	if err != nil {
		log.Warnf("error removing the tgz file %q: %s", tgzFile.Name(), err)
	}
	return metadata, nil
}

// SenderOption defines the function type to by used by specific options
type SenderOption func(*senderOption)

type senderOption struct {
	chunkSize   int
	tarDoneChan chan<- bool
}

func newSenderOption(opts ...SenderOption) *senderOption {
	so := &senderOption{
		chunkSize: common.GetCMPChunkSize(),
	}
	for _, opt := range opts {
		opt(so)
	}
	return so
}

func WithTarDoneChan(ch chan<- bool) SenderOption {
	return func(opt *senderOption) {
		opt.tarDoneChan = ch
	}
}

// SendRepoStream will compress the files under the given rootPath and send
// them using the plugin stream sender.
func SendRepoStream(ctx context.Context, appPath, rootPath string, sender StreamSender, env []string, excludedGlobs []string, opts ...SenderOption) error {
	opt := newSenderOption(opts...)

	tgz, mr, err := GetCompressedRepoAndMetadata(rootPath, appPath, env, excludedGlobs, opt)
	if err != nil {
		return err
	}
	defer tgzstream.CloseAndDelete(tgz)
	err = sender.Send(mr)
	if err != nil {
		return fmt.Errorf("error sending generate manifest metadata to cmp-server: %w", err)
	}

	// send the compressed file
	err = sendFile(ctx, sender, tgz, opt)
	if err != nil {
		return fmt.Errorf("error sending tgz file to cmp-server: %w", err)
	}
	return nil
}

func GetCompressedRepoAndMetadata(rootPath string, appPath string, env []string, excludedGlobs []string, opt *senderOption) (*os.File, *pluginclient.AppStreamRequest, error) {
	// compress all files in rootPath in tgz
	tgz, filesWritten, checksum, err := tgzstream.CompressFiles(rootPath, nil, excludedGlobs)
	if err != nil {
		return nil, nil, fmt.Errorf("error compressing repo files: %w", err)
	}
	if filesWritten == 0 {
		return nil, nil, fmt.Errorf("no files to send(%s)", rootPath)
	}
	if opt != nil && opt.tarDoneChan != nil {
		opt.tarDoneChan <- true
		close(opt.tarDoneChan)
	}

	fi, err := tgz.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("error getting tgz stat: %w", err)
	}
	appRelPath, err := files.RelativePath(appPath, rootPath)
	if err != nil {
		return nil, nil, fmt.Errorf("error building app relative path: %w", err)
	}
	// send metadata first
	mr := appMetadataRequest(filepath.Base(appPath), appRelPath, env, checksum, fi.Size())
	return tgz, mr, err
}

// sendFile will send the file over the gRPC stream using a
// buffer.
func sendFile(ctx context.Context, sender StreamSender, file *os.File, opt *senderOption) error {
	reader := bufio.NewReader(file)
	chunk := make([]byte, opt.chunkSize)
	for {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("client stream context error: %w", err)
			}
		}
		n, err := reader.Read(chunk)
		if n > 0 {
			fr := AppFileRequest(chunk[:n])
			if e := sender.Send(fr); e != nil {
				return fmt.Errorf("error sending stream: %w", e)
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

// receiveFile will receive the file from the gRPC stream and save it in the dst folder.
// Returns error if checksum doesn't match the one provided in the fileMetadata.
// It is responsibility of the caller to close the returned file.
func receiveFile(ctx context.Context, receiver StreamReceiver, checksum, dst string) (*os.File, error) {
	hasher := sha256.New()
	file, err := os.CreateTemp(dst, "")
	if err != nil {
		return nil, fmt.Errorf("error creating file: %w", err)
	}
	for {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("stream context error: %w", err)
			}
		}
		req, err := receiver.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("stream Recv error: %w", err)
		}
		f := req.GetFile()
		if f == nil {
			return nil, fmt.Errorf("stream request file is nil")
		}
		_, err = file.Write(f.Chunk)
		if err != nil {
			return nil, fmt.Errorf("error writing file: %w", err)
		}
		_, err = hasher.Write(f.Chunk)
		if err != nil {
			return nil, fmt.Errorf("error writing hasher: %w", err)
		}
	}
	if hex.EncodeToString(hasher.Sum(nil)) != checksum {
		return nil, fmt.Errorf("file checksum validation error")
	}

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		tgzstream.CloseAndDelete(file)
		return nil, fmt.Errorf("seek error: %w", err)
	}
	return file, nil
}

// AppFileRequest build the file payload for the ManifestRequest
func AppFileRequest(chunk []byte) *pluginclient.AppStreamRequest {
	return &pluginclient.AppStreamRequest{
		Request: &pluginclient.AppStreamRequest_File{
			File: &pluginclient.File{
				Chunk: chunk,
			},
		},
	}
}

// appMetadataRequest build the metadata payload for the ManifestRequest
func appMetadataRequest(appName, appRelPath string, env []string, checksum string, size int64) *pluginclient.AppStreamRequest {
	return &pluginclient.AppStreamRequest{
		Request: &pluginclient.AppStreamRequest_Metadata{
			Metadata: &pluginclient.ManifestRequestMetadata{
				AppName:    appName,
				AppRelPath: appRelPath,
				Checksum:   checksum,
				Size_:      size,
				Env:        toEnvEntry(env),
			},
		},
	}
}

func toEnvEntry(envVars []string) []*pluginclient.EnvEntry {
	envEntry := make([]*pluginclient.EnvEntry, 0)
	for _, env := range envVars {
		pair := strings.SplitN(env, "=", 2)
		if len(pair) < 2 {
			continue
		}
		envEntry = append(envEntry, &pluginclient.EnvEntry{Name: pair[0], Value: pair[1]})
	}
	return envEntry
}
