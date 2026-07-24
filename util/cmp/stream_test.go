package cmp_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginclient "github.com/argoproj/argo-cd/v3/cmpserver/apiclient"
	"github.com/argoproj/argo-cd/v3/test"
	"github.com/argoproj/argo-cd/v3/util/cmp"
	"github.com/argoproj/argo-cd/v3/util/io/files"
)

type streamMock struct {
	messages chan *pluginclient.AppStreamRequest
	done     chan bool
}

func (m *streamMock) Recv() (*pluginclient.AppStreamRequest, error) {
	select {
	case message := <-m.messages:
		return message, nil
	case <-m.done:
		return nil, io.EOF
	case <-time.After(500 * time.Millisecond):
		return nil, errors.New("timeout receiving message mock")
	}
}

func (m *streamMock) Send(message *pluginclient.AppStreamRequest) error {
	m.messages <- message
	return nil
}

func newStreamMock() *streamMock {
	messagesCh := make(chan *pluginclient.AppStreamRequest)
	doneCh := make(chan bool)
	return &streamMock{
		messages: messagesCh,
		done:     doneCh,
	}
}

func TestReceiveApplicationStream(t *testing.T) {
	t.Parallel()
	t.Run("will receive the application stream successfully", func(t *testing.T) {
		// given
		t.Parallel()
		streamMock := newStreamMock()
		appDir := filepath.Join(getTestDataDir(t), "app")
		workdir, err := files.CreateTempDir("")
		require.NoError(t, err)
		defer func() {
			close(streamMock.messages)
			os.RemoveAll(workdir)
		}()
		go streamMock.sendFile(t.Context(), t, appDir, streamMock, []string{"env1", "env2"}, []string{"DUMMY.md", "dum*"})

		// when
		env, err := cmp.ReceiveRepoStream(t.Context(), streamMock, workdir, false)

		// then
		require.NoError(t, err)
		assert.NotEmpty(t, workdir)
		files, err := os.ReadDir(workdir)
		require.NoError(t, err)
		require.Len(t, files, 2)
		names := []string{}
		for _, f := range files {
			names = append(names, f.Name())
		}
		assert.Contains(t, names, "README.md")
		assert.Contains(t, names, "applicationset")
		assert.NotContains(t, names, "DUMMY.md")
		assert.NotContains(t, names, "dummy")
		assert.NotNil(t, env)
	})

	t.Run("slash-pattern in plugin-tar-exclude excludes by relative path", func(t *testing.T) {
		t.Parallel()
		streamMock := newStreamMock()
		appDir := filepath.Join(getTestDataDir(t), "app")
		workdir, err := files.CreateTempDir("")
		require.NoError(t, err)
		defer func() {
			close(streamMock.messages)
			if os.RemoveAll(workdir) != nil {
				t.Fatal(err)
			}
		}()
		go streamMock.sendFile(t.Context(), t, appDir, streamMock, nil, []string{"applicationset/latest/**"})

		_, err = cmp.ReceiveRepoStream(t.Context(), streamMock, workdir, false)
		require.NoError(t, err)
		latestDir := filepath.Join(workdir, "applicationset", "latest")
		stableDir := filepath.Join(workdir, "applicationset", "stable")
		_, statErr := os.Stat(filepath.Join(latestDir, "kustomization.yaml"))
		assert.True(t, os.IsNotExist(statErr), "applicationset/latest/kustomization.yaml should be excluded")
		_, statErr = os.Stat(filepath.Join(stableDir, "kustomization.yaml"))
		assert.NoError(t, statErr, "applicationset/stable/kustomization.yaml should be present")
	})
}

func (m *streamMock) sendFile(ctx context.Context, t *testing.T, basedir string, sender cmp.StreamSender, env []string, excludedGlobs []string) {
	t.Helper()
	defer func() {
		m.done <- true
	}()
	err := cmp.SendRepoStream(ctx, basedir, basedir, sender, env, excludedGlobs)
	require.NoError(t, err)
}

// getTestDataDir will return the full path of the testdata dir
// under the running test folder.
func getTestDataDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(test.GetTestDir(t), "testdata")
}
