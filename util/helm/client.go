package helm

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	executil "github.com/argoproj/argo-cd/v3/util/exec"

	"github.com/argoproj/pkg/v2/sync"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/util/cache"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/io/files"
	"github.com/argoproj/argo-cd/v3/util/proxy"
)

var (
	globalLock = sync.NewKeyLock()

	ErrOCINotEnabled = errors.New("could not perform the action when oci is not enabled")
)

// userAgentTransport wraps an http.RoundTripper to add User-Agent header to all requests
type userAgentTransport struct {
	Transport http.RoundTripper
	UserAgent string
}

// RoundTrip implements the http.RoundTripper interface
func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", t.UserAgent)
	return t.Transport.RoundTrip(req)
}

type indexCache interface {
	SetHelmIndex(repo string, indexData []byte) error
	GetHelmIndex(repo string, indexData *[]byte) error
}

// OCI tags retain the revision-cache policy rather than HTTP index freshness.
type ociTagsCache interface {
	SetOCITags(repo string, data []byte) error
	GetOCITags(repo string, data *[]byte) error
}

// WithIndexCacheScope restricts the index-cache keys used by the client to
// one credential view: the cache key becomes "<repoURL>\x00<scope>" instead
// of the bare repository URL. The scope is a versioned fingerprint of the
// effective authentication/TLS context, so a fetch made with private
// credentials can never publish index metadata that an anonymous (or
// differently-credentialed) reader would consume — the two views see
// disjoint cache keys. Without this option the bare URL is used, preserving
// the legacy shared-key behavior.
func WithIndexCacheScope(scope string) ClientOpts {
	return func(c *nativeHelmChart) {
		c.indexCacheScope = scope
	}
}

// WithIndexCacheScopedByCreds attaches the index cache and automatically
// scopes its keys to the client's credential view (see WithIndexCacheScope
// and credsScope). All in-repo call sites should prefer this over a bare
// WithIndexCache so private-credentialed fetches can never share cache
// entries with anonymous readers.
func WithIndexCacheScopedByCreds(indexCache indexCache) ClientOpts {
	return func(c *nativeHelmChart) {
		c.indexCache = indexCache
		c.scopedByCreds = true
	}
}

type Client interface {
	CleanChartCache(chart string, version string) error
	ExtractChart(ctx context.Context, chart string, version string, passCredentials bool, manifestMaxExtractedSize int64, disableManifestMaxExtractedSize bool) (string, utilio.Closer, error)
	GetIndex(ctx context.Context, noCache bool, maxIndexSize int64) (*Index, error)
	GetTags(ctx context.Context, chart string, noCache bool) ([]string, error)
	TestHelmOCI(ctx context.Context) (bool, error)
}

type ClientOpts func(c *nativeHelmChart)

func WithIndexCache(indexCache indexCache) ClientOpts {
	return func(c *nativeHelmChart) {
		c.indexCache = indexCache
	}
}

func WithChartPaths(chartPaths utilio.TempPaths) ClientOpts {
	return func(c *nativeHelmChart) {
		c.chartCachePaths = chartPaths
	}
}

// WithUserAgent sets a custom User-Agent string for HTTP requests.
// If not set, a default User-Agent will be generated automatically.
func WithUserAgent(userAgent string) ClientOpts {
	return func(c *nativeHelmChart) {
		c.customUserAgent = userAgent
	}
}

// WithPlainHTTP configures the client to use plain HTTP instead of HTTPS for OCI registry operations.
func WithPlainHTTP() ClientOpts {
	return func(c *nativeHelmChart) {
		c.plainHTTP = true
	}
}

func NewClient(repoURL string, creds Creds, enableOci bool, proxy string, noProxy string, opts ...ClientOpts) Client {
	return NewClientWithLock(repoURL, creds, globalLock, enableOci, proxy, noProxy, opts...)
}

func NewClientWithLock(repoURL string, creds Creds, repoLock sync.KeyLock, enableOci bool, proxy string, noProxy string, opts ...ClientOpts) Client {
	c := &nativeHelmChart{
		repoURL:         repoURL,
		creds:           creds,
		repoLock:        repoLock,
		enableOci:       enableOci,
		proxy:           proxy,
		noProxy:         noProxy,
		chartCachePaths: utilio.NewRandomizedTempPaths(os.TempDir()),
	}
	for i := range opts {
		opts[i](c)
	}
	return c
}

// WithHelmChartCacheExpiration sets the cache expiration duration for chart cache.
func WithHelmChartCacheExpiration(helmChartCacheExpiration time.Duration) ClientOpts {
	return func(c *nativeHelmChart) {
		c.helmChartCacheExpiration = helmChartCacheExpiration
	}
}

var _ Client = &nativeHelmChart{}

