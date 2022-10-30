package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	goio "io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/io/files"
	"github.com/argoproj/argo-cd/v2/util/manifeststream"

	"github.com/Masterminds/semver/v3"
	"github.com/TomOnTime/utfutil"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	textutils "github.com/argoproj/gitops-engine/pkg/utils/text"
	"github.com/argoproj/pkg/sync"
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/ghodss/yaml"
	gogit "github.com/go-git/go-git/v5"
	"github.com/google/go-jsonnet"
	"github.com/google/uuid"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	pluginclient "github.com/argoproj/argo-cd/v2/cmpserver/apiclient"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/reposerver/cache"
	reposervercache "github.com/argoproj/argo-cd/v2/reposerver/cache"
	"github.com/argoproj/argo-cd/v2/reposerver/metrics"
	"github.com/argoproj/argo-cd/v2/util/app/discovery"
	argopath "github.com/argoproj/argo-cd/v2/util/app/path"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/cmp"
	executil "github.com/argoproj/argo-cd/v2/util/exec"
	"github.com/argoproj/argo-cd/v2/util/git"
	"github.com/argoproj/argo-cd/v2/util/glob"
	"github.com/argoproj/argo-cd/v2/util/gpg"
	"github.com/argoproj/argo-cd/v2/util/grpc"
	"github.com/argoproj/argo-cd/v2/util/helm"
	"github.com/argoproj/argo-cd/v2/util/io"
	pathutil "github.com/argoproj/argo-cd/v2/util/io/path"
	"github.com/argoproj/argo-cd/v2/util/kustomize"
	"github.com/argoproj/argo-cd/v2/util/text"
)

const (
	cachedManifestGenerationPrefix = "Manifest generation error (cached)"
	pluginNotSupported             = "config management plugin not supported."
	helmDepUpMarkerFile            = ".argocd-helm-dep-up"
	allowConcurrencyFile           = ".argocd-allow-concurrency"
	repoSourceFile                 = ".argocd-source.yaml"
	appSourceFile                  = ".argocd-source-%s.yaml"
	ociPrefix                      = "oci://"
)

var ErrExceededMaxCombinedManifestFileSize = errors.New("exceeded max combined manifest file size")

// Service implements ManifestService interface
type Service struct {
	gitCredsStore             git.CredsStore
	rootDir                   string
	gitRepoPaths              *io.TempPaths
	chartPaths                *io.TempPaths
	gitRepoInitializer        func(rootPath string) goio.Closer
	repoLock                  *repositoryLock
	cache                     *reposervercache.Cache
	parallelismLimitSemaphore *semaphore.Weighted
	metricsServer             *metrics.MetricsServer
	resourceTracking          argo.ResourceTracking
	newGitClient              func(rawRepoURL string, root string, creds git.Creds, insecure bool, enableLfs bool, proxy string, opts ...git.ClientOpts) (git.Client, error)
	newHelmClient             func(repoURL string, creds helm.Creds, enableOci bool, proxy string, opts ...helm.ClientOpts) helm.Client
	initConstants             RepoServerInitConstants
	// now is usually just time.Now, but may be replaced by unit tests for testing purposes
	now func() time.Time
}

type RepoServerInitConstants struct {
	ParallelismLimit                             int64
	PauseGenerationAfterFailedGenerationAttempts int
	PauseGenerationOnFailureForMinutes           int
	PauseGenerationOnFailureForRequests          int
	SubmoduleEnabled                             bool
	MaxCombinedDirectoryManifestsSize            resource.Quantity
	CMPTarExcludedGlobs                          []string
	AllowOutOfBoundsSymlinks                     bool
	StreamedManifestMaxExtractedSize             int64
	StreamedManifestMaxTarSize                   int64
}

// NewService returns a new instance of the Manifest service
func NewService(metricsServer *metrics.MetricsServer, cache *reposervercache.Cache, initConstants RepoServerInitConstants, resourceTracking argo.ResourceTracking, gitCredsStore git.CredsStore, rootDir string) *Service {
	var parallelismLimitSemaphore *semaphore.Weighted
	if initConstants.ParallelismLimit > 0 {
		parallelismLimitSemaphore = semaphore.NewWeighted(initConstants.ParallelismLimit)
	}
	repoLock := NewRepositoryLock()
	return &Service{
		parallelismLimitSemaphore: parallelismLimitSemaphore,
		repoLock:                  repoLock,
		cache:                     cache,
		metricsServer:             metricsServer,
		newGitClient:              git.NewClientExt,
		resourceTracking:          resourceTracking,
		newHelmClient: func(repoURL string, creds helm.Creds, enableOci bool, proxy string, opts ...helm.ClientOpts) helm.Client {
			return helm.NewClientWithLock(repoURL, creds, sync.NewKeyLock(), enableOci, proxy, opts...)
		},
		initConstants:      initConstants,
		now:                time.Now,
		gitCredsStore:      gitCredsStore,
		gitRepoPaths:       io.NewTempPaths(rootDir),
		chartPaths:         io.NewTempPaths(rootDir),
		gitRepoInitializer: directoryPermissionInitializer,
		rootDir:            rootDir,
	}
}

func (s *Service) Init() error {
	_, err := os.Stat(s.rootDir)
	if os.IsNotExist(err) {
		return os.MkdirAll(s.rootDir, 0300)
	}
	if err == nil {
		// give itself read permissions to list previously written directories
		err = os.Chmod(s.rootDir, 0700)
	}
	var files []fs.DirEntry
	if err == nil {
		files, err = os.ReadDir(s.rootDir)
	}
	if err != nil {
		log.Warnf("Failed to restore cloned repositories paths: %v", err)
		return nil
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		fullPath := filepath.Join(s.rootDir, file.Name())
		closer := s.gitRepoInitializer(fullPath)
		if repo, err := gogit.PlainOpen(fullPath); err == nil {
			if remotes, err := repo.Remotes(); err == nil && len(remotes) > 0 && len(remotes[0].Config().URLs) > 0 {
				s.gitRepoPaths.Add(git.NormalizeGitURL(remotes[0].Config().URLs[0]), fullPath)
			}
		}
		io.Close(closer)
	}
	// remove read permissions since no-one should be able to list the directories
	return os.Chmod(s.rootDir, 0300)
}

// List a subset of the refs (currently, branches and tags) of a git repo
func (s *Service) ListRefs(ctx context.Context, q *apiclient.ListRefsRequest) (*apiclient.Refs, error) {
	gitClient, err := s.newClient(q.Repo)
	if err != nil {
		return nil, err
	}

	s.metricsServer.IncPendingRepoRequest(q.Repo.Repo)
	defer s.metricsServer.DecPendingRepoRequest(q.Repo.Repo)

	refs, err := gitClient.LsRefs()
	if err != nil {
		return nil, err
	}

	res := apiclient.Refs{
		Branches: refs.Branches,
		Tags:     refs.Tags,
	}

	return &res, nil
}

// ListApps lists the contents of a GitHub repo
func (s *Service) ListApps(ctx context.Context, q *apiclient.ListAppsRequest) (*apiclient.AppList, error) {
	gitClient, commitSHA, err := s.newClientResolveRevision(q.Repo, q.Revision)
	if err != nil {
		return nil, err
	}
	if apps, err := s.cache.ListApps(q.Repo.Repo, commitSHA); err == nil {
		log.Infof("cache hit: %s/%s", q.Repo.Repo, q.Revision)
		return &apiclient.AppList{Apps: apps}, nil
	}

	s.metricsServer.IncPendingRepoRequest(q.Repo.Repo)
	defer s.metricsServer.DecPendingRepoRequest(q.Repo.Repo)

	closer, err := s.repoLock.Lock(gitClient.Root(), commitSHA, true, func() (goio.Closer, error) {
		return s.checkoutRevision(gitClient, commitSHA, s.initConstants.SubmoduleEnabled)
	})

	if err != nil {
		return nil, err
	}

	defer io.Close(closer)
	apps, err := discovery.Discover(ctx, gitClient.Root(), q.EnabledSourceTypes, s.initConstants.CMPTarExcludedGlobs)
	if err != nil {
		return nil, err
	}
	err = s.cache.SetApps(q.Repo.Repo, commitSHA, apps)
	if err != nil {
		log.Warnf("cache set error %s/%s: %v", q.Repo.Repo, commitSHA, err)
	}
	res := apiclient.AppList{Apps: apps}
	return &res, nil
}

type operationSettings struct {
	sem             *semaphore.Weighted
	noCache         bool
	noRevisionCache bool
	allowConcurrent bool
}

// operationContext contains request values which are generated by runRepoOperation (on demand) by a call to the
// provided operationContextSrc function.
type operationContext struct {

	// application path or helm chart path
	appPath string

	// output of 'git verify-(tag/commit)', if signature verification is enabled (otherwise "")
	verificationResult string
}

// The 'operation' function parameter of 'runRepoOperation' may call this function to retrieve
// the appPath or GPG verificationResult.
// Failure to generate either of these values will return an error which may be cached by
// the calling function (for example, 'runManifestGen')
type operationContextSrc = func() (*operationContext, error)

