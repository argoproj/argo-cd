package repository

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/cache"
	"github.com/argoproj/argo-cd/v3/reposerver/metrics"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/helm"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/oci"
	"github.com/argoproj/argo-cd/v3/util/versions"
	"github.com/argoproj/pkg/v2/sync"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CacheFn defines the signature for a cache checking function.
// Parameters:
//   - cacheKey: the cache key to check
//   - refSourceCommitSHAs: map of reference source names to their commit SHAs
//   - firstInvocation: true if this is the first cache check, false for double-check locking
//
// Returns:
//   - bool: true if cache hit and operation should skip, false if cache miss
//   - error: any error that occurred during cache checking
type CacheFn func(cacheKey string, refSourceCommitSHAs cache.ResolvedRevisions, firstInvocation bool) (bool, error)

// SourceClient is a unified interface for interacting with different source types (Git, Helm, OCI).
// It abstracts the common operations needed to retrieve and process application sources.
type SourceClient interface {
	// ResolveRevision resolves a revision reference (branch, tag, version, etc.) to a concrete revision identifier
	ResolveRevision(ctx context.Context, revision string, noCache bool) (string, error)

	// CleanCache removes cached artifacts for the specified revision
	CleanCache(revision string) error

	// Extract retrieves the source content for the specified revision and returns:
	// - root path where the content was extracted
	// - a closer to clean up resources
	// - any error that occurred
	Extract(ctx context.Context, revision string) (rootPath string, closer io.Closer, err error)

	// GetDigest returns the canonical digest/SHA for the content
	// Must be called after Extract() for Git sources (to ensure checkout has occurred)
	// For Git: returns the actual commit SHA after checkout
	// For OCI/Helm: returns the revision parameter as-is (digest or version)
	// hasMultipleSources: for Git only, if true, returns the revision parameter instead of calling CommitSHA()
	GetDigest(revision string, hasMultipleSources bool) (string, error)

	// VerifySignature verifies the signature of the given revision
	// For Git: verifies GPG signature of commit/tag (handles annotated tag logic internally)
	// For OCI/Helm: returns empty string (signature verification not applicable)
	// Parameters:
	//   - resolvedRevision: the concrete revision (commit SHA, digest, version)
	//   - unresolvedRevision: the original revision reference (branch, tag name, etc.)
	VerifySignature(resolvedRevision string, unresolvedRevision string) (string, error)
}

// ociSourceClient adapts an OCI client to the SourceClient interface
type ociSourceClient struct {
	client oci.Client
	repo   *v1alpha1.Repository
}

func NewOCISourceClient(repo *v1alpha1.Repository, opts ...oci.ClientOpts) (SourceClient, error) {
	client, err := oci.NewClient(
		repo.Repo,
		repo.GetOCICreds(),
		repo.Proxy,
		repo.NoProxy,
		opts...,
	)

	if err != nil {
		return nil, err
	}

	return &ociSourceClient{
		client: client,
		repo:   repo,
	}, nil
}

func (c *ociSourceClient) ResolveRevision(ctx context.Context, revision string, noCache bool) (string, error) {
	digest, err := c.client.ResolveRevision(ctx, revision, noCache)
	if err != nil {
		return "", fmt.Errorf("failed to resolve revision %q: %w", revision, err)
	}
	return digest, nil
}

func (c *ociSourceClient) CleanCache(revision string) error {
	return c.client.CleanCache(revision)
}

func (c *ociSourceClient) Extract(ctx context.Context, revision string) (string, io.Closer, error) {
	return c.client.Extract(ctx, revision)
}

func (c *ociSourceClient) GetDigest(revision string, hasMultipleSources bool) (string, error) {
	// For OCI, the digest IS the revision (already resolved from ResolveRevision)
	return revision, nil
}

func (c *ociSourceClient) VerifySignature(resolvedRevision string, unresolvedRevision string) (string, error) {
	// OCI doesn't support signature verification through this interface
	return "", nil
}

// helmSourceClient adapts a Helm client to the SourceClient interface
type helmSourceClient struct {
	client helm.Client
	repo   *v1alpha1.Repository
	chart  string
}

func NewHelmSourceClient(repo *v1alpha1.Repository, chart string, opts ...helm.ClientOpts) SourceClient {
	return &helmSourceClient{
		client: helm.NewClientWithLock(
			repo.Repo,
			repo.GetHelmCreds(),
			sync.NewKeyLock(),
			repo.EnableOCI || helm.IsHelmOciRepo(repo.Repo),
			repo.Proxy,
			repo.NoProxy,
			opts...,
		),
		chart: chart,
		repo:  repo,
	}
}

