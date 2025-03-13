package path

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

// ResolvedFilePath represents a resolved file path and is intended to prevent unintentional use of an unverified file
// path. It is always either a URL or an absolute path.
type ResolvedFilePath string

// ResolvedFileOrDirectoryPath represents a resolved file or directory path and is intended to prevent unintentional use
// of an unverified file or directory path. It is an absolute path.
type ResolvedFileOrDirectoryPath string

// resolveSymbolicLinkRecursive resolves the symlink path recursively to its
// canonical path on the file system, with a maximum nesting level of maxDepth.
// If path is not a symlink, returns the verbatim copy of path and err of nil.
func resolveSymbolicLinkRecursive(path string, maxDepth int) (string, error) {
	resolved, err := os.Readlink(path)
	if err != nil {
		// path is not a symbolic link
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			return path, nil
		}
		// Other error has occurred
		return "", err
	}

	if maxDepth == 0 {
		return "", fmt.Errorf("maximum nesting level reached")
	}

	// If we resolved to a relative symlink, make sure we use the absolute
	// path for further resolving
	if !strings.HasPrefix(resolved, string(os.PathSeparator)) {
		basePath := filepath.Dir(path)
		resolved = filepath.Join(basePath, resolved)
	}

	return resolveSymbolicLinkRecursive(resolved, maxDepth-1)
}

// isURLSchemeAllowed returns true if the protocol scheme is in the list of
// allowed URL schemes.
func isURLSchemeAllowed(scheme string, allowed []string) bool {
	isAllowed := false
	if len(allowed) > 0 {
		for _, s := range allowed {
			if strings.EqualFold(scheme, s) {
				isAllowed = true
				break
			}
		}
	}

	// Empty scheme means local file
	return isAllowed && scheme != ""
}

// We do not provide the path in the error message, because it will be
// returned to the user and could be used for information gathering.
// Instead, we log the concrete error details.
func resolveFailure(path string, err error) error {
	log.Errorf("failed to resolve path '%s': %v", path, err)
	return fmt.Errorf("internal error: failed to resolve path. Check logs for more details")
}

func ResolveFileOrDirectoryPath(appPath, repoRoot, dir string) (ResolvedFileOrDirectoryPath, error) {
	path, err := resolveFileOrDirectory(appPath, repoRoot, dir, true)
	if err != nil {
		return "", err
	}

	return ResolvedFileOrDirectoryPath(path), nil
}

// ResolveValueFilePathOrUrl will inspect and resolve given file, and make sure that its final path is within the boundaries of
// the path specified in repoRoot.
//
// appPath is the path we're operating in, e.g. where a Helm chart was unpacked
// to. repoRoot is the path to the root of the repository.
//
// If either appPath or repoRoot is relative, it will be treated as relative
// to the current working directory.
//
// valueFile is the path to a value file, relative to appPath. If valueFile is
// specified as an absolute path (i.e. leading slash), it will be treated as
// relative to the repoRoot. In case valueFile is a symlink in the extracted
// chart, it will be resolved recursively and the decision of whether it is in
// the boundary of repoRoot will be made using the final resolved path.
// valueFile can also be a remote URL with a protocol scheme as prefix,
// in which case the scheme must be included in the list of allowed schemes
// specified by allowedURLSchemes.
//
// Will return an error if either valueFile is outside the boundaries of the
// repoRoot, valueFile is an URL with a forbidden protocol scheme or if
// valueFile is a recursive symlink nested too deep. May return errors for
// other reasons as well.
//
// resolvedPath will hold the absolute, resolved path for valueFile on success
// or set to the empty string on failure.
//
// isRemote will be set to true if valueFile is an URL using an allowed
// protocol scheme, or to false if it resolved to a local file.
func ResolveValueFilePathOrUrl(appPath, repoRoot, valueFile string, allowedURLSchemes []string) (resolvedPath ResolvedFilePath, isRemote bool, err error) {
	// A value file can be specified as an URL to a remote resource.
	// We only allow certain URL schemes for remote value files.
	url, err := url.Parse(valueFile)
	if err == nil {
		// If scheme is empty, it means we parsed a path only
		if url.Scheme != "" {
			if isURLSchemeAllowed(url.Scheme, allowedURLSchemes) {
				return ResolvedFilePath(valueFile), true, nil
			} else {
				return "", false, fmt.Errorf("the URL scheme '%s' is not allowed", url.Scheme)
			}
		}
	}

	path, err := resolveFileOrDirectory(appPath, repoRoot, valueFile, false)
	if err != nil {
		return "", false, err
	}

	return ResolvedFilePath(path), false, nil
}

func resolveFileOrDirectory(appPath string, repoRoot string, fileOrDirectory string, allowResolveToRoot bool) (string, error) {
	// Ensure that our repository root is absolute
	absRepoPath, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", resolveFailure(repoRoot, err)
	}

	// If the path to the file or directory is relative, join it with the current working directory (appPath)
	// Otherwise, join it with the repository's root
	path := fileOrDirectory
	if !filepath.IsAbs(path) {
		absWorkDir, err := filepath.Abs(appPath)
		if err != nil {
			return "", resolveFailure(repoRoot, err)
		}
		path = filepath.Join(absWorkDir, path)
	} else {
		path = filepath.Join(absRepoPath, path)
	}

	// Ensure any symbolic link is resolved before we evaluate the path
	delinkedPath, err := resolveSymbolicLinkRecursive(path, 10)
	if err != nil {
		return "", resolveFailure(repoRoot, err)
	}
	path = delinkedPath

	// Resolve the joined path to an absolute path
	path, err = filepath.Abs(path)
	if err != nil {
		return "", resolveFailure(repoRoot, err)
	}

	// Ensure our root path has a trailing slash, otherwise the following check
	// would return true if root is /foo and path would be /foo2
	requiredRootPath := absRepoPath
	if !strings.HasSuffix(requiredRootPath, string(os.PathSeparator)) {
		requiredRootPath += string(os.PathSeparator)
	}

	resolvedToRoot := path+string(os.PathSeparator) == requiredRootPath
	if resolvedToRoot {
		if !allowResolveToRoot {
			return "", resolveFailure(path, fmt.Errorf("path resolved to repository root, which is not allowed"))
		}
	} else {
		// Make sure that the resolved path to file is within the repository's root path
		if !strings.HasPrefix(path, requiredRootPath) {
			return "", fmt.Errorf("file '%s' resolved to outside repository root", fileOrDirectory)
		}
	}

	return path, nil
}
