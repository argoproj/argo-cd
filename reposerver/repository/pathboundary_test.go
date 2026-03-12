package repository

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
)

func TestValidatePathBoundaries_RejectsKustomize(t *testing.T) {
	err := validatePathBoundaries(v1alpha1.ApplicationSourceTypeKustomize, "/app", "/repo", &apiclient.ManifestRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Kustomize source type is not supported")
}

func TestValidatePathBoundaries_RejectsJsonnetConfig(t *testing.T) {
	q := &apiclient.ManifestRequest{
		ApplicationSource: &v1alpha1.ApplicationSource{
			Directory: &v1alpha1.ApplicationSourceDirectory{
				Jsonnet: v1alpha1.ApplicationSourceJsonnet{
					Libs: []string{"vendor"},
				},
			},
		},
	}
	err := validatePathBoundaries(v1alpha1.ApplicationSourceTypeDirectory, "/app", "/repo", q)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Jsonnet is not supported")
}

func TestValidatePathBoundaries_RejectsJsonnetFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.jsonnet"), []byte("{}"), 0o644))

	q := &apiclient.ManifestRequest{
		ApplicationSource: &v1alpha1.ApplicationSource{},
	}
	err := validatePathBoundaries(v1alpha1.ApplicationSourceTypeDirectory, dir, "/repo", q)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Jsonnet is not supported")
}

func TestValidatePathBoundaries_AllowsDirectoryWithoutJsonnet(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "deployment.yaml"), []byte("apiVersion: v1"), 0o644))

	q := &apiclient.ManifestRequest{
		ApplicationSource: &v1alpha1.ApplicationSource{},
	}
	err := validatePathBoundaries(v1alpha1.ApplicationSourceTypeDirectory, dir, "/repo", q)
	require.NoError(t, err)
}

func TestValidatePathBoundaries_HelmInBoundDependency(t *testing.T) {
	dir := t.TempDir()
	chartYaml := `dependencies:
- repository: "file://./subchart"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chartYaml), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subchart"), 0o755))

	q := &apiclient.ManifestRequest{
		ApplicationSource: &v1alpha1.ApplicationSource{},
	}
	err := validatePathBoundaries(v1alpha1.ApplicationSourceTypeHelm, dir, "/repo", q)
	require.NoError(t, err)
}

func TestValidatePathBoundaries_HelmOutOfBoundDependency(t *testing.T) {
	dir := t.TempDir()
	chartYaml := `dependencies:
- repository: "file://../../other"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chartYaml), 0o644))

	q := &apiclient.ManifestRequest{
		ApplicationSource: &v1alpha1.ApplicationSource{},
	}
	err := validatePathBoundaries(v1alpha1.ApplicationSourceTypeHelm, dir, "/repo", q)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "references path outside source directory")
}

func TestValidatePathBoundaries_HelmOutOfBoundValueFile(t *testing.T) {
	dir := t.TempDir()

	q := &apiclient.ManifestRequest{
		ApplicationSource: &v1alpha1.ApplicationSource{
			Helm: &v1alpha1.ApplicationSourceHelm{
				ValueFiles: []string{"../../secrets/values.yaml"},
			},
		},
	}
	err := validatePathBoundaries(v1alpha1.ApplicationSourceTypeHelm, dir, "/repo", q)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "references path outside source directory")
}

func TestValidatePathBoundaries_HelmInBoundValueFile(t *testing.T) {
	dir := t.TempDir()

	q := &apiclient.ManifestRequest{
		ApplicationSource: &v1alpha1.ApplicationSource{
			Helm: &v1alpha1.ApplicationSourceHelm{
				ValueFiles: []string{"values-prod.yaml"},
			},
		},
	}
	err := validatePathBoundaries(v1alpha1.ApplicationSourceTypeHelm, dir, "/repo", q)
	require.NoError(t, err)
}

func TestValidatePathBoundaries_HelmRemoteValueFileSkipped(t *testing.T) {
	dir := t.TempDir()

	q := &apiclient.ManifestRequest{
		ApplicationSource: &v1alpha1.ApplicationSource{
			Helm: &v1alpha1.ApplicationSourceHelm{
				ValueFiles: []string{"https://example.com/values.yaml"},
			},
		},
	}
	err := validatePathBoundaries(v1alpha1.ApplicationSourceTypeHelm, dir, "/repo", q)
	require.NoError(t, err)
}

func TestValidatePathBoundaries_HelmRefSourceValueFileSkipped(t *testing.T) {
	dir := t.TempDir()

	q := &apiclient.ManifestRequest{
		ApplicationSource: &v1alpha1.ApplicationSource{
			Helm: &v1alpha1.ApplicationSourceHelm{
				ValueFiles: []string{"$ref/values.yaml"},
			},
		},
	}
	err := validatePathBoundaries(v1alpha1.ApplicationSourceTypeHelm, dir, "/repo", q)
	require.NoError(t, err)
}

func TestValidatePathBoundaries_PluginAllowed(t *testing.T) {
	err := validatePathBoundaries(v1alpha1.ApplicationSourceTypePlugin, "/app", "/repo", &apiclient.ManifestRequest{})
	require.NoError(t, err)
}
