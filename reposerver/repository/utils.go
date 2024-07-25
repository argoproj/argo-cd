package repository

import (
	"path/filepath"
	"strings"

	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
)

// GetApplicationRootPath returns the common root path among a set of application-related paths for manifest generation.
func GetApplicationRootPath(q *apiclient.ManifestRequest, appPath, repoPath string) string {
	paths := getPaths(q, appPath, repoPath)

	if len(paths) == 0 {
		// backward compatibility, by default the root path is the repoPath
		return repoPath
	}

	commonParts := strings.Split(paths[0], "/")

	for _, path := range paths[1:] {
		parts := strings.Split(path, "/")
		// Find the minimum length between the current common parts and the current path
		minLen := func(a, b int) int {
			if a < b {
				return a
			}
			return b
		}(len(commonParts), len(parts))

		for i := 0; i < minLen; i++ {
			if commonParts[i] != parts[i] {
				commonParts = commonParts[:i]
				break
			}
		}
	}
	return strings.Join(commonParts, "/")
}

// getPaths retrieves all absolute paths associated with the generation of application manifests.
func getPaths(q *apiclient.ManifestRequest, appPath, repoPath string) []string {
	var paths []string
	for _, annotationPath := range strings.Split(q.AnnotationManifestGeneratePaths, ";") {
		if annotationPath == "" {
			continue
		}
		if filepath.IsAbs(annotationPath) {
			paths = append(paths, filepath.Clean(filepath.Join(repoPath, annotationPath)))
		} else {
			paths = append(paths, filepath.Clean(filepath.Join(appPath, annotationPath)))
		}
	}
	return paths
}