type nativeHelmChart struct {
	indexNotModified         bool
	cacheObservation         indexRecordObservation
	cacheReadAttempted       bool
	responseURLIdentity      string
	lastValidators           http.Header // response validators consumed by the publication owner
	chartCachePaths          utilio.TempPaths
	repoURL                  string
	requireCompleteIndex     bool
	indexCacheScope          string // credential-view fingerprint scoping index-cache keys (empty = legacy bare-URL keys)
	scopedByCreds            bool   // derive the cache scope from each operation's resolved credentials
	creds                    Creds
	repoLock                 sync.KeyLock
	enableOci                bool
	plainHTTP                bool
	indexCache               indexCache
	proxy                    string
	noProxy                  string
	customUserAgent          string        // Custom User-Agent string (optional)
	helmChartCacheExpiration time.Duration // Cache expiration for chart cache
}

// getUserAgent returns the User-Agent string to use for HTTP requests.
// If a custom User-Agent is set, it will be used; otherwise, a default will be generated.
func (c *nativeHelmChart) getUserAgent() string {
	if c.customUserAgent != "" {
		return c.customUserAgent
	}

	// Default User-Agent with version and platform info
	version := common.GetVersion()
	return fmt.Sprintf("argocd-repo-server/%s (%s)", version.Version, version.Platform)
}

// withResolvedCredentials copies the client for one operation. Resolve before
// any cache read, and use the same immutable credentials for authentication,
// TLS, and every cache read/write. Keep the original provider on the reusable
// client so the next operation can observe rotation or a lookup failure.
func (c *nativeHelmChart) withResolvedCredentials(ctx context.Context) (*nativeHelmChart, error) {
	snap, err := snapshotCreds(c.creds)
	if err != nil {
		return nil, err
	}
	operation := *c
	operation.creds = snap
	operation.lastValidators = nil
	operation.cacheReadAttempted = false
	operation.cacheObservation = indexRecordObservation{}
	if _, paired := operation.indexCache.(indexCacheRecordReader); paired {
		operation.requireCompleteIndex = true
	}
	if operation.scopedByCreds {
		operation.indexCacheScope, err = credsScopeContext(ctx, snap)
		if err != nil {
			return nil, err
		}
	}
	return &operation, nil
}

// cacheRepoKey returns the cache key for an operation's resolved credential
// view, or the explicit/legacy scope for clients without automatic scoping.
func (c *nativeHelmChart) cacheRepoKey(ctx context.Context) (string, error) {
	return c.cacheRepoKeyScoped(ctx, c.indexCacheScope)
}

// cacheRepoKeyScoped returns the repo cache key under an explicit credential
// scope. An empty scope preserves the legacy bare-URL key.
func (c *nativeHelmChart) cacheRepoKeyScoped(ctx context.Context, scope string) (string, error) {
	if scope == "" {
		return c.repoURL, nil
	}
	return scopedIndexIdentityContext(ctx, c.repoURL, scope, c.proxy, c.noProxy)
}

// cacheTagsKey returns the cache key for an OCI tags URL under this client's
// credential scope (tags share the index cache's key space).
func (c *nativeHelmChart) cacheTagsKey(ctx context.Context, tagsURL string) (string, error) {
	if c.indexCacheScope == "" {
		return "helm-oci-tags-v1|" + tagsURL, nil
	}
	scheme := "https"
	if c.plainHTTP {
		scheme = "http"
	}
	key, err := scopedIndexIdentityContext(ctx, tagsURL, c.indexCacheScope+"\x00"+scheme, c.proxy, c.noProxy)
	return "helm-oci-tags-v1|" + key, err
}

func fileExist(filePath string) (bool, error) {
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("error checking file existence for %s: %w", filePath, err)
	}
	return true, nil
}

func (c *nativeHelmChart) CleanChartCache(chart string, version string) error {
	cachePath, err := c.getCachedChartPath(chart, version)
	if err != nil {
		return fmt.Errorf("error getting cached chart path: %w", err)
	}

	// ExtractChart reads and extracts the cached archive while holding this key lock.
	// Acquire it here as well so a hard refresh cannot delete the archive mid-extraction.
	c.repoLock.Lock(cachePath)
	defer c.repoLock.Unlock(cachePath)

	if err := os.RemoveAll(cachePath); err != nil {
		return fmt.Errorf("error removing chart cache at %s: %w", cachePath, err)
	}
	return nil
}

