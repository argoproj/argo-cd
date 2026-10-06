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

// newFakeGitWrapper writes a fake "real git" that prints each argument it receives on its own line,
// then generates the wrapper script pointing at it. It returns the path to the wrapper executable.
func newFakeGitWrapper(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("git wrapper is a POSIX shell script, not supported on Windows")
	}
	dir := t.TempDir()

	// Fake git: print args one per line, wrapped in markers so leading/trailing whitespace and
	// empty args are unambiguous.
	fakeGit := filepath.Join(dir, "realgit")
	fakeGitScript := "#!/bin/sh\nfor a in \"$@\"; do printf '[%s]\\n' \"$a\"; done\n"
	require.NoError(t, os.WriteFile(fakeGit, []byte(fakeGitScript), 0o700))

	wrapper := filepath.Join(dir, "git")
	require.NoError(t, os.WriteFile(wrapper, []byte(buildGitWrapperScript(fakeGit)), 0o700))
	return wrapper
}

// runWrapper runs the wrapper with the given args and returns the args the fake git observed.
func runWrapper(t *testing.T, wrapper string, args ...string) []string {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), wrapper, args...).CombinedOutput()
	require.NoError(t, err, "wrapper output: %s", string(out))
	var observed []string
	for line := range strings.SplitSeq(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		observed = append(observed, strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
	}
	return observed
}

func TestGitWrapper_InsertsEndOfOptionsForKnownFetchShape(t *testing.T) {
	wrapper := newFakeGitWrapper(t)

	testCases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "benign fetch gets --end-of-options before the repository",
			in:   []string{"fetch", "--depth=1", "https://example.com/repo", "main"},
			want: []string{"fetch", "--depth=1", "--end-of-options", "https://example.com/repo", "main"},
		},
		{
			name: "malicious ref is kept as a positional and never parsed as an option",
			in:   []string{"fetch", "--depth=1", "https://example.com/repo", "--upload-pack=touch /tmp/pwned"},
			want: []string{"fetch", "--depth=1", "--end-of-options", "https://example.com/repo", "--upload-pack=touch /tmp/pwned"},
		},
		{
			name: "non-fetch subcommand passes through unchanged",
			in:   []string{"checkout", "FETCH_HEAD"},
			want: []string{"checkout", "FETCH_HEAD"},
		},
		{
			name: "init passes through unchanged",
			in:   []string{"init"},
			want: []string{"init"},
		},
		{
			name: "arguments containing spaces are preserved exactly",
			in:   []string{"fetch", "--depth=1", "file:///path with spaces/repo.git", "a ref"},
			want: []string{"fetch", "--depth=1", "--end-of-options", "file:///path with spaces/repo.git", "a ref"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, runWrapper(t, wrapper, tc.in...))
		})
	}
}

// TestGitWrapper_RefusesUnrecognizedFetchShape verifies the wrapper fails closed - refusing the
// fetch rather than passing it through unprotected - for any `git fetch` invocation that doesn't
// match kustomize's known, exact shape (`fetch --depth=1 <repository> <ref>`). This is the
// trade-off of matching that literal shape instead of generically locating the option/positional
// boundary: a kustomize version that changes its git invocation breaks builds loudly here, rather
// than silently losing the argument-injection protection.
func TestGitWrapper_RefusesUnrecognizedFetchShape(t *testing.T) {
	wrapper := newFakeGitWrapper(t)

	testCases := []struct {
		name string
		args []string
	}{
		{name: "bare fetch", args: []string{"fetch"}},
		{name: "fetch with only options, no positionals", args: []string{"fetch", "--all"}},
		{name: "missing the ref positional", args: []string{"fetch", "--depth=1", "https://example.com/repo"}},
		{name: "wrong depth flag", args: []string{"fetch", "--depth=2", "https://example.com/repo", "main"}},
		{name: "an extra flag kustomize doesn't send today", args: []string{"fetch", "--depth=1", "--tags", "https://example.com/repo", "main"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), wrapper, tc.args...)
			out, err := cmd.CombinedOutput()
			require.Error(t, err, "unrecognized fetch shape must be refused, not passed through; output: %s", string(out))
			assert.Contains(t, string(out), "unrecognized")
		})
	}
}

func TestWithGitWrapper_PrependsPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("git wrapper is a POSIX shell script, not supported on Windows")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}

	env, err := withGitWrapper([]string{"PATH=/usr/bin:/bin", "FOO=bar"})
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
	// The wrapper directory must come first so kustomize resolves it ahead of the real git.
	first, _, _ := strings.Cut(pathVal, string(os.PathListSeparator))
	assert.DirExists(t, first)
	assert.FileExists(t, filepath.Join(first, "git"))
	assert.True(t, strings.HasSuffix(pathVal, "/usr/bin:/bin"), "original PATH must be preserved after the wrapper dir: %s", pathVal)
}
