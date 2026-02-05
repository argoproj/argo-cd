package discovery

import (
	"context"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscover(t *testing.T) {
	apps, err := Discover(t.Context(), "./testdata", "./testdata", map[string]bool{}, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"foo": "Kustomize",
		"baz": "Helm",
	}, apps)
}

func TestAppType(t *testing.T) {
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
	// Use a temp dir as the base plugin socket dir and provide a fileName that
	// resolves outside it to trigger the Inbound check.
	pluginSockFilePath := t.TempDir()
	// fileName with a path traversal that causes the address to be outside the plugin socket dir
	fileName := "../outside.sock"

	conn, client, found, err := cmpSupports(context.Background(), pluginSockFilePath, "appPath", "repoPath", fileName, nil, nil, true)
	require.False(t, found)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside plugin socket dir")
	assert.Nil(t, conn)
	assert.Nil(t, client)
}

func Test_cmpSupports_dialFailure_returnsError(t *testing.T) {
	// Use a temp dir as the base plugin socket dir and provide a socket filename
	// that does not have a listening server; dialing should fail, and the error
	// returned should reflect a dialing problem.
	pluginSockFilePath := t.TempDir()
	fileName := "nonexistent.sock"

	conn, client, found, err := cmpSupports(context.Background(), pluginSockFilePath, "appPath", "repoPath", fileName, nil, nil, true)
	require.False(t, found)
	require.Error(t, err)
	// We expect the error to at least indicate dialing failure; exact wording may vary,
	// check for the dialing error prefix used in cmpSupports.
	assert.Contains(t, err.Error(), "error dialing to cmp-server")
	assert.Nil(t, conn)
	assert.Nil(t, client)
}
