package discovery

import (
	"context"
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscover(t *testing.T) {
	apps, err := Discover(context.Background(), "./testdata", "./testdata", map[string]bool{}, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"foo": "Kustomize",
		"baz": "Helm",
	}, apps)
}

func TestAppType(t *testing.T) {
	appType, err := AppType(context.Background(), "./testdata/foo", "./testdata", map[string]bool{}, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Kustomize", appType)

	appType, err = AppType(context.Background(), "./testdata/baz", "./testdata", map[string]bool{}, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Helm", appType)

	appType, err = AppType(context.Background(), "./testdata", "./testdata", map[string]bool{}, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Directory", appType)
}

func TestAppType_Disabled(t *testing.T) {
	enableManifestGeneration := map[string]bool{
		string(v1alpha1.ApplicationSourceTypeKustomize): false,
		string(v1alpha1.ApplicationSourceTypeHelm):      false,
	}
	appType, err := AppType(context.Background(), "./testdata/foo", "./testdata", enableManifestGeneration, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Directory", appType)

	appType, err = AppType(context.Background(), "./testdata/baz", "./testdata", enableManifestGeneration, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Directory", appType)

	appType, err = AppType(context.Background(), "./testdata", "./testdata", enableManifestGeneration, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Directory", appType)
}
