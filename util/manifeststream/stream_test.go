package manifeststream_test

import (
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/test"
	"github.com/argoproj/argo-cd/v3/util/io/files"
	"github.com/argoproj/argo-cd/v3/util/manifeststream"
)

type applicationStreamMock struct {
	messages chan *applicationpkg.ApplicationManifestQueryWithFilesWrapper
	done     chan bool
}

type failingApplicationStreamSender struct{}

func (failingApplicationStreamSender) Send(*applicationpkg.ApplicationManifestQueryWithFilesWrapper) error {
	return errors.New("send failed")
}

func TestSendApplicationManifestQueryWithFilesCleansTemporaryDirectoryWhenHeaderSendFails(t *testing.T) {
	tempRoot := t.TempDir()
	t.Setenv("TMP", tempRoot)
	t.Setenv("TEMP", tempRoot)
	t.Setenv("TMPDIR", tempRoot)

	appPath := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(appPath, "config.yaml"), []byte("kind: ConfigMap\n"), 0o600))

	err := manifeststream.SendApplicationManifestQueryWithFiles(t.Context(), failingApplicationStreamSender{}, "test", "test", appPath, nil)
	require.ErrorContains(t, err, "send failed")

	entries, err := os.ReadDir(tempRoot)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func (m *applicationStreamMock) Recv() (*applicationpkg.ApplicationManifestQueryWithFilesWrapper, error) {
	select {
	case message := <-m.messages:
		return message, nil
	case <-m.done:
		return nil, io.EOF
	case <-time.After(500 * time.Millisecond):
		return nil, errors.New("timeout receiving message mock")
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
		return nil, errors.New("timeout receiving message mock")
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
	t.Parallel()
	appStreamMock := newApplicationStreamMock()
	repoStreamMock := newRepoStreamMock()
	workdir, err := files.CreateTempDir("")
	require.NoError(t, err)

	appDir := filepath.Join(getTestDataDir(t), "app")

	go func() {
		err := manifeststream.SendApplicationManifestQueryWithFiles(t.Context(), appStreamMock, "test", "test", appDir, nil)
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

	req2, meta, err := manifeststream.ReceiveManifestFileStream(t.Context(), repoStreamMock, workdir, math.MaxInt64, math.MaxInt64)
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
	t.Helper()
	return filepath.Join(test.GetTestDir(t), "testdata")
}
