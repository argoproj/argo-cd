package repository

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/argoproj/argo-cd/v3/common"
	apppathutil "github.com/argoproj/argo-cd/v3/util/app/path"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/reposerver/cache"
	"github.com/argoproj/argo-cd/v3/reposerver/metrics"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/gpg"
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

// BaseSourceClient is the base interface for interacting with different source types (Git, Helm, OCI).
// It abstracts the common operations needed to retrieve and process application sources.
type BaseSourceClient interface {
	// ResolveRevision resolves a revision reference (branch, tag, version, etc.) to a concrete revision identifier
	ResolveRevision(ctx context.Context, revision string, noCache bool) (string, string, error)

	// Extract retrieves the source content for the specified revision and returns:
	// - root path where the content was extracted
	// - a closer to clean up resources
	// - any error that occurred
	Extract(ctx context.Context, revision string, noCache bool, allowOutOfBoundsSymlinks bool) (rootPath string, closer io.Closer, err error)

	// VerifySignature verifies the signature of the given revision
	// For Git: verifies GPG signature of commit/tag (handles annotated tag logic internally)
	// For OCI/Helm: returns empty string (signature verification not applicable)
	// Parameters:
	//   - resolvedRevision: the concrete revision (commit SHA, digest, version)
	//   - unresolvedRevision: the original revision reference (branch, tag name, etc.)
	VerifySignature(resolvedRevision string, unresolvedRevision string) (string, error)

	// ListRefs lists the available references (branches and/or tags) for the source
	// For Git: returns both branches and tags
	// For OCI/Helm: returns only tags
	ListRefs(ctx context.Context, noCache bool) (*apiclient.Refs, error)

	// ResolveReferencedSources resolves the revisions for referenced sources in multi-source applications.
	// This is used to invalidate the cache when one or more referenced sources change.
	// For Git: resolves referenced Git repositories and returns their commit SHAs
	// For OCI/Helm: returns empty map (not yet supported for ref sources)
	// Parameters:
	//   - hasMultipleSources: whether the application has multiple sources
	//   - source: the Helm source configuration (may be nil)
	//   - refSources: map of ref variable names to their target repositories and revisions
	// Returns:
	//   - map of normalized repository URLs to their resolved commit SHAs
	//   - error if resolution fails
	ResolveReferencedSources(hasMultipleSources bool, source *v1alpha1.ApplicationSourceHelm, refSources map[string]*v1alpha1.RefTarget) (map[string]string, error)
}

// SourceClient is a generic interface that extends BaseSourceClient with type-safe metadata retrieval.
// The type parameter T represents the metadata type specific to each source type:
//   - Git: *v1alpha1.RevisionMetadata
//   - OCI: *v1alpha1.OCIMetadata
//   - Helm: *v1alpha1.ChartDetails
type SourceClient[T any] interface {
	BaseSourceClient

	// GetRevisionMetadata retrieves metadata for a specific revision
	// The concrete return type T depends on the implementation.
	GetRevisionMetadata(ctx context.Context, revision string, checkSignature bool) (T, error)
}

// ociSourceClient adapts an OCI client to the SourceClient interface
type ociSourceClient struct {
	client oci.Client
	repo   *v1alpha1.Repository
}

func NewOCISourceClient(repo *v1alpha1.Repository, opts ...oci.ClientOpts) (SourceClient[*v1alpha1.OCIMetadata], error) {
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

func (c *ociSourceClient) ResolveRevision(ctx context.Context, revision string, noCache bool) (string, string, error) {
	digest, err := c.client.ResolveRevision(ctx, revision, noCache)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve revision %q: %w", revision, err)
	}
	return digest, digest, nil
}