// runRepoOperation downloads either git folder or helm chart and executes specified operation
// - Returns a value from the cache if present (by calling getCached(...)); if no value is present, the
// provide operation(...) is called. The specific return type of this function is determined by the
// calling function, via the provided  getCached(...) and operation(...) function.
func (s *Service) runRepoOperation(
	ctx context.Context,
	revision string,
	repo *v1alpha1.Repository,
	source *v1alpha1.ApplicationSource,
	verifyCommit bool,
	cacheFn func(cacheKey string, firstInvocation bool) (bool, error),
	operation func(repoRoot, commitSHA, cacheKey string, ctxSrc operationContextSrc) error,
	settings operationSettings) error {

	if sanitizer, ok := grpc.SanitizerFromContext(ctx); ok {
		// make sure randomized path replaced with '.' in the error message
		sanitizer.AddRegexReplacement(regexp.MustCompile(`(`+regexp.QuoteMeta(s.rootDir)+`/.*?)/`), ".")
	}

	var gitClient git.Client
	var helmClient helm.Client
	var err error
	revision = textutils.FirstNonEmpty(revision, source.TargetRevision)
	if source.IsHelm() {
		helmClient, revision, err = s.newHelmClientResolveRevision(repo, revision, source.Chart, settings.noCache || settings.noRevisionCache)
		if err != nil {
			return err
		}
	} else {
		gitClient, revision, err = s.newClientResolveRevision(repo, revision, git.WithCache(s.cache, !settings.noRevisionCache && !settings.noCache))
		if err != nil {
			return err
		}
	}

	if !settings.noCache {
		if ok, err := cacheFn(revision, true); ok {
			return err
		}
	}

	s.metricsServer.IncPendingRepoRequest(repo.Repo)
	defer s.metricsServer.DecPendingRepoRequest(repo.Repo)

	if settings.sem != nil {
		err = settings.sem.Acquire(ctx, 1)
		if err != nil {
			return err
		}
		defer settings.sem.Release(1)
	}

	if source.IsHelm() {
		if settings.noCache {
			err = helmClient.CleanChartCache(source.Chart, revision)
			if err != nil {
				return err
			}
		}
		helmPassCredentials := false
		if source.Helm != nil {
			helmPassCredentials = source.Helm.PassCredentials
		}
		chartPath, closer, err := helmClient.ExtractChart(source.Chart, revision, helmPassCredentials)
		if err != nil {
			return err
		}
		defer io.Close(closer)
		if !s.initConstants.AllowOutOfBoundsSymlinks {
			err := argopath.CheckOutOfBoundsSymlinks(chartPath)
			if err != nil {
				oobError := &argopath.OutOfBoundsSymlinkError{}
				if errors.As(err, &oobError) {
					log.WithFields(log.Fields{
						common.SecurityField: common.SecurityHigh,
						"chart":              source.Chart,
						"revision":           revision,
						"file":               oobError.File,
					}).Warn("chart contains out-of-bounds symlink")
					return fmt.Errorf("chart contains out-of-bounds symlinks. file: %s", oobError.File)
				} else {
					return err
				}
			}
		}
		return operation(chartPath, revision, revision, func() (*operationContext, error) {
			return &operationContext{chartPath, ""}, nil
		})
	} else {
		closer, err := s.repoLock.Lock(gitClient.Root(), revision, settings.allowConcurrent, func() (goio.Closer, error) {
			return s.checkoutRevision(gitClient, revision, s.initConstants.SubmoduleEnabled)
		})

		if err != nil {
			return err
		}

		defer io.Close(closer)

		if !s.initConstants.AllowOutOfBoundsSymlinks {
			err := argopath.CheckOutOfBoundsSymlinks(gitClient.Root())
			if err != nil {
				oobError := &argopath.OutOfBoundsSymlinkError{}
				if errors.As(err, &oobError) {
					log.WithFields(log.Fields{
						common.SecurityField: common.SecurityHigh,
						"repo":               repo.Repo,
						"revision":           revision,
						"file":               oobError.File,
					}).Warn("repository contains out-of-bounds symlink")
					return fmt.Errorf("repository contains out-of-bounds symlinks. file: %s", oobError.File)
				} else {
					return err
				}
			}
		}

		commitSHA, err := gitClient.CommitSHA()
		if err != nil {
			return err
		}

		// double-check locking
		if !settings.noCache {
			if ok, err := cacheFn(revision, false); ok {
				return err
			}
		}

		// Here commitSHA refers to the SHA of the actual commit, whereas revision refers to the branch/tag name etc
		// We use the commitSHA to generate manifests and store them in cache, and revision to retrieve them from cache
		return operation(gitClient.Root(), commitSHA, revision, func() (*operationContext, error) {
			var signature string
			if verifyCommit {
				signature, err = gitClient.VerifyCommitSignature(revision)
				if err != nil {
					return nil, err
				}
			}
			appPath, err := argopath.Path(gitClient.Root(), source.Path)
			if err != nil {
				return nil, err
			}
			return &operationContext{appPath, signature}, nil
		})
	}
}

func (s *Service) GenerateManifest(ctx context.Context, q *apiclient.ManifestRequest) (*apiclient.ManifestResponse, error) {
	var res *apiclient.ManifestResponse
	var err error

	cacheFn := func(cacheKey string, firstInvocation bool) (bool, error) {
		ok, resp, err := s.getManifestCacheEntry(cacheKey, q, firstInvocation)
		res = resp
		return ok, err
	}

	tarConcluded := false
	var promise *ManifestResponsePromise

	operation := func(repoRoot, commitSHA, cacheKey string, ctxSrc operationContextSrc) error {
		promise = s.runManifestGen(ctx, repoRoot, commitSHA, cacheKey, ctxSrc, q)
		// The fist channel to send the message will resume this operation.
		// The main purpose for using channels here is to be able to unlock
		// the repository as soon as the lock in not required anymore. In
		// case of CMP the repo is compressed (tgz) and sent to the cmp-server
		// for manifest generation.
		select {
		case err := <-promise.errCh:
			return err
		case resp := <-promise.responseCh:
			res = resp
		case tarDone := <-promise.tarDoneCh:
			tarConcluded = tarDone
		}
		return nil
	}

	settings := operationSettings{sem: s.parallelismLimitSemaphore, noCache: q.NoCache, noRevisionCache: q.NoRevisionCache, allowConcurrent: q.ApplicationSource.AllowsConcurrentProcessing()}
	err = s.runRepoOperation(ctx, q.Revision, q.Repo, q.ApplicationSource, q.VerifySignature, cacheFn, operation, settings)

	// if the tarDoneCh message is sent it means that the manifest
	// generation is being managed by the cmp-server. In this case
	// we have to wait for the responseCh to send the manifest
	// response.
	if tarConcluded && res == nil {
		select {
		case resp := <-promise.responseCh:
			res = resp
		case err := <-promise.errCh:
			return nil, err
		}
	}

	return res, err
}

func (s *Service) GenerateManifestWithFiles(stream apiclient.RepoServerService_GenerateManifestWithFilesServer) error {
	workDir, err := files.CreateTempDir("")
	if err != nil {
		return fmt.Errorf("error creating temp dir: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(workDir); err != nil {
			// we panic here as the workDir may contain sensitive information
			log.WithField(common.SecurityField, common.SecurityCritical).Errorf("error removing generate manifest workdir: %v", err)
			panic(fmt.Sprintf("error removing generate manifest workdir: %s", err))
		}
	}()

	req, metadata, err := manifeststream.ReceiveManifestFileStream(stream.Context(), stream, workDir, s.initConstants.StreamedManifestMaxTarSize, s.initConstants.StreamedManifestMaxExtractedSize)

	if err != nil {
		return fmt.Errorf("error receiving manifest file stream: %w", err)
	}

	if !s.initConstants.AllowOutOfBoundsSymlinks {
		err := argopath.CheckOutOfBoundsSymlinks(workDir)
		if err != nil {
			oobError := &argopath.OutOfBoundsSymlinkError{}
			if errors.As(err, &oobError) {
				log.WithFields(log.Fields{
					common.SecurityField: common.SecurityHigh,
					"file":               oobError.File,
				}).Warn("streamed files contains out-of-bounds symlink")
				return fmt.Errorf("streamed files contains out-of-bounds symlinks. file: %s", oobError.File)
			} else {
				return err
			}
		}
	}

	promise := s.runManifestGen(stream.Context(), workDir, "streamed", metadata.Checksum, func() (*operationContext, error) {
		appPath, err := argopath.Path(workDir, req.ApplicationSource.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to get app path: %w", err)
		}
		return &operationContext{appPath, ""}, nil
	}, req)

	var res *apiclient.ManifestResponse
	tarConcluded := false

	select {
	case err := <-promise.errCh:
		return err
	case tarDone := <-promise.tarDoneCh:
		tarConcluded = tarDone
	case resp := <-promise.responseCh:
		res = resp
	}

	if tarConcluded && res == nil {
		select {
		case resp := <-promise.responseCh:
			res = resp
		case err := <-promise.errCh:
			return err
		}
	}

	err = stream.SendAndClose(res)
	return err
}

type ManifestResponsePromise struct {
	responseCh <-chan *apiclient.ManifestResponse
	tarDoneCh  <-chan bool
	errCh      <-chan error
}

func NewManifestResponsePromise(responseCh <-chan *apiclient.ManifestResponse, tarDoneCh <-chan bool, errCh chan error) *ManifestResponsePromise {
	return &ManifestResponsePromise{
		responseCh: responseCh,
		tarDoneCh:  tarDoneCh,
		errCh:      errCh,
	}
}

type generateManifestCh struct {
	responseCh chan<- *apiclient.ManifestResponse
	tarDoneCh  chan<- bool
	errCh      chan<- error
}

// runManifestGen will be called by runRepoOperation if:
// - the cache does not contain a value for this key
// - or, the cache does contain a value for this key, but it is an expired manifest generation entry
// - or, NoCache is true
// Returns a ManifestResponse, or an error, but not both
func (s *Service) runManifestGen(ctx context.Context, repoRoot, commitSHA, cacheKey string, opContextSrc operationContextSrc, q *apiclient.ManifestRequest) *ManifestResponsePromise {

	responseCh := make(chan *apiclient.ManifestResponse)
	tarDoneCh := make(chan bool)
	errCh := make(chan error)
	responsePromise := NewManifestResponsePromise(responseCh, tarDoneCh, errCh)

	channels := &generateManifestCh{
		responseCh: responseCh,
		tarDoneCh:  tarDoneCh,
		errCh:      errCh,
	}
	go s.runManifestGenAsync(ctx, repoRoot, commitSHA, cacheKey, opContextSrc, q, channels)
	return responsePromise
}

