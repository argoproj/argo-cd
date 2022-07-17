package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/io/files"
	log "github.com/sirupsen/logrus"
)

// StreamSender defines the contract to send manifest files
type StreamSender interface {
	Send(*apiclient.ManifestRequestWithFiles) error
}

// StreamReceiver defines the contract for receiving manifest files
type StreamReceiver interface {
	Recv() (*apiclient.ManifestRequestWithFiles, error)
}

func ReceiveManifestFileStream(ctx context.Context, receiver StreamReceiver, destDir string) (*apiclient.ManifestRequest, *apiclient.ManifestFileMetadata, error) {
	header, err := receiver.Recv()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to receive header: %w", err)
	}
	if header == nil || header.GetRequest() == nil {
		return nil, nil, fmt.Errorf("error getting stream request: request is nil")
	}
	request := header.GetRequest()

	header2, err := receiver.Recv()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to receive header: %w", err)
	}
	if header2 == nil || header2.GetMetadata() == nil {
		return nil, nil, fmt.Errorf("error getting stream metadata: metadata is nil")
	}
	metadata := header2.GetMetadata()

	tgzFile, err := receiveFile(ctx, receiver, metadata.GetChecksum(), destDir)
	if err != nil {
		return nil, nil, fmt.Errorf("error receiving tgz file: %w", err)
	}
	err = files.Untgz(destDir, tgzFile)
	if err != nil {
		return nil, nil, fmt.Errorf("error decompressing tgz file: %w", err)
	}
	err = os.Remove(tgzFile.Name())
	if err != nil {
		log.Warnf("error removing the tgz file %q: %s", tgzFile.Name(), err)
	}
	return request, metadata, nil

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
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("stream Recv error: %w", err)
		}
		c := req.GetChunk()
		if c == nil {
			return nil, fmt.Errorf("stream request chunk is nil")
		}
		_, err = file.Write(c.Chunk)
		if err != nil {
			return nil, fmt.Errorf("error writing file: %w", err)
		}
		_, err = hasher.Write(c.Chunk)
		if err != nil {
			return nil, fmt.Errorf("error writing hasher: %w", err)
		}
	}
	hasherChecksum := hex.EncodeToString(hasher.Sum(nil))
	if hasherChecksum != checksum {
		return nil, fmt.Errorf("file checksum validation error: calc %s sent %s", hasherChecksum, checksum)
	}

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		closeAndDelete(file)
		return nil, fmt.Errorf("seek error: %w", err)
	}
	return file, nil
}

func closeAndDelete(f *os.File) {
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
