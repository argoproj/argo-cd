package repository

import (
	"fmt"
	"os"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/git"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	pathutil "github.com/argoproj/argo-cd/v3/util/io/path"
)

// ValueFileResolver handles resolution of Helm value files from various sources
type ValueFileResolver struct {
	appPath                  string
	repoRoot                 string
	env                      *v1alpha1.Env
	allowedValueFilesSchemas []string
	refSources               map[string]*v1alpha1.RefTarget
	gitRepoPaths             utilio.TempPaths
	ociPaths                 utilio.TempPaths
	ignoreMissingValueFiles  bool
}

// NewValueFileResolver creates a new instance of ValueFileResolver
func NewValueFileResolver(
	appPath string,
	repoRoot string,
	env *v1alpha1.Env,
	allowedValueFilesSchemas []string,
	refSources map[string]*v1alpha1.RefTarget,
	gitRepoPaths utilio.TempPaths,
	ociPaths utilio.TempPaths,
	ignoreMissingValueFiles bool,
) *ValueFileResolver {
	return &ValueFileResolver{
		appPath:                  appPath,
		repoRoot:                 repoRoot,
		env:                      env,
		allowedValueFilesSchemas: allowedValueFilesSchemas,
		refSources:               refSources,
		gitRepoPaths:             gitRepoPaths,
		ociPaths:                 ociPaths,
		ignoreMissingValueFiles:  ignoreMissingValueFiles,
	}
}

// ResolveValueFiles resolves a list of raw value file paths to their resolved paths,
// handling local files, $ref Git/OCI sources, and glob expansion.
func (r *ValueFileResolver) ResolveValueFiles(rawValueFiles []string) ([]pathutil.ResolvedFilePath, error) {
	// Pre-collect resolved paths for all explicit (non-glob) entries. This allows glob
	// expansion to skip files that also appear explicitly, so the explicit entry controls
	// the final position. For example, with ["*.yaml", "c.yaml"], c.yaml is excluded from
	// the glob expansion and placed at the end where it was explicitly listed.
	explicitPaths := make(map[pathutil.ResolvedFilePath]struct{})
	for _, rawValueFile := range rawValueFiles {
		resolved, err := r.resolveRawPath(rawValueFile)
		if err != nil {
			continue // resolution errors will be surfaced in the main loop below
		}
		if !isGlobPath(string(resolved.Path)) {
			explicitPaths[resolved.Path] = struct{}{}
		}
	}

	var resolvedValueFiles []pathutil.ResolvedFilePath
	seen := make(map[pathutil.ResolvedFilePath]struct{})
	appendUnique := func(p pathutil.ResolvedFilePath) {
		if _, ok := seen[p]; !ok {
			seen[p] = struct{}{}
			resolvedValueFiles = append(resolvedValueFiles, p)
		}
	}
	for _, rawValueFile := range rawValueFiles {
		resolved, err := r.resolveRawPath(rawValueFile)
		if err != nil {
			return nil, fmt.Errorf("error resolving value file path: %w", err)
		}

		// If the resolved path contains a glob pattern, expand it to all matching files.
		// doublestar.FilepathGlob is used (consistent with AppSet generators) because it supports
		// ** for recursive matching in addition to all standard glob patterns (*,?,[).
		// Matches are returned in lexical order, which determines helm's merge precedence
		// (later files override earlier ones). Glob patterns are only expanded for local files;
		// remote value file URLs (e.g. https://...) are passed through as-is.
		// If the glob matches no files and ignoreMissingValueFiles is true, skip it silently.
		// Otherwise, return an error — consistent with how missing non-glob value files are handled.
		if !resolved.IsRemote && isGlobPath(string(resolved.Path)) {
			matches, err := doublestar.FilepathGlob(string(resolved.Path))
			if err != nil {
				return nil, fmt.Errorf("error expanding glob pattern %q: %w", rawValueFile, err)
			}
			if len(matches) == 0 {
				if r.ignoreMissingValueFiles {
					log.Debugf(" %s values file glob matched no files", rawValueFile)
					continue
				}
				return nil, &GlobNoMatchError{Pattern: rawValueFile}
			}
			if err := verifyGlobMatchesWithinRoot(matches, resolved.EffectiveRoot); err != nil {
				return nil, fmt.Errorf("glob pattern %q: %w", rawValueFile, err)
			}
			for _, match := range matches {
				// Skip files that are also listed explicitly - they will be placed
				// at their explicit position rather than the glob's position.
				if _, isExplicit := explicitPaths[pathutil.ResolvedFilePath(match)]; !isExplicit {
					appendUnique(pathutil.ResolvedFilePath(match))
				}
			}
			continue
		}

		if !resolved.IsRemote && r.checkFileExists(resolved.Path) {
			continue
		}

		appendUnique(resolved.Path)
	}
	log.Infof("resolved value files: %v", resolvedValueFiles)
	return resolvedValueFiles, nil
}

type resolveRawPathResult struct {
	Path          pathutil.ResolvedFilePath
	IsRemote      bool
	EffectiveRoot string
}

