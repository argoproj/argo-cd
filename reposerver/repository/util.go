package repository

import (
	"path/filepath"
	"strings"

	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
)

// GetCommonRootPath identifies the common root path among a set of application-related paths.
func GetCommonRootPath(q *apiclient.ManifestRequest, appPath, repoPath string) string {
	paths := GetAppPaths(q, appPath, repoPath)
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

// GetAppPaths retrieves all absolute paths associated with the generation of application manifests.
func GetAppPaths(q *apiclient.ManifestRequest, appPath, repoPath string) []string {
	var appPaths []string
	if q.AnnotationManifestGeneratePaths != "" {
		for _, annotationPath := range strings.Split(q.AnnotationManifestGeneratePaths, ";") {
			if annotationPath == "" {
				continue
			}
			if filepath.IsAbs(annotationPath) {
				appPaths = append(appPaths, filepath.Clean(filepath.Join(repoPath, annotationPath)))
			} else {
				appPaths = append(appPaths, filepath.Clean(filepath.Join(appPath, annotationPath)))
			}
		}
	} else {
		appPaths = append(appPaths, appPath)
	}
	return appPaths
}