func untarChart(ctx context.Context, tempDir string, cachedChartPath string, manifestMaxExtractedSize int64, disableManifestMaxExtractedSize bool) error {
	if disableManifestMaxExtractedSize {
		cmd := exec.CommandContext(ctx, "tar", "-zxvf", cachedChartPath)
		cmd.Dir = tempDir
		_, err := executil.Run(cmd)
		if err != nil {
			return fmt.Errorf("error executing tar command: %w", err)
		}
		return nil
	}
	reader, err := os.Open(cachedChartPath)
	if err != nil {
		return fmt.Errorf("error opening cached chart path %s: %w", cachedChartPath, err)
	}
	return files.Untgz(tempDir, reader, manifestMaxExtractedSize, false)
}

func (c *nativeHelmChart) ExtractChart(ctx context.Context, chart string, version string, passCredentials bool, manifestMaxExtractedSize int64, disableManifestMaxExtractedSize bool) (string, utilio.Closer, error) {
	// always use Helm V3 since we don't have chart content to determine correct Helm version
	helmCmd, err := NewCmdWithVersion("", c.enableOci, c.proxy, c.noProxy)
	if err != nil {
		return "", nil, fmt.Errorf("error creating Helm command: %w", err)
	}
	defer helmCmd.Close()

	// throw away temp directory that stores extracted chart and should be deleted as soon as no longer needed by returned closer
	tempDir, err := files.CreateTempDir(os.TempDir())
	if err != nil {
		return "", nil, fmt.Errorf("error creating temporary directory: %w", err)
	}

	cachedChartPath, err := c.getCachedChartPath(chart, version)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", nil, fmt.Errorf("error getting cached chart path: %w", err)
	}

	c.repoLock.Lock(cachedChartPath)
	defer c.repoLock.Unlock(cachedChartPath)

	// check if chart tar is already downloaded
	exists, err := fileExist(cachedChartPath)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", nil, fmt.Errorf("error checking existence of cached chart path: %w", err)
	}

	// if chart tar exists, check if it is expired based on cache expiration duration configuration.
	if exists && c.helmChartCacheExpiration > 0 {
		info, err := os.Stat(cachedChartPath)
		if err != nil {
			log.WithFields(log.Fields{
				"path": cachedChartPath,
				"err":  err,
			}).Warn("error checking cached Helm chart; removing cached archive")
			_ = os.RemoveAll(cachedChartPath)
			exists = false
		} else if time.Since(info.ModTime()) > c.helmChartCacheExpiration {
			if err := os.RemoveAll(cachedChartPath); err != nil {
				_ = os.RemoveAll(tempDir)
				return "", nil, fmt.Errorf(
					"error removing expired cached chart: %w",
					err,
				)
			}
			exists = false
		}
	}

	if !exists {
		// create empty temp directory to extract chart from the registry
		tempDest, err := files.CreateTempDir(os.TempDir())
		if err != nil {
			_ = os.RemoveAll(tempDir)
			return "", nil, fmt.Errorf("error creating temporary destination directory: %w", err)
		}
		defer func() { _ = os.RemoveAll(tempDest) }()

		if c.enableOci {
			helmPassword, err := c.creds.GetPassword()
			if err != nil {
				return "", nil, fmt.Errorf("failed to get password for helm registry: %w", err)
			}
			if helmPassword != "" && c.creds.GetUsername() != "" {
				_, err = helmCmd.RegistryLogin(ctx, c.repoURL, c.creds, c.plainHTTP)
				if err != nil {
					_ = os.RemoveAll(tempDir)
					return "", nil, fmt.Errorf("error logging into OCI registry: %w", err)
				}

				defer func() {
					_, _ = helmCmd.RegistryLogout(c.repoURL, c.creds)
				}()
			}

			// 'helm pull' ensures that chart is downloaded into temp directory
			_, err = helmCmd.PullOCI(c.repoURL, chart, version, tempDest, c.creds, c.plainHTTP)
			if err != nil {
				_ = os.RemoveAll(tempDir)
				return "", nil, fmt.Errorf("error pulling OCI chart: %w", err)
			}
		} else {
			_, err = helmCmd.Fetch(c.repoURL, chart, version, tempDest, c.creds, passCredentials)
			if err != nil {
				_ = os.RemoveAll(tempDir)
				return "", nil, fmt.Errorf("error fetching chart: %w", err)
			}
		}

		// 'helm pull/fetch' file downloads chart into the tgz file and we move that to where we want it
		infos, err := os.ReadDir(tempDest)
		if err != nil {
			return "", nil, fmt.Errorf("error reading directory %s: %w", tempDest, err)
		}
		if len(infos) != 1 {
			return "", nil, fmt.Errorf("expected 1 file, found %v", len(infos))
		}

		chartFilePath := filepath.Join(tempDest, infos[0].Name())

		err = os.Rename(chartFilePath, cachedChartPath)
		if err != nil {
			return "", nil, fmt.Errorf("error renaming file from %s to %s: %w", chartFilePath, cachedChartPath, err)
		}
	}

	err = untarChart(ctx, tempDir, cachedChartPath, manifestMaxExtractedSize, disableManifestMaxExtractedSize)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", nil, fmt.Errorf("error untarring chart: %w", err)
	}
	return path.Join(tempDir, normalizeChartName(chart)), utilio.NewCloser(func() error {
		return os.RemoveAll(tempDir)
	}), nil
}