func (c *helmSourceClient) ResolveRevision(ctx context.Context, revision string, noCache bool) (string, error) {
	enableOCI := c.repo.EnableOCI || helm.IsHelmOciRepo(c.repo.Repo)

	// If it's already a version, return it
	if versions.IsVersion(revision) {
		return revision, nil
	}

	var tags []string
	if enableOCI {
		var err error
		tags, err = c.client.GetTags(c.chart, noCache)
		if err != nil {
			return "", fmt.Errorf("unable to get tags: %w", err)
		}
	} else {
		index, err := c.client.GetIndex(noCache)
		if err != nil {
			return "", err
		}
		entries, err := index.GetEntries(c.chart)
		if err != nil {
			return "", err
		}
		tags = entries.Tags()
	}

	maxV, err := versions.MaxVersion(revision, tags)
	if err != nil {
		return "", fmt.Errorf("invalid revision: %w", err)
	}

	return maxV, nil
}

func (c *helmSourceClient) CleanCache(revision string) error {
	return c.client.CleanChartCache(c.chart, revision)
}

func (c *helmSourceClient) Extract(ctx context.Context, revision string) (string, io.Closer, error) {
	return c.client.ExtractChart(c.chart, revision)
}

func (c *helmSourceClient) GetDigest(revision string, hasMultipleSources bool) (string, error) {
	// For Helm, the version IS the revision (already resolved from ResolveRevision)
	return revision, nil
}

func (c *helmSourceClient) VerifySignature(resolvedRevision string, unresolvedRevision string) (string, error) {
	// Helm doesn't support signature verification through this interface
	return "", nil
}

// gitSourceClient adapts a Git client to the SourceClient interface
type gitSourceClient struct {
	client                         git.Client
	depth                          int64
	repo                           *v1alpha1.Repository
	metrics                        *metrics.MetricsServer
	repositoryLock                 *repositoryLock
	directoryPermissionInitializer func(rootPath string) io.Closer
	cacheFn                        CacheFn
	submoduleEnabled               bool
	cache                          *cache.Cache
	loadRefFromCache               bool
	gitRepoPaths                   utilio.TempPaths
	credsStore                     git.CredsStore
}

// GitSourceClientOpt is a functional option for configuring gitSourceClient
type GitSourceClientOpt func(*gitSourceClient)

// WithCacheFn configures the cache checking function for the git source client.
// The cache function will be called to check if cached manifests exist before
// performing expensive git operations.
func WithCacheFn(fn CacheFn) GitSourceClientOpt {
	return func(c *gitSourceClient) {
		c.cacheFn = fn
	}
}

func WithSubmoduleEnabled(submoduleEnabled bool) GitSourceClientOpt {
	return func(c *gitSourceClient) {
		c.submoduleEnabled = submoduleEnabled
	}
}

// WithCache sets git revisions cacher as well as specifies if client should tries to use cached resolved revision
func WithCache(cache *cache.Cache, loadRefFromCache bool) GitSourceClientOpt {
	return func(c *gitSourceClient) {
		c.cache = cache
		c.loadRefFromCache = loadRefFromCache
	}
}

func NewGitSourceClient(repo *v1alpha1.Repository, depth int64, gitRepoPaths utilio.TempPaths, metrics *metrics.MetricsServer, credStore git.CredsStore, repoLock *repositoryLock, initializer func(rootPath string) io.Closer, sourceOpts ...GitSourceClientOpt) (SourceClient, error) {
	root, err := gitRepoPaths.GetPath(git.NormalizeGitURL(repo.Repo))
	if err != nil {
		return nil, err
	}

	gsc := &gitSourceClient{
		repo:                           repo,
		depth:                          depth,
		metrics:                        metrics,
		repositoryLock:                 repoLock,
		directoryPermissionInitializer: initializer,
		gitRepoPaths:                   gitRepoPaths,
		credsStore:                     credStore,
	}

	for _, opt := range sourceOpts {
		opt(gsc)
	}

	client, err := newGitClient(repo, root, credStore, gsc)
	if err != nil {
		return nil, err
	}

	gsc.client = client

	return gsc, nil
}

func (c *gitSourceClient) ResolveRevision(ctx context.Context, revision string, noCache bool) (string, error) {
	commitSHA, err := c.resolveRevision(c.client, revision)
	if err != nil {
		return "", err
	}

	if c.cacheFn != nil && !noCache {
		// TODO: add params
		repoRefs, err := c.resolveReferencedSources(true, nil, nil)
		if err != nil {
			return "", err
		}

		if ok, err := c.cacheFn(commitSHA, repoRefs, true); ok {
			return "", err
		}
	}

	return commitSHA, nil
}

func (c *gitSourceClient) resolveRevision(client git.Client, revision string) (string, error) {
	commitSHA, err := client.LsRemote(revision)
	if err != nil {
		if c.metrics != nil {
			c.metrics.IncGitLsRemoteFail(c.client.Root(), revision)
		}
		return "", err
	}

	return commitSHA, nil
}

