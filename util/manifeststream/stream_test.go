package manifeststream_test

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/test"
	"github.com/argoproj/argo-cd/v2/util/io/files"
	"github.com/argoproj/argo-cd/v2/util/manifeststream"
)

type applicationStreamMock struct {
	messages chan *applicationpkg.ApplicationManifestQueryWithFilesWrapper
	done     chan bool
}

func (m *applicationStreamMock) Recv() (*applicationpkg.ApplicationManifestQueryWithFilesWrapper, error) {
	select {
	case message := <-m.messages:
		return message, nil
	case <-m.done:
		return nil, io.EOF
	case <-time.After(500 * time.Millisecond):
		return nil, fmt.Errorf("timeout receiving message mock")
	}
}

func (m *applicationStreamMock) Send(message *applicationpkg.ApplicationManifestQueryWithFilesWrapper) error {
	m.messages <- message
	return nil
}

func newApplicationStreamMock() *applicationStreamMock {
	messagesCh := make(chan *applicationpkg.ApplicationManifestQueryWithFilesWrapper)
	doneCh := make(chan bool)
	return &applicationStreamMock{
		messages: messagesCh,
		done:     doneCh,
	}
}

type repoStreamMock struct {
	messages chan *apiclient.ManifestRequestWithFiles
	done     chan bool
}

func (m *repoStreamMock) Recv() (*apiclient.ManifestRequestWithFiles, error) {
	select {
	case message := <-m.messages:
		return message, nil
	case <-m.done:
		return nil, io.EOF
	case <-time.After(500 * time.Millisecond):
		return nil, fmt.Errorf("timeout receiving message mock")
	}
}

func (m *repoStreamMock) Send(message *apiclient.ManifestRequestWithFiles) error {
	m.messages <- message
	return nil
}

func newRepoStreamMock() *repoStreamMock {
	messagesCh := make(chan *apiclient.ManifestRequestWithFiles)
	doneCh := make(chan bool)
	return &repoStreamMock{
		messages: messagesCh,
		done:     doneCh,
	}
}

func TestManifestStream(t *testing.T) {
	appStreamMock := newApplicationStreamMock()
	repoStreamMock := newRepoStreamMock()
	workdir, err := files.CreateTempDir("")
	require.NoError(t, err)

	appDir := filepath.Join(getTestDataDir(t), "app")

	go func() {
		err := manifeststream.SendApplicationManifestQueryWithFiles(context.Background(), appStreamMock, "test", "test", appDir, nil)
		assert.NoError(t, err)
		appStreamMock.done <- true
	}()

	query, err := manifeststream.ReceiveApplicationManifestQueryWithFiles(appStreamMock)
	require.NoError(t, err)
	require.NotNil(t, query)

	req := &apiclient.ManifestRequest{}

	go func() {
		err = manifeststream.SendRepoStream(repoStreamMock, appStreamMock, req, *query.Checksum)
		assert.NoError(t, err)
		repoStreamMock.done <- true
	}()

	req2, meta, err := manifeststream.ReceiveManifestFileStream(context.Background(), repoStreamMock, workdir, math.MaxInt64, math.MaxInt64)
	require.NoError(t, err)
	require.NotNil(t, req2)
	require.NotNil(t, meta)

	files, err := os.ReadDir(workdir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	names := []string{}
	for _, f := range files {
		names = append(names, f.Name())
	}
	assert.Contains(t, names, "DUMMY.md")
}

func getTestDataDir(t *testing.T) string {
	return filepath.Join(test.GetTestDir(t), "testdata")
}
