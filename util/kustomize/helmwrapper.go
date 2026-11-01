package kustomize

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// When --enable-helm is set, kustomize's HelmChartInflationGenerator invokes `helm` with an
// environment built as `append(os.Environ(), "HELM_CONFIG_HOME=<configHome>",
// "HELM_CACHE_HOME=<configHome>/.cache", "HELM_DATA_HOME=<configHome>/.data")` (see
// sigs.k8s.io/kustomize api/internal/builtins/HelmChartInflationGenerator.go), where configHome
// defaults to a kustomize-owned tmp dir but can be overridden by the kustomization's own
// `helmGlobals.configHome` field - fully attacker/app-controlled content. Helm auto-loads any
// plugin under its plugins directory with no explicit `helm plugin install` step, and invokes a
// plugin's declared "downloader" command verbatim for a repo URL matching its protocol - arbitrary
// command execution once a helmCharts entry's repo uses a matching custom scheme. See
// GHSA-fw5c-w8rc-j7fx.
//
// Helm resolves its plugins directory from HELM_PLUGINS, which takes precedence over the
// HELM_DATA_HOME/plugins default. Kustomize never sets HELM_PLUGINS and builds helm's environment
// by appending to os.Environ(), so a HELM_PLUGINS value we place in the `kustomize build`
// subprocess environment passes straight through to every helm invocation untouched. Pointing it
// at a fixed, empty, Argo CD-owned directory keeps --enable-helm's chart-inflation feature working
// normally - fetching a helmCharts entry from a plain http/https/oci repo needs no plugin at all -
// while no kustomization file, trusted or not, can make Helm load a plugin from a directory it
// names itself via helmGlobals.configHome.
//
// This replaces an earlier approach that prepended a shell wrapper script to the subprocess PATH.
// That wrapper had to be written as an executable file under os.TempDir(), which broke on clusters
// whose runtime-security policies forbid executing binaries from /tmp (the helm call was SIGKILLed
// before it could run). Setting an environment variable needs no executable on disk.

var (
	helmPluginsOnce sync.Once
	helmPluginsDir  string
	helmPluginsErr  error
)

// safeHelmPluginsDir creates a fixed, empty plugins directory (once per process) and returns it,
// suitable for use as HELM_PLUGINS for a `kustomize build` subprocess. The directory is never
// executed - helm only reads it to enumerate plugins - so unlike an executable wrapper it is not
// subject to noexec or exec-blocking security policies on its containing filesystem.
func safeHelmPluginsDir() (string, error) {
	helmPluginsOnce.Do(func() {
		dir, err := os.MkdirTemp("", "argocd-helm-plugins-")
		if err != nil {
			helmPluginsErr = fmt.Errorf("failed to create safe HELM_PLUGINS dir: %w", err)
			return
		}
		plugins := filepath.Join(dir, "plugins")
		if err := os.MkdirAll(plugins, 0o700); err != nil {
			helmPluginsErr = fmt.Errorf("failed to create safe HELM_PLUGINS dir: %w", err)
			return
		}
		helmPluginsDir = plugins
	})
	return helmPluginsDir, helmPluginsErr
}

// withHelmWrapper returns env with HELM_PLUGINS set to a fixed, empty, Argo CD-owned directory so
// that any helm invocation a `kustomize build` subprocess makes for chart inflation cannot load a
// plugin from a directory the kustomization chose via helmGlobals.configHome. Any HELM_PLUGINS
// already present is replaced. See GHSA-fw5c-w8rc-j7fx.
func withHelmWrapper(env []string) ([]string, error) {
	plugins, err := safeHelmPluginsDir()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(env)+1)
	found := false
	for _, e := range env {
		if _, ok := strings.CutPrefix(e, "HELM_PLUGINS="); ok {
			out = append(out, "HELM_PLUGINS="+plugins)
			found = true
		} else {
			out = append(out, e)
		}
	}
	if !found {
		out = append(out, "HELM_PLUGINS="+plugins)
	}
	return out, nil
}
