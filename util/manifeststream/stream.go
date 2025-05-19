package manifeststream

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"

	log "github.com/sirupsen/logrus"

	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/io/files"
	"github.com/argoproj/argo-cd/v2/util/tgzstream"
)

// Defines the contract for the application sender, i.e. the CLI
type ApplicationStreamSender interface {
	Send(*applicationpkg.ApplicationManifestQueryWithFilesWrapper) error
}

// Defines the contract for the application receiver, i.e. API server
type ApplicationStreamReceiver interface {
	Recv() (*applicationpkg.ApplicationManifestQueryWithFilesWrapper, error)
}

// Defines the contract for the repo stream sender, i.e. the API server
type RepoStreamSender interface {
	Send(*apiclient.ManifestRequestWithFiles) error
}

// Defines the contract for the repo stream receiver, i.e. the repo server
type RepoStreamReceiver interface {
	Recv() (*apiclient.ManifestRequestWithFiles, error)
}

// SendApplicationManifestQueryWithFiles compresses a folder and sends it over the stream
func SendApplicationManifestQueryWithFiles(ctx context.Context, stream ApplicationStreamSender, appName string, appNs string, dir string, inclusions []string) error {
	f, filesWritten, checksum, err := tgzstream.CompressFiles(dir, inclusions, nil)
	if err != nil {
		return fmt.Errorf("failed to compress files: %w", err)
	}
	if filesWritten == 0 {
		return fmt.Errorf("no files to send")
	}

	err = stream.Send(&applicationpkg.ApplicationManifestQueryWithFilesWrapper{
		Part: &applicationpkg.ApplicationManifestQueryWithFilesWrapper_Query{
			Query: &applicationpkg.ApplicationManifestQueryWithFiles{
				Name:         &appName,
				Checksum:     &checksum,
				AppNamespace: &appNs,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send manifest stream header: %w", err)
	}

	err = sendFile(ctx, stream, f)
	if err != nil {
		return fmt.Errorf("failed to send manifest stream file: %w", err)
	}

	return nil
}

func sendFile(ctx context.Context, sender ApplicationStreamSender, file *os.File) error {
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

func ReceiveApplicationManifestQueryWithFiles(stream ApplicationStreamReceiver) (*applicationpkg.ApplicationManifestQueryWithFiles, error) {
	header, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive header: %w", err)
	}
	if header == nil || header.GetQuery() == nil {
		return nil, fmt.Errorf("error getting stream query: query is nil")
	}
	return header.GetQuery(), nil
}

func SendRepoStream(repoStream RepoStreamSender, appStream ApplicationStreamReceiver, req *apiclient.ManifestRequest, checksum string) error {
	err := repoStream.Send(&apiclient.ManifestRequestWithFiles{
		Part: &apiclient.ManifestRequestWithFiles_Request{
			Request: req,
		},
	})
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}

	err = repoStream.Send(&apiclient.ManifestRequestWithFiles{
		Part: &apiclient.ManifestRequestWithFiles_Metadata{
			Metadata: &apiclient.ManifestFileMetadata{
				Checksum: checksum,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("error sending metadata: %w", err)
	}

	for {
		part, err := appStream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("stream Recv error: %w", err)
		}
		if part == nil || part.GetChunk() == nil {
			return fmt.Errorf("error getting stream chunk: chunk is nil")
		}

		err = repoStream.Send(&apiclient.ManifestRequestWithFiles{
			Part: &apiclient.ManifestRequestWithFiles_Chunk{
				Chunk: &apiclient.ManifestFileChunk{
					Chunk: part.GetChunk().GetChunk(),
				},
			},
		})
		if err != nil {
			return fmt.Errorf("error sending chunk: %w", err)
		}
	}

	return nil
}

func ReceiveManifestFileStream(ctx context.Context, receiver RepoStreamReceiver, destDir string, maxTarSize int64, maxExtractedSize int64) (*apiclient.ManifestRequest, *apiclient.ManifestFileMetadata, error) {
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

	tgzFile, err := receiveFile(ctx, receiver, metadata.GetChecksum(), maxTarSize)
	if err != nil {
		return nil, nil, fmt.Errorf("error receiving tgz file: %w", err)
	}
	err = files.Untgz(destDir, tgzFile, maxExtractedSize, false)
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
func receiveFile(ctx context.Context, receiver RepoStreamReceiver, checksum string, maxSize int64) (*os.File, error) {
	hasher := sha256.New()
	tmpDir, err := files.CreateTempDir("")
	if err != nil {
		return nil, fmt.Errorf("error creating tmp dir: %w", err)
	}
	file, err := os.CreateTemp(tmpDir, "")
	if err != nil {
		return nil, fmt.Errorf("error creating file: %w", err)
	}
	size := 0
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
		c := req.GetChunk()
		if c == nil {
			return nil, fmt.Errorf("stream request chunk is nil")
		}
		size += len(c.Chunk)
		if size > int(maxSize) {
			return nil, fmt.Errorf("file exceeded max size of %d bytes", maxSize)
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
		tgzstream.CloseAndDelete(file)
		return nil, fmt.Errorf("seek error: %w", err)
	}
	return file, nil
}
