package repository

import (
	"context"
	"errors"
	"fmt"
	goio "io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	apppathutil "github.com/argoproj/argo-cd/v3/util/app/path"
	"github.com/argoproj/argo-cd/v3/util/git"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	pathutil "github.com/argoproj/argo-cd/v3/util/io/path"
	"github.com/argoproj/argo-cd/v3/util/oci"
)

// valueFileResolver handles resolution of Helm value files from various sources
type valueFileResolver struct {
	appPath                  string
	repoRoot                 string
	env                      *v1alpha1.Env
	allowedValueFilesSchemas []string
	refSources               map[string]*v1alpha1.RefTarget
	gitRepoPaths             utilio.TempPaths
	ociPaths                 utilio.TempPaths
	ignoreMissingValueFiles  bool
}

// ResolveValueFiles resolves a list of raw value file paths to their resolved paths,
// handling local files, $ref Git/OCI sources, and glob expansion.
func (r *valueFileResolver) ResolveValueFiles(rawValueFiles []string) ([]pathutil.ResolvedFilePath, error) {
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

		if !resolved.IsRemote && r.checkFileExists(rawValueFile, resolved.Path) {
			continue
		}

		appendUnique(resolved.Path)
	}
	// Log only the count: resolved paths may point inside the randomized OCI extraction
	// directories, which must not leak the reposerver filesystem layout into logs.
	log.Infof("resolved %d value files", len(resolvedValueFiles))
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
func (r *valueFileResolver) resolveRawPath(rawValueFile string) (*resolveRawPathResult, error) {
	if referencedSource := getReferencedSource(rawValueFile, r.refSources); referencedSource != nil {
		// If the $-prefixed path appears to reference another source, do env substitution _after_ resolving that source.
		// The referenced source's root becomes the boundary for the glob symlink check.
		resolvedPath, root, err := resolveRefValueFile(rawValueFile, r.env, r.allowedValueFilesSchemas, referencedSource.Repo.Repo, r.gitRepoPaths, r.ociPaths)
		if err != nil {
			return nil, err
		}
		return &resolveRawPathResult{
			Path:          resolvedPath,
			IsRemote:      false,
			EffectiveRoot: root,
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
		EffectiveRoot: r.repoRoot,
	}, nil
}

// checkFileExists checks if a file exists and determines if it should be skipped. Only the raw
// entry is logged: the resolved path may sit inside a randomized OCI extraction directory.
func (r *valueFileResolver) checkFileExists(rawValueFile string, resolvedPath pathutil.ResolvedFilePath) bool {
	_, err := os.Stat(string(resolvedPath))
	if os.IsNotExist(err) {
		if r.ignoreMissingValueFiles {
			log.Debugf(" %s values file does not exist", rawValueFile)
			return true
		}
	}
	return false
}

// ResolveFileParameter resolves a single Helm --set-file path. A $ref entry is resolved
// against the referenced Git/OCI source, anything else against the application path.
func (r *valueFileResolver) ResolveFileParameter(rawPath string) (pathutil.ResolvedFilePath, error) {
	if referencedSource := getReferencedSource(rawPath, r.refSources); referencedSource != nil {
		resolvedPath, _, err := resolveRefValueFile(rawPath, r.env, r.allowedValueFilesSchemas, referencedSource.Repo.Repo, r.gitRepoPaths, r.ociPaths)
		if err != nil {
			return "", fmt.Errorf("error resolving set-file path: %w", err)
		}
		return resolvedPath, nil
	}
	resolvedPath, _, err := pathutil.ResolveValueFilePathOrUrl(r.appPath, r.repoRoot, r.env.Envsubst(rawPath), r.allowedValueFilesSchemas)
	if err != nil {
		return "", fmt.Errorf("error resolving helm value file path: %w", err)
	}
	return resolvedPath, nil
}

// resolveRefValueFile resolves a $ref value file against the referenced source's extracted
// directory (OCI) or checkout (Git) and returns the resolved path together with that root, which
// callers use as the boundary for the glob symlink check.
func resolveRefValueFile(
	rawValueFile string,
	env *v1alpha1.Env,
	allowedValueFilesSchemas []string,
	refSourceRepo string,
	gitRepoPaths utilio.TempPaths,
	ociPaths utilio.TempPaths,
) (pathutil.ResolvedFilePath, string, error) {
	// Remove the first segment (the ref variable name); pathutil re-inserts the root. Keep the
	// leading slash: pathutil joins an absolute path onto the root, and a bare "https://..."
	// remainder would otherwise be treated as a remote URL instead of a file inside the source.
	pathStrings := strings.Split(rawValueFile, "/")
	pathStrings[0] = ""
	substitutedPath := strings.Join(pathStrings, "/")

	var root string
	if v1alpha1.IsOCIURL(refSourceRepo) {
		// Paths are keyed by the normalized repo URL, matching the key used when the extracted
		// content was registered.
		normalizedKey := v1alpha1.NormalizeOCIURL(refSourceRepo)
		root = ociPaths.GetPathIfExists(normalizedKey)
		if root == "" {
			// Log only the registered keys (repo URLs), not GetPaths(), which would leak the
			// randomized temp extraction directories into logs. The keys are enough to spot a
			// URL mismatch when debugging.
			registered := make([]string, 0, len(ociPaths.GetPaths()))
			for key := range ociPaths.GetPaths() {
				registered = append(registered, key)
			}
			sort.Strings(registered)
			log.Debugf("OCI ref source %q (lookup key %q) not found among registered OCI sources %v", refSourceRepo, normalizedKey, registered)
			return "", "", fmt.Errorf("OCI ref source %q was not successfully extracted. Ensure the repository is accessible and properly configured", refSourceRepo)
		}
		// Reject inputs like "$oci" or "$oci/" that have no path segment.
		if strings.Trim(substitutedPath, "/") == "" {
			return "", "", fmt.Errorf("invalid OCI value file path %q: no file path after the ref name", rawValueFile)
		}
	} else {
		root = gitRepoPaths.GetPathIfExists(git.NormalizeGitURL(refSourceRepo))
		if root == "" {
			return "", "", fmt.Errorf("failed to find repo %q", refSourceRepo)
		}
	}

	// Resolve the path relative to the referenced source and block any attempt at traversal.
	resolvedPath, _, err := pathutil.ResolveValueFilePathOrUrl(root, root, env.Envsubst(substitutedPath), allowedValueFilesSchemas)
	if err != nil {
		return "", "", fmt.Errorf("error resolving value file path: %w", err)
	}
	return resolvedPath, root, nil
}

type gitClientGetter func(repo *v1alpha1.Repository, revision string, opts ...git.ClientOpts) (git.Client, string, error)

type ociClientGetter func(ctx context.Context, repo *v1alpha1.Repository, revision string, noRevisionCache bool) (oci.Client, string, error)

// refSourceResolver bundles the revision-resolution dependencies and cache settings used
// when resolving referenced sources. It keeps resolveReferencedSources from taking a long
// list of positional arguments and ensures the Git and OCI paths honor the same cache
// settings: gitClientOpts already encodes the caller's no-cache request for Git, and
// ociNoRevisionCache carries the equivalent flag for OCI.
type refSourceResolver struct {
	// hasMultipleSources, helmSource and refSources are the inputs being resolved: the Helm
	// source whose value files may reference other sources, and the ref sources it may name.
	hasMultipleSources          bool
	helmSource                  *v1alpha1.ApplicationSourceHelm
	refSources                  map[string]*v1alpha1.RefTarget
	newClientResolveRevision    gitClientGetter
	newOCIClientResolveRevision ociClientGetter
	gitClientOpts               git.ClientOpts
	ociNoRevisionCache          bool
	// sourceRepoURL and sourceRevision describe the primary (application) source when it is a Git
	// repository, and are empty otherwise. sourceCommitSHA is that source's already-resolved
	// commit. A referenced source that points at the same repository and revision reuses
	// sourceCommitSHA instead of resolving the revision a second time.
	sourceRepoURL   string
	sourceRevision  string
	sourceCommitSHA string
}

// resolveReferencedSources resolves the revisions for the given referenced sources. This lets us invalidate the cached
// when one or more referenced sources change.
//
// Much of this logic is duplicated in runManifestGenAsync. If making changes here, check whether runManifestGenAsync
// should be updated.
func resolveReferencedSources(ctx context.Context, resolver refSourceResolver) (map[string]string, error) {
	repoRefs := make(map[string]string)
	source, refSources := resolver.helmSource, resolver.refSources
	if !resolver.hasMultipleSources || source == nil {
		return repoRefs, nil
	}

	// referencedRevisions remembers which target revision (and which $ref) first claimed a
	// repository. A second $ref to the same repository written with the same revision is skipped;
	// one written differently is resolved and rejected only if it resolves to another commit or
	// digest, so `main` and the SHA `main` points to may be used together. repoRefs is keyed by
	// repository URL only, so without this guard both refs would share the first ref's resolved
	// revision in the cache key and the later conflict check in runManifestGenAsync could be
	// bypassed by a cache hit.
	type referencedRevision struct {
		revision string
		refVar   string
	}
	referencedRevisions := make(map[string]referencedRevision)

	refFileParams := make([]string, 0)
	for _, fileParam := range source.FileParameters {
		refFileParams = append(refFileParams, fileParam.Path)
	}
	refCandidates := append(source.ValueFiles, refFileParams...)

	for _, valueFile := range refCandidates {
		if !strings.HasPrefix(valueFile, "$") {
			continue
		}
		refVar, _, _ := strings.Cut(valueFile, "/")

		refSourceMapping, ok := refSources[refVar]
		if !ok {
			if len(refSources) == 0 {
				return nil, fmt.Errorf("source referenced %q, but no source has a 'ref' field defined", refVar)
			}
			refKeys := make([]string, 0)
			for refKey := range refSources {
				refKeys = append(refKeys, refKey)
			}
			return nil, fmt.Errorf("source referenced %q, which is not one of the available sources (%s)", refVar, strings.Join(refKeys, ", "))
		}

		// The 'chart' field is not honored for ref sources: ref resolution keys off the
		// repository URL only and does not incorporate 'chart', so accepting it (for Git or
		// OCI) would silently ignore it. Reject it instead of misleading users.
		if refSourceMapping.Chart != "" {
			return nil, errors.New("source has a 'chart' field defined, but the 'chart' field is not supported for 'ref' sources")
		}

		// Key must match runManifestGenAsync, which keys refSourceCommitSHAs the same way;
		// both feed the manifest cache key.
		normalizedRepoURL := refSourceMapping.Repo.NormalizeRepoURL()

		prev, seen := referencedRevisions[normalizedRepoURL]
		if seen && prev.revision == refSourceMapping.TargetRevision {
			continue
		}

		var referencedCommitSHA string
		var err error

		switch {
		case refSourceMapping.Repo.IsOCI():
			_, referencedCommitSHA, err = resolver.newOCIClientResolveRevision(ctx, &refSourceMapping.Repo, refSourceMapping.TargetRevision, resolver.ociNoRevisionCache)
			if err != nil {
				log.Errorf("Failed to get OCI client for repo %s: %v", refSourceMapping.Repo.Repo, err)
				return nil, fmt.Errorf("failed to get OCI client for repo %s", refSourceMapping.Repo.Repo)
			}
		case isSameRepoAndRevision(resolver.sourceRepoURL, resolver.sourceRevision, refSourceMapping):
			// The primary source already resolved this repository at this revision, so reuse its
			// commit SHA instead of paying for another ls-remote.
			referencedCommitSHA = resolver.sourceCommitSHA
		default:
			_, referencedCommitSHA, err = resolver.newClientResolveRevision(&refSourceMapping.Repo, refSourceMapping.TargetRevision, resolver.gitClientOpts)
			if err != nil {
				log.Errorf("Failed to get git client for repo %s: %v", refSourceMapping.Repo.Repo, err)
				return nil, fmt.Errorf("failed to get git client for repo %s", refSourceMapping.Repo.Repo)
			}
		}

		if seen {
			if repoRefs[normalizedRepoURL] != referencedCommitSHA {
				return nil, fmt.Errorf("cannot reference multiple revisions for the same repository (%s references %q which resolves to %q while %s references %q which resolves to %q)", refVar, refSourceMapping.TargetRevision, referencedCommitSHA, prev.refVar, prev.revision, repoRefs[normalizedRepoURL])
			}
			continue
		}
		repoRefs[normalizedRepoURL] = referencedCommitSHA
		referencedRevisions[normalizedRepoURL] = referencedRevision{revision: refSourceMapping.TargetRevision, refVar: refVar}
	}
	return repoRefs, nil
}

// isSameRepoAndRevision returns true if the referenced source points to the same git repository and revision as the
// source referencing it.
func isSameRepoAndRevision(sourceRepoURL, sourceRevision string, refTarget *v1alpha1.RefTarget) bool {
	return sourceRepoURL != "" && sourceRevision != "" &&
		refTarget.TargetRevision == sourceRevision &&
		git.NormalizeGitURL(refTarget.Repo.Repo) == git.NormalizeGitURL(sourceRepoURL)
}

type repoRef struct {
	// revision is the git revision - can be any valid revision like a branch, tag, or commit SHA.
	revision string
	// commitSHA is the actual commit to which revision refers.
	commitSHA string
	// key is the name of the key which was used to reference this repo.
	key string
}

// refSourceResolveRequest bundles the inputs shared by resolveOCIRefSource and resolveGitRefSource
// when resolving a referenced ($ref) source during manifest generation.
type refSourceResolveRequest struct {
	refSourceMapping  *v1alpha1.RefTarget
	normalizedRepoURL string
	refVar            string
	// noCache and noRevisionCache carry the caller's cache preferences. They are passed explicitly
	// rather than as a request object because ref sources are resolved both for manifest generation
	// and for GetAppDetails, which use different request types.
	noCache         bool
	noRevisionCache bool
	// appRepoURL and appRevision describe the primary (application) source. The git resolver uses
	// them to reject referencing a different revision of the same repository.
	appRepoURL  string
	appRevision string
	// commitSHA is the resolved commit of the primary (application) source. The git resolver uses it
	// to reject referencing a different revision of the same repository and for the out-of-bounds
	// symlink check.
	commitSHA string
	// sameRepoAndRevisionAsApp reports whether the referenced source points at the same Git
	// repository and revision as the primary (application) source. When it does, the git resolver
	// reuses commitSHA instead of resolving the revision again. It is always false when the primary
	// source is not a Git repository, because commitSHA is then not a Git commit.
	sameRepoAndRevisionAsApp bool
	// ociRefPaths receives the extracted OCI directory (keyed by normalizedRepoURL) for later value
	// file resolution. Only used by the OCI resolver.
	ociRefPaths utilio.TempPaths
	// resolvedRevision, when set, is the commit SHA (Git) or digest (OCI) the caller already
	// resolved TargetRevision to and recorded in the cache key. The resolvers use it as-is instead
	// of resolving the revision again, so a branch or tag moved between the two resolutions cannot
	// store the new content under the revision recorded in the key.
	resolvedRevision string
}

// resolveOCIRefSource resolves and extracts an OCI $ref source for the current request (manifest
// generation or app details). It registers the extracted directory in req.ociRefPaths (keyed by
// req.normalizedRepoURL) so value files can be resolved against it, and returns a repoRef
// describing the resolution along with a closer that removes the extracted directory. The OCI
// client's per-image lock is released inside Extract; the closer only owns the extracted copy, so
// the caller must hold it until the referenced value files have been read. Errors returned for
// external causes are redacted for surfacing to the client; the detailed cause is logged here.
//
// Unlike resolveGitRefSource, there is deliberately no "same repository, different revision" guard.
// That git check exists because a referenced git source and the primary source can share a single
// checkout directory, making two revisions of one repo contradictory. OCI ref content is instead
// extracted into a request-scoped directory of its own (ociRefPaths), so there is no shared-checkout
// conflict to detect, and the primary source's revision (a git commit SHA) is not comparable to an
// OCI digest.
func (s *Service) resolveOCIRefSource(ctx context.Context, req refSourceResolveRequest) (repoRef, goio.Closer, error) {
	refSourceMapping := req.refSourceMapping
	ociClient, err := s.newOCIClient(refSourceMapping.Repo.Repo, refSourceMapping.Repo.GetOCICreds(), refSourceMapping.Repo.Proxy, refSourceMapping.Repo.NoProxy, s.initConstants.OCIMediaTypes, s.ociClientStandardOpts()...)
	if err != nil {
		log.Errorf("Failed to create OCI client for repo %s: %v", refSourceMapping.Repo.Repo, err)
		return repoRef{}, nil, fmt.Errorf("failed to create OCI client for repo %s", refSourceMapping.Repo.Repo)
	}

	referencedDigest := req.resolvedRevision
	if referencedDigest == "" {
		referencedDigest, err = ociClient.ResolveRevision(ctx, refSourceMapping.TargetRevision, req.noCache || req.noRevisionCache)
		if err != nil {
			log.Errorf("Failed to resolve OCI revision %s: %v", refSourceMapping.TargetRevision, err)
			return repoRef{}, nil, fmt.Errorf("failed to resolve OCI revision %s", refSourceMapping.TargetRevision)
		}
	}

	// Mirror the primary OCI source: a hard refresh must also discard a cached (possibly corrupt)
	// archive of the referenced image, not just its revision lookup.
	if req.noCache {
		if err := ociClient.CleanCache(referencedDigest); err != nil {
			log.Errorf("Failed to clean OCI cache for repo %s: %v", refSourceMapping.Repo.Repo, err)
			return repoRef{}, nil, fmt.Errorf("failed to clean OCI cache for repo %s", refSourceMapping.Repo.Repo)
		}
	}

	ociPath, closer, err := ociClient.Extract(ctx, referencedDigest)
	if err != nil {
		log.Errorf("Failed to extract OCI image %s: %v", refSourceMapping.Repo.Repo, err)
		return repoRef{}, nil, fmt.Errorf("failed to extract OCI image %s", refSourceMapping.Repo.Repo)
	}

	// Check for out-of-bounds symlinks
	if !s.initConstants.AllowOutOfBoundsSymlinks {
		if err := apppathutil.CheckOutOfBoundsSymlinks(ociPath); err != nil {
			utilio.Close(closer)
			oobError := &apppathutil.OutOfBoundsSymlinkError{}
			if errors.As(err, &oobError) {
				log.WithFields(log.Fields{
					common.SecurityField: common.SecurityHigh,
					"repo":               refSourceMapping.Repo.Repo,
					"revision":           refSourceMapping.TargetRevision,
					"file":               oobError.File,
				}).Warn("oci image contains out-of-bounds symlink")
				return repoRef{}, nil, fmt.Errorf("oci image contains out-of-bounds symlinks. file: %s", oobError.File)
			}
			return repoRef{}, nil, err
		}
	}

	// Register the extracted path for value file resolution during this generation
	req.ociRefPaths.Add(req.normalizedRepoURL, ociPath)
	return repoRef{revision: refSourceMapping.TargetRevision, commitSHA: referencedDigest, key: req.refVar}, closer, nil
}

// resolveGitRefSource checks out a referenced git source at its target revision for the current
// manifest generation. It returns a repoRef describing the resolution along with a closer that
// releases the repo lock. The caller must hold the returned closer until manifest generation
// completes. Errors returned for external causes are redacted for surfacing to the client; the
// detailed cause is logged here.
func (s *Service) resolveGitRefSource(ctx context.Context, req refSourceResolveRequest) (repoRef, goio.Closer, error) {
	refSourceMapping := req.refSourceMapping
	var gitClient git.Client
	var referencedCommitSHA string
	var err error
	gitClientOpts := git.WithCache(s.cache, !req.noRevisionCache && !req.noCache)
	switch {
	case req.sameRepoAndRevisionAsApp:
		// The primary source already resolved this repository at this revision, so reuse its commit
		// SHA instead of paying for another ls-remote.
		gitClient, err = s.newClient(&refSourceMapping.Repo, gitClientOpts)
		referencedCommitSHA = req.commitSHA
	case req.resolvedRevision != "":
		gitClient, err = s.newClient(&refSourceMapping.Repo, gitClientOpts)
		referencedCommitSHA = req.resolvedRevision
	default:
		gitClient, referencedCommitSHA, err = s.newClientResolveRevision(&refSourceMapping.Repo, refSourceMapping.TargetRevision, gitClientOpts)
	}
	if err != nil {
		log.Errorf("Failed to get git client for repo %s: %v", refSourceMapping.Repo.Repo, err)
		return repoRef{}, nil, fmt.Errorf("failed to get git client for repo %s", refSourceMapping.Repo.Repo)
	}

	if git.NormalizeGitURL(req.appRepoURL) == req.normalizedRepoURL && req.commitSHA != referencedCommitSHA {
		return repoRef{}, nil, fmt.Errorf("cannot reference a different revision of the same repository (%s references %q which resolves to %q while the application references %q which resolves to %q)", req.refVar, refSourceMapping.TargetRevision, referencedCommitSHA, req.appRevision, req.commitSHA)
	}

	closer, err := s.repoLock.Lock(gitClient.Root(), referencedCommitSHA, true, func(clean bool) (goio.Closer, error) {
		// Use the referenced source's own depth instead of the primary source's depth.
		// For multi-source Applications where the primary source is a Helm/OCI artifact,
		// q.Repo.Depth is unset (0), which would otherwise force a full fetch of the
		// referenced git repository regardless of its configured depth.
		return s.checkoutRevision(ctx, gitClient, referencedCommitSHA, s.initConstants.SubmoduleEnabled, refSourceMapping.Repo.Depth, clean)
	})
	if err != nil {
		log.Errorf("failed to acquire lock for referenced source %s", req.normalizedRepoURL)
		return repoRef{}, nil, fmt.Errorf("failed to acquire lock for referenced repo %q: %w", refSourceMapping.Repo.Repo, err)
	}

	// Symlink check must happen after acquiring lock.
	if !s.initConstants.AllowOutOfBoundsSymlinks {
		if err := s.checkOutOfBoundsSymlinks(gitClient.Root(), referencedCommitSHA, req.noCache, ".git"); err != nil {
			utilio.Close(closer)
			oobError := &apppathutil.OutOfBoundsSymlinkError{}
			if errors.As(err, &oobError) {
				log.WithFields(log.Fields{
					common.SecurityField: common.SecurityHigh,
					"repo":               refSourceMapping.Repo.Repo,
					"revision":           refSourceMapping.TargetRevision,
					"file":               oobError.File,
				}).Warn("repository contains out-of-bounds symlink")
				return repoRef{}, nil, fmt.Errorf("repository contains out-of-bounds symlinks. file: %s", oobError.File)
			}
			return repoRef{}, nil, err
		}
	}

	return repoRef{revision: refSourceMapping.TargetRevision, commitSHA: referencedCommitSHA, key: req.refVar}, closer, nil
}

// resolveRefRevision resolves req's TargetRevision to a commit SHA (Git) or digest (OCI), preferring
// a revision the caller already resolved for the cache key so a branch moving mid-request cannot
// change the answer.
func (s *Service) resolveRefRevision(ctx context.Context, req refSourceResolveRequest) (string, error) {
	switch {
	case req.resolvedRevision != "":
		return req.resolvedRevision, nil
	case req.sameRepoAndRevisionAsApp:
		return req.commitSHA, nil
	case req.refSourceMapping.Repo.IsOCI():
		_, digest, err := s.newOCIClientResolveRevision(ctx, &req.refSourceMapping.Repo, req.refSourceMapping.TargetRevision, req.noCache || req.noRevisionCache)
		return digest, err
	default:
		_, sha, err := s.newClientResolveRevision(&req.refSourceMapping.Repo, req.refSourceMapping.TargetRevision, git.WithCache(s.cache, !req.noRevisionCache && !req.noCache))
		return sha, err
	}
}

// resolveRefSource resolves one $ref source for the current request (manifest generation or app
// details) and records it in repoRefs, keyed by req.normalizedRepoURL. A repository already
// resolved for an earlier $ref is reused; referencing it with a revision that resolves to a
// different commit or digest is rejected. The returned closer releases the Git checkout or the
// extracted OCI artifact and must be held until the referenced value files have been read.
func (s *Service) resolveRefSource(ctx context.Context, req refSourceResolveRequest, repoRefs map[string]repoRef) (goio.Closer, error) {
	if prev, ok := repoRefs[req.normalizedRepoURL]; ok {
		if prev.revision == req.refSourceMapping.TargetRevision {
			return utilio.NopCloser, nil
		}
		resolved, err := s.resolveRefRevision(ctx, req)
		if err != nil {
			log.Errorf("Failed to resolve revision %s for repo %s: %v", req.refSourceMapping.TargetRevision, req.refSourceMapping.Repo.Repo, err)
			return nil, fmt.Errorf("failed to resolve revision %s for repo %s", req.refSourceMapping.TargetRevision, req.refSourceMapping.Repo.Repo)
		}
		if prev.commitSHA != resolved {
			return nil, fmt.Errorf("cannot reference multiple revisions for the same repository (%s references %q which resolves to %q while %s references %q which resolves to %q)", req.refVar, req.refSourceMapping.TargetRevision, resolved, prev.key, prev.revision, prev.commitSHA)
		}
		return utilio.NopCloser, nil
	}
	var ref repoRef
	var closer goio.Closer
	var err error
	if req.refSourceMapping.Repo.IsOCI() {
		ref, closer, err = s.resolveOCIRefSource(ctx, req)
	} else {
		ref, closer, err = s.resolveGitRefSource(ctx, req)
	}
	if err != nil {
		return nil, err
	}
	repoRefs[req.normalizedRepoURL] = ref
	return closer, nil
}

// redactPaths removes temp repo paths, since those paths are randomized (and therefore not helpful for the user) and
// sensitive (so not suitable for logging). It also replaces the path of the randomly-named values file which is used
// to hold the `spec.source.helm.values` or `valuesObject` contents. All supplied path sets (e.g. Git checkouts and
// extracted OCI artifacts) are redacted.
func redactPaths(s string, extraValuesPath pathutil.ResolvedFilePath, pathSets ...utilio.TempPaths) string {
	for _, paths := range pathSets {
		if paths == nil {
			continue
		}
		for _, p := range paths.GetPaths() {
			s = strings.ReplaceAll(s, p, ".")
		}
	}
	if extraValuesPath != "" {
		// Replace with a placeholder so that the user knows what this values file was for.
		s = strings.ReplaceAll(s, string(extraValuesPath), "<temp file with values from source.helm.values/valuesObject>")
	}
	return s
}

// redactedError overrides an error's message with a path-redacted version while preserving
// the wrapped error via Unwrap, so errors.Is/errors.As continue to see the original error
// identity (e.g. context.Canceled/DeadlineExceeded, *exec.ExitError, gRPC status errors).
type redactedError struct {
	msg string
	err error
}

func (e *redactedError) Error() string { return e.msg }

func (e *redactedError) Unwrap() error { return e.err }

// redactPathsInError redacts sensitive temp paths from an error message. Helm template/build
// failures embed the rendered command (including `--values <temp path>`), which would otherwise
// leak randomized reposerver filesystem paths for Git checkouts and extracted OCI artifacts.
// The original error is preserved for errors.Is/errors.As; only the surfaced message is redacted.
func redactPathsInError(err error, extraValuesPath pathutil.ResolvedFilePath, pathSets ...utilio.TempPaths) error {
	if err == nil {
		return nil
	}
	return &redactedError{
		msg: redactPaths(err.Error(), extraValuesPath, pathSets...),
		err: err,
	}
}

func getReferencedSources(rawValueFiles []string, refSources map[string]*v1alpha1.RefTarget) map[string]*v1alpha1.RefTarget {
	result := make(map[string]*v1alpha1.RefTarget, len(refSources))
	for _, rawValueFile := range rawValueFiles {
		refName := getReferencedSourceName(rawValueFile)
		if refName != "" {
			refSource := refSources[refName]
			if refSource != nil {
				result[refName] = refSource
			}
		}
	}
	return result
}

func getReferencedSourceName(rawValueFile string) string {
	if !strings.HasPrefix(rawValueFile, "$") {
		return ""
	}
	return strings.Split(rawValueFile, "/")[0]
}

func getReferencedSource(rawValueFile string, refSources map[string]*v1alpha1.RefTarget) *v1alpha1.RefTarget {
	refName := getReferencedSourceName(rawValueFile)
	if refName == "" {
		return nil
	}
	return refSources[refName]
}

// GlobNoMatchError is returned when a glob pattern in valueFiles matches no files.
// It is a runtime condition (the files may be added later), not a spec error.
type GlobNoMatchError struct {
	Pattern string
}

func (e *GlobNoMatchError) Error() string {
	return fmt.Sprintf("values file glob %q matched no files", e.Pattern)
}

// isGlobPath reports whether path contains any glob metacharacters
// supported by doublestar: *, ?, or [. The ** pattern is covered by *.
func isGlobPath(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

// verifyGlobMatchesWithinRoot resolves symlinks for each glob match and verifies
// that the resolved target is within effectiveRoot. It protects against symlinks
// inside the repository that point to targets outside it.
//
// doublestar.FilepathGlob uses os.Lstat, so it returns the path of the symlink
// itself (which lives inside the repo) rather than the symlink target. If the
// target is outside the repo, Helm would still follow the link and read the
// external file. This function catches that case before the paths reach Helm.
//
// Both effectiveRoot and each match are canonicalized via filepath.EvalSymlinks
// so the prefix comparison is correct on systems where the working directory is
// itself under a symlink chain (e.g. /var -> /private/var on macOS).
func verifyGlobMatchesWithinRoot(matches []string, effectiveRoot string) error {
	absRoot, err := filepath.Abs(effectiveRoot)
	if err != nil {
		return fmt.Errorf("error resolving repo root: %w", err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return fmt.Errorf("error resolving symlinks in repo root: %w", err)
	}
	requiredRootPath := canonicalRoot
	if !strings.HasSuffix(requiredRootPath, string(os.PathSeparator)) {
		requiredRootPath += string(os.PathSeparator)
	}
	for _, match := range matches {
		realMatch, err := filepath.EvalSymlinks(match)
		if err != nil {
			return fmt.Errorf("error resolving symlink for glob match %q: %w", match, err)
		}
		// Allow the match to resolve exactly to the root (realMatch+sep == requiredRootPath)
		// or to any path beneath it (HasPrefix).
		if realMatch+string(os.PathSeparator) != requiredRootPath && !strings.HasPrefix(realMatch, requiredRootPath) {
			return fmt.Errorf("glob match %q resolved to outside repository root", match)
		}
	}
	return nil
}
