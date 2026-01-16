package repository

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
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

// ResolveValueFiles resolves a list of raw value file paths to their resolved paths
func (r *ValueFileResolver) ResolveValueFiles(rawValueFiles []string) ([]pathutil.ResolvedFilePath, error) {
	var resolvedValueFiles []pathutil.ResolvedFilePath

	for _, rawValueFile := range rawValueFiles {
		resolvedPath, shouldSkip, err := r.resolveValueFile(rawValueFile)
		if err != nil {
			return nil, err
		}
		if shouldSkip {
			continue
		}
		resolvedValueFiles = append(resolvedValueFiles, resolvedPath)
	}

	return resolvedValueFiles, nil
}

// resolveValueFile resolves a single value file path
func (r *ValueFileResolver) resolveValueFile(rawValueFile string) (pathutil.ResolvedFilePath, bool, error) {
	referencedSource := getReferencedSource(rawValueFile, r.refSources)

	if referencedSource != nil {
		return r.resolveReferencedValueFile(rawValueFile, referencedSource)
	}

	return r.resolveLocalValueFile(rawValueFile)
}

// resolveReferencedValueFile handles value files that reference other sources
func (r *ValueFileResolver) resolveReferencedValueFile(
	rawValueFile string,
	referencedSource *v1alpha1.RefTarget,
) (pathutil.ResolvedFilePath, bool, error) {
	var resolvedPath pathutil.ResolvedFilePath
	var err error

	if referencedSource.Repo.IsOCI() {
		resolvedPath, err = getResolvedOCIRefValueFile(
			rawValueFile,
			r.env,
			r.allowedValueFilesSchemas,
			referencedSource.Repo.Repo,
			r.ociPaths,
		)
		if err != nil {
			return "", false, fmt.Errorf("error resolving OCI value file path: %w", err)
		}
	} else {
		resolvedPath, err = getResolvedRefValueFile(
			rawValueFile,
			r.env,
			r.allowedValueFilesSchemas,
			referencedSource.Repo.Repo,
			r.gitRepoPaths,
		)
		if err != nil {
			return "", false, fmt.Errorf("error resolving value file path: %w", err)
		}
	}

	return resolvedPath, false, nil
}

// resolveLocalValueFile handles value files that are local to the repository
func (r *ValueFileResolver) resolveLocalValueFile(rawValueFile string) (pathutil.ResolvedFilePath, bool, error) {
	// This will resolve val to an absolute path (or a URL)
	resolvedPath, isRemote, err := pathutil.ResolveValueFilePathOrUrl(
		r.appPath,
		r.repoRoot,
		r.env.Envsubst(rawValueFile),
		r.allowedValueFilesSchemas,
	)
	if err != nil {
		return "", false, fmt.Errorf("error resolving value file path: %w", err)
	}

	if !isRemote {
		if r.checkFileExists(resolvedPath) {
			return "", true, nil
		}
	}

	return resolvedPath, false, nil
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