func (s *Service) runManifestGenAsync(ctx context.Context, repoRoot, commitSHA, cacheKey string, opContextSrc operationContextSrc, q *apiclient.ManifestRequest, ch *generateManifestCh) {
	defer func() {
		close(ch.errCh)
		close(ch.responseCh)
	}()

	// GenerateManifests mutates the source (applies overrides). Those overrides shouldn't be reflected in the cache
	// key. Overrides will break the cache anyway, because changes to overrides will change the revision.
	appSourceCopy := q.ApplicationSource.DeepCopy()

	var manifestGenResult *apiclient.ManifestResponse
	opContext, err := opContextSrc()
	if err == nil {
		manifestGenResult, err = GenerateManifests(ctx, opContext.appPath, repoRoot, commitSHA, q, false, s.gitCredsStore, s.initConstants.MaxCombinedDirectoryManifestsSize, WithCMPTarDoneChannel(ch.tarDoneCh), WithCMPTarExcludedGlobs(s.initConstants.CMPTarExcludedGlobs))
	}
	if err != nil {
		// If manifest generation error caching is enabled
		if s.initConstants.PauseGenerationAfterFailedGenerationAttempts > 0 {
			cache.LogDebugManifestCacheKeyFields("getting manifests cache", "GenerateManifests error", cacheKey, q.ApplicationSource, q, q.Namespace, q.TrackingMethod, q.AppLabelKey, q.AppName)

			// Retrieve a new copy (if available) of the cached response: this ensures we are updating the latest copy of the cache,
			// rather than a copy of the cache that occurred before (a potentially lengthy) manifest generation.
			innerRes := &cache.CachedManifestResponse{}
			cacheErr := s.cache.GetManifests(cacheKey, appSourceCopy, q, q.Namespace, q.TrackingMethod, q.AppLabelKey, q.AppName, innerRes)
			if cacheErr != nil && cacheErr != reposervercache.ErrCacheMiss {
				log.Warnf("manifest cache set error %s: %v", appSourceCopy.String(), cacheErr)
				ch.errCh <- cacheErr
				return
			}

			// If this is the first error we have seen, store the time (we only use the first failure, as this
			// value is used for PauseGenerationOnFailureForMinutes)
			if innerRes.FirstFailureTimestamp == 0 {
				innerRes.FirstFailureTimestamp = s.now().Unix()
			}

			cache.LogDebugManifestCacheKeyFields("setting manifests cache", "GenerateManifests error", cacheKey, q.ApplicationSource, q, q.Namespace, q.TrackingMethod, q.AppLabelKey, q.AppName)

			// Update the cache to include failure information
			innerRes.NumberOfConsecutiveFailures++
			innerRes.MostRecentError = err.Error()
			cacheErr = s.cache.SetManifests(cacheKey, appSourceCopy, q, q.Namespace, q.TrackingMethod, q.AppLabelKey, q.AppName, innerRes)
			if cacheErr != nil {
				log.Warnf("manifest cache set error %s: %v", appSourceCopy.String(), cacheErr)
				ch.errCh <- cacheErr
				return
			}

		}
		ch.errCh <- err
		return
	}

	cache.LogDebugManifestCacheKeyFields("setting manifests cache", "fresh GenerateManifests response", cacheKey, q.ApplicationSource, q, q.Namespace, q.TrackingMethod, q.AppLabelKey, q.AppName)

	// Otherwise, no error occurred, so ensure the manifest generation error data in the cache entry is reset before we cache the value
	manifestGenCacheEntry := cache.CachedManifestResponse{
		ManifestResponse:                manifestGenResult,
		NumberOfCachedResponsesReturned: 0,
		NumberOfConsecutiveFailures:     0,
		FirstFailureTimestamp:           0,
		MostRecentError:                 "",
	}
	manifestGenResult.Revision = commitSHA
	manifestGenResult.VerifyResult = opContext.verificationResult
	err = s.cache.SetManifests(cacheKey, appSourceCopy, q, q.Namespace, q.TrackingMethod, q.AppLabelKey, q.AppName, &manifestGenCacheEntry)
	if err != nil {
		log.Warnf("manifest cache set error %s/%s: %v", appSourceCopy.String(), cacheKey, err)
	}
	ch.responseCh <- manifestGenCacheEntry.ManifestResponse
}

// getManifestCacheEntry returns false if the 'generate manifests' operation should be run by runRepoOperation, e.g.:
// - If the cache result is empty for the requested key
// - If the cache is not empty, but the cached value is a manifest generation error AND we have not yet met the failure threshold (e.g. res.NumberOfConsecutiveFailures > 0 && res.NumberOfConsecutiveFailures <  s.initConstants.PauseGenerationAfterFailedGenerationAttempts)
// - If the cache is not empty, but the cache value is an error AND that generation error has expired
// and returns true otherwise.
// If true is returned, either the second or third parameter (but not both) will contain a value from the cache (a ManifestResponse, or error, respectively)
func (s *Service) getManifestCacheEntry(cacheKey string, q *apiclient.ManifestRequest, firstInvocation bool) (bool, *apiclient.ManifestResponse, error) {
	cache.LogDebugManifestCacheKeyFields("getting manifests cache", "GenerateManifest API call", cacheKey, q.ApplicationSource, q, q.Namespace, q.TrackingMethod, q.AppLabelKey, q.AppName)

	res := cache.CachedManifestResponse{}
	err := s.cache.GetManifests(cacheKey, q.ApplicationSource, q, q.Namespace, q.TrackingMethod, q.AppLabelKey, q.AppName, &res)
	if err == nil {

		// The cache contains an existing value

		// If caching of manifest generation errors is enabled, and res is a cached manifest generation error...
		if s.initConstants.PauseGenerationAfterFailedGenerationAttempts > 0 && res.FirstFailureTimestamp > 0 {

			// If we are already in the 'manifest generation caching' state, due to too many consecutive failures...
			if res.NumberOfConsecutiveFailures >= s.initConstants.PauseGenerationAfterFailedGenerationAttempts {

				// Check if enough time has passed to try generation again (e.g. to exit the 'manifest generation caching' state)
				if s.initConstants.PauseGenerationOnFailureForMinutes > 0 {

					elapsedTimeInMinutes := int((s.now().Unix() - res.FirstFailureTimestamp) / 60)

					// After X minutes, reset the cache and retry the operation (e.g. perhaps the error is ephemeral and has passed)
					if elapsedTimeInMinutes >= s.initConstants.PauseGenerationOnFailureForMinutes {
						cache.LogDebugManifestCacheKeyFields("deleting manifests cache", "manifest hash did not match or cached response is empty", cacheKey, q.ApplicationSource, q, q.Namespace, q.TrackingMethod, q.AppLabelKey, q.AppName)

						// We can now try again, so reset the cache state and run the operation below
						err = s.cache.DeleteManifests(cacheKey, q.ApplicationSource, q, q.Namespace, q.TrackingMethod, q.AppLabelKey, q.AppName)
						if err != nil {
							log.Warnf("manifest cache set error %s/%s: %v", q.ApplicationSource.String(), cacheKey, err)
						}
						log.Infof("manifest error cache hit and reset: %s/%s", q.ApplicationSource.String(), cacheKey)
						return false, nil, nil
					}
				}

				// Check if enough cached responses have been returned to try generation again (e.g. to exit the 'manifest generation caching' state)
				if s.initConstants.PauseGenerationOnFailureForRequests > 0 && res.NumberOfCachedResponsesReturned > 0 {

					if res.NumberOfCachedResponsesReturned >= s.initConstants.PauseGenerationOnFailureForRequests {
						cache.LogDebugManifestCacheKeyFields("deleting manifests cache", "reset after paused generation count", cacheKey, q.ApplicationSource, q, q.Namespace, q.TrackingMethod, q.AppLabelKey, q.AppName)

						// We can now try again, so reset the error cache state and run the operation below
						err = s.cache.DeleteManifests(cacheKey, q.ApplicationSource, q, q.Namespace, q.TrackingMethod, q.AppLabelKey, q.AppName)
						if err != nil {
							log.Warnf("manifest cache set error %s/%s: %v", q.ApplicationSource.String(), cacheKey, err)
						}
						log.Infof("manifest error cache hit and reset: %s/%s", q.ApplicationSource.String(), cacheKey)
						return false, nil, nil
					}
				}

				// Otherwise, manifest generation is still paused
				log.Infof("manifest error cache hit: %s/%s", q.ApplicationSource.String(), cacheKey)

				cachedErrorResponse := fmt.Errorf(cachedManifestGenerationPrefix+": %s", res.MostRecentError)

				if firstInvocation {
					cache.LogDebugManifestCacheKeyFields("setting manifests cache", "update error count", cacheKey, q.ApplicationSource, q, q.Namespace, q.TrackingMethod, q.AppLabelKey, q.AppName)

					// Increment the number of returned cached responses and push that new value to the cache
					// (if we have not already done so previously in this function)
					res.NumberOfCachedResponsesReturned++
					err = s.cache.SetManifests(cacheKey, q.ApplicationSource, q, q.Namespace, q.TrackingMethod, q.AppLabelKey, q.AppName, &res)
					if err != nil {
						log.Warnf("manifest cache set error %s/%s: %v", q.ApplicationSource.String(), cacheKey, err)
					}
				}

				return true, nil, cachedErrorResponse

			}

			// Otherwise we are not yet in the manifest generation error state, and not enough consecutive errors have
			// yet occurred to put us in that state.
			log.Infof("manifest error cache miss: %s/%s", q.ApplicationSource.String(), cacheKey)
			return false, res.ManifestResponse, nil
		}

		log.Infof("manifest cache hit: %s/%s", q.ApplicationSource.String(), cacheKey)
		return true, res.ManifestResponse, nil
	}

	if err != reposervercache.ErrCacheMiss {
		log.Warnf("manifest cache error %s: %v", q.ApplicationSource.String(), err)
	} else {
		log.Infof("manifest cache miss: %s/%s", q.ApplicationSource.String(), cacheKey)
	}

	return false, nil, nil
}

func getHelmRepos(repositories []*v1alpha1.Repository) []helm.HelmRepository {
	repos := make([]helm.HelmRepository, 0)
	for _, repo := range repositories {
		repos = append(repos, helm.HelmRepository{Name: repo.Name, Repo: repo.Repo, Creds: repo.GetHelmCreds(), EnableOci: repo.EnableOCI})
	}
	return repos
}

type dependencies struct {
	Dependencies []repositories `yaml:"dependencies"`
}

type repositories struct {
	Repository string `yaml:"repository"`
}

func getHelmDependencyRepos(appPath string) ([]*v1alpha1.Repository, error) {
	repos := make([]*v1alpha1.Repository, 0)
	f, err := os.ReadFile(filepath.Join(appPath, "Chart.yaml"))
	if err != nil {
		return nil, err
	}

	d := &dependencies{}
	if err = yaml.Unmarshal(f, d); err != nil {
		return nil, err
	}

	for _, r := range d.Dependencies {
		if u, err := url.Parse(r.Repository); err == nil && (u.Scheme == "https" || u.Scheme == "oci") {
			repo := &v1alpha1.Repository{
				Repo:      r.Repository,
				Name:      sanitizeRepoName(r.Repository),
				EnableOCI: u.Scheme == "oci",
			}
			repos = append(repos, repo)
		}
	}

	return repos, nil
}

func sanitizeRepoName(repoName string) string {
	return strings.ReplaceAll(repoName, "/", "-")
}

func repoExists(repo string, repos []*v1alpha1.Repository) bool {
	for _, r := range repos {
		if strings.TrimPrefix(repo, ociPrefix) == strings.TrimPrefix(r.Repo, ociPrefix) {
			return true
		}
	}
	return false
}

func isConcurrencyAllowed(appPath string) bool {
	if _, err := os.Stat(path.Join(appPath, allowConcurrencyFile)); err == nil {
		return true
	}
	return false
}

var manifestGenerateLock = sync.NewKeyLock()

