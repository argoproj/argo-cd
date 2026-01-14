package discovery

import (
	"os"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscover(t *testing.T) {
	repoRoot, err := os.OpenRoot("./testdata")
	require.NoError(t, err)
	apps, err := Discover(t.Context(), "./testdata", repoRoot, map[string]bool{}, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"foo": "Kustomize",
		"baz": "Helm",
	}, apps)
}

func TestAppType(t *testing.T) {
	repoRoot, err := os.OpenRoot("./testdata")
	require.NoError(t, err)
	appType, err := AppType(t.Context(), "./testdata/foo", repoRoot, map[string]bool{}, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Kustomize", appType)

	appType, err = AppType(t.Context(), "./testdata/baz", repoRoot, map[string]bool{}, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Helm", appType)

	appType, err = AppType(t.Context(), "./testdata", repoRoot, map[string]bool{}, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Directory", appType)
}

func TestAppType_Disabled(t *testing.T) {
	enableManifestGeneration := map[string]bool{
		string(v1alpha1.ApplicationSourceTypeKustomize): false,
		string(v1alpha1.ApplicationSourceTypeHelm):      false,
	}
	repoRoot, err := os.OpenRoot("./testdata")
	require.NoError(t, err)
	appType, err := AppType(t.Context(), "./testdata/foo", repoRoot, enableManifestGeneration, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Directory", appType)

	appType, err = AppType(t.Context(), "./testdata/baz", repoRoot, enableManifestGeneration, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Directory", appType)

	appType, err = AppType(t.Context(), "./testdata", repoRoot, enableManifestGeneration, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Directory", appType)
}
