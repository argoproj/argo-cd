package repository

import (
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/io/files"
)

// getApplicationRootPath returns the common root path (shortest shared structure between all paths) among a
// set of application-related paths for manifest generation. AppPath is the lower possible value
func getApplicationRootPath(q *apiclient.ManifestRequest, appPath, repoPath string) string {
	paths := getPaths(q, appPath, repoPath)

	if len(paths) == 0 {
		// backward compatibility, by default the root path is the repoPath
		return repoPath
	}

	// the app path must be the lower possible value
	commonParts := strings.Split(appPath, string(filepath.Separator))

	var disjoint bool
	for _, path := range paths {
		parts := strings.Split(path, string(filepath.Separator))
		// find the minimum length between the current common parts and the current path
		minLen := func(a, b int) int {
			if a < b {
				return a
			}
			return b
		}(len(commonParts), len(parts))

		// check if diverge /disjoint in some point
		for i := 0; i < minLen; i++ {
			if commonParts[i] != parts[i] {
				commonParts = commonParts[:i]
				disjoint = true
				break
			}
		}

		// for non-disjoint paths
		if !disjoint && minLen < len(commonParts) {
			commonParts = commonParts[:minLen]
		}
	}
	return string(filepath.Separator) + filepath.Join(commonParts...)
}

// getPaths retrieves all absolute paths associated with the generation of application manifests.
func getPaths(q *apiclient.ManifestRequest, appPath, repoPath string) []string {
	var paths []string
	for _, annotationPath := range strings.Split(q.AnnotationManifestGeneratePaths, ";") {
		if annotationPath == "" {
			continue
		}
		var err error
		var path, unsafePath string

		if filepath.IsAbs(annotationPath) {
			unsafePath = filepath.Clean(annotationPath)
		} else {
			appRelPath, err := files.RelativePath(appPath, repoPath)
			if err != nil {
				log.Errorf("error building app relative path: %v", err)
				continue
			}
			unsafePath = filepath.Clean(filepath.Join(appRelPath, annotationPath))
		}

		path, err = securejoin.SecureJoin(repoPath, unsafePath)
		if err != nil {
			log.Errorf("error joining repoPath %q and absolute unsafePath %q: %v", repoPath, unsafePath, err)
			continue
		}

		paths = append(paths, path)
	}
	return paths
}