// runHelmBuild executes `helm dependency build` in a given path and ensures that it is executed only once
// if multiple threads are trying to run it.
// Multiple goroutines might process same helm app in one repo concurrently when repo server process multiple
// manifest generation requests of the same commit.
func runHelmBuild(appPath string, h helm.Helm) error {
	manifestGenerateLock.Lock(appPath)
	defer manifestGenerateLock.Unlock(appPath)

	// the `helm dependency build` is potentially time consuming 1~2 seconds
	// marker file is used to check if command already run to avoid running it again unnecessary
	// file is removed when repository re-initialized (e.g. when another commit is processed)
	markerFile := path.Join(appPath, helmDepUpMarkerFile)
	_, err := os.Stat(markerFile)
	if err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	err = h.DependencyBuild()
	if err != nil {
		return err
	}
	return os.WriteFile(markerFile, []byte("marker"), 0644)
}

func helmTemplate(appPath string, repoRoot string, env *v1alpha1.Env, q *apiclient.ManifestRequest, isLocal bool) ([]*unstructured.Unstructured, error) {
	concurrencyAllowed := isConcurrencyAllowed(appPath)
	if !concurrencyAllowed {
		manifestGenerateLock.Lock(appPath)
		defer manifestGenerateLock.Unlock(appPath)
	}

	// We use the app name as Helm's release name property, which must not
	// contain any underscore characters and must not exceed 53 characters.
	// We are not interested in the fully qualified application name while
	// templating, thus, we just use the name part of the identifier.
	appName, _ := argo.ParseAppInstanceName(q.AppName, "")

	templateOpts := &helm.TemplateOpts{
		Name:        appName,
		Namespace:   q.Namespace,
		KubeVersion: text.SemVer(q.KubeVersion),
		APIVersions: q.ApiVersions,
		Set:         map[string]string{},
		SetString:   map[string]string{},
		SetFile:     map[string]pathutil.ResolvedFilePath{},
	}

	appHelm := q.ApplicationSource.Helm
	var version string
	var passCredentials bool
	if appHelm != nil {
		if appHelm.Version != "" {
			version = appHelm.Version
		}
		if appHelm.ReleaseName != "" {
			templateOpts.Name = appHelm.ReleaseName
		}

		for _, val := range appHelm.ValueFiles {

			// This will resolve val to an absolute path (or an URL)
			path, isRemote, err := pathutil.ResolveFilePath(appPath, repoRoot, env.Envsubst(val), q.GetValuesFileSchemes())
			if err != nil {
				return nil, err
			}

			if !isRemote {
				_, err = os.Stat(string(path))
				if os.IsNotExist(err) {
					if appHelm.IgnoreMissingValueFiles {
						log.Debugf(" %s values file does not exist", path)
						continue
					}
				}
			}

			templateOpts.Values = append(templateOpts.Values, path)
		}

		if appHelm.Values != "" {
			rand, err := uuid.NewRandom()
			if err != nil {
				return nil, err
			}
			p := path.Join(os.TempDir(), rand.String())
			defer func() { _ = os.RemoveAll(p) }()
			err = os.WriteFile(p, []byte(appHelm.Values), 0644)
			if err != nil {
				return nil, err
			}
			templateOpts.Values = append(templateOpts.Values, pathutil.ResolvedFilePath(p))
		}

		for _, p := range appHelm.Parameters {
			if p.ForceString {
				templateOpts.SetString[p.Name] = p.Value
			} else {
				templateOpts.Set[p.Name] = p.Value
			}
		}
		for _, p := range appHelm.FileParameters {
			resolvedPath, _, err := pathutil.ResolveFilePath(appPath, repoRoot, env.Envsubst(p.Path), q.GetValuesFileSchemes())
			if err != nil {
				return nil, err
			}
			templateOpts.SetFile[p.Name] = resolvedPath
		}
		passCredentials = appHelm.PassCredentials
		templateOpts.SkipCrds = appHelm.SkipCrds
	}
	if templateOpts.Name == "" {
		templateOpts.Name = q.AppName
	}
	for i, j := range templateOpts.Set {
		templateOpts.Set[i] = env.Envsubst(j)
	}
	for i, j := range templateOpts.SetString {
		templateOpts.SetString[i] = env.Envsubst(j)
	}

	repos, err := getHelmDependencyRepos(appPath)
	if err != nil {
		return nil, err
	}

	for _, r := range repos {
		if !repoExists(r.Repo, q.Repos) {
			repositoryCredential := getRepoCredential(q.HelmRepoCreds, r.Repo)
			if repositoryCredential != nil {
				r.EnableOCI = repositoryCredential.EnableOCI
				r.Password = repositoryCredential.Password
				r.Username = repositoryCredential.Username
				r.SSHPrivateKey = repositoryCredential.SSHPrivateKey
				r.TLSClientCertData = repositoryCredential.TLSClientCertData
				r.TLSClientCertKey = repositoryCredential.TLSClientCertKey
			}
			q.Repos = append(q.Repos, r)
		}
	}

	var proxy string
	if q.Repo != nil {
		proxy = q.Repo.Proxy
	}

	h, err := helm.NewHelmApp(appPath, getHelmRepos(q.Repos), isLocal, version, proxy, passCredentials)
	if err != nil {
		return nil, err
	}

	defer h.Dispose()
	err = h.Init()
	if err != nil {
		return nil, err
	}

	out, err := h.Template(templateOpts)
	if err != nil {
		if !helm.IsMissingDependencyErr(err) {
			return nil, err
		}

		if concurrencyAllowed {
			err = runHelmBuild(appPath, h)
		} else {
			err = h.DependencyBuild()
		}

		if err != nil {
			return nil, err
		}

		out, err = h.Template(templateOpts)
		if err != nil {
			return nil, err
		}
	}
	return kube.SplitYAML([]byte(out))
}

func getRepoCredential(repoCredentials []*v1alpha1.RepoCreds, repoURL string) *v1alpha1.RepoCreds {
	for _, cred := range repoCredentials {
		url := strings.TrimPrefix(repoURL, ociPrefix)
		if strings.HasPrefix(url, cred.URL) {
			return cred
		}
	}
	return nil
}

type GenerateManifestOpt func(*generateManifestOpt)
type generateManifestOpt struct {
	cmpTarDoneCh        chan<- bool
	cmpTarExcludedGlobs []string
}