func (c *ociSourceClient) Extract(ctx context.Context, revision string, noCache bool, allowOutOfBoundsSymlinks bool) (string, io.Closer, error) {
	if noCache {
		err := c.client.CleanCache(revision)
		if err != nil {
			return "", nil, err
		}
	}

	rootPath, closer, err := c.client.Extract(ctx, revision)
	if err == nil && !allowOutOfBoundsSymlinks {
		if err = apppathutil.CheckOutOfBoundsSymlinks(rootPath); err != nil {
			oobError := &apppathutil.OutOfBoundsSymlinkError{}
			var serr *apppathutil.OutOfBoundsSymlinkError
			if errors.As(err, &serr) {
				oobError = serr
				log.WithFields(log.Fields{
					common.SecurityField: common.SecurityHigh,
					"repo":               c.repo.Repo,
					"digest":             revision,
					"file":               oobError.File,
				}).Warn("oci image contains out-of-bounds symlink")
				return rootPath, closer, fmt.Errorf("oci image contains out-of-bounds symlinks. file: %s", oobError.File)
			}
		}
	}

	return rootPath, closer, err
}

func (c *ociSourceClient) VerifySignature(resolvedRevision string, unresolvedRevision string) (string, error) {
	// OCI doesn't support signature verification through this interface
	return "", nil
}

func (c *ociSourceClient) GetRevisionMetadata(ctx context.Context, revision string, checkSignature bool) (*v1alpha1.OCIMetadata, error) {
	metadata, err := c.client.DigestMetadata(ctx, revision)
	if err != nil {
		return nil, fmt.Errorf("failed to extract digest metadata for revision %q: %w", revision, err)
	}

	a := metadata.Annotations

	return &v1alpha1.OCIMetadata{
		CreatedAt: a["org.opencontainers.image.created"],
		Authors:   a["org.opencontainers.image.authors"],
		// TODO: add this field at a later stage
		// ImageURL:    a["org.opencontainers.image.url"],
		DocsURL:     a["org.opencontainers.image.documentation"],
		SourceURL:   a["org.opencontainers.image.source"],
		Version:     a["org.opencontainers.image.version"],
		Description: a["org.opencontainers.image.description"],
	}, nil
}

func (c *ociSourceClient) ListRefs(ctx context.Context, noCache bool) (*apiclient.Refs, error) {
	tags, err := c.client.GetTags(ctx, noCache)
	if err != nil {
		return nil, err
	}

	return &apiclient.Refs{
		Tags: tags,
	}, nil
}

func (c *ociSourceClient) ResolveReferencedSources(hasMultipleSources bool, source *v1alpha1.ApplicationSourceHelm, refSources map[string]*v1alpha1.RefTarget) (map[string]string, error) {
	return make(map[string]string), nil
}

// helmSourceClient adapts a Helm client to the SourceClient interface
type helmSourceClient struct {
	client helm.Client
	repo   *v1alpha1.Repository
	cache  *cache.Cache
	chart  string
}

