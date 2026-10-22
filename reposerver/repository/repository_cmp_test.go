package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pluginclient "github.com/argoproj/argo-cd/v3/cmpserver/apiclient"
	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
)

// fakeCMP is a minimal sidecar config management plugin. A successful
// generation returns a ConfigMap named cm-<version>, so a fresh response can
// be told apart from a stale cached one.
type fakeCMP struct {
	pluginclient.UnimplementedConfigManagementPluginServiceServer
	fail    atomic.Bool
	version atomic.Int32
}

func (f *fakeCMP) CheckPluginConfiguration(context.Context, *emptypb.Empty) (*pluginclient.CheckPluginConfigurationResponse, error) {
	return &pluginclient.CheckPluginConfigurationResponse{}, nil
}

func (f *fakeCMP) GenerateManifest(stream pluginclient.ConfigManagementPluginService_GenerateManifestServer) error {
	for {
		if _, err := stream.Recv(); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
	}
	if f.fail.Load() {
		return status.Error(codes.Unknown, "plugin generate failed")
	}
	return stream.SendAndClose(&pluginclient.ManifestResponse{
		Manifests: []string{fmt.Sprintf(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"cm-%d"}}`, f.version.Load())},
	})
}

// startFakeCMP serves a fakeCMP on <sockDir>/<name>.sock and points the
// repo-server plugin discovery at it.
func startFakeCMP(t *testing.T, name string) *fakeCMP {
	t.Helper()
	// Not t.TempDir(): unix socket paths are limited to ~108 bytes.
	sockDir, err := os.MkdirTemp("", "cmp")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(sockDir) })
	t.Setenv(common.EnvPluginSockFilePath, sockDir)

	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", filepath.Join(sockDir, name+".sock"))
	require.NoError(t, err)
	cmp := &fakeCMP{}
	srv := grpc.NewServer()
	pluginclient.RegisterConfigManagementPluginServiceServer(srv, cmp)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return cmp
}

// runManifestGenAsyncGoroutines returns the number of live goroutines running runManifestGenAsync.
// It returns both the count and any profiling error; the caller must handle the error on the main test goroutine.
func runManifestGenAsyncGoroutines() (int, error) {
	// Resolved from the method itself so that renaming it fails to compile
	// instead of silently matching nothing.
	name := runtime.FuncForPC(reflect.ValueOf((*Service).runManifestGenAsync).Pointer()).Name()
	var buf bytes.Buffer
	if err := pprof.Lookup("goroutine").WriteTo(&buf, 2); err != nil {
		return 0, err
	}
	return strings.Count(buf.String(), name+"("), nil
}

// A manifest cache entry that holds a previous response and a recorded
// generation failure (below PauseGenerationAfterFailedGenerationAttempts)
// makes cacheFn set res to the stale response while still asking for a new
// generation. With a sidecar CMP, GenerateManifest must still wait for the
// result of that generation after tarDoneCh fires.
func TestGenerateManifest_CMPWithFailureStampedCacheEntry(t *testing.T) {
	cmp := startFakeCMP(t, "test-plugin")

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "app"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "app", "values.yaml"), []byte("key: value\n"), 0o644))

	service := newService(t, root)
	service.initConstants = RepoServerInitConstants{
		ParallelismLimit: 1,
		PauseGenerationAfterFailedGenerationAttempts: 3,
		PauseGenerationOnFailureForMinutes:           60,
	}
	request := func(noCache bool) *apiclient.ManifestRequest {
		return &apiclient.ManifestRequest{
			Repo:    &v1alpha1.Repository{},
			AppName: "test",
			ApplicationSource: &v1alpha1.ApplicationSource{
				Path:   "app",
				Plugin: &v1alpha1.ApplicationSourcePlugin{Name: "test-plugin"},
			},
			NoCache: noCache,
		}
	}

	// 1) Successful generation populates the manifest cache.
	cmp.version.Store(1)
	res, err := service.GenerateManifest(t.Context(), request(false))
	require.NoError(t, err)
	require.Len(t, res.Manifests, 1)
	assert.Contains(t, res.Manifests[0], "cm-1")

	// 2) A failed hard refresh records the failure on the cache entry, which
	// keeps the previous response.
	cmp.fail.Store(true)
	_, err = service.GenerateManifest(t.Context(), request(true))
	require.ErrorContains(t, err, "plugin generate failed")

	// 3) The next generation also fails. The error must be returned, not the
	// stale cached response.
	res, err = service.GenerateManifest(t.Context(), request(false))
	require.ErrorContains(t, err, "plugin generate failed")
	assert.NotContains(t, err.Error(), cachedManifestGenerationPrefix)
	assert.Nil(t, res)

	// 4) The plugin recovers. The fresh response must be returned, not the
	// stale cached one.
	cmp.fail.Store(false)
	cmp.version.Store(2)
	res, err = service.GenerateManifest(t.Context(), request(false))
	require.NoError(t, err)
	require.Len(t, res.Manifests, 1)
	assert.Contains(t, res.Manifests[0], "cm-2")

	// 5) No runManifestGenAsync goroutine is left blocked on a channel send.
	deadline := time.Now().Add(5 * time.Second)
	for {
		count, err := runManifestGenAsyncGoroutines()
		require.NoError(t, err)
		if count == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("runManifestGenAsync goroutines leaked: %d still running after 5s", count)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