func newGenerateManifestOpt(opts ...GenerateManifestOpt) *generateManifestOpt {
	o := &generateManifestOpt{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithCMPTarDoneChannel defines the channel to be used to signalize when the tarball
// generation is concluded when generating manifests with the CMP server. This is used
// to unlock the git repo as soon as possible.
func WithCMPTarDoneChannel(ch chan<- bool) GenerateManifestOpt {
	return func(o *generateManifestOpt) {
		o.cmpTarDoneCh = ch
	}
}

// WithCMPTarExcludedGlobs defines globs for files to filter out when streaming the tarball
// to a CMP sidecar.
func WithCMPTarExcludedGlobs(excludedGlobs []string) GenerateManifestOpt {
	return func(o *generateManifestOpt) {
		o.cmpTarExcludedGlobs = excludedGlobs
	}
}

// GenerateManifests generates manifests from a path. Overrides are applied as a side effect on the given ApplicationSource.
func GenerateManifests(ctx context.Context, appPath, repoRoot, revision string, q *apiclient.ManifestRequest, isLocal bool, gitCredsStore git.CredsStore, maxCombinedManifestQuantity resource.Quantity, opts ...GenerateManifestOpt) (*apiclient.ManifestResponse, error) {
	opt := newGenerateManifestOpt(opts...)
	var targetObjs []*unstructured.Unstructured
	var dest *v1alpha1.ApplicationDestination

	resourceTracking := argo.NewResourceTracking()

	appSourceType, err := GetAppSourceType(ctx, q.ApplicationSource, appPath, q.AppName, q.EnabledSourceTypes, opt.cmpTarExcludedGlobs)
	if err != nil {
		return nil, err
	}
	repoURL := ""
	if q.Repo != nil {
		repoURL = q.Repo.Repo
	}
	env := newEnv(q, revision)

	switch appSourceType {
	case v1alpha1.ApplicationSourceTypeHelm:
		targetObjs, err = helmTemplate(appPath, repoRoot, env, q, isLocal)
	case v1alpha1.ApplicationSourceTypeKustomize:
		kustomizeBinary := ""
		if q.KustomizeOptions != nil {
			kustomizeBinary = q.KustomizeOptions.BinaryPath
		}
		k := kustomize.NewKustomizeApp(appPath, q.Repo.GetGitCreds(gitCredsStore), repoURL, kustomizeBinary)
		targetObjs, _, err = k.Build(q.ApplicationSource.Kustomize, q.KustomizeOptions, env)
	case v1alpha1.ApplicationSourceTypePlugin:
		if q.ApplicationSource.Plugin != nil && q.ApplicationSource.Plugin.Name != "" {
			log.WithFields(map[string]interface{}{
				"application": q.AppName,
				"plugin":      q.ApplicationSource.Plugin.Name,
			}).Warnf(common.ConfigMapPluginDeprecationWarning)

			targetObjs, err = runConfigManagementPlugin(appPath, repoRoot, env, q, q.Repo.GetGitCreds(gitCredsStore))
		} else {
			targetObjs, err = runConfigManagementPluginSidecars(ctx, appPath, repoRoot, env, q, q.Repo.GetGitCreds(gitCredsStore), opt.cmpTarDoneCh, opt.cmpTarExcludedGlobs)
			if err != nil {
				err = fmt.Errorf("plugin sidecar failed. %s", err.Error())
			}
		}
	case v1alpha1.ApplicationSourceTypeDirectory:
		var directory *v1alpha1.ApplicationSourceDirectory
		if directory = q.ApplicationSource.Directory; directory == nil {
			directory = &v1alpha1.ApplicationSourceDirectory{}
		}
		logCtx := log.WithField("application", q.AppName)
		targetObjs, err = findManifests(logCtx, appPath, repoRoot, env, *directory, q.EnabledSourceTypes, maxCombinedManifestQuantity)
	}
	if err != nil {
		return nil, err
	}

	manifests := make([]string, 0)
	for _, obj := range targetObjs {
		if obj == nil {
			continue
		}

		var targets []*unstructured.Unstructured
		if obj.IsList() {
			err = obj.EachListItem(func(object runtime.Object) error {
				unstructuredObj, ok := object.(*unstructured.Unstructured)
				if ok {
					targets = append(targets, unstructuredObj)
					return nil
				}
				return fmt.Errorf("resource list item has unexpected type")
			})
			if err != nil {
				return nil, err
			}
		} else if isNullList(obj) {
			// noop
		} else {
			targets = []*unstructured.Unstructured{obj}
		}

		for _, target := range targets {
			if q.AppLabelKey != "" && q.AppName != "" && !kube.IsCRD(target) {
				err = resourceTracking.SetAppInstance(target, q.AppLabelKey, q.AppName, q.Namespace, v1alpha1.TrackingMethod(q.TrackingMethod))
				if err != nil {
					return nil, err
				}
			}
			manifestStr, err := json.Marshal(target.Object)
			if err != nil {
				return nil, err
			}
			manifests = append(manifests, string(manifestStr))
		}
	}

	res := apiclient.ManifestResponse{
		Manifests:  manifests,
		SourceType: string(appSourceType),
	}
	if dest != nil {
		res.Namespace = dest.Namespace
		res.Server = dest.Server
	}
	return &res, nil
}

func newEnv(q *apiclient.ManifestRequest, revision string) *v1alpha1.Env {
	return &v1alpha1.Env{
		&v1alpha1.EnvEntry{Name: "ARGOCD_APP_NAME", Value: q.AppName},
		&v1alpha1.EnvEntry{Name: "ARGOCD_APP_NAMESPACE", Value: q.Namespace},
		&v1alpha1.EnvEntry{Name: "ARGOCD_APP_REVISION", Value: revision},
		&v1alpha1.EnvEntry{Name: "ARGOCD_APP_SOURCE_REPO_URL", Value: q.Repo.Repo},
		&v1alpha1.EnvEntry{Name: "ARGOCD_APP_SOURCE_PATH", Value: q.ApplicationSource.Path},
		&v1alpha1.EnvEntry{Name: "ARGOCD_APP_SOURCE_TARGET_REVISION", Value: q.ApplicationSource.TargetRevision},
	}
}

// mergeSourceParameters merges parameter overrides from one or more files in
// the Git repo into the given ApplicationSource objects.
//
// If .argocd-source.yaml exists at application's path in repository, it will
// be read and merged. If appName is not the empty string, and a file named
// .argocd-source-<appName>.yaml exists, it will also be read and merged.
func mergeSourceParameters(source *v1alpha1.ApplicationSource, path, appName string) error {
	repoFilePath := filepath.Join(path, repoSourceFile)
	overrides := []string{repoFilePath}
	if appName != "" {
		overrides = append(overrides, filepath.Join(path, fmt.Sprintf(appSourceFile, appName)))
	}

	var merged = *source.DeepCopy()

	for _, filename := range overrides {
		info, err := os.Stat(filename)
		if os.IsNotExist(err) {
			continue
		} else if info != nil && info.IsDir() {
			continue
		} else if err != nil {
			// filename should be part of error message here
			return err
		}

		data, err := json.Marshal(merged)
		if err != nil {
			return fmt.Errorf("%s: %v", filename, err)
		}
		patch, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("%s: %v", filename, err)
		}
		patch, err = yaml.YAMLToJSON(patch)
		if err != nil {
			return fmt.Errorf("%s: %v", filename, err)
		}
		data, err = jsonpatch.MergePatch(data, patch)
		if err != nil {
			return fmt.Errorf("%s: %v", filename, err)
		}
		err = json.Unmarshal(data, &merged)
		if err != nil {
			return fmt.Errorf("%s: %v", filename, err)
		}
	}

	// make sure only config management tools related properties are used and ignore everything else
	merged.Chart = source.Chart
	merged.Path = source.Path
	merged.RepoURL = source.RepoURL
	merged.TargetRevision = source.TargetRevision

	*source = merged
	return nil
}

// GetAppSourceType returns explicit application source type or examines a directory and determines its application source type
func GetAppSourceType(ctx context.Context, source *v1alpha1.ApplicationSource, path, appName string, enableGenerateManifests map[string]bool, tarExcludedGlobs []string) (v1alpha1.ApplicationSourceType, error) {
	err := mergeSourceParameters(source, path, appName)
	if err != nil {
		return "", fmt.Errorf("error while parsing source parameters: %v", err)
	}

	appSourceType, err := source.ExplicitType()
	if err != nil {
		return "", err
	}
	if appSourceType != nil {
		if !discovery.IsManifestGenerationEnabled(*appSourceType, enableGenerateManifests) {
			log.Debugf("Manifest generation is disabled for '%s'. Assuming plain YAML manifest.", *appSourceType)
			return v1alpha1.ApplicationSourceTypeDirectory, nil
		}
		return *appSourceType, nil
	}
	appType, err := discovery.AppType(ctx, path, enableGenerateManifests, tarExcludedGlobs)
	if err != nil {
		return "", err
	}
	return v1alpha1.ApplicationSourceType(appType), nil
}

// isNullList checks if the object is a "List" type where items is null instead of an empty list.
// Handles a corner case where obj.IsList() returns false when a manifest is like:
// ---
// apiVersion: v1
// items: null
// kind: ConfigMapList
func isNullList(obj *unstructured.Unstructured) bool {
	if _, ok := obj.Object["spec"]; ok {
		return false
	}
	if _, ok := obj.Object["status"]; ok {
		return false
	}
	field, ok := obj.Object["items"]
	if !ok {
		return false
	}
	return field == nil
}

var manifestFile = regexp.MustCompile(`^.*\.(yaml|yml|json|jsonnet)$`)

// findManifests looks at all yaml files in a directory and unmarshals them into a list of unstructured objects
func findManifests(logCtx *log.Entry, appPath string, repoRoot string, env *v1alpha1.Env, directory v1alpha1.ApplicationSourceDirectory, enabledManifestGeneration map[string]bool, maxCombinedManifestQuantity resource.Quantity) ([]*unstructured.Unstructured, error) {
	// Validate the directory before loading any manifests to save memory.
	potentiallyValidManifests, err := getPotentiallyValidManifests(logCtx, appPath, repoRoot, directory.Recurse, directory.Include, directory.Exclude, maxCombinedManifestQuantity)
	if err != nil {
		logCtx.Errorf("failed to get potentially valid manifests: %s", err)
		return nil, fmt.Errorf("failed to get potentially valid manifests: %w", err)
	}

	var objs []*unstructured.Unstructured
	for _, potentiallyValidManifest := range potentiallyValidManifests {
		manifestPath := potentiallyValidManifest.path
		manifestFileInfo := potentiallyValidManifest.fileInfo

		if strings.HasSuffix(manifestFileInfo.Name(), ".jsonnet") {
			if !discovery.IsManifestGenerationEnabled(v1alpha1.ApplicationSourceTypeDirectory, enabledManifestGeneration) {
				continue
			}
			vm, err := makeJsonnetVm(appPath, repoRoot, directory.Jsonnet, env)
			if err != nil {
				return nil, err
			}
			jsonStr, err := vm.EvaluateFile(manifestPath)
			if err != nil {
				return nil, status.Errorf(codes.FailedPrecondition, "Failed to evaluate jsonnet %q: %v", manifestFileInfo.Name(), err)
			}

			// attempt to unmarshal either array or single object
			var jsonObjs []*unstructured.Unstructured
			err = json.Unmarshal([]byte(jsonStr), &jsonObjs)
			if err == nil {
				objs = append(objs, jsonObjs...)
			} else {
				var jsonObj unstructured.Unstructured
				err = json.Unmarshal([]byte(jsonStr), &jsonObj)
				if err != nil {
					return nil, status.Errorf(codes.FailedPrecondition, "Failed to unmarshal generated json %q: %v", manifestFileInfo.Name(), err)
				}
				objs = append(objs, &jsonObj)
			}
		} else {
			err := getObjsFromYAMLOrJson(logCtx, manifestPath, manifestFileInfo.Name(), &objs)
			if err != nil {
				return nil, err
			}
		}
	}
	return objs, nil
}

// getObjsFromYAMLOrJson unmarshals the given yaml or json file and appends it to the given list of objects.
func getObjsFromYAMLOrJson(logCtx *log.Entry, manifestPath string, filename string, objs *[]*unstructured.Unstructured) error {
	reader, err := utfutil.OpenFile(manifestPath, utfutil.UTF8)
	if err != nil {
		return status.Errorf(codes.FailedPrecondition, "Failed to open %q", manifestPath)
	}
	defer func() {
		err := reader.Close()
		if err != nil {
			logCtx.Errorf("failed to close %q - potential memory leak", manifestPath)
		}
	}()
	if strings.HasSuffix(filename, ".json") {
		var obj unstructured.Unstructured
		decoder := json.NewDecoder(reader)
		err = decoder.Decode(&obj)
		if err != nil {
			return status.Errorf(codes.FailedPrecondition, "Failed to unmarshal %q: %v", filename, err)
		}
		if decoder.More() {
			return status.Errorf(codes.FailedPrecondition, "Found multiple objects in %q. Only single objects are allowed in JSON files.", filename)
		}
		*objs = append(*objs, &obj)
	} else {
		yamlObjs, err := splitYAMLOrJSON(reader)
		if err != nil {
			if len(yamlObjs) > 0 {
				// If we get here, we had a multiple objects in a single YAML file which had some
				// valid k8s objects, but errors parsing others (within the same file). It's very
				// likely the user messed up a portion of the YAML, so report on that.
				return status.Errorf(codes.FailedPrecondition, "Failed to unmarshal %q: %v", filename, err)
			}
			// Read the whole file to check whether it looks like a manifest.
			out, err := utfutil.ReadFile(manifestPath, utfutil.UTF8)
			// Otherwise, let's see if it looks like a resource, if yes, we return error
			if bytes.Contains(out, []byte("apiVersion:")) &&
				bytes.Contains(out, []byte("kind:")) &&
				bytes.Contains(out, []byte("metadata:")) {
				return status.Errorf(codes.FailedPrecondition, "Failed to unmarshal %q: %v", filename, err)
			}
			// Otherwise, it might be an unrelated YAML file which we will ignore
		}
		*objs = append(*objs, yamlObjs...)
	}
	return nil
}

// splitYAMLOrJSON reads a YAML or JSON file and gets each document as an unstructured object. If the unmarshaller
// encounters an error, objects read up until the error are returned.
func splitYAMLOrJSON(reader goio.Reader) ([]*unstructured.Unstructured, error) {
	d := kubeyaml.NewYAMLOrJSONDecoder(reader, 4096)
	var objs []*unstructured.Unstructured
	for {
		u := &unstructured.Unstructured{}
		if err := d.Decode(&u); err != nil {
			if err == goio.EOF {
				break
			}
			return objs, fmt.Errorf("failed to unmarshal manifest: %v", err)
		}
		if u == nil {
			continue
		}
		objs = append(objs, u)
	}
	return objs, nil
}

// getPotentiallyValidManifestFile checks whether the given path/FileInfo may be a valid manifest file. Returns a non-nil error if
// there was an error that should not be handled by ignoring the file. Returns non-nil realFileInfo if the file is a
// potential manifest. Returns a non-empty ignoreMessage if there's a message that should be logged about why the file
// was skipped. If realFileInfo is nil and the ignoreMessage is empty, there's no need to log the ignoreMessage; the
// file was skipped for a mundane reason.
//
// The file is still only a "potentially" valid manifest file because it could be invalid JSON or YAML, or it might not
// be a valid Kubernetes resource. This function tests everything possible without actually reading the file.
//
// repoPath must be absolute.
func getPotentiallyValidManifestFile(path string, f os.FileInfo, appPath, repoRoot, include, exclude string) (realFileInfo os.FileInfo, warning string, err error) {
	relPath, err := filepath.Rel(appPath, path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get relative path of %q: %w", path, err)
	}

	if !manifestFile.MatchString(f.Name()) {
		return nil, "", nil
	}

	// If the file is a symlink, these will be overridden with the destination file's info.
	var relRealPath = relPath
	realFileInfo = f

	if files.IsSymlink(f) {
		realPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Sprintf("destination of symlink %q is missing", relPath), nil
			}
			return nil, "", fmt.Errorf("failed to evaluate symlink at %q: %w", relPath, err)
		}
		if !files.Inbound(realPath, repoRoot) {
			return nil, "", fmt.Errorf("illegal filepath in symlink at %q", relPath)
		}
		realFileInfo, err = os.Stat(realPath)
		if err != nil {
			if os.IsNotExist(err) {
				// This should have been caught by filepath.EvalSymlinks, but check again since that function's docs
				// don't promise to return this error.
				return nil, fmt.Sprintf("destination of symlink %q is missing at %q", relPath, realPath), nil
			}
			return nil, "", fmt.Errorf("failed to get file info for symlink at %q to %q: %w", relPath, realPath, err)
		}
		relRealPath, err = filepath.Rel(repoRoot, realPath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get relative path of %q: %w", realPath, err)
		}
	}

	// FileInfo.Size() behavior is platform-specific for non-regular files. Allow only regular files, so we guarantee
	// accurate file sizes.
	if !realFileInfo.Mode().IsRegular() {
		return nil, fmt.Sprintf("ignoring symlink at %q to non-regular file %q", relPath, relRealPath), nil
	}

	if exclude != "" && glob.Match(exclude, relPath) {
		return nil, "", nil
	}

	if include != "" && !glob.Match(include, relPath) {
		return nil, "", nil
	}

	return realFileInfo, "", nil
}

