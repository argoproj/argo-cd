package kustomize

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func helmPluginsOf(t *testing.T, env []string) (string, bool) {
	t.Helper()
	var val string
	found := false
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, "HELM_PLUGINS="); ok {
			val = v
			found = true
		}
	}
	return val, found
}

// TestWithHelmWrapper_SetsSafePluginsDir is the core regression test for the wrapper's env
// substitution: whatever HELM_PLUGINS the `kustomize build` subprocess env already carries -
// including a value a kustomization's own helmGlobals.configHome would produce - helm must only
// ever see a fixed, empty, Argo CD-owned plugins directory, so it cannot load a plugin the
// kustomization planted. See GHSA-fw5c-w8rc-j7fx.
func TestWithHelmWrapper_SetsSafePluginsDir(t *testing.T) {
	testCases := []struct {
		name string
		env  []string
	}{
		{
			name: "no HELM_PLUGINS present",
			env:  []string{"PATH=/usr/bin:/bin", "FOO=bar"},
		},
		{
			name: "attacker-controlled HELM_PLUGINS is replaced",
			env:  []string{"FOO=bar", "HELM_PLUGINS=/attacker/helm-home/.data/plugins"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env, err := withHelmWrapper(tc.env)
			require.NoError(t, err)

			plugins, found := helmPluginsOf(t, env)
			require.True(t, found, "HELM_PLUGINS must be set")
			assert.DirExists(t, plugins, "HELM_PLUGINS must point at an existing directory")
			assert.NotEqual(t, "/attacker/helm-home/.data/plugins", plugins, "any pre-existing HELM_PLUGINS must be overridden")

			entries, err := os.ReadDir(plugins)
			require.NoError(t, err)
			assert.Empty(t, entries, "the safe plugins dir must be empty so no plugin is ever auto-loaded")
		})
	}
}

func TestWithHelmWrapper_PreservesUnrelatedEnv(t *testing.T) {
	env, err := withHelmWrapper([]string{"PATH=/usr/bin:/bin", "FOO=bar"})
	require.NoError(t, err)

	assert.Contains(t, env, "PATH=/usr/bin:/bin", "unrelated env vars must be preserved untouched")
	assert.Contains(t, env, "FOO=bar", "unrelated env vars must be preserved untouched")

	// Exactly one HELM_PLUGINS entry must be present (no duplicates).
	count := 0
	for _, e := range env {
		if strings.HasPrefix(e, "HELM_PLUGINS=") {
			count++
		}
	}
	assert.Equal(t, 1, count, "exactly one HELM_PLUGINS entry must be present")
}

func TestSafeHelmPluginsDir_Idempotent(t *testing.T) {
	dir1, err := safeHelmPluginsDir()
	require.NoError(t, err)
	dir2, err := safeHelmPluginsDir()
	require.NoError(t, err)
	assert.Equal(t, dir1, dir2, "the safe plugins dir must only be created once per process")
}
