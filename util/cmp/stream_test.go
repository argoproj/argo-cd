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
}

func TestSendRepoStreamWithIncludePaths(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		includePaths []string
		expected     []string
		unexpected   []string
	}{
		{
			name:         "will send only the included paths",
			includePaths: []string{"applicationset/stable", "README.md"},
			expected:     []string{"README.md", "applicationset"},
			unexpected:   []string{"DUMMY.md", "dummy"},
		},
		{
			name:         "will send everything when no path matches",
			includePaths: []string{"does-not-exist"},
			expected:     []string{"README.md", "applicationset", "DUMMY.md", "dummy"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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
			go streamMock.sendFile(t.Context(), t, appDir, streamMock, nil, nil, cmp.WithIncludePaths(tc.includePaths))

			// when
			metadata, err := cmp.ReceiveRepoStream(t.Context(), streamMock, workdir, false)

			// then
			require.NoError(t, err)
			require.NotNil(t, metadata)
			entries, err := os.ReadDir(workdir)
			require.NoError(t, err)
			names := []string{}
			for _, entry := range entries {
				names = append(names, entry.Name())
			}
			for _, name := range tc.expected {
				assert.Contains(t, names, name)
			}
			for _, name := range tc.unexpected {
				assert.NotContains(t, names, name)
			}
		})
	}

	t.Run("will send the whole subtree of an included directory", func(t *testing.T) {
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
		go streamMock.sendFile(t.Context(), t, appDir, streamMock, nil, nil, cmp.WithIncludePaths([]string{"applicationset"}))

		// when
		_, err = cmp.ReceiveRepoStream(t.Context(), streamMock, workdir, false)

		// then
		require.NoError(t, err)
		assert.FileExists(t, filepath.Join(workdir, "applicationset", "latest", "kustomization.yaml"))
		assert.FileExists(t, filepath.Join(workdir, "applicationset", "stable", "kustomization.yaml"))
		assert.NoFileExists(t, filepath.Join(workdir, "README.md"))
	})

	t.Run("will send everything when the included paths select no file", func(t *testing.T) {
		// given
		t.Parallel()
		streamMock := newStreamMock()
		basedir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(basedir, "README.md"), []byte("read me"), 0o600))
		require.NoError(t, os.Mkdir(filepath.Join(basedir, "links"), 0o700))
		require.NoError(t, os.Symlink(filepath.Join("..", "README.md"), filepath.Join(basedir, "links", "readme-symlink")))
		workdir, err := files.CreateTempDir("")
		require.NoError(t, err)
		defer func() {
			close(streamMock.messages)
			os.RemoveAll(workdir)
		}()
		// The selected directory holds a symlink and nothing else, which leaves
		// the plugin without a file to render, so the fallback kicks in as it
		// does for a selection that matched nothing.
		go streamMock.sendFile(t.Context(), t, basedir, streamMock, nil, nil, cmp.WithIncludePaths([]string{"links"}))

		// when
		_, err = cmp.ReceiveRepoStream(t.Context(), streamMock, workdir, false)

		// then
		require.NoError(t, err)
		assert.FileExists(t, filepath.Join(workdir, "README.md"))
		assert.FileExists(t, filepath.Join(workdir, "links", "readme-symlink"))
	})
}

func (m *streamMock) sendFile(ctx context.Context, t *testing.T, basedir string, sender cmp.StreamSender, env []string, excludedGlobs []string, opts ...cmp.SenderOption) {
	t.Helper()
	defer func() {
		m.done <- true
	}()
	err := cmp.SendRepoStream(ctx, basedir, basedir, sender, env, excludedGlobs, opts...)
	require.NoError(t, err)
}

// getTestDataDir will return the full path of the testdata dir
// under the running test folder.
func getTestDataDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(test.GetTestDir(t), "testdata")
}
