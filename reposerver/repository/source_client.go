package repository

import (
	"context"
	"fmt"
	"io"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	apppathutil "github.com/argoproj/argo-cd/v3/util/app/path"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/helm"
	"github.com/argoproj/argo-cd/v3/util/oci"
	"github.com/argoproj/argo-cd/v3/util/versions"
)

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

	// GetRootPath returns the root path where this client stores its content
	// For git this is the git repository root, for OCI/Helm it's a temp directory
	GetRootPath() string

	// CheckOutOfBoundsSymlinks validates that no symlinks point outside the root path
	CheckOutOfBoundsSymlinks(rootPath string, repo *v1alpha1.Repository, revision string) error
}

// ociSourceClient adapts an OCI client to the SourceClient interface
type ociSourceClient struct {
	client oci.Client
	repo   *v1alpha1.Repository
}

func NewOCISourceClient(client oci.Client, repo *v1alpha1.Repository) SourceClient {
	return &ociSourceClient{
		client: client,
		repo:   repo,
	}
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

func (c *ociSourceClient) GetRootPath() string {
	// OCI doesn't have a persistent root path like git
	return ""
}

func (c *ociSourceClient) CheckOutOfBoundsSymlinks(rootPath string, repo *v1alpha1.Repository, revision string) error {
	err := apppathutil.CheckOutOfBoundsSymlinks(rootPath)
	if err != nil {
		oobError := &apppathutil.OutOfBoundsSymlinkError{}
		if serr, ok := err.(*apppathutil.OutOfBoundsSymlinkError); ok {
			oobError = serr
			log.WithFields(log.Fields{
				common.SecurityField: common.SecurityHigh,
				"repo":               repo.Repo,
				"digest":             revision,
				"file":               oobError.File,
			}).Warn("oci image contains out-of-bounds symlink")
			return fmt.Errorf("oci image contains out-of-bounds symlinks. file: %s", oobError.File)
		}
		return err
	}
	return nil
}

// helmSourceClient adapts a Helm client to the SourceClient interface
type helmSourceClient struct {
	client                          helm.Client
	repo                            *v1alpha1.Repository
	chart                           string
	passCredentials                 bool
	helmManifestMaxExtractedSize    int64
	disableHelmManifestMaxExtracted bool
	helmRegistryMaxIndexSize        int64
}

type HelmSourceClientOpts struct {
	Chart                           string
	PassCredentials                 bool
	HelmManifestMaxExtractedSize    int64
	DisableHelmManifestMaxExtracted bool
	HelmRegistryMaxIndexSize        int64
}

func NewHelmSourceClient(client helm.Client, repo *v1alpha1.Repository, opts HelmSourceClientOpts) SourceClient {
	return &helmSourceClient{
		client:                          client,
		repo:                            repo,
		chart:                           opts.Chart,
		passCredentials:                 opts.PassCredentials,
		helmManifestMaxExtractedSize:    opts.HelmManifestMaxExtractedSize,
		disableHelmManifestMaxExtracted: opts.DisableHelmManifestMaxExtracted,
		helmRegistryMaxIndexSize:        opts.HelmRegistryMaxIndexSize,
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
		index, err := c.client.GetIndex(noCache, c.helmRegistryMaxIndexSize)
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
	return c.client.ExtractChart(c.chart, revision, c.passCredentials, c.helmManifestMaxExtractedSize, c.disableHelmManifestMaxExtracted)
}

func (c *helmSourceClient) GetRootPath() string {
	// Helm doesn't have a persistent root path like git
	return ""
}

func (c *helmSourceClient) CheckOutOfBoundsSymlinks(rootPath string, repo *v1alpha1.Repository, revision string) error {
	err := apppathutil.CheckOutOfBoundsSymlinks(rootPath)
	if err != nil {
		oobError := &apppathutil.OutOfBoundsSymlinkError{}
		if serr, ok := err.(*apppathutil.OutOfBoundsSymlinkError); ok {
			oobError = serr
			log.WithFields(log.Fields{
				common.SecurityField: common.SecurityHigh,
				"chart":              c.chart,
				"revision":           revision,
				"file":               oobError.File,
			}).Warn("chart contains out-of-bounds symlink")
			return fmt.Errorf("chart contains out-of-bounds symlinks. file: %s", oobError.File)
		}
		return err
	}
	return nil
}

// gitSourceClient adapts a Git client to the SourceClient interface
type gitSourceClient struct {
	client           git.Client
	repo             *v1alpha1.Repository
	repoLock         *repositoryLock
	submoduleEnabled bool
	depth            int64
	allowConcurrent  bool
	checkoutFn       func(git.Client, string, bool, int64) (io.Closer, error)
	metricsServer    interface{ IncGitLsRemoteFail(repoURL, revision string) }
}

type GitSourceClientOpts struct {
	SubmoduleEnabled bool
	Depth            int64
	AllowConcurrent  bool
	MetricsServer    interface{ IncGitLsRemoteFail(repoURL, revision string) }
}

func NewGitSourceClient(client git.Client, repo *v1alpha1.Repository, repoLock *repositoryLock, checkoutFn func(git.Client, string, bool, int64) (io.Closer, error), opts GitSourceClientOpts) SourceClient {
	return &gitSourceClient{
		client:           client,
		repo:             repo,
		repoLock:         repoLock,
		submoduleEnabled: opts.SubmoduleEnabled,
		depth:            opts.Depth,
		allowConcurrent:  opts.AllowConcurrent,
		checkoutFn:       checkoutFn,
		metricsServer:    opts.MetricsServer,
	}
}

func (c *gitSourceClient) ResolveRevision(ctx context.Context, revision string, noCache bool) (string, error) {
	commitSHA, err := c.client.LsRemote(revision)
	if err != nil {
		if c.metricsServer != nil {
			c.metricsServer.IncGitLsRemoteFail(c.client.Root(), revision)
		}
		return "", err
	}
	return commitSHA, nil
}

func (c *gitSourceClient) CleanCache(revision string) error {
	// Git doesn't support cache cleaning in the same way as OCI/Helm
	return nil
}

func (c *gitSourceClient) Extract(ctx context.Context, revision string) (string, io.Closer, error) {
	closer, err := c.repoLock.Lock(c.client.Root(), revision, c.allowConcurrent, func() (io.Closer, error) {
		return c.checkoutFn(c.client, revision, c.submoduleEnabled, c.depth)
	})
	if err != nil {
		return "", nil, err
	}
	return c.client.Root(), closer, nil
}

func (c *gitSourceClient) GetRootPath() string {
	return c.client.Root()
}

func (c *gitSourceClient) CheckOutOfBoundsSymlinks(rootPath string, repo *v1alpha1.Repository, revision string) error {
	err := apppathutil.CheckOutOfBoundsSymlinks(rootPath)
	if err != nil {
		oobError := &apppathutil.OutOfBoundsSymlinkError{}
		if serr, ok := err.(*apppathutil.OutOfBoundsSymlinkError); ok {
			oobError = serr
			log.WithFields(log.Fields{
				common.SecurityField: common.SecurityHigh,
				"repo":               repo.Repo,
				"revision":           revision,
				"file":               oobError.File,
			}).Warn("repository contains out-of-bounds symlink")
			return fmt.Errorf("repository contains out-of-bounds symlinks. file: %s", oobError.File)
		}
		return err
	}
	return nil
}
