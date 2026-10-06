package kustomize

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFakeHelmWrapper writes a fake "real helm" that prints the HELM_DATA_HOME, HELM_CONFIG_HOME
// and HELM_PLUGINS it was invoked with, plus each argument it received, then generates the
// wrapper script pointing at it with safeDataHome as the fixed substitute directory. It returns
// the path to the wrapper executable.
func newFakeHelmWrapper(t *testing.T, safeDataHome string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("helm wrapper is a POSIX shell script, not supported on Windows")
	}
	dir := t.TempDir()

	fakeHelm := filepath.Join(dir, "realhelm")
	fakeHelmScript := "#!/bin/sh\n" +
		"printf 'HELM_DATA_HOME=[%s]\\n' \"$HELM_DATA_HOME\"\n" +
		"printf 'HELM_CONFIG_HOME=[%s]\\n' \"$HELM_CONFIG_HOME\"\n" +
		"printf 'HELM_PLUGINS=[%s]\\n' \"$HELM_PLUGINS\"\n" +
		"for a in \"$@\"; do printf 'ARG=[%s]\\n' \"$a\"; done\n"
	require.NoError(t, os.WriteFile(fakeHelm, []byte(fakeHelmScript), 0o700))

	wrapper := filepath.Join(dir, "helm")
	require.NoError(t, os.WriteFile(wrapper, []byte(buildHelmWrapperScript(fakeHelm, safeDataHome)), 0o700))
	return wrapper
}

// observedHelmInvocation is what the fake real helm reported it was actually invoked with.
type observedHelmInvocation struct {
	dataHome, configHome, plugins string
	args                          []string
}

func runHelmWrapper(t *testing.T, wrapper string, env []string, args ...string) observedHelmInvocation {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), wrapper, args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "wrapper output: %s", string(out))

	var observed observedHelmInvocation
	for line := range strings.SplitSeq(strings.TrimRight(string(out), "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "HELM_DATA_HOME=["):
			observed.dataHome = strings.TrimSuffix(strings.TrimPrefix(line, "HELM_DATA_HOME=["), "]")
		case strings.HasPrefix(line, "HELM_CONFIG_HOME=["):
			observed.configHome = strings.TrimSuffix(strings.TrimPrefix(line, "HELM_CONFIG_HOME=["), "]")
		case strings.HasPrefix(line, "HELM_PLUGINS=["):
			observed.plugins = strings.TrimSuffix(strings.TrimPrefix(line, "HELM_PLUGINS=["), "]")
		case strings.HasPrefix(line, "ARG=["):
			observed.args = append(observed.args, strings.TrimSuffix(strings.TrimPrefix(line, "ARG=["), "]"))
		}
	}
	return observed
}

// TestHelmWrapper_OverridesConfigHomeRegardlessOfInvokedEnv is the core regression test for the
// wrapper's substitution logic in isolation from installHelmWrapperDir/kustomize/helm: whatever
// HELM_CONFIG_HOME/HELM_DATA_HOME/HELM_PLUGINS the wrapper is invoked with - including values a
// kustomization's own helmGlobals.configHome would produce - the real helm binary must only ever
// see the fixed, safe directory baked into the wrapper at install time. See GHSA-fw5c-w8rc-j7fx.
func TestHelmWrapper_OverridesConfigHomeRegardlessOfInvokedEnv(t *testing.T) {
	safeDataHome := "/safe/fixed/data-home"
	wrapper := newFakeHelmWrapper(t, safeDataHome)

	testCases := []struct {
		name string
		env  []string
	}{
		{
			name: "attacker-controlled configHome/dataHome, as kustomize sets them from helmGlobals.configHome",
			env: []string{
				"HELM_CONFIG_HOME=/attacker/helm-home",
				"HELM_DATA_HOME=/attacker/helm-home/.data",
			},
		},
		{
			name: "no HELM_* vars set at all",
			env:  []string{},
		},
		{
			name: "HELM_PLUGINS separately set by an attacker",
			env: []string{
				"HELM_CONFIG_HOME=/attacker/helm-home",
				"HELM_DATA_HOME=/attacker/helm-home/.data",
				"HELM_PLUGINS=/attacker/helm-home/.data/plugins",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			observed := runHelmWrapper(t, wrapper, tc.env, "pull", "--repo", "probe://chart")
			assert.Equal(t, safeDataHome, observed.dataHome)
			assert.Equal(t, safeDataHome, observed.configHome)
			assert.Equal(t, safeDataHome+"/plugins", observed.plugins)
			assert.Equal(t, []string{"pull", "--repo", "probe://chart"}, observed.args, "arguments must be passed through unchanged")
		})
	}
}

func TestWithHelmWrapper_PrependsPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("helm wrapper is a POSIX shell script, not supported on Windows")
	}
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not available on PATH")
	}

	env, err := withHelmWrapper([]string{"PATH=/usr/bin:/bin", "FOO=bar"})
	require.NoError(t, err)

	var pathVal string
	var sawFoo bool
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, "PATH="); ok {
			pathVal = v
		}
		if e == "FOO=bar" {
			sawFoo = true
		}
	}
	assert.True(t, sawFoo, "unrelated env vars should be preserved")
	require.NotEmpty(t, pathVal)
	// The wrapper directory must come first so kustomize resolves it ahead of the real helm.
	first, _, _ := strings.Cut(pathVal, string(os.PathListSeparator))
	assert.DirExists(t, first)
	assert.FileExists(t, filepath.Join(first, "helm"))
	assert.True(t, strings.HasSuffix(pathVal, "/usr/bin:/bin"), "original PATH must be preserved after the wrapper dir: %s", pathVal)
}

func TestWithHelmWrapper_NoPathVarPresent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("helm wrapper is a POSIX shell script, not supported on Windows")
	}
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not available on PATH")
	}

	env, err := withHelmWrapper([]string{"FOO=bar"})
	require.NoError(t, err)

	var pathVal string
	found := false
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, "PATH="); ok {
			pathVal = v
			found = true
		}
	}
	require.True(t, found, "a PATH entry must be added even if none was present in env")
	assert.DirExists(t, pathVal)
}

func TestInstallHelmWrapperDir_Idempotent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("helm wrapper is a POSIX shell script, not supported on Windows")
	}
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not available on PATH")
	}

	dir1, err := installHelmWrapperDir()
	require.NoError(t, err)
	dir2, err := installHelmWrapperDir()
	require.NoError(t, err)
	assert.Equal(t, dir1, dir2, "the wrapper dir must only be installed once per process")
}