type potentiallyValidManifest struct {
	path     string
	fileInfo os.FileInfo
}

// getPotentiallyValidManifests ensures that 1) there are no errors while checking for potential manifest files in the given dir
// and 2) the combined file size of the potentially-valid manifest files does not exceed the limit.
func getPotentiallyValidManifests(logCtx *log.Entry, appPath string, repoRoot string, recurse bool, include string, exclude string, maxCombinedManifestQuantity resource.Quantity) ([]potentiallyValidManifest, error) {
	maxCombinedManifestFileSize := maxCombinedManifestQuantity.Value()
	var currentCombinedManifestFileSize = int64(0)

	var potentiallyValidManifests []potentiallyValidManifest
	err := filepath.Walk(appPath, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if f.IsDir() {
			if path != appPath && !recurse {
				return filepath.SkipDir
			}
			return nil
		}

		realFileInfo, warning, err := getPotentiallyValidManifestFile(path, f, appPath, repoRoot, include, exclude)
		if err != nil {
			return fmt.Errorf("invalid manifest file %q: %w", path, err)
		}
		if realFileInfo == nil {
			if warning != "" {
				logCtx.Warnf("skipping manifest file %q: %s", path, warning)
			}
			return nil
		}
		// Don't count jsonnet file size against max. It's jsonnet's responsibility to manage memory usage.
		if !strings.HasSuffix(f.Name(), ".jsonnet") {
			// We use the realFileInfo size (which is guaranteed to be a regular file instead of a symlink or other
			// non-regular file) because .Size() behavior is platform-specific for non-regular files.
			currentCombinedManifestFileSize += realFileInfo.Size()
			if maxCombinedManifestFileSize != 0 && currentCombinedManifestFileSize > maxCombinedManifestFileSize {
				return ErrExceededMaxCombinedManifestFileSize
			}
		}
		potentiallyValidManifests = append(potentiallyValidManifests, potentiallyValidManifest{path: path, fileInfo: f})
		return nil
	})
	if err != nil {
		// Not wrapping, because this error should be wrapped by the caller.
		return nil, err
	}

	return potentiallyValidManifests, nil
}

func makeJsonnetVm(appPath string, repoRoot string, sourceJsonnet v1alpha1.ApplicationSourceJsonnet, env *v1alpha1.Env) (*jsonnet.VM, error) {

	vm := jsonnet.MakeVM()
	for i, j := range sourceJsonnet.TLAs {
		sourceJsonnet.TLAs[i].Value = env.Envsubst(j.Value)
	}
	for i, j := range sourceJsonnet.ExtVars {
		sourceJsonnet.ExtVars[i].Value = env.Envsubst(j.Value)
	}
	for _, arg := range sourceJsonnet.TLAs {
		if arg.Code {
			vm.TLACode(arg.Name, arg.Value)
		} else {
			vm.TLAVar(arg.Name, arg.Value)
		}
	}
	for _, extVar := range sourceJsonnet.ExtVars {
		if extVar.Code {
			vm.ExtCode(extVar.Name, extVar.Value)
		} else {
			vm.ExtVar(extVar.Name, extVar.Value)
		}
	}

	// Jsonnet Imports relative to the repository path
	jpaths := []string{appPath}
	for _, p := range sourceJsonnet.Libs {
		// the jsonnet library path is relative to the repository root, not application path
		jpath, _, err := pathutil.ResolveFilePath(repoRoot, repoRoot, p, nil)
		if err != nil {
			return nil, err
		}
		jpaths = append(jpaths, string(jpath))
	}

	vm.Importer(&jsonnet.FileImporter{
		JPaths: jpaths,
	})

	return vm, nil
}

func runCommand(command v1alpha1.Command, path string, env []string) (string, error) {
	if len(command.Command) == 0 {
		return "", fmt.Errorf("Command is empty")
	}
	cmd := exec.Command(command.Command[0], append(command.Command[1:], command.Args...)...)
	cmd.Env = env
	cmd.Dir = path
	return executil.Run(cmd)
}

func findPlugin(plugins []*v1alpha1.ConfigManagementPlugin, name string) *v1alpha1.ConfigManagementPlugin {
	for _, plugin := range plugins {
		if plugin.Name == name {
			return plugin
		}
	}
	return nil
}

func runConfigManagementPlugin(appPath, repoRoot string, envVars *v1alpha1.Env, q *apiclient.ManifestRequest, creds git.Creds) ([]*unstructured.Unstructured, error) {
	plugin := findPlugin(q.Plugins, q.ApplicationSource.Plugin.Name)
	if plugin == nil {
		return nil, fmt.Errorf(pluginNotSupported+" plugin name %s", q.ApplicationSource.Plugin.Name)
	}

	// Plugins can request to lock the complete repository when they need to
	// use git client operations.
	if plugin.LockRepo {
		manifestGenerateLock.Lock(repoRoot)
		defer manifestGenerateLock.Unlock(repoRoot)
	} else {
		concurrencyAllowed := isConcurrencyAllowed(appPath)
		if !concurrencyAllowed {
			manifestGenerateLock.Lock(appPath)
			defer manifestGenerateLock.Unlock(appPath)
		}
	}

	env, err := getPluginEnvs(envVars, q, creds, false)
	if err != nil {
		return nil, err
	}

	if plugin.Init != nil {
		_, err := runCommand(*plugin.Init, appPath, env)
		if err != nil {
			return nil, err
		}
	}
	out, err := runCommand(plugin.Generate, appPath, env)
	if err != nil {
		return nil, err
	}
	return kube.SplitYAML([]byte(out))
}

func getPluginEnvs(envVars *v1alpha1.Env, q *apiclient.ManifestRequest, creds git.Creds, remote bool) ([]string, error) {
	env := envVars.Environ()
	// Local plugins need also to have access to the local environment variables.
	// Remote side car plugins will use the environment in the side car
	// container.
	if !remote {
		env = append(os.Environ(), env...)
	}
	if creds != nil {
		closer, environ, err := creds.Environ()
		if err != nil {
			return nil, err
		}
		defer func() { _ = closer.Close() }()
		env = append(env, environ...)
	}
	env = append(env, "KUBE_VERSION="+text.SemVer(q.KubeVersion))
	env = append(env, "KUBE_API_VERSIONS="+strings.Join(q.ApiVersions, ","))

	parsedEnv := make(v1alpha1.Env, len(env))
	for i, v := range env {
		parsedVar, err := v1alpha1.NewEnvEntry(v)
		if err != nil {
			return nil, fmt.Errorf("failed to parse env vars")
		}
		parsedEnv[i] = parsedVar
	}

	if q.ApplicationSource.Plugin != nil {
		pluginEnv := q.ApplicationSource.Plugin.Env
		for _, entry := range pluginEnv {
			newValue := parsedEnv.Envsubst(entry.Value)
			env = append(env, fmt.Sprintf("ARGOCD_ENV_%s=%s", entry.Name, newValue))
		}
	}
	return env, nil
}

func runConfigManagementPluginSidecars(ctx context.Context, appPath, repoPath string, envVars *v1alpha1.Env, q *apiclient.ManifestRequest, creds git.Creds, tarDoneCh chan<- bool, tarExcludedGlobs []string) ([]*unstructured.Unstructured, error) {
	// compute variables.
	env, err := getPluginEnvs(envVars, q, creds, true)
	if err != nil {
		return nil, err
	}

	// detect config management plugin server (sidecar)
	conn, cmpClient, err := discovery.DetectConfigManagementPlugin(ctx, appPath, env, tarExcludedGlobs)
	if err != nil {
		return nil, err
	}
	defer io.Close(conn)

	// generate manifests using commands provided in plugin config file in detected cmp-server sidecar
	cmpManifests, err := generateManifestsCMP(ctx, appPath, repoPath, env, cmpClient, tarDoneCh, tarExcludedGlobs)
	if err != nil {
		return nil, fmt.Errorf("error generating manifests in cmp: %s", err)
	}
	var manifests []*unstructured.Unstructured
	for _, manifestString := range cmpManifests.Manifests {
		manifestObjs, err := kube.SplitYAML([]byte(manifestString))
		if err != nil {
			return nil, fmt.Errorf("failed to convert CMP manifests to unstructured objects: %s", err.Error())
		}
		manifests = append(manifests, manifestObjs...)
	}
	return manifests, nil
}