func (c *nativeHelmChart) GetIndex(ctx context.Context, noCache bool, maxIndexSize int64) (*Index, error) {
	operation, err := c.withResolvedCredentials(ctx)
	if err != nil {
		return nil, err
	}
	c = operation

	repoKey, err := c.cacheRepoKey(ctx)
	if err != nil {
		return nil, err
	}
	unlock, err := indexClientLocks.acquire(ctx, "index\x00"+repoKey)
	if err != nil {
		return nil, err
	}
	defer unlock()

	var data []byte
	if !noCache && c.indexCache != nil {
		// The per-key lock above must not pin this request on a slow cache:
		// the raw CacheClient interface has no ctx-aware Get, so abandon the
		// read past the shared read budget (the same policy as retained
		// reads). The worker owns its own byte slice — a late cache result
		// must never overwrite the origin body in `data`.
		readContext, cancelRead := observeIndexReadContext(ctx)
		type cacheRead struct {
			body []byte
			err  error
		}
		readDone := make(chan cacheRead, 1)
		go func() {
			var cached []byte
			err := c.indexCache.GetHelmIndex(repoKey, &cached)
			// Capture the result only after the read completes: evaluating
			// body: cached inside the composite literal would race the same
			// call that mutates cached, and a successful read could publish
			// a stale nil body.
			readDone <- cacheRead{body: cached, err: err}
		}()
		var readResult cacheRead
		select {
		case readResult = <-readDone:
		case <-readContext.Done():
			log.WithError(readContext.Err()).Debug("abandoned slow helm index cache read")
		}
		cancelRead()
		if readResult.err != nil {
			data = nil
		} else if readResult.body != nil {
			data = readResult.body
		}
		if data != nil {
			var identity string
			reader, isReader := c.indexCache.(indexCacheRecordReader)
			if isReader {
				c.cacheObservation = observeIndexRecord(ctx, reader, repoKey)
				c.cacheReadAttempted = true
				if record := c.cacheObservation.record; record != nil && bytes.Equal(data, record.Body) {
					identity = record.URLIdentity
				}
			} else {
				_, _, identity = sharedInMemoryValidators.getForResponse(repoKey, data)
			}
			// A body-only cache cannot bind the hit to a request profile, so
			// it can never authorize a conditional GET. Unscoped deployments
			// keep master's behavior — serve the warm body as-is (it was
			// validated by the writer at fetch time); scoped deployments
			// treat an unattributable body as a miss, since credential
			// isolation is the very thing the identity check protects.
			if !c.scopedByCreds && !isReader {
				// serve regardless of identity
			} else if !c.matchesIndexRequest(ctx, identity) {
				data = nil
			}
		}
	}

	if len(data) > 0 && (int64(len(data)) > maxIndexSize || (c.requireCompleteIndex || c.scopedByCreds) && validateIndexBody(data) != nil) {
		data = nil
	}

	index := &Index{}
	if len(data) > 0 {
		if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(index); err != nil {
			data = nil
		}
	}
	if len(data) == 0 {
		start := time.Now()
		data, err = c.loadRepoIndex(ctx, maxIndexSize)
		if err != nil {
			return nil, fmt.Errorf("error loading repo index: %w", err)
		}
		index = &Index{}
		if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(index); err != nil {
			return nil, fmt.Errorf("error decoding index: %w", err)
		}
		log.WithFields(log.Fields{"seconds": time.Since(start).Seconds()}).Info("took to get index")

		if c.indexCache != nil {
			recorder, isRecorder := c.indexCache.(indexCacheRecorder)
			retainedWriter, isRetainedWriter := c.indexCache.(indexCacheRetainedWriter)
			_, isReader := c.indexCache.(indexCacheRecordReader)
			_, isValidators := c.indexCache.(indexCacheWithValidators)
			// The record API publishes body+validators in one atomic record —
			// the writer path for caches implementing the record contract.
			// The retained writer carries the fetch's URL identity, so later
			// revalidations can re-send conditional headers for this record.
			// Only legacy raw-body caches retain their original setter
			// contract. A 304 keeps the previous generation: the record is
			// still current, never republished.
			switch {
			case isRetainedWriter && !c.indexNotModified:
				if err := retainedWriter.SetHelmIndexRetainedRecord(repoKey, c.fetchedIndexRecord(data)); err != nil {
					log.WithError(err).Debug("failed to publish retained helm index record")
				}
			case isRecorder && !c.indexNotModified:
				etag, modified := responseValidatorHeaders(c.lastValidators)
				if err := recorder.SetHelmIndexRecord(repoKey, data, etag, modified); err != nil {
					log.WithError(err).Debug("failed to publish helm index record")
				}
			case !isReader && !isRecorder && !isValidators && !c.indexNotModified:
				if err := c.indexCache.SetHelmIndex(repoKey, data); err != nil {
					log.WithError(err).Debug("failed to publish legacy helm index body")
				}
			}
			if !isReader {
				etag, modified := responseValidatorHeaders(c.lastValidators)
				// Bind fallback validators only when the cache actually holds these bytes.
				var cached []byte
				if err := c.indexCache.GetHelmIndex(repoKey, &cached); err == nil && bytes.Equal(cached, data) {
					sharedInMemoryValidators.setForResponse(repoKey, data, etag, modified, c.responseURLIdentity)
				}
			}
		}
	}

	return index, nil
}

