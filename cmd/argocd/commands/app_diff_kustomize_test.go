package commands

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/settings"
	argoappv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func sourceWithKustomizeVersion(version string) argoappv1.ApplicationSource {
	return argoappv1.ApplicationSource{Kustomize: &argoappv1.ApplicationSourceKustomize{Version: version}}
}

func TestLocalKustomizeOptions(t *testing.T) {
	localBinary := filepath.Join(t.TempDir(), "kustomize-v1.2.3")
	require.NoError(t, os.WriteFile(localBinary, []byte("#!/bin/sh\n"), 0o700))
	missingBinary := filepath.Join(t.TempDir(), "does-not-exist", "kustomize")

	tests := []struct {
		name            string
		settings        *settings.Settings
		source          argoappv1.ApplicationSource
		expected        *argoappv1.KustomizeOptions
		expectedWarning string
	}{
		{
			name:     "no options from the server and no version requested",
			settings: &settings.Settings{},
			source:   argoappv1.ApplicationSource{},
			expected: &argoappv1.KustomizeOptions{},
		},
		{
			name: "no version requested keeps the server options",
			settings: &settings.Settings{KustomizeOptions: &argoappv1.KustomizeOptions{
				BuildOptions: "--global",
				Versions:     []argoappv1.KustomizeVersion{{Name: "v1.2.3", Path: missingBinary}},
			}},
			source: argoappv1.ApplicationSource{},
			expected: &argoappv1.KustomizeOptions{
				BuildOptions: "--global",
				Versions:     []argoappv1.KustomizeVersion{{Name: "v1.2.3", Path: missingBinary}},
			},
		},
		{
			name: "binary of the requested version is available locally",
			settings: &settings.Settings{KustomizeOptions: &argoappv1.KustomizeOptions{
				Versions: []argoappv1.KustomizeVersion{{Name: "v1.2.3", Path: localBinary, BuildOptions: "--enable-helm"}},
			}},
			source: sourceWithKustomizeVersion("v1.2.3"),
			expected: &argoappv1.KustomizeOptions{
				Versions: []argoappv1.KustomizeVersion{{Name: "v1.2.3", Path: localBinary, BuildOptions: "--enable-helm"}},
			},
		},
		{
			name: "binary of the requested version is not available locally",
			settings: &settings.Settings{KustomizeOptions: &argoappv1.KustomizeOptions{
				Versions: []argoappv1.KustomizeVersion{
					{Name: "v1.2.3", Path: missingBinary, BuildOptions: "--enable-helm"},
					{Name: "v4.5.6", Path: missingBinary},
				},
			}},
			source: sourceWithKustomizeVersion("v1.2.3"),
			expected: &argoappv1.KustomizeOptions{
				Versions: []argoappv1.KustomizeVersion{
					{Name: "v1.2.3", BuildOptions: "--enable-helm"},
					{Name: "v4.5.6", Path: missingBinary},
				},
			},
			expectedWarning: `Warning: Kustomize v1.2.3 binary "` + missingBinary + `" is not available locally, using kustomize from PATH instead`,
		},
		{
			name: "server only provides the names of registered versions",
			settings: &settings.Settings{
				KustomizeOptions:  &argoappv1.KustomizeOptions{BuildOptions: "--global"},
				KustomizeVersions: []string{"v1.2.3"},
			},
			source: sourceWithKustomizeVersion("v1.2.3"),
			expected: &argoappv1.KustomizeOptions{
				BuildOptions: "--global",
				Versions:     []argoappv1.KustomizeVersion{{Name: "v1.2.3"}},
			},
			expectedWarning: "Warning: the server did not provide the settings for Kustomize v1.2.3, using kustomize from PATH instead",
		},
		{
			name: "version unknown to the server is left for manifest generation to reject",
			settings: &settings.Settings{
				KustomizeOptions:  &argoappv1.KustomizeOptions{BuildOptions: "--global"},
				KustomizeVersions: []string{"v4.5.6"},
			},
			source:   sourceWithKustomizeVersion("v1.2.3"),
			expected: &argoappv1.KustomizeOptions{BuildOptions: "--global"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := tt.settings.KustomizeOptions.DeepCopy()
			var warnings bytes.Buffer

			actual := localKustomizeOptions(tt.settings, tt.source, &warnings)

			assert.Equal(t, tt.expected, actual)
			if tt.expectedWarning == "" {
				assert.Empty(t, warnings.String())
			} else {
				assert.Equal(t, tt.expectedWarning+"\n", warnings.String())
			}
			assert.Equal(t, original, tt.settings.KustomizeOptions, "server settings must not be modified")
		})
	}
}

func TestLocalKustomizeOptionsRelativeBinaryPath(t *testing.T) {
	cliDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(cliDir, "tools"), 0o700))
	binary := filepath.Join(cliDir, "tools", "kustomize")
	require.NoError(t, os.WriteFile(binary, []byte("#!/bin/sh\n"), 0o700))
	t.Chdir(cliDir)

	argoSettings := &settings.Settings{KustomizeOptions: &argoappv1.KustomizeOptions{
		Versions: []argoappv1.KustomizeVersion{{Name: "v1.2.3", Path: "./tools/kustomize"}},
	}}
	var warnings bytes.Buffer

	actual := localKustomizeOptions(argoSettings, sourceWithKustomizeVersion("v1.2.3"), &warnings)

	assert.Empty(t, warnings.String())
	require.Len(t, actual.Versions, 1)
	resolved := actual.Versions[0].Path
	require.True(t, filepath.IsAbs(resolved), "expected an absolute path, got %q", resolved)
	expectedInfo, err := os.Stat(binary)
	require.NoError(t, err)
	resolvedInfo, err := os.Stat(resolved)
	require.NoError(t, err)
	assert.True(t, os.SameFile(expectedInfo, resolvedInfo), "expected %q to point at %q", resolved, binary)

	// Kustomize runs the binary from the application's directory, which differs from the CLI's.
	cmd := exec.CommandContext(t.Context(), resolved)
	cmd.Dir = t.TempDir()
	require.NoError(t, cmd.Run())
	assert.Equal(t, "./tools/kustomize", argoSettings.KustomizeOptions.Versions[0].Path, "server settings must not be modified")
}
