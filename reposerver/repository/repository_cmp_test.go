package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
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
	cmputil "github.com/argoproj/argo-cd/v3/util/cmp"
)

// fakeCMP is a minimal sidecar config management plugin. A successful
// generation returns a ConfigMap named cm-<version>, so a fresh response can
// be told apart from a stale cached one.
type fakeCMP struct {
	pluginclient.UnimplementedConfigManagementPluginServiceServer
	fail    atomic.Bool
	version atomic.Int32
	// recordFiles makes GenerateManifest extract the received archive and
	// record the files it holds, relative to the archive root.
	recordFiles   atomic.Bool
	mu            sync.Mutex
	receivedFiles []string
}

func (f *fakeCMP) CheckPluginConfiguration(context.Context, *emptypb.Empty) (*pluginclient.CheckPluginConfigurationResponse, error) {
	return &pluginclient.CheckPluginConfigurationResponse{}, nil
}

func (f *fakeCMP) received() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.receivedFiles
}

func (f *fakeCMP) receiveFiles(stream pluginclient.ConfigManagementPluginService_GenerateManifestServer) error {
	destDir, err := os.MkdirTemp("", "cmp-received")
	if err != nil {
		return err
	}
	defer os.RemoveAll(destDir)
	if _, err := cmputil.ReceiveRepoStream(stream.Context(), stream, destDir, false); err != nil {
		return err
	}
	var received []string
	err = filepath.WalkDir(destDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		relPath, err := filepath.Rel(destDir, path)
		if err != nil {
			return err
		}
		received = append(received, filepath.ToSlash(relPath))
		return nil
	})
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.receivedFiles = received
	return nil
}

func (f *fakeCMP) GenerateManifest(stream pluginclient.ConfigManagementPluginService_GenerateManifestServer) error {
	if f.recordFiles.Load() {
		if err := f.receiveFiles(stream); err != nil {
			return err
		}
	}
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

// The repo-server options must reach the sender, so that the sidecar only
// receives the paths the manifest generate paths annotation names.
func TestGenerateManifest_CMPManifestGeneratePathsOnly(t *testing.T) {
	allFiles := []string{"app/values.yaml", "infra/infra.yaml", "other/other.yaml"}
	tests := []struct {
		name                      string
		annotation                string
		useManifestGeneratePaths  bool
		manifestGeneratePathsOnly bool
		// expected is relative to the root of the received archive.
		expected []string
	}{
		{
			name:       "whole repository by default",
			annotation: ".;../infra",
			expected:   allFiles,
		},
		{
			name:                     "whole common root without paths only",
			annotation:               ".",
			useManifestGeneratePaths: true,
			expected:                 []string{"values.yaml"},
		},
		{
			name:                     "common root of all annotated paths without paths only",
			annotation:               ".;../infra",
			useManifestGeneratePaths: true,
			expected:                 allFiles,
		},
		{
			name:                      "paths only has no effect without manifest generate paths",
			annotation:                ".;../infra",
			manifestGeneratePathsOnly: true,
			expected:                  allFiles,
		},
		{
			name:                      "annotated paths only",
			annotation:                ".;../infra",
			useManifestGeneratePaths:  true,
			manifestGeneratePathsOnly: true,
			expected:                  []string{"app/values.yaml", "infra/infra.yaml"},
		},
		{
			name:                      "whole repository without annotation",
			useManifestGeneratePaths:  true,
			manifestGeneratePathsOnly: true,
			expected:                  allFiles,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmp := startFakeCMP(t, "test-plugin")
			cmp.recordFiles.Store(true)

			root := t.TempDir()
			for _, file := range allFiles {
				require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.Dir(file)), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(root, file), []byte("key: value\n"), 0o644))
			}

			service := newService(t, root)
			service.initConstants = RepoServerInitConstants{
				ParallelismLimit:                1,
				CMPUseManifestGeneratePaths:     tt.useManifestGeneratePaths,
				CMPTarManifestGeneratePathsOnly: tt.manifestGeneratePathsOnly,
			}
			_, err := service.GenerateManifest(t.Context(), &apiclient.ManifestRequest{
				Repo:    &v1alpha1.Repository{},
				AppName: "test",
				ApplicationSource: &v1alpha1.ApplicationSource{
					Path:   "app",
					Plugin: &v1alpha1.ApplicationSourcePlugin{Name: "test-plugin"},
				},
				AnnotationManifestGeneratePaths: tt.annotation,
				NoCache:                         true,
			})
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.expected, cmp.received())
		})
	}
}
