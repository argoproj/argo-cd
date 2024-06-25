package path

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/io/files"
	"github.com/argoproj/argo-cd/v2/util/security"
)

func Path(root, path string) (string, error) {
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("%s: app path is absolute", path)
	}
	appPath := filepath.Join(root, path)
	if !strings.HasPrefix(appPath, filepath.Clean(root)) {
		return "", fmt.Errorf("%s: app path outside root", path)
	}
	info, err := os.Stat(appPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("%s: app path does not exist", path)
	}
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s: app path is not a directory", path)
	}
	return appPath, nil
}

type OutOfBoundsSymlinkError struct {
	File string
	Err  error
}

func (e *OutOfBoundsSymlinkError) Error() string {
	return "out of bounds symlink found"
}

// CheckOutOfBoundsSymlinks determines if basePath contains any symlinks that
// are absolute or point to a path outside of the basePath. If found, an
// OutOfBoundsSymlinkError is returned.
func CheckOutOfBoundsSymlinks(basePath string) error {
	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	return filepath.Walk(absBasePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk for symlinks in %s: %w", absBasePath, err)
		}
		if files.IsSymlink(info) {
			// We don't use filepath.EvalSymlinks because it fails without returning a path
			// if the target doesn't exist.
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("failed to read link %s: %w", path, err)
			}
			// get the path of the symlink relative to basePath, used for error description
			linkRelPath, err := filepath.Rel(absBasePath, path)
			if err != nil {
				return fmt.Errorf("failed to get relative path for symlink: %w", err)
			}
			// deny all absolute symlinks
			if filepath.IsAbs(linkTarget) {
				return &OutOfBoundsSymlinkError{File: linkRelPath}
			}
			// get the parent directory of the symlink
			currentDir := filepath.Dir(path)

			// walk each part of the symlink target to make sure it never leaves basePath
			parts := strings.Split(linkTarget, string(os.PathSeparator))
			for _, part := range parts {
				newDir := filepath.Join(currentDir, part)
				rel, err := filepath.Rel(absBasePath, newDir)
				if err != nil {
					return fmt.Errorf("failed to get relative path for symlink target: %w", err)
				}
				if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
					// return an error so we don't keep traversing the tree
					return &OutOfBoundsSymlinkError{File: linkRelPath}
				}
				currentDir = newDir
			}
		}
		return nil
	})
}

// GetAppRefreshPaths returns the list of paths that should trigger a refresh for an application
func GetAppRefreshPaths(app *v1alpha1.Application) []string {
	var paths []string
	if val, ok := app.Annotations[v1alpha1.AnnotationKeyManifestGeneratePaths]; ok && val != "" {
		for _, item := range strings.Split(val, ";") {
			if item == "" {
				continue
			}
			if filepath.IsAbs(item) {
				paths = append(paths, item[1:])
			} else {
				for _, source := range app.Spec.GetSources() {
					paths = append(paths, filepath.Clean(filepath.Join(source.Path, item)))
				}
			}
		}
	}
	return paths
}

// AppFilesHaveChanged returns true if any of the changed files are under the given refresh paths
// If refreshPaths or changedFiles are empty, it will always return true
func AppFilesHaveChanged(refreshPaths []string, changedFiles []string) bool {
	// an empty slice of changed files means that the payload didn't include a list
	// of changed files and we have to assume that a refresh is required
	if len(changedFiles) == 0 {
		return true
	}

	if len(refreshPaths) == 0 {
		// Apps without a given refreshed paths always be refreshed, regardless of changed files
		// this is the "default" behavior
		return true
	}

	// At last one changed file must be under refresh path
	for _, f := range changedFiles {
		f = ensureAbsPath(f)
		for _, item := range refreshPaths {
			item = ensureAbsPath(item)
			if f == item {
				return true
			} else if _, err := security.EnforceToCurrentRoot(item, f); err == nil {
				return true
			} else if matched, err := filepath.Match(item, f); err == nil && matched {
				return true
			}
		}
	}

	return false
}

func ensureAbsPath(input string) string {
	if !filepath.IsAbs(input) {
		return string(filepath.Separator) + input
	}
	return input
}
