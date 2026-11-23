package commands

import (
	"fmt"
	"io"
	"os/exec"
	"slices"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/settings"
	argoappv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// localKustomizeOptions returns the Kustomize options to use when an application's manifests are
// rendered on the local machine, e.g. by `argocd app diff --local`.
//
// The Kustomize binary paths registered in argocd-cm refer to the repo-server's filesystem and are
// often not present on the machine running the CLI. When the binary for the application's Kustomize
// version is not available locally, the version entry is kept but its path is cleared, so that the
// kustomize binary found in PATH is used, and a warning is written. A version that the server does
// not know about is left untouched, so local manifest generation fails the same way the repo-server
// does.
func localKustomizeOptions(argoSettings *settings.Settings, source argoappv1.ApplicationSource, warnings io.Writer) *argoappv1.KustomizeOptions {
	opts := &argoappv1.KustomizeOptions{}
	if argoSettings.KustomizeOptions != nil {
		opts = argoSettings.KustomizeOptions.DeepCopy()
	}
	if source.Kustomize == nil || source.Kustomize.Version == "" {
		return opts
	}
	version := source.Kustomize.Version

	idx := slices.IndexFunc(opts.Versions, func(v argoappv1.KustomizeVersion) bool { return v.Name == version })
	if idx >= 0 {
		if _, err := exec.LookPath(opts.Versions[idx].Path); err == nil {
			return opts
		}
		_, _ = fmt.Fprintf(warnings, "Warning: Kustomize %s binary %q is not available locally, using kustomize from PATH instead\n", version, opts.Versions[idx].Path)
		opts.Versions[idx].Path = ""
		return opts
	}

	// Servers that predate sharing version-specific Kustomize settings with the CLI only return the
	// names of the registered versions.
	if slices.Contains(argoSettings.KustomizeVersions, version) {
		_, _ = fmt.Fprintf(warnings, "Warning: the server did not provide the settings for Kustomize %s, using kustomize from PATH instead\n", version)
		opts.Versions = append(opts.Versions, argoappv1.KustomizeVersion{Name: version})
	}
	return opts
}
