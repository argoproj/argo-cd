package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/util/io/files"
)

// validatePathBoundaries checks that tool configurations don't reference paths outside appPath
// when strict manifest generation policy is active. It rejects Kustomize and Jsonnet source types
// entirely, and validates Helm dependencies and value files stay within bounds.
func validatePathBoundaries(appSourceType v1alpha1.ApplicationSourceType, appPath, repoRoot string, q *apiclient.ManifestRequest) error {
	switch appSourceType {
	case v1alpha1.ApplicationSourceTypeKustomize:
		return fmt.Errorf("Kustomize source type is not supported under strict manifest generation policy")
	case v1alpha1.ApplicationSourceTypeDirectory:
		if err := validateNoJsonnet(appPath, q); err != nil {
			return err
		}
	case v1alpha1.ApplicationSourceTypeHelm:
		if err := validateHelmPathBoundaries(appPath, repoRoot, q); err != nil {
			return err
		}
	}
	return nil
}

// validateNoJsonnet checks that the Directory source type doesn't use Jsonnet files or configuration.
func validateNoJsonnet(appPath string, q *apiclient.ManifestRequest) error {
	// Check if the source has explicit Jsonnet configuration
	if q.ApplicationSource != nil && q.ApplicationSource.Directory != nil &&
		q.ApplicationSource.Directory.Jsonnet.Libs != nil {
		return fmt.Errorf("Jsonnet is not supported under strict manifest generation policy")
	}

	// Check for .jsonnet or .libsonnet files in the app directory
	entries, err := os.ReadDir(appPath)
	if err != nil {
		return fmt.Errorf("error reading app directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".jsonnet") || strings.HasSuffix(name, ".libsonnet") {
			return fmt.Errorf("Jsonnet is not supported under strict manifest generation policy: found %s", name)
		}
	}

	return nil
}

// helmDependencies mirrors the structure for parsing Chart.yaml dependencies.
type helmDependencies struct {
	Dependencies []helmDependency `yaml:"dependencies"`
}

type helmDependency struct {
	Repository string `yaml:"repository"`
}

// validateHelmPathBoundaries checks that Helm chart dependencies with file:// repositories
// and value files don't reference paths outside appPath.
func validateHelmPathBoundaries(appPath, repoRoot string, q *apiclient.ManifestRequest) error {
	// Check Chart.yaml dependencies for file:// references
	chartPath := filepath.Join(appPath, "Chart.yaml")
	if chartData, err := os.ReadFile(chartPath); err == nil {
		var deps helmDependencies
		if err := yaml.Unmarshal(chartData, &deps); err == nil {
			for _, dep := range deps.Dependencies {
				if strings.HasPrefix(dep.Repository, "file://") {
					depPath := strings.TrimPrefix(dep.Repository, "file://")
					resolved := filepath.Clean(filepath.Join(appPath, depPath))
					if !files.Inbound(resolved, appPath) {
						return fmt.Errorf("Helm dependency %q references path outside source directory", dep.Repository)
					}
				}
			}
		}
	}

	// Check value files
	if q.ApplicationSource != nil && q.ApplicationSource.Helm != nil {
		for _, vf := range q.ApplicationSource.Helm.ValueFiles {
			if strings.Contains(vf, "$") {
				// Referenced sources are external repos — skip boundary check
				continue
			}
			if isRemoteURL(vf) {
				continue
			}
			resolved := filepath.Clean(filepath.Join(appPath, vf))
			if !files.Inbound(resolved, appPath) {
				return fmt.Errorf("Helm values file %q references path outside source directory", vf)
			}
		}
	}

	return nil
}

// isRemoteURL checks if the given path is a remote URL (http/https/oci).
func isRemoteURL(path string) bool {
	return strings.HasPrefix(path, "http://") ||
		strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "oci://")
}
