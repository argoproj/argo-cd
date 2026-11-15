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

// helmWrapperScriptTmpl is a POSIX shell wrapper that is installed ahead of the real `helm` binary
// on the PATH of the `kustomize build` subprocess.
//
// When --enable-helm is set, kustomize's HelmChartInflationGenerator invokes `helm` with an
// environment built as `append(os.Environ(), "HELM_CONFIG_HOME=<configHome>",
// "HELM_DATA_HOME=<configHome>/.data")` (see sigs.k8s.io/kustomize
// api/internal/builtins/HelmChartInflationGenerator.go), where configHome defaults to a
// kustomize-owned tmp dir but can be overridden by the kustomization's own `helmGlobals.configHome`
// field - fully attacker/app-controlled content. Helm auto-loads any plugin under
// $HELM_DATA_HOME/plugins/*/plugin.yaml with no explicit `helm plugin install` step, and invokes a
// plugin's declared "downloader" command verbatim for a repo URL matching its protocol - arbitrary
// command execution once a helmCharts entry's repo uses a matching custom scheme. See
// GHSA-fw5c-w8rc-j7fx.
//
// The wrapper discards whatever HELM_CONFIG_HOME/HELM_DATA_HOME/HELM_PLUGINS it is invoked with and
// always substitutes a fixed, Argo CD-owned directory before exec'ing the real helm binary. This
// keeps --enable-helm's chart-inflation feature working normally - fetching a helmCharts entry from
// a plain http/https/oci repo needs no plugin at all - while no kustomization file, trusted or not,
// can make Helm load a plugin from a directory it names itself.
//
// Unlike the git wrapper, this one does not need to match a specific dangerous invocation shape and
// pass everything else through unprotected: every helm invocation kustomize makes for chart
// inflation is safe to run against a fixed, empty plugin directory, so the substitution applies
// unconditionally to every argument list.
//
// %[1]s is the absolute path of the real helm binary, resolved once at install time so the wrapper
// never recurses into itself; %[2]s is the fixed, empty HELM_DATA_HOME.
const helmWrapperScriptTmpl = `#!/bin/sh
REAL_HELM='%[1]s'
export HELM_DATA_HOME='%[2]s'
export HELM_CONFIG_HOME='%[2]s'
export HELM_PLUGINS='%[2]s/plugins'
exec "$REAL_HELM" "$@"
`

// buildHelmWrapperScript returns the wrapper script that shells out to realHelm with
// safeDataHome substituted for HELM_DATA_HOME/HELM_CONFIG_HOME/HELM_PLUGINS.
func buildHelmWrapperScript(realHelm, safeDataHome string) string {
	return fmt.Sprintf(helmWrapperScriptTmpl, realHelm, safeDataHome)
}

var (
	helmWrapperOnce sync.Once
	helmWrapperDir  string
	helmWrapperErr  error
)

// installHelmWrapperDir writes the helm wrapper script and its fixed, empty HELM_DATA_HOME into a
// temp directory (once per process) and returns that directory, suitable for prepending to the
// PATH of a `kustomize build` subprocess.
//
// If helm is not on the PATH there is nothing to wrap (--enable-helm has nothing to invoke), so it
// returns an empty dir and no error. On Windows the shell wrapper is not supported, so it is a
// no-op; the repo-server that runs kustomize builds only runs on Linux.
func installHelmWrapperDir() (string, error) {
	helmWrapperOnce.Do(func() {
		if runtime.GOOS == "windows" {
			return
		}
		realHelm, err := exec.LookPath("helm")
		if err != nil {
			// No helm available: kustomize's --enable-helm has nothing to invoke, so there is
			// nothing to protect. Leave the wrapper uninstalled rather than failing local builds.
			log.Warnf("helm not found on PATH; kustomize helm-plugin-loading guard not installed: %v", err)
			return
		}
		if strings.ContainsAny(realHelm, "'\n") {
			helmWrapperErr = fmt.Errorf("resolved helm path %q contains an unexpected character; refusing to install helm wrapper", realHelm)
			return
		}
		dir, err := os.MkdirTemp("", "argocd-helm-wrapper-")
		if err != nil {
			helmWrapperErr = fmt.Errorf("failed to create helm wrapper dir: %w", err)
			return
		}
		safeDataHome := filepath.Join(dir, "data")
		if err := os.MkdirAll(safeDataHome, 0o700); err != nil {
			helmWrapperErr = fmt.Errorf("failed to create safe HELM_DATA_HOME: %w", err)
			return
		}
		script := buildHelmWrapperScript(realHelm, safeDataHome)
		if err := os.WriteFile(filepath.Join(dir, "helm"), []byte(script), 0o700); err != nil {
			helmWrapperErr = fmt.Errorf("failed to write helm wrapper: %w", err)
			return
		}
		helmWrapperDir = dir
	})
	return helmWrapperDir, helmWrapperErr
}

// withHelmWrapper returns env with the helm wrapper directory prepended to PATH so that a
// `kustomize build` subprocess resolves our wrapper instead of the real helm for any chart
// inflation --enable-helm triggers. If the wrapper could not be installed because helm is
// unavailable (or on Windows), env is returned unchanged. See helmWrapperScriptTmpl /
// GHSA-fw5c-w8rc-j7fx.
func withHelmWrapper(env []string) ([]string, error) {
	dir, err := installHelmWrapperDir()
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