// generateManifestsCMP will send the appPath files to the cmp-server over a gRPC stream.
// The cmp-server will generate the manifests. Returns a response object with the generated
// manifests.
func generateManifestsCMP(ctx context.Context, appPath, repoPath string, env []string, cmpClient pluginclient.ConfigManagementPluginServiceClient, tarDoneCh chan<- bool, tarExcludedGlobs []string) (*pluginclient.ManifestResponse, error) {
	generateManifestStream, err := cmpClient.GenerateManifest(ctx, grpc_retry.Disable())
	if err != nil {
		return nil, fmt.Errorf("error getting generateManifestStream: %s", err)
	}
	opts := []cmp.SenderOption{
		cmp.WithTarDoneChan(tarDoneCh),
	}

	err = cmp.SendRepoStream(generateManifestStream.Context(), appPath, repoPath, generateManifestStream, env, tarExcludedGlobs, opts...)
	if err != nil {
		return nil, fmt.Errorf("error sending file to cmp-server: %s", err)
	}

	return generateManifestStream.CloseAndRecv()
}

func (s *Service) GetAppDetails(ctx context.Context, q *apiclient.RepoServerAppDetailsQuery) (*apiclient.RepoAppDetailsResponse, error) {
	res := &apiclient.RepoAppDetailsResponse{}

	cacheFn := s.createGetAppDetailsCacheHandler(res, q)
	operation := func(repoRoot, commitSHA, revision string, ctxSrc operationContextSrc) error {
		opContext, err := ctxSrc()
		if err != nil {
			return err
		}

		appSourceType, err := GetAppSourceType(ctx, q.Source, opContext.appPath, q.AppName, q.EnabledSourceTypes, s.initConstants.CMPTarExcludedGlobs)
		if err != nil {
			return err
		}

		res.Type = string(appSourceType)

		switch appSourceType {
		case v1alpha1.ApplicationSourceTypeHelm:
			if err := populateHelmAppDetails(res, opContext.appPath, repoRoot, q); err != nil {
				return err
			}
		case v1alpha1.ApplicationSourceTypeKustomize:
			if err := populateKustomizeAppDetails(res, q, opContext.appPath, commitSHA, s.gitCredsStore); err != nil {
				return err
			}
		}
		_ = s.cache.SetAppDetails(revision, q.Source, res, v1alpha1.TrackingMethod(q.TrackingMethod))
		return nil
	}

	settings := operationSettings{allowConcurrent: q.Source.AllowsConcurrentProcessing(), noCache: q.NoCache, noRevisionCache: q.NoCache || q.NoRevisionCache}
	err := s.runRepoOperation(ctx, q.Source.TargetRevision, q.Repo, q.Source, false, cacheFn, operation, settings)

	return res, err
}

func (s *Service) createGetAppDetailsCacheHandler(res *apiclient.RepoAppDetailsResponse, q *apiclient.RepoServerAppDetailsQuery) func(revision string, _ bool) (bool, error) {
	return func(revision string, _ bool) (bool, error) {
		err := s.cache.GetAppDetails(revision, q.Source, res, v1alpha1.TrackingMethod(q.TrackingMethod))
		if err == nil {
			log.Infof("app details cache hit: %s/%s", revision, q.Source.Path)
			return true, nil
		}

		if err != reposervercache.ErrCacheMiss {
			log.Warnf("app details cache error %s: %v", revision, q.Source)
		} else {
			log.Infof("app details cache miss: %s/%s", revision, q.Source)
		}
		return false, nil
	}
}

func populateHelmAppDetails(res *apiclient.RepoAppDetailsResponse, appPath string, repoRoot string, q *apiclient.RepoServerAppDetailsQuery) error {
	var selectedValueFiles []string

	if q.Source.Helm != nil {
		selectedValueFiles = q.Source.Helm.ValueFiles
	}

	availableValueFiles, err := findHelmValueFilesInPath(appPath)
	if err != nil {
		return err
	}

	res.Helm = &apiclient.HelmAppSpec{ValueFiles: availableValueFiles}
	var version string
	var passCredentials bool
	if q.Source.Helm != nil {
		if q.Source.Helm.Version != "" {
			version = q.Source.Helm.Version
		}
		passCredentials = q.Source.Helm.PassCredentials
	}
	h, err := helm.NewHelmApp(appPath, getHelmRepos(q.Repos), false, version, q.Repo.Proxy, passCredentials)
	if err != nil {
		return err
	}
	defer h.Dispose()
	err = h.Init()
	if err != nil {
		return err
	}

	if resolvedValuesPath, _, err := pathutil.ResolveFilePath(appPath, repoRoot, "values.yaml", []string{}); err == nil {
		if err := loadFileIntoIfExists(resolvedValuesPath, &res.Helm.Values); err != nil {
			return err
		}
	} else {
		log.Warnf("Values file %s is not allowed: %v", filepath.Join(appPath, "values.yaml"), err)
	}
	var resolvedSelectedValueFiles []pathutil.ResolvedFilePath
	// drop not allowed values files
	for _, file := range selectedValueFiles {
		if resolvedFile, _, err := pathutil.ResolveFilePath(appPath, repoRoot, file, q.GetValuesFileSchemes()); err == nil {
			resolvedSelectedValueFiles = append(resolvedSelectedValueFiles, resolvedFile)
		} else {
			log.Warnf("Values file %s is not allowed: %v", file, err)
		}
	}
	params, err := h.GetParameters(resolvedSelectedValueFiles, appPath, repoRoot)
	if err != nil {
		return err
	}
	for k, v := range params {
		res.Helm.Parameters = append(res.Helm.Parameters, &v1alpha1.HelmParameter{
			Name:  k,
			Value: v,
		})
	}
	for _, v := range fileParameters(q) {
		res.Helm.FileParameters = append(res.Helm.FileParameters, &v1alpha1.HelmFileParameter{
			Name: v.Name,
			Path: v.Path, //filepath.Join(appPath, v.Path),
		})
	}
	return nil
}

func loadFileIntoIfExists(path pathutil.ResolvedFilePath, destination *string) error {
	stringPath := string(path)
	info, err := os.Stat(stringPath)

	if err == nil && !info.IsDir() {
		bytes, err := os.ReadFile(stringPath)
		if err != nil {
			return err
		}
		*destination = string(bytes)
	}

	return nil
}

func findHelmValueFilesInPath(path string) ([]string, error) {
	var result []string

	files, err := os.ReadDir(path)
	if err != nil {
		return result, err
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		filename := f.Name()
		fileNameExt := strings.ToLower(filepath.Ext(filename))
		if strings.Contains(filename, "values") && (fileNameExt == ".yaml" || fileNameExt == ".yml") {
			result = append(result, filename)
		}
	}

	return result, nil
}

func populateKustomizeAppDetails(res *apiclient.RepoAppDetailsResponse, q *apiclient.RepoServerAppDetailsQuery, appPath string, reversion string, credsStore git.CredsStore) error {
	res.Kustomize = &apiclient.KustomizeAppSpec{}
	kustomizeBinary := ""
	if q.KustomizeOptions != nil {
		kustomizeBinary = q.KustomizeOptions.BinaryPath
	}
	k := kustomize.NewKustomizeApp(appPath, q.Repo.GetGitCreds(credsStore), q.Repo.Repo, kustomizeBinary)
	fakeManifestRequest := apiclient.ManifestRequest{
		AppName:           q.AppName,
		Namespace:         "", // FIXME: omit it for now
		Repo:              q.Repo,
		ApplicationSource: q.Source,
	}
	env := newEnv(&fakeManifestRequest, reversion)
	_, images, err := k.Build(q.Source.Kustomize, q.KustomizeOptions, env)
	if err != nil {
		return err
	}
	res.Kustomize.Images = images
	return nil
}

func (s *Service) GetRevisionMetadata(ctx context.Context, q *apiclient.RepoServerRevisionMetadataRequest) (*v1alpha1.RevisionMetadata, error) {
	if !(git.IsCommitSHA(q.Revision) || git.IsTruncatedCommitSHA(q.Revision)) {
		return nil, fmt.Errorf("revision %s must be resolved", q.Revision)
	}
	metadata, err := s.cache.GetRevisionMetadata(q.Repo.Repo, q.Revision)
	if err == nil {
		// The logic here is that if a signature check on metadata is requested,
		// but there is none in the cache, we handle as if we have a cache miss
		// and re-generate the meta data. Otherwise, if there is signature info
		// in the metadata, but none was requested, we remove it from the data
		// that we return.
		if q.CheckSignature && metadata.SignatureInfo == "" {
			log.Infof("revision metadata cache hit, but need to regenerate due to missing signature info: %s/%s", q.Repo.Repo, q.Revision)
		} else {
			log.Infof("revision metadata cache hit: %s/%s", q.Repo.Repo, q.Revision)
			if !q.CheckSignature {
				metadata.SignatureInfo = ""
			}
			return metadata, nil
		}
	} else {
		if err != reposervercache.ErrCacheMiss {
			log.Warnf("revision metadata cache error %s/%s: %v", q.Repo.Repo, q.Revision, err)
		} else {
			log.Infof("revision metadata cache miss: %s/%s", q.Repo.Repo, q.Revision)
		}
	}

	gitClient, _, err := s.newClientResolveRevision(q.Repo, q.Revision)
	if err != nil {
		return nil, err
	}

	s.metricsServer.IncPendingRepoRequest(q.Repo.Repo)
	defer s.metricsServer.DecPendingRepoRequest(q.Repo.Repo)

	closer, err := s.repoLock.Lock(gitClient.Root(), q.Revision, true, func() (goio.Closer, error) {
		return s.checkoutRevision(gitClient, q.Revision, s.initConstants.SubmoduleEnabled)
	})

	if err != nil {
		return nil, err
	}

	defer io.Close(closer)

	m, err := gitClient.RevisionMetadata(q.Revision)
	if err != nil {
		return nil, err
	}

	// Run gpg verify-commit on the revision
	signatureInfo := ""
	if gpg.IsGPGEnabled() && q.CheckSignature {
		cs, err := gitClient.VerifyCommitSignature(q.Revision)
		if err != nil {
			log.Errorf("error verifying signature of commit '%s' in repo '%s': %v", q.Revision, q.Repo.Repo, err)
			return nil, err
		}

		if cs != "" {
			vr := gpg.ParseGitCommitVerification(cs)
			if vr.Result == gpg.VerifyResultUnknown {
				signatureInfo = fmt.Sprintf("UNKNOWN signature: %s", vr.Message)
			} else {
				signatureInfo = fmt.Sprintf("%s signature from %s key %s", vr.Result, vr.Cipher, gpg.KeyID(vr.KeyID))
			}
		} else {
			signatureInfo = "Revision is not signed."
		}
	}

	metadata = &v1alpha1.RevisionMetadata{Author: m.Author, Date: metav1.Time{Time: m.Date}, Tags: m.Tags, Message: m.Message, SignatureInfo: signatureInfo}
	_ = s.cache.SetRevisionMetadata(q.Repo.Repo, q.Revision, metadata)
	return metadata, nil
}