func (c *gitSourceClient) resolveReferencedSources(hasMultipleSources bool, source *v1alpha1.ApplicationSourceHelm, refSources map[string]*v1alpha1.RefTarget) (map[string]string, error) {
	repoRefs := make(map[string]string)
	if !hasMultipleSources || source == nil {
		return repoRefs, nil
	}

	refFileParams := make([]string, 0)
	for _, fileParam := range source.FileParameters {
		refFileParams = append(refFileParams, fileParam.Path)
	}
	refCandidates := append(source.ValueFiles, refFileParams...)

	for _, valueFile := range refCandidates {
		if !strings.HasPrefix(valueFile, "$") {
			continue
		}
		refVar := strings.Split(valueFile, "/")[0]

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
		if refSourceMapping.Chart != "" {
			return nil, errors.New("source has a 'chart' field defined, but Helm charts are not yet not supported for 'ref' sources")
		}
		normalizedRepoURL := git.NormalizeGitURL(refSourceMapping.Repo.Repo)
		_, ok = repoRefs[normalizedRepoURL]
		if !ok {
			repo := refSourceMapping.Repo
			root, err := c.gitRepoPaths.GetPath(git.NormalizeGitURL(repo.Repo))
			if err != nil {
				return nil, err
			}

			client, err := newGitClient(&repo, root, c.credsStore, c)
			if err != nil {
				return nil, err
			}
			referencedCommitSHA, err := c.resolveRevision(client, refSourceMapping.TargetRevision)
			if err != nil {
				log.Errorf("Failed to get git client for repo %s: %v", refSourceMapping.Repo.Repo, err)
				return nil, fmt.Errorf("failed to get git client for repo %s", refSourceMapping.Repo.Repo)
			}
			repoRefs[normalizedRepoURL] = referencedCommitSHA
		}
	}
	return repoRefs, nil
}

func (c *gitSourceClient) CleanCache(_ string) error {
	// Git doesn't support cache cleaning in the same way as OCI/Helm
	return nil
}

func (c *gitSourceClient) Extract(ctx context.Context, revision string) (string, io.Closer, error) {
	closer, err := c.repositoryLock.Lock(c.client.Root(), revision, true, func() (io.Closer, error) {
		closer := c.directoryPermissionInitializer(c.client.Root())
		err := c.client.Init()
		if err != nil {
			return closer, status.Errorf(codes.Internal, "Failed to initialize git repo: %v", err)
		}

		revisionPresent := c.client.IsRevisionPresent(revision)

		log.WithFields(map[string]any{
			"skipFetch": revisionPresent,
		}).Debugf("Checking out revision %v", revision)

		// Fetching can be skipped if the revision is already present locally.
		if !revisionPresent {
			if c.depth > 0 {
				err = c.client.Fetch(revision, c.depth)
			} else {
				// Fetching with no revision first. Fetching with an explicit version can cause repo bloat. https://github.com/argoproj/argo-cd/issues/8845
				err = c.client.Fetch("", c.depth)
			}

			if err != nil {
				return closer, status.Errorf(codes.Internal, "Failed to fetch default: %v", err)
			}
		}

		_, err = c.client.Checkout(revision, c.submoduleEnabled)
		if err != nil {
			// When fetching with no revision, only refs/heads/* and refs/remotes/origin/* are fetched. If checkout fails
			// for the given revision, try explicitly fetching it.
			log.Infof("Failed to checkout revision %s: %v", revision, err)
			log.Infof("Fallback to fetching specific revision %s. ref might not have been in the default refspec fetched.", revision)
			err = c.client.Fetch(revision, c.depth)
			if err != nil {
				return closer, status.Errorf(codes.Internal, "Failed to checkout revision %s: %v", revision, err)
			}

			_, err = c.client.Checkout("FETCH_HEAD", c.submoduleEnabled)
			if err != nil {
				return closer, status.Errorf(codes.Internal, "Failed to checkout FETCH_HEAD: %v", err)
			}
		}

		return closer, err
	})

	if err != nil {
		return "", nil, err
	}

	return c.client.Root(), closer, nil
}

func (c *gitSourceClient) GetDigest(revision string, hasMultipleSources bool) (string, error) {
	// For Git with multiple sources, use the revision as-is
	if hasMultipleSources {
		return revision, nil
	}
	// For single source, get the actual commit SHA after checkout
	return c.client.CommitSHA()
}

func (c *gitSourceClient) VerifySignature(resolvedRevision string, unresolvedRevision string) (string, error) {
	// Determine which revision to verify:
	// When the revision is an annotated tag, we need to pass the unresolved revision (i.e. the tag name)
	// For everything else, we work with the SHA that the target revision is pointing to (i.e. the resolved revision)
	revisionToVerify := resolvedRevision
	if c.client.IsAnnotatedTag(resolvedRevision) {
		revisionToVerify = unresolvedRevision
	}

	return c.client.VerifyCommitSignature(revisionToVerify)
}

func newGitClient(repo *v1alpha1.Repository, root string, credStore git.CredsStore, gsc *gitSourceClient) (git.Client, error) {
	return git.NewClientExt(
		repo.Repo,
		root,
		repo.GetGitCreds(credStore),
		repo.IsInsecure(),
		repo.EnableLFS,
		repo.Proxy,
		repo.NoProxy,
		git.WithCache(gsc.cache, gsc.loadRefFromCache),
		git.WithEventHandlers(metrics.NewGitClientEventHandlers(gsc.metrics)),
	)
}
