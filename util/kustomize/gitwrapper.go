package kustomize

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

// gitWrapperScriptTmpl is a POSIX shell wrapper that is installed ahead of the real `git` binary on
// the PATH of the `kustomize build` subprocess.
//
// Kustomize resolves remote bases/resources/components with exactly one shape of git invocation:
// `git fetch --depth=1 <repository> <ref>` (see sigs.k8s.io/kustomize api/internal/git/cloner.go).
// The <ref>, extracted from the resource URL's `?ref=`/`?version=` query, is passed as a trailing
// positional argument WITHOUT a `--`/`--end-of-options` separator and without rejecting refs that
// begin with `-`. A crafted ref such as `--upload-pack=<cmd>` is therefore parsed by git as an
// option, achieving arbitrary command execution on the repo-server.
//
// The wrapper closes this by matching that exact invocation shape and inserting `--end-of-options`
// between the known `--depth=1` flag and the two positionals, so <repository> and <ref> can never
// be interpreted as git options. Every other git subcommand is passed through unchanged. Unlike
// setting GIT_ALLOW_PROTOCOL to an empty allowlist (which disables all remote bases entirely), this
// preserves the remote-base feature for trusted, Git-backed Applications while still neutralizing
// the injection. Because kustomize resolves remote bases recursively and performs each fetch as a
// separate `git` exec, the wrapper also protects transitively-referenced remote bases, which a
// pre-flight scan of the on-disk kustomization could not see.
//
// A `fetch` invocation that doesn't match this exact shape - including a future kustomize version
// calling git differently - is refused outright rather than passed through unprotected, since there
// is no safe way to know where its positionals start without a case this wrapper wasn't written to
// handle. This trades resilience to a kustomize invocation change (which now breaks builds loudly
// instead of continuing to work unprotected) for a much smaller, more literal script than one that
// generically locates the option/positional boundary. See GHSA-9v9p-x54c-58gc.
//
// `--end-of-options` is understood by git's option parser since v2.24 (2019); the repo-server ships
// a newer git. The %s is replaced with the absolute path of the real git binary, resolved once at
// install time so the wrapper never recurses into itself.
const gitWrapperScriptTmpl = `#!/bin/sh
REAL_GIT='%s'
if [ "$1" != "fetch" ]; then
	exec "$REAL_GIT" "$@"
fi
if [ "$#" -eq 4 ] && [ "$2" = "--depth=1" ]; then
	exec "$REAL_GIT" "$1" "$2" --end-of-options "$3" "$4"
fi
echo "git wrapper: unrecognized 'git fetch' invocation, refusing (see GHSA-9v9p-x54c-58gc)" >&2
exit 1
`

// buildGitWrapperScript returns the wrapper script that shells out to realGit.
func buildGitWrapperScript(realGit string) string {
	return fmt.Sprintf(gitWrapperScriptTmpl, realGit)
}

var (
	gitWrapperOnce sync.Once
	gitWrapperDir  string
	gitWrapperErr  error
)

// installGitWrapperDir writes the git wrapper script into a temp directory (once per process) and
// returns that directory, suitable for prepending to the PATH of a `kustomize build` subprocess.
//
// If git is not on the PATH there is nothing to wrap (and no fetch can occur), so it returns an
// empty dir and no error. On Windows the shell wrapper is not supported, so it is a no-op; the
// repo-server that runs kustomize builds only runs on Linux.
func installGitWrapperDir() (string, error) {
	gitWrapperOnce.Do(func() {
		if runtime.GOOS == "windows" {
			return
		}
		realGit, err := exec.LookPath("git")
		if err != nil {
			// No git available: kustomize cannot fetch remote bases, so there is nothing to
			// protect. Leave the wrapper uninstalled rather than failing local builds.
			log.Warnf("git not found on PATH; kustomize remote-base argument-injection guard not installed: %v", err)
			return
		}
		if strings.ContainsAny(realGit, "'\n") {
			gitWrapperErr = fmt.Errorf("resolved git path %q contains an unexpected character; refusing to install git wrapper", realGit)
			return
		}
		dir, err := os.MkdirTemp("", "argocd-git-wrapper-")
		if err != nil {
			gitWrapperErr = fmt.Errorf("failed to create git wrapper dir: %w", err)
			return
		}
		if err := os.WriteFile(filepath.Join(dir, "git"), []byte(buildGitWrapperScript(realGit)), 0o700); err != nil {
			gitWrapperErr = fmt.Errorf("failed to write git wrapper: %w", err)
			return
		}
		gitWrapperDir = dir
	})
	return gitWrapperDir, gitWrapperErr
}

// withGitWrapper returns env with the git wrapper directory prepended to PATH so that a
// `kustomize build` subprocess resolves our wrapper instead of the real git for its remote-base
// fetches. If the wrapper could not be installed because git is unavailable (or on Windows), env is
// returned unchanged. See gitWrapperScriptTmpl / GHSA-9v9p-x54c-58gc.
func withGitWrapper(env []string) ([]string, error) {
	dir, err := installGitWrapperDir()
	if err != nil {
		return nil, err
	}
	if dir == "" {
		return env, nil
	}
	out := make([]string, 0, len(env)+1)
	found := false
	for _, e := range env {
		if path, ok := strings.CutPrefix(e, "PATH="); ok {
			out = append(out, "PATH="+dir+string(os.PathListSeparator)+path)
			found = true
		} else {
			out = append(out, e)
		}
	}
	if !found {
		out = append(out, "PATH="+dir)
	}
	return out, nil
}