// resolveRawPath resolves a single raw value file entry to its path without expanding
// globs or checking for existence. It returns whether the path is a remote URL and the
// effective repository root used for the glob symlink-boundary check (the external repo's
// checkout directory for $ref Git sources, otherwise the main repo root).
func (r *ValueFileResolver) resolveRawPath(rawValueFile string) (*resolveRawPathResult, error) {
	referencedSource := getReferencedSource(rawValueFile, r.refSources)
	effectiveRoot := r.repoRoot

	if referencedSource != nil {
		// If the $-prefixed path appears to reference another source, do env substitution _after_ resolving that source.
		resolvedPath, err := getResolvedRefValueFile(
			rawValueFile,
			r.env,
			r.allowedValueFilesSchemas,
			referencedSource.Repo.Repo,
			r.gitRepoPaths,
			r.ociPaths,
		)
		if err != nil {
			return nil, err
		}
		if refRepoPath := r.gitRepoPaths.GetPathIfExists(git.NormalizeGitURL(referencedSource.Repo.Repo)); refRepoPath != "" {
			effectiveRoot = refRepoPath
		}
		return &resolveRawPathResult{
			Path:          resolvedPath,
			IsRemote:      false,
			EffectiveRoot: effectiveRoot,
		}, nil
	}

	// This will resolve val to an absolute path (or a URL)
	resolvedPath, isRemote, err := pathutil.ResolveValueFilePathOrUrl(
		r.appPath,
		r.repoRoot,
		r.env.Envsubst(rawValueFile),
		r.allowedValueFilesSchemas,
	)
	if err != nil {
		return nil, err
	}

	return &resolveRawPathResult{
		Path:          resolvedPath,
		IsRemote:      isRemote,
		EffectiveRoot: effectiveRoot,
	}, nil
}

// checkFileExists checks if a file exists and determines if it should be skipped
func (r *ValueFileResolver) checkFileExists(resolvedPath pathutil.ResolvedFilePath) bool {
	_, err := os.Stat(string(resolvedPath))
	if os.IsNotExist(err) {
		if r.ignoreMissingValueFiles {
			log.Debugf(" %s values file does not exist", resolvedPath)
			return true
		}
	}
	return false
}

func getResolvedRefValueFile(
	rawValueFile string,
	env *v1alpha1.Env,
	allowedValueFilesSchemas []string,
	refSourceRepo string,
	gitRepoPaths utilio.TempPaths,
	ociPaths utilio.TempPaths,
) (pathutil.ResolvedFilePath, error) {
	pathStrings := strings.Split(rawValueFile, "/")

	// Check if the reference source is an OCI repository
	if v1alpha1.IsOCIURL(refSourceRepo) {
		return getResolvedOCIRefValueFile(rawValueFile, env, allowedValueFilesSchemas, refSourceRepo, ociPaths)
	}

	// Original Git repository handling
	repoPath := gitRepoPaths.GetPathIfExists(git.NormalizeGitURL(refSourceRepo))
	if repoPath == "" {
		return "", fmt.Errorf("failed to find repo %q", refSourceRepo)
	}
	pathStrings[0] = "" // Remove first segment. It will be inserted by pathutil.ResolveValueFilePathOrUrl.
	substitutedPath := strings.Join(pathStrings, "/")

	// Resolve the path relative to the referenced repo and block any attempt at traversal.
	resolvedPath, _, err := pathutil.ResolveValueFilePathOrUrl(repoPath, repoPath, env.Envsubst(substitutedPath), allowedValueFilesSchemas)
	if err != nil {
		return "", fmt.Errorf("error resolving value file path: %w", err)
	}
	return resolvedPath, nil
}

// getResolvedOCIRefValueFile handles OCI ref values by using the already extracted OCI content
func getResolvedOCIRefValueFile(
	rawValueFile string,
	env *v1alpha1.Env,
	allowedValueFilesSchemas []string,
	refSourceRepo string,
	ociPaths utilio.TempPaths,
) (pathutil.ResolvedFilePath, error) {
	// Get the OCI path from the ociPaths
	ociPath := ociPaths.GetPathIfExists(refSourceRepo)
	if ociPath == "" {
    	log.Errorf("OCI repo %q not found in extracted paths. Available paths: %v", refSourceRepo, ociPaths.GetPaths())
    	return "", fmt.Errorf("OCI ref source %q was not successfully extracted. Ensure the repository is accessible and properly configured", refSourceRepo)
	}

	// Remove first segment (the ref variable name) and resolve the path
	pathStrings := strings.Split(rawValueFile, "/")
	if len(pathStrings) == 0 {
		return "", fmt.Errorf("invalid OCI value file path %q: path is empty", rawValueFile)
	}
	pathStrings[0] = "" // Remove first segment. It will be inserted by pathutil.ResolveValueFilePathOrUrl.
	substitutedPath := strings.Join(pathStrings, "/")

	// Remove leading slash if present (OCI paths are relative to the archive root)
	substitutedPath = strings.TrimPrefix(substitutedPath, "/")

	// Resolve the path relative to the extracted OCI content
	resolvedPath, _, err := pathutil.ResolveValueFilePathOrUrl(ociPath, ociPath, env.Envsubst(substitutedPath), allowedValueFilesSchemas)
	if err != nil {
		return "", fmt.Errorf("error resolving OCI value file path: %w", err)
	}

	return resolvedPath, nil
}