func NewHelmSourceClient(repo *v1alpha1.Repository, chart string, opts ...helm.ClientOpts) SourceClient[*v1alpha1.ChartDetails] {
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

func (c *helmSourceClient) ResolveRevision(ctx context.Context, revision string, noCache bool) (string, string, error) {
	enableOCI := c.repo.EnableOCI || helm.IsHelmOciRepo(c.repo.Repo)

	// If it's already a version, return it
	if versions.IsVersion(revision) {
		return revision, revision, nil
	}

	var tags []string
	if enableOCI {
		var err error
		tags, err = c.client.GetTags(c.chart, noCache)
		if err != nil {
			return "", "", fmt.Errorf("unable to get tags: %w", err)
		}
	} else {
		index, err := c.client.GetIndex(noCache)
		if err != nil {
			return "", "", err
		}
		entries, err := index.GetEntries(c.chart)
		if err != nil {
			return "", "", err
		}
		tags = entries.Tags()
	}

	maxV, err := versions.MaxVersion(revision, tags)
	if err != nil {
		return "", "", fmt.Errorf("invalid revision: %w", err)
	}

	return maxV, maxV, nil
}

func (c *helmSourceClient) Extract(ctx context.Context, revision string, noCache bool, allowOutOfBoundsSymlinks bool) (string, io.Closer, error) {
	if noCache {
		err := c.client.CleanChartCache(c.chart, revision)
		if err != nil {
			return "", nil, err
		}
	}
	rootPath, closer, err := c.client.ExtractChart(c.chart, revision)
	if err == nil && !allowOutOfBoundsSymlinks {
		if err = apppathutil.CheckOutOfBoundsSymlinks(rootPath); err != nil {
			oobError := &apppathutil.OutOfBoundsSymlinkError{}
			var serr *apppathutil.OutOfBoundsSymlinkError
			if errors.As(err, &serr) {
				oobError = serr
				log.WithFields(log.Fields{
					common.SecurityField: common.SecurityHigh,
					"chart":              c.chart,
					"revision":           revision,
					"file":               oobError.File,
				}).Warn("chart contains out-of-bounds symlink")
			}
		}
	}
	return rootPath, closer, err
}

func (c *helmSourceClient) VerifySignature(resolvedRevision string, unresolvedRevision string) (string, error) {
	// Helm doesn't support signature verification through this interface
	return "", nil
}

func (c *helmSourceClient) GetRevisionMetadata(ctx context.Context, revision string, checkSignature bool) (*v1alpha1.ChartDetails, error) {
	repo := c.repo.Repo
	details, err := c.cache.GetRevisionChartDetails(repo, c.chart, revision)
	if err == nil {
		log.Infof("revision chart details cache hit: %s/%s/%s", repo, c.chart, revision)
		return details, nil
	}
	if errors.Is(err, cache.ErrCacheMiss) {
		log.Infof("revision metadata cache miss: %s/%s/%s", repo, c.chart, revision)
	} else {
		log.Warnf("revision metadata cache error %s/%s/%s: %v", repo, c.chart, revision, err)
	}

	_, resolvedRevision, err := c.ResolveRevision(ctx, revision, checkSignature)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve revision: %w", err)
	}

	chartPath, closer, err := c.client.ExtractChart(c.chart, resolvedRevision)
	if err != nil {
		return nil, fmt.Errorf("error extracting chart: %w", err)
	}
	defer utilio.Close(closer)

	helmCmd, err := helm.NewCmdWithVersion(chartPath, c.repo.EnableOCI, c.repo.Proxy, c.repo.NoProxy)
	if err != nil {
		return nil, fmt.Errorf("error creating helm cmd: %w", err)
	}
	defer helmCmd.Close()

	helmDetails, err := helmCmd.InspectChart()
	if err != nil {
		return nil, fmt.Errorf("error inspecting chart: %w", err)
	}

	details, err = getChartDetails(helmDetails)
	if err != nil {
		return nil, fmt.Errorf("error getting chart details: %w", err)
	}

	_ = c.cache.SetRevisionChartDetails(repo, c.chart, revision, details)
	return details, nil
}

func (c *helmSourceClient) ListRefs(ctx context.Context, noCache bool) (*apiclient.Refs, error) {
	enableOCI := c.repo.EnableOCI || helm.IsHelmOciRepo(c.repo.Repo)

	var tags []string
	if enableOCI {
		var err error
		tags, err = c.client.GetTags(c.chart, noCache)
		if err != nil {
			return nil, fmt.Errorf("unable to get tags: %w", err)
		}
	} else {
		index, err := c.client.GetIndex(noCache)
		if err != nil {
			return nil, err
		}
		entries, err := index.GetEntries(c.chart)
		if err != nil {
			return nil, err
		}
		tags = entries.Tags()
	}

	return &apiclient.Refs{
		Tags: tags,
	}, nil
}

func (c *helmSourceClient) ResolveReferencedSources(hasMultipleSources bool, source *v1alpha1.ApplicationSourceHelm, refSources map[string]*v1alpha1.RefTarget) (map[string]string, error) {
	return make(map[string]string), nil
}