func fileParameters(q *apiclient.RepoServerAppDetailsQuery) []v1alpha1.HelmFileParameter {
	if q.Source.Helm == nil {
		return nil
	}
	return q.Source.Helm.FileParameters
}

func (s *Service) newClient(repo *v1alpha1.Repository, opts ...git.ClientOpts) (git.Client, error) {
	repoPath, err := s.gitRepoPaths.GetPath(git.NormalizeGitURL(repo.Repo))
	if err != nil {
		return nil, err
	}
	opts = append(opts, git.WithEventHandlers(metrics.NewGitClientEventHandlers(s.metricsServer)))
	return s.newGitClient(repo.Repo, repoPath, repo.GetGitCreds(s.gitCredsStore), repo.IsInsecure(), repo.EnableLFS, repo.Proxy, opts...)
}

// newClientResolveRevision is a helper to perform the common task of instantiating a git client
// and resolving a revision to a commit SHA
func (s *Service) newClientResolveRevision(repo *v1alpha1.Repository, revision string, opts ...git.ClientOpts) (git.Client, string, error) {
	gitClient, err := s.newClient(repo, opts...)
	if err != nil {
		return nil, "", err
	}
	commitSHA, err := gitClient.LsRemote(revision)
	if err != nil {
		return nil, "", err
	}
	return gitClient, commitSHA, nil
}

func (s *Service) newHelmClientResolveRevision(repo *v1alpha1.Repository, revision string, chart string, noRevisionCache bool) (helm.Client, string, error) {
	enableOCI := repo.EnableOCI || helm.IsHelmOciRepo(repo.Repo)
	helmClient := s.newHelmClient(repo.Repo, repo.GetHelmCreds(), enableOCI, repo.Proxy, helm.WithIndexCache(s.cache), helm.WithChartPaths(s.chartPaths))
	// OCI helm registers don't support semver ranges. Assuming that given revision is exact version
	if helm.IsVersion(revision) || enableOCI {
		return helmClient, revision, nil
	}
	constraints, err := semver.NewConstraint(revision)
	if err != nil {
		return nil, "", fmt.Errorf("invalid revision '%s': %v", revision, err)
	}
	index, err := helmClient.GetIndex(noRevisionCache)
	if err != nil {
		return nil, "", err
	}
	entries, err := index.GetEntries(chart)
	if err != nil {
		return nil, "", err
	}
	version, err := entries.MaxVersion(constraints)
	if err != nil {
		return nil, "", err
	}
	return helmClient, version.String(), nil
}

// directoryPermissionInitializer ensures the directory has read/write/execute permissions and returns
// a function that can be used to remove all permissions.
func directoryPermissionInitializer(rootPath string) goio.Closer {
	if _, err := os.Stat(rootPath); err == nil {
		if err := os.Chmod(rootPath, 0700); err != nil {
			log.Warnf("Failed to restore read/write/execute permissions on %s: %v", rootPath, err)
		} else {
			log.Debugf("Successfully restored read/write/execute permissions on %s", rootPath)
		}
	}

	return io.NewCloser(func() error {
		if err := os.Chmod(rootPath, 0000); err != nil {
			log.Warnf("Failed to remove permissions on %s: %v", rootPath, err)
		} else {
			log.Debugf("Successfully removed permissions on %s", rootPath)
		}
		return nil
	})
}

// checkoutRevision is a convenience function to initialize a repo, fetch, and checkout a revision
// Returns the 40 character commit SHA after the checkout has been performed
// nolint:unparam
func (s *Service) checkoutRevision(gitClient git.Client, revision string, submoduleEnabled bool) (goio.Closer, error) {
	closer := s.gitRepoInitializer(gitClient.Root())
	return closer, checkoutRevision(gitClient, revision, submoduleEnabled)
}

func checkoutRevision(gitClient git.Client, revision string, submoduleEnabled bool) error {
	err := gitClient.Init()
	if err != nil {
		return status.Errorf(codes.Internal, "Failed to initialize git repo: %v", err)
	}

	// Fetching with no revision first. Fetching with an explicit version can cause repo bloat. https://github.com/argoproj/argo-cd/issues/8845
	err = gitClient.Fetch("")
	if err != nil {
		return status.Errorf(codes.Internal, "Failed to fetch default: %v", err)
	}

	err = gitClient.Checkout(revision, submoduleEnabled)
	if err != nil {
		// When fetching with no revision, only refs/heads/* and refs/remotes/origin/* are fetched. If checkout fails
		// for the given revision, try explicitly fetching it.
		log.Infof("Failed to checkout revision %s: %v", revision, err)
		log.Infof("Fallback to fetching specific revision %s. ref might not have been in the default refspec fetched.", revision)

		err = gitClient.Fetch(revision)
		if err != nil {
			return status.Errorf(codes.Internal, "Failed to checkout revision %s: %v", revision, err)
		}

		err = gitClient.Checkout("FETCH_HEAD", submoduleEnabled)
		if err != nil {
			return status.Errorf(codes.Internal, "Failed to checkout FETCH_HEAD: %v", err)
		}
	}

	return err
}

func (s *Service) GetHelmCharts(ctx context.Context, q *apiclient.HelmChartsRequest) (*apiclient.HelmChartsResponse, error) {
	index, err := s.newHelmClient(q.Repo.Repo, q.Repo.GetHelmCreds(), q.Repo.EnableOCI, q.Repo.Proxy, helm.WithChartPaths(s.chartPaths)).GetIndex(true)
	if err != nil {
		return nil, err
	}
	res := apiclient.HelmChartsResponse{}
	for chartName, entries := range index.Entries {
		chart := apiclient.HelmChart{
			Name: chartName,
		}
		for _, entry := range entries {
			chart.Versions = append(chart.Versions, entry.Version)
		}
		res.Items = append(res.Items, &chart)
	}
	return &res, nil
}

func (s *Service) TestRepository(ctx context.Context, q *apiclient.TestRepositoryRequest) (*apiclient.TestRepositoryResponse, error) {
	repo := q.Repo
	// per Type doc, "git" should be assumed if empty or absent
	if repo.Type == "" {
		repo.Type = "git"
	}
	checks := map[string]func() error{
		"git": func() error {
			return git.TestRepo(repo.Repo, repo.GetGitCreds(s.gitCredsStore), repo.IsInsecure(), repo.IsLFSEnabled(), repo.Proxy)
		},
		"helm": func() error {
			if repo.EnableOCI {
				if !helm.IsHelmOciRepo(repo.Repo) {
					return errors.New("OCI Helm repository URL should include hostname and port only")
				}
				_, err := helm.NewClient(repo.Repo, repo.GetHelmCreds(), repo.EnableOCI, repo.Proxy).TestHelmOCI()
				return err
			} else {
				_, err := helm.NewClient(repo.Repo, repo.GetHelmCreds(), repo.EnableOCI, repo.Proxy).GetIndex(false)
				return err
			}
		},
	}
	check := checks[repo.Type]
	apiResp := &apiclient.TestRepositoryResponse{VerifiedRepository: false}
	err := check()
	if err != nil {
		return apiResp, fmt.Errorf("error testing repository connectivity: %w", err)
	}
	return apiResp, nil
}

// ResolveRevision resolves the revision/ambiguousRevision specified in the ResolveRevisionRequest request into a concrete revision.
func (s *Service) ResolveRevision(ctx context.Context, q *apiclient.ResolveRevisionRequest) (*apiclient.ResolveRevisionResponse, error) {

	repo := q.Repo
	app := q.App
	ambiguousRevision := q.AmbiguousRevision
	var revision string
	if app.Spec.Source.IsHelm() {

		if helm.IsVersion(ambiguousRevision) {
			return &apiclient.ResolveRevisionResponse{Revision: ambiguousRevision, AmbiguousRevision: ambiguousRevision}, nil
		}
		client := helm.NewClient(repo.Repo, repo.GetHelmCreds(), repo.EnableOCI || app.Spec.Source.IsHelmOci(), repo.Proxy, helm.WithChartPaths(s.chartPaths))
		index, err := client.GetIndex(false)
		if err != nil {
			return &apiclient.ResolveRevisionResponse{Revision: "", AmbiguousRevision: ""}, err
		}
		entries, err := index.GetEntries(app.Spec.Source.Chart)
		if err != nil {
			return &apiclient.ResolveRevisionResponse{Revision: "", AmbiguousRevision: ""}, err
		}
		constraints, err := semver.NewConstraint(ambiguousRevision)
		if err != nil {
			return &apiclient.ResolveRevisionResponse{Revision: "", AmbiguousRevision: ""}, err
		}
		version, err := entries.MaxVersion(constraints)
		if err != nil {
			return &apiclient.ResolveRevisionResponse{Revision: "", AmbiguousRevision: ""}, err
		}
		return &apiclient.ResolveRevisionResponse{
			Revision:          version.String(),
			AmbiguousRevision: fmt.Sprintf("%v (%v)", ambiguousRevision, version.String()),
		}, nil
	} else {
		gitClient, err := git.NewClient(repo.Repo, repo.GetGitCreds(s.gitCredsStore), repo.IsInsecure(), repo.IsLFSEnabled(), repo.Proxy)
		if err != nil {
			return &apiclient.ResolveRevisionResponse{Revision: "", AmbiguousRevision: ""}, err
		}
		revision, err = gitClient.LsRemote(ambiguousRevision)
		if err != nil {
			return &apiclient.ResolveRevisionResponse{Revision: "", AmbiguousRevision: ""}, err
		}
		return &apiclient.ResolveRevisionResponse{
			Revision:          revision,
			AmbiguousRevision: fmt.Sprintf("%s (%s)", ambiguousRevision, revision),
		}, nil
	}
}