func (c *nativeHelmChart) fetchedIndexRecord(data []byte) *HelmIndexRecord {
	etag, modified := responseValidatorHeaders(c.lastValidators)
	return &HelmIndexRecord{Body: data, ETag: etag, LastModified: modified, URLIdentity: c.responseURLIdentity, StoredAt: time.Now().UTC().Format(time.RFC3339Nano)}
}

func (c *nativeHelmChart) TestHelmOCI(ctx context.Context) (bool, error) {
	start := time.Now()

	tmpDir, err := os.MkdirTemp("", "helm")
	if err != nil {
		return false, fmt.Errorf("error creating temporary directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	helmCmd, err := NewCmdWithVersion(tmpDir, c.enableOci, c.proxy, c.noProxy)
	if err != nil {
		return false, fmt.Errorf("error creating Helm command: %w", err)
	}
	defer helmCmd.Close()

	// Looks like there is no good way to test access to OCI repo if credentials are not provided
	// just assume it is accessible
	helmPassword, err := c.creds.GetPassword()
	if err != nil {
		return false, fmt.Errorf("failed to get password for helm registry: %w", err)
	}
	if c.creds.GetUsername() != "" && helmPassword != "" {
		_, err = helmCmd.RegistryLogin(ctx, c.repoURL, c.creds, c.plainHTTP)
		if err != nil {
			return false, fmt.Errorf("error logging into OCI registry: %w", err)
		}
		defer func() {
			_, _ = helmCmd.RegistryLogout(c.repoURL, c.creds)
		}()

		log.WithFields(log.Fields{"seconds": time.Since(start).Seconds()}).Info("took to test helm oci repository")
	}
	return true, nil
}

func (c *nativeHelmChart) loadRepoIndex(ctx context.Context, maxIndexSize int64) ([]byte, error) {
	c.indexNotModified = false
	if !c.cacheReadAttempted {
		c.cacheObservation = indexRecordObservation{}
	}
	c.responseURLIdentity = ""
	c.lastValidators = nil
	indexURL, err := getIndexURL(c.repoURL)
	if err != nil {
		return nil, fmt.Errorf("error getting index URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %w", err)
	}

	// Set User-Agent header to comply with robot policies
	req.Header.Set("User-Agent", c.getUserAgent())
	// Every fetch here requires origin validation. Without this directive a
	// proxy could satisfy even a forced, unconditional GET from its own cache.
	req.Header.Set("Cache-Control", "no-cache")

	if c.creds.GetUsername() != "" || func() string { p, _ := c.creds.GetPassword(); return p }() != "" {
		// only basic supported
		req.SetBasicAuth(c.creds.GetUsername(), func() string { p, _ := c.creds.GetPassword(); return p }())
	}

	// Conditional GET: when validators (ETag / Last-Modified) from the
	// previous successful fetch are known, revalidate instead of re-downloading.
	// A 304 response lets us keep the cached body; validation failures remain
	// errors. Cache keys use the scope of THIS operation's snapshot: the
	// request authenticates with the snapshot's token, so it must read and
	// publish under the snapshot's identity.
	repoKey, err := c.cacheRepoKey(ctx)
	if err != nil {
		return nil, err
	}
	var cachedBody []byte
	var cachedRecordETag, cachedRecordLastModified, cachedURLIdentity string
	var retainedRecord, atomicReader bool
	if c.indexCache != nil {
		// Read the COMPLETE record in one call when the cache supports it:
		// reading body and validators separately lets a concurrent replica
		// publication slip a new validator in between, so a 304 would pair
		// the new validator with the previous body.
		if reader, ok := c.indexCache.(indexCacheRecordReader); ok {
			atomicReader = true
			if !c.cacheReadAttempted {
				c.cacheObservation = observeIndexRecord(ctx, reader, repoKey)
			}
			if rec := c.cacheObservation.record; rec != nil && len(rec.Body) > 0 {
				cachedBody = rec.Body
				cachedRecordETag, cachedRecordLastModified, cachedURLIdentity = rec.ETag, rec.LastModified, rec.URLIdentity
				retainedRecord = true
			}
		}
		if !retainedRecord && !atomicReader {
			if err := c.indexCache.GetHelmIndex(repoKey, &cachedBody); err != nil {
				cachedBody = nil
			}
			cachedBody = bytes.Clone(cachedBody)
		}
		if len(cachedBody) > 0 && (int64(len(cachedBody)) > maxIndexSize || (c.requireCompleteIndex || c.scopedByCreds || atomicReader) && validateIndexBody(cachedBody) != nil) {
			cachedBody = nil
			retainedRecord = false
			cachedRecordETag, cachedRecordLastModified = "", ""
			cachedURLIdentity = ""
		}

		if len(cachedBody) > 0 {
			etag, lastModified := cachedRecordETag, cachedRecordLastModified
			if !retainedRecord {
				// Split body/validator reads cannot establish a generation.
				// Reuse only validators bound to these exact bytes by a
				// successful publication in this process.
				etag, lastModified, cachedURLIdentity = sharedInMemoryValidators.getForResponse(repoKey, cachedBody)
			}
			cachedRecordETag, cachedRecordLastModified = etag, lastModified
		}
	}

	tlsConf, err := newTLSConfig(c.creds)
	if err != nil {
		return nil, fmt.Errorf("error creating TLS config: %w", err)
	}

	tr := &http.Transport{
		Proxy:             proxy.GetCallback(c.proxy, c.noProxy),
		TLSClientConfig:   tlsConf,
		DisableKeepAlives: true,
		ForceAttemptHTTP2: true,
	}
	allowConditional := true
	applyCondition := func(request *http.Request) error {
		request.Header.Del("If-None-Match")
		request.Header.Del("If-Modified-Since")
		if allowConditional && len(cachedBody) > 0 && cachedURLIdentity != "" {
			identity, err := c.indexURLIdentityContext(request.Context(), request.URL.String())
			if err != nil {
				return err
			}
			if identity == cachedURLIdentity {
				applyConditionalHeaders(request, cachedRecordETag, cachedRecordLastModified)
			}
		}
		return nil
	}
	if err := applyCondition(req); err != nil {
		return nil, err
	}
	cacheableVariant := true
	client := http.Client{Transport: tr, CheckRedirect: func(request *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if request.Response != nil && !c.supportsIndexVary(request.Response.Header) {
			cacheableVariant = false
		}
		return applyCondition(request)
	}}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making HTTP request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	needURLIdentity := c.requireCompleteIndex || c.indexCache != nil
	if needURLIdentity {
		c.responseURLIdentity, err = c.indexURLIdentityContext(ctx, resp.Request.URL.String())
		if err != nil {
			return nil, err
		}
		if !cacheableVariant || !c.supportsIndexVary(resp.Header) {
			c.responseURLIdentity = "unmatched-vary"
		}
	}
	sentValidator := resp.Request.Header.Get("If-None-Match") != "" || resp.Request.Header.Get("If-Modified-Since") != ""
	if resp.StatusCode == http.StatusNotModified && len(cachedBody) > 0 && sentValidator && c.responseURLIdentity == cachedURLIdentity && matchingNotModifiedETag(cachedRecordETag, singleResponseValidator(resp.Header, "ETag")) {
		c.indexNotModified = true
		log.Info("helm index not modified (HTTP 304), using cached body")
		// Keep the request's validators when a 304 omits them. The owner can
		// renew this generation atomically after the HTTP read completes.
		if c.lastValidators == nil {
			c.lastValidators = http.Header{}
		}
		if cachedRecordETag != "" {
			c.lastValidators.Set("ETag", cachedRecordETag)
		}
		if cachedRecordLastModified != "" {
			c.lastValidators.Set("Last-Modified", cachedRecordLastModified)
		}
		for _, name := range []string{"ETag", "Last-Modified"} {
			if value := singleResponseValidator(resp.Header, name); value != "" {
				c.lastValidators.Set(name, value)
			}
		}

		// The fetch owner publishes the validated snapshot once.
		return cachedBody, nil
	}
	if resp.StatusCode == http.StatusNotModified {
		// A 304 requires both a usable body and a condition actually sent.
		log.Debug("unusable helm index 304, retrying without validators")
		allowConditional = false
		clearValidators := req.Clone(ctx)
		clearValidators.Header.Del("If-None-Match")
		clearValidators.Header.Del("If-Modified-Since")
		resp2, err := client.Do(clearValidators)
		if err != nil {
			return nil, fmt.Errorf("error making HTTP request: %w", err)
		}
		defer func() { _ = resp2.Body.Close() }()
		if resp2.StatusCode == http.StatusOK {
			data, err := c.readIndexResponse(resp2.Body, maxIndexSize)
			if err != nil {
				return nil, fmt.Errorf("error reading index response body: %w", err)
			}
			c.lastValidators = resp2.Header
			if needURLIdentity {
				c.responseURLIdentity, err = c.indexURLIdentityContext(ctx, resp2.Request.URL.String())
				if err != nil {
					return nil, err
				}
				if !cacheableVariant || !c.supportsIndexVary(resp2.Header) {
					c.responseURLIdentity = "unmatched-vary"
				}
			}
			return data, nil
		}
		return nil, errors.New("failed to get index: " + resp2.Status)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to get index: " + resp.Status)
	}

	data, err := c.readIndexResponse(resp.Body, maxIndexSize)
	if err != nil {
		return nil, fmt.Errorf("error reading index response body: %w", err)
	}
	c.lastValidators = resp.Header
	// Validators are published by the CALLER together with the body: storing
	// them here (before the body is in the cache) would let a later 304 pair
	// a new validator with the previous body.
	return data, nil
}

// Cache records and Helm snapshots must contain complete original responses.
// Keep the legacy uncached parser's size-limited behavior for existing callers.
func (c *nativeHelmChart) readIndexResponse(body io.Reader, maximum int64) ([]byte, error) {
	strict := c.requireCompleteIndex || c.indexCache != nil
	limit := maximum
	if strict && maximum < 1<<63-1 {
		limit++
	}
	data, err := io.ReadAll(io.LimitReader(body, limit))
	if err != nil {
		return nil, err
	}
	if strict && int64(len(data)) > maximum {
		return nil, fmt.Errorf("helm repository index exceeds maximum size of %d bytes", maximum)
	}
	if c.requireCompleteIndex || c.scopedByCreds {
		if err := validateIndexBody(data); err != nil {
			return nil, err
		}
	}
	return data, nil
}

func newTLSConfig(creds Creds) (*tls.Config, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: creds.GetInsecureSkipVerify()}

	// Prefer CA bytes already resolved on a credential snapshot: the cache
	// fingerprint captured those exact bytes, so TLS must verify against
	// them — rereading the path could rotate the CA under the cached
	// identity.
	var caData []byte
	if caCreds, ok := creds.(CACredentials); ok {
		caData = caCreds.GetCAData()
	}
	if len(caData) == 0 && creds.GetCAPath() != "" {
		var err error
		caData, err = os.ReadFile(creds.GetCAPath())
		if err != nil {
			return nil, fmt.Errorf("error reading CA file %s: %w", creds.GetCAPath(), err)
		}
	}
	if len(caData) > 0 {
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caData)
		tlsConfig.RootCAs = caCertPool
	}

	// If a client cert & key is provided then configure TLS config accordingly.
	if len(creds.GetCertData()) > 0 && len(creds.GetKeyData()) > 0 {
		cert, err := tls.X509KeyPair(creds.GetCertData(), creds.GetKeyData())
		if err != nil {
			return nil, fmt.Errorf("error creating X509 key pair: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	//nolint:staticcheck
	tlsConfig.BuildNameToCertificate()

	return tlsConfig, nil
}

// Normalize a chart name for file system use, that is, if chart name is foo/bar/baz, returns the last component as chart name.
func normalizeChartName(chart string) string {
	_, nc := path.Split(chart)
	// We do not want to return the empty string or something else related to filesystem access
	// Instead, return original string
	if nc == "" || nc == "." || nc == ".." {
		return chart
	}
	return nc
}

func (c *nativeHelmChart) getCachedChartPath(chart string, version string) (string, error) {
	keyData, err := json.Marshal(map[string]string{"url": c.repoURL, "chart": chart, "version": version})
	if err != nil {
		return "", fmt.Errorf("error marshaling cache key data: %w", err)
	}
	return c.chartCachePaths.GetPath(string(keyData))
}

// Ensures that given OCI registries URL does not have protocol
func IsHelmOciRepo(repoURL string) bool {
	if repoURL == "" || strings.Contains(repoURL, "://") {
		return false
	}
	// A scheme-less registry reference such as "demo.goharbor.io:8080" or
	// "192.168.100.150:1843" cannot be parsed as a URL on its own, so prepend a
	// dummy scheme and require a host, the same way oras-go validates a registry
	// (registry.Reference.ValidateRegistry).
	parsed, err := url.ParseRequestURI("dummy://" + repoURL)
	return err == nil && parsed.Host != ""
}

func getIndexURL(rawURL string) (string, error) {
	indexFile := "index.yaml"
	repoURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("error parsing repository URL: %w", err)
	}
	repoURL.Path = path.Join(repoURL.Path, indexFile)
	repoURL.RawPath = path.Join(repoURL.RawPath, indexFile)
	return repoURL.String(), nil
}

func (c *nativeHelmChart) GetTags(ctx context.Context, chart string, noCache bool) ([]string, error) {
	if !c.enableOci {
		return nil, ErrOCINotEnabled
	}

	tagsURL := strings.Replace(fmt.Sprintf("%s/%s", c.repoURL, chart), "https://", "", 1)
	operation, err := c.withResolvedCredentials(ctx)
	if err != nil {
		return nil, err
	}
	c = operation
	tagsKey, err := c.cacheTagsKey(ctx, tagsURL)
	if err != nil {
		return nil, err
	}

	unlock, err := indexClientLocks.acquire(ctx, "tags\x00"+tagsKey)
	if err != nil {
		return nil, err
	}
	defer unlock()

	var data []byte
	if !noCache && c.indexCache != nil {
		// OCI tag lists keep their own keyspace and expiration policy: they
		// are read via GetOCITags when the cache provides it, never inheriting
		// the HTTP index's freshness window.
		if tagsCache, ok := c.indexCache.(ociTagsCache); ok {
			err = tagsCache.GetOCITags(tagsKey, &data)
		} else {
			err = c.indexCache.GetHelmIndex(tagsKey, &data)
		}
		if err != nil && !errors.Is(err, cache.ErrCacheMiss) {
			log.Warnf("Failed to load index cache for repo: %s: %v", tagsURL, err)
		}
	}

	type entriesStruct struct {
		Tags []string
	}

	entries := &entriesStruct{}
	if len(data) == 0 {
		start := time.Now()
		repo, err := remote.NewRepository(tagsURL)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize repository: %w", err)
		}
		repo.PlainHTTP = c.plainHTTP
		tlsConf, err := newTLSConfig(c.creds)
		if err != nil {
			return nil, fmt.Errorf("failed setup tlsConfig: %w", err)
		}

		// Create base transport with TLS config and proxy
		baseTransport := &http.Transport{
			Proxy:             proxy.GetCallback(c.proxy, c.noProxy),
			TLSClientConfig:   tlsConf,
			DisableKeepAlives: true,
			ForceAttemptHTTP2: true,
		}

		// Wrap transport to add User-Agent header to all requests
		client := &http.Client{
			Transport: &userAgentTransport{
				Transport: baseTransport,
				UserAgent: c.getUserAgent(),
			},
		}

		repoHost, _, _ := strings.Cut(tagsURL, "/")

		helmPassword, err := c.creds.GetPassword()
		if err != nil {
			return nil, fmt.Errorf("failed to get password for helm registry: %w", err)
		}
		credential := auth.StaticCredential(repoHost, auth.Credential{
			Username: c.creds.GetUsername(),
			Password: helmPassword,
		})

		// Try to fallback to the environment config, but we shouldn't error if the file is not set
		if c.creds.GetUsername() == "" && helmPassword == "" {
			store, _ := credentials.NewStoreFromDocker(credentials.StoreOptions{})
			if store != nil {
				credential = credentials.Credential(store)
			}
		}

		repo.Client = &auth.Client{
			Client:     client,
			Cache:      nil,
			Credential: credential,
		}

		err = repo.Tags(ctx, "", func(tagsResult []string) error {
			for _, tag := range tagsResult {
				// By convention: Change underscore (_) back to plus (+) to get valid SemVer
				convertedTag := strings.ReplaceAll(tag, "_", "+")
				entries.Tags = append(entries.Tags, convertedTag)
			}

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get tags: %w", err)
		}
		log.WithFields(
			log.Fields{"seconds": time.Since(start).Seconds(), "chart": chart, "repo": c.repoURL},
		).Info("took to get tags")

		if c.indexCache != nil {
			cacheData, err := json.Marshal(entries)
			if err != nil {
				return nil, fmt.Errorf("failed to encode tags: %w", err)
			}
			if tagsCache, ok := c.indexCache.(ociTagsCache); ok {
				if err := tagsCache.SetOCITags(tagsKey, cacheData); err != nil {
					log.Warnf("Failed to store tags list cache for repo: %s: %v", tagsURL, err)
				}
			} else if err := c.indexCache.SetHelmIndex(tagsKey, cacheData); err != nil {
				log.Warnf("Failed to store tags list cache for repo: %s: %v", tagsURL, err)
			}
		}
	} else {
		err := json.Unmarshal(data, entries)
		if err != nil {
			return nil, fmt.Errorf("failed to decode tags: %w", err)
		}
	}

	return entries.Tags, nil
}