// gitSourceClient adapts a Git client to the SourceClient interface
type gitSourceClient struct {
	client                         git.Client
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
	hasMultipleSources             bool
	helmSource                     *v1alpha1.ApplicationSourceHelm
	refSources                     map[string]*v1alpha1.RefTarget
	repoRefs                       map[string]string
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

func WithHelmSource(source *v1alpha1.ApplicationSourceHelm) GitSourceClientOpt {
	return func(c *gitSourceClient) {
		c.helmSource = source
	}
}

func WithRefSources(refSources map[string]*v1alpha1.RefTarget) GitSourceClientOpt {
	return func(c *gitSourceClient) {
		c.refSources = refSources
	}
}

func NewGitSourceClient(repo *v1alpha1.Repository, gitRepoPaths utilio.TempPaths, metrics *metrics.MetricsServer, credStore git.CredsStore, repoLock *repositoryLock, initializer func(rootPath string) io.Closer, hasMultipleSources bool, sourceOpts ...GitSourceClientOpt) (SourceClient[*v1alpha1.RevisionMetadata], error) {
	root, err := gitRepoPaths.GetPath(git.NormalizeGitURL(repo.Repo))
	if err != nil {
		return nil, err
	}

	gsc := &gitSourceClient{
		repo:                           repo,
		metrics:                        metrics,
		repositoryLock:                 repoLock,
		directoryPermissionInitializer: initializer,
		gitRepoPaths:                   gitRepoPaths,
		credsStore:                     credStore,
		hasMultipleSources:             hasMultipleSources,
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

func (c *gitSourceClient) ResolveRevision(ctx context.Context, revision string, noCache bool) (string, string, error) {
	commitSHA, err := c.resolveRevision(c.client, revision)
	if err != nil {
		return "", "", err
	}

	if c.cacheFn != nil && !noCache {
		repoRefs, err := c.ResolveReferencedSources(c.hasMultipleSources, c.helmSource, c.refSources)
		if err != nil {
			return "", "", err
		}

		if ok, err := c.cacheFn(commitSHA, repoRefs, true); ok {
			return "", "", err
		}

		// TODO: This is ugly, but can't think of a better way atm
		c.repoRefs = repoRefs
	}

	// For Git with multiple sources, use the revision as-is
	if c.hasMultipleSources {
		return revision, revision, nil
	}
	// For single source, get the actual commit SHA after checkout
	sha, err := c.client.CommitSHA()
	if err != nil {
		return "", "", err
	}

	return sha, revision, nil
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

func (c *gitSourceClient) ResolveReferencedSources(hasMultipleSources bool, source *v1alpha1.ApplicationSourceHelm, refSources map[string]*v1alpha1.RefTarget) (map[string]string, error) {
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

func (c *gitSourceClient) Extract(ctx context.Context, revision string, noCache bool, allowOutOfBoundsSymlinks bool) (string, io.Closer, error) {
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
			if c.repo.Depth > 0 {
				err = c.client.Fetch(revision, c.repo.Depth)
			} else {
				// Fetching with no revision first. Fetching with an explicit version can cause repo bloat. https://github.com/argoproj/argo-cd/issues/8845
				err = c.client.Fetch("", c.repo.Depth)
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
			err = c.client.Fetch(revision, c.repo.Depth)
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

	if !allowOutOfBoundsSymlinks {
		if err = apppathutil.CheckOutOfBoundsSymlinks(c.client.Root()); err != nil {
			oobError := &apppathutil.OutOfBoundsSymlinkError{}
			var serr *apppathutil.OutOfBoundsSymlinkError
			if errors.As(err, &serr) {
				oobError = serr
				log.WithFields(log.Fields{
					common.SecurityField: common.SecurityHigh,
					"repo":               c.repo.Repo,
					"revision":           revision,
					"file":               oobError.File,
				}).Warn("repository contains out-of-bounds symlink")
				return "", nil, fmt.Errorf("repository contains out-of-bounds symlinks. file: %s", oobError.File)
			}
		}
	}

	if c.cacheFn != nil && !noCache {
		if ok, err := c.cacheFn(revision, c.repoRefs, false); ok {
			return "", closer, err
		}
	}

	return c.client.Root(), closer, nil
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

func (c *gitSourceClient) GetRevisionMetadata(ctx context.Context, revision string, checkSignature bool) (*v1alpha1.RevisionMetadata, error) {
	// Validate that the revision is a commit SHA
	if !git.IsCommitSHA(revision) && !git.IsTruncatedCommitSHA(revision) {
		return nil, fmt.Errorf("revision %s must be resolved", revision)
	}

	repo := c.repo.Repo
	metadata, err := c.cache.GetRevisionMetadata(repo, revision)
	if err == nil {
		// The logic here is that if a signature check on metadata is requested,
		// but there is none in the cache, we handle as if we have a cache miss
		// and re-generate the metadata. Otherwise, if there is signature info
		// in the metadata, but none was requested, we remove it from the data
		// that we return.
		if !checkSignature || metadata.SignatureInfo != "" {
			log.Infof("revision metadata cache hit: %s/%s", repo, revision)
			if !checkSignature {
				metadata.SignatureInfo = ""
			}
			return metadata, nil
		}
		log.Infof("revision metadata cache hit, but need to regenerate due to missing signature info: %s/%s", repo, revision)
	} else {
		if !errors.Is(err, cache.ErrCacheMiss) {
			log.Warnf("revision metadata cache error %s/%s: %v", repo, revision, err)
		} else {
			log.Infof("revision metadata cache miss: %s/%s", repo, revision)
		}
	}

	c.metrics.IncPendingRepoRequest(repo)
	defer c.metrics.DecPendingRepoRequest(repo)

	// Lock and checkout the revision
	closer, err := c.repositoryLock.Lock(c.client.Root(), revision, true, func() (io.Closer, error) {
		closer := c.directoryPermissionInitializer(c.client.Root())
		err := c.client.Init()
		if err != nil {
			return closer, status.Errorf(codes.Internal, "Failed to initialize git repo: %v", err)
		}

		revisionPresent := c.client.IsRevisionPresent(revision)
		if !revisionPresent {
			if c.repo.Depth > 0 {
				err = c.client.Fetch(revision, c.repo.Depth)
			} else {
				err = c.client.Fetch("", c.repo.Depth)
			}

			if err != nil {
				return closer, status.Errorf(codes.Internal, "Failed to fetch default: %v", err)
			}
		}

		_, err = c.client.Checkout(revision, c.submoduleEnabled)
		if err != nil {
			log.Infof("Failed to checkout revision %s: %v", revision, err)
			log.Infof("Fallback to fetching specific revision %s", revision)
			err = c.client.Fetch(revision, c.repo.Depth)
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
		return nil, fmt.Errorf("error acquiring repo lock: %w", err)
	}
	defer utilio.Close(closer)

	// Get the basic revision metadata
	m, err := c.client.RevisionMetadata(revision)
	if err != nil {
		return nil, err
	}

	// Optionally verify the signature
	signatureInfo := ""
	if checkSignature {
		cs, err := c.client.VerifyCommitSignature(revision)
		if err != nil {
			log.Errorf("error verifying signature of commit '%s' in repo '%s': %v", revision, repo, err)
			return nil, err
		}

		if cs != "" {
			vr := gpg.ParseGitCommitVerification(cs)
			if vr.Result == gpg.VerifyResultUnknown {
				signatureInfo = "UNKNOWN signature: " + vr.Message
			} else {
				signatureInfo = fmt.Sprintf("%s signature from %s key %s", vr.Result, vr.Cipher, gpg.KeyID(vr.KeyID))
			}
		} else {
			signatureInfo = "Revision is not signed."
		}
	}

	// Build related revisions
	relatedRevisions := make([]v1alpha1.RevisionReference, len(m.References))
	for i := range m.References {
		if m.References[i].Commit == nil {
			continue
		}

		relatedRevisions[i] = v1alpha1.RevisionReference{
			Commit: &v1alpha1.CommitMetadata{
				Author:  m.References[i].Commit.Author.String(),
				Date:    m.References[i].Commit.Date,
				Subject: m.References[i].Commit.Subject,
				Body:    m.References[i].Commit.Body,
				SHA:     m.References[i].Commit.SHA,
				RepoURL: m.References[i].Commit.RepoURL,
			},
		}
	}

	details := &v1alpha1.RevisionMetadata{
		Author:        m.Author,
		Date:          &metav1.Time{Time: m.Date},
		Tags:          m.Tags,
		Message:       m.Message,
		SignatureInfo: signatureInfo,
		References:    relatedRevisions,
	}

	_ = c.cache.SetRevisionMetadata(repo, revision, details)
	return details, nil
}

func (c *gitSourceClient) ListRefs(ctx context.Context, noCache bool) (*apiclient.Refs, error) {
	refs, err := c.client.LsRefs()
	if err != nil {
		return nil, err
	}

	return &apiclient.Refs{
		Branches: refs.Branches,
		Tags:     refs.Tags,
	}, nil
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
