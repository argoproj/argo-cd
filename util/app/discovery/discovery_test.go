package discovery

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pluginclient "github.com/argoproj/argo-cd/v3/cmpserver/apiclient"
	"github.com/argoproj/argo-cd/v3/common"
)

func TestDiscover(t *testing.T) {
	t.Parallel()
	apps, err := Discover(t.Context(), "./testdata", "./testdata", map[string]bool{}, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"foo": "Kustomize",
		"baz": "Helm",
	}, apps)
}

func TestAppType(t *testing.T) {
	t.Parallel()
	appType, err := AppType(t.Context(), "./testdata/foo", "./testdata", map[string]bool{}, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Kustomize", appType)

	appType, err = AppType(t.Context(), "./testdata/baz", "./testdata", map[string]bool{}, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Helm", appType)

	appType, err = AppType(t.Context(), "./testdata", "./testdata", map[string]bool{}, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Directory", appType)
}

func TestAppType_Disabled(t *testing.T) {
	t.Parallel()
	enableManifestGeneration := map[string]bool{
		string(v1alpha1.ApplicationSourceTypeKustomize): false,
		string(v1alpha1.ApplicationSourceTypeHelm):      false,
	}
	appType, err := AppType(t.Context(), "./testdata/foo", "./testdata", enableManifestGeneration, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Directory", appType)

	appType, err = AppType(t.Context(), "./testdata/baz", "./testdata", enableManifestGeneration, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Directory", appType)

	appType, err = AppType(t.Context(), "./testdata", "./testdata", enableManifestGeneration, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Directory", appType)
}

func Test_cmpSupports_invalidSocketPath_outsideDir(t *testing.T) {
	t.Parallel()
	// Use a temp dir as the base plugin socket dir and provide a fileName that
	// resolves outside it to trigger the Inbound check.
	pluginSockFilePath := t.TempDir()
	// fileName with a path traversal that causes the address to be outside the plugin socket dir
	fileName := "../outside.sock"

	conn, client, found, err := cmpSupports(t.Context(), pluginSockFilePath, "appPath", "repoPath", fileName, nil, nil, true)
	require.False(t, found)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside plugin socket dir")
	assert.Nil(t, conn)
	assert.Nil(t, client)
}

func Test_cmpSupports_dialFailure_returnsError(t *testing.T) {
	t.Parallel()
	// Use a temp dir as the base plugin socket dir and provide a socket filename
	// that does not have a listening server; dialing should fail, and the error
	// returned should reflect a dialing problem.
	pluginSockFilePath := t.TempDir()
	fileName := "nonexistent.sock"

	conn, client, found, err := cmpSupports(t.Context(), pluginSockFilePath, "appPath", "repoPath", fileName, nil, nil, true)
	require.False(t, found)
	require.Error(t, err)
	// We expect the error to at least indicate dialing failure; exact wording may vary,
	// check for the dialing error prefix used in cmpSupports.
	assert.Contains(t, err.Error(), "error dialing to cmp-server")
	assert.Nil(t, conn)
	assert.Nil(t, client)
}

// canceledPluginServer is a cmp-server whose pre-flight check fails the way an in-flight
// discovery call fails when the repo-server's own deadline expires while it is running.
type canceledPluginServer struct {
	pluginclient.UnimplementedConfigManagementPluginServiceServer
	code codes.Code
}

func (s *canceledPluginServer) CheckPluginConfiguration(_ context.Context, _ *emptypb.Empty) (*pluginclient.CheckPluginConfigurationResponse, error) {
	return nil, status.Error(s.code, "context canceled")
}

// startPluginServer serves a cmp-server on a socket in a fresh plugin socket directory and
// points ARGOCD_PLUGINSOCKFILEPATH at it.
func startPluginServer(t *testing.T, srv pluginclient.ConfigManagementPluginServiceServer) {
	t.Helper()

	sockDir := t.TempDir()
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "unix", filepath.Join(sockDir, "test-plugin.sock"))
	require.NoError(t, err)

	server := grpc.NewServer()
	pluginclient.RegisterConfigManagementPluginServiceServer(server, srv)
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(server.Stop)

	t.Setenv(common.EnvPluginSockFilePath, sockDir)
}

func TestDiscover_PluginCheckCanceled(t *testing.T) {
	startPluginServer(t, &canceledPluginServer{code: codes.Canceled})

	_, err := Discover(t.Context(), "./testdata", "./testdata", map[string]bool{}, []string{}, []string{})
	require.ErrorContains(t, err, "failed to check for config management plugins")
}

// A cancelled plugin check must not be reported as the Directory type, because that result is
// cached against the revision and the manifests are generated from the undiscovered repo.
func TestAppType_PluginCheckCanceled(t *testing.T) {
	startPluginServer(t, &canceledPluginServer{code: codes.Canceled})

	appType, err := AppType(t.Context(), "./testdata", "./testdata", map[string]bool{}, []string{}, []string{})
	require.Error(t, err)
	assert.Empty(t, appType)
}

// matchRepositoryPluginServer is discovery-configured (as a fileName-based plugin is), so the
// repo-server gets past the pre-flight check and fails inside the MatchRepository stream.
type matchRepositoryPluginServer struct {
	pluginclient.UnimplementedConfigManagementPluginServiceServer
	code codes.Code
}

func (s *matchRepositoryPluginServer) CheckPluginConfiguration(_ context.Context, _ *emptypb.Empty) (*pluginclient.CheckPluginConfigurationResponse, error) {
	return &pluginclient.CheckPluginConfigurationResponse{IsDiscoveryConfigured: true}, nil
}

func (s *matchRepositoryPluginServer) MatchRepository(_ pluginclient.ConfigManagementPluginService_MatchRepositoryServer) error {
	return status.Error(s.code, "discovery did not complete")
}

// A discovery check that never completed must not be reported as the Directory type, whichever
// step of the check failed.
func TestAppType_MatchRepositoryNotCompleted(t *testing.T) {
	for _, code := range []codes.Code{codes.Canceled, codes.DeadlineExceeded, codes.Unavailable} {
		t.Run(code.String(), func(t *testing.T) {
			startPluginServer(t, &matchRepositoryPluginServer{code: code})

			appType, err := AppType(t.Context(), "./testdata", "./testdata", map[string]bool{}, []string{}, []string{})
			require.Error(t, err)
			assert.Empty(t, appType)
		})
	}
}

func TestIsDiscoveryIncompleteError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{name: "nil", err: nil, expected: false},
		{name: "plugin does not match", err: errors.New("could not find plugin supporting the given repository"), expected: false},
		{name: "context canceled", err: fmt.Errorf("error checking plugin configuration: %w", context.Canceled), expected: true},
		{name: "context deadline exceeded", err: fmt.Errorf("error checking plugin configuration: %w", context.DeadlineExceeded), expected: true},
		{name: "grpc canceled", err: fmt.Errorf("wrapped: %w", status.Error(codes.Canceled, "context canceled")), expected: true},
		{name: "grpc deadline exceeded", err: fmt.Errorf("wrapped: %w", status.Error(codes.DeadlineExceeded, "deadline")), expected: true},
		{name: "grpc unavailable", err: fmt.Errorf("wrapped: %w", status.Error(codes.Unavailable, "socket gone")), expected: true},
		{name: "grpc internal", err: fmt.Errorf("wrapped: %w", status.Error(codes.Internal, "boom")), expected: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, isDiscoveryIncompleteError(tc.err))
		})
	}
}
