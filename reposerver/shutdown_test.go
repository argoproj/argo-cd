package reposerver

import (
	"context"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/argoproj/argo-cd/v3/util/git"
)

func TestCancelOnShutdownUnaryInterceptor(t *testing.T) {
	t.Parallel()
	shutdownCtx, shutdown := context.WithCancel(t.Context())
	interceptor := cancelOnShutdownUnaryInterceptor(shutdownCtx)

	handlerCtx := make(chan context.Context, 1)
	done := make(chan error, 1)
	go func() {
		_, err := interceptor(t.Context(), nil, nil, func(ctx context.Context, _ any) (any, error) {
			handlerCtx <- ctx
			<-ctx.Done()
			return nil, ctx.Err()
		})
		done <- err
	}()

	ctx := <-handlerCtx
	require.NoError(t, ctx.Err(), "request context must not be cancelled while the server is running")
	shutdown()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(3 * time.Second):
		t.Fatal("handler context was not cancelled on shutdown")
	}
}

func TestCancelOnShutdownUnaryInterceptorReturnsHandlerResult(t *testing.T) {
	t.Parallel()
	shutdownCtx, shutdown := context.WithCancel(t.Context())
	defer shutdown()

	res, err := cancelOnShutdownUnaryInterceptor(shutdownCtx)(t.Context(), nil, nil, func(ctx context.Context, _ any) (any, error) {
		return "ok", ctx.Err()
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", res)
}

type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *fakeServerStream) Context() context.Context { return s.ctx }

func TestCancelOnShutdownStreamInterceptor(t *testing.T) {
	t.Parallel()
	shutdownCtx, shutdown := context.WithCancel(t.Context())
	interceptor := cancelOnShutdownStreamInterceptor(shutdownCtx)

	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- interceptor(nil, &fakeServerStream{ctx: t.Context()}, nil, func(_ any, stream grpc.ServerStream) error {
			close(started)
			<-stream.Context().Done()
			return stream.Context().Err()
		})
	}()

	<-started
	shutdown()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(3 * time.Second):
		t.Fatal("stream context was not cancelled on shutdown")
	}
}

// TestCancelOnShutdownCleansUpGitLocks is the end-to-end claim: cancelling on shutdown lets git
// remove .git/index.lock, which would otherwise outlive a container restart in the pod's emptyDir.
func TestCancelOnShutdownCleansUpGitLocks(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	client, err := git.NewClientExt("file://"+repoDir, repoDir, git.NopCreds{}, true, false, "", "")
	require.NoError(t, err)
	require.NoError(t, client.Init())

	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = client.Root()
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	require.NoError(t, os.WriteFile(path.Join(client.Root(), "manifest.yaml"), []byte("kind: ConfigMap\n"), 0o600))
	require.NoError(t, os.WriteFile(path.Join(client.Root(), ".gitattributes"), []byte("manifest.yaml filter=slow\n"), 0o600))
	// Slow enough to outlive the drain window; the sentinel marks index.lock as registered for
	// cleanup, so cancelling cannot race that registration.
	run("config", "filter.slow.smudge", "sh -c 'touch .filter-started; sleep 10; cat'")
	run("add", "manifest.yaml", ".gitattributes")
	run("commit", "-m", "initial commit")
	require.NoError(t, os.Remove(path.Join(client.Root(), "manifest.yaml")))

	revParse := exec.CommandContext(t.Context(), "git", "rev-parse", "HEAD")
	revParse.Dir = client.Root()
	sha, err := revParse.Output()
	require.NoError(t, err)

	shutdownCtx, shutdown := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, err := cancelOnShutdownUnaryInterceptor(shutdownCtx)(t.Context(), nil, nil, func(ctx context.Context, _ any) (any, error) {
			_, checkoutErr := client.Checkout(ctx, strings.TrimSpace(string(sha)), false, true)
			return nil, checkoutErr
		})
		done <- err
	}()

	lockPath := path.Join(client.Root(), ".git", "index.lock")
	require.Eventually(t, func() bool {
		_, lockErr := os.Stat(lockPath)
		_, filterErr := os.Stat(path.Join(client.Root(), ".filter-started"))
		return lockErr == nil && filterErr == nil
	}, 10*time.Second, 5*time.Millisecond, "Git never reached the filter while holding the index lock")

	shutdown()
	select {
	case err = <-done:
		require.Error(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("checkout did not return after shutdown cancelled the request")
	}
	assert.NoFileExists(t, lockPath, "shutdown left a stale index lock, poisoning the repo for the life of the pod")
}
