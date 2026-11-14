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

var _ Client = &nativeHelmChart{}

type nativeHelmChart struct {
	lastValidators           http.Header // ETag/Last-Modified of the most recent 200 from loadRepoIndex; consumed by GetIndex for atomic publish
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

// withResolvedCredentials copies the client for one operation. Resolve before
// any cache read, and use the same immutable credentials for authentication,
// TLS, and every cache read/write. Keep the original provider on the reusable
// client so the next operation can observe rotation or a lookup failure.
func (c *nativeHelmChart) withResolvedCredentials() (*nativeHelmChart, error) {
	snap, err := snapshotCreds(c.creds)
	if err != nil {
		return nil, err
	}
	operation := *c
	operation.creds = snap
	operation.lastValidators = nil
	if operation.scopedByCreds {
		operation.indexCacheScope = credsScope(snap)
	}
	return &operation, nil
}

// cacheRepoKey returns the cache key for an operation's resolved credential
// view, or the explicit/legacy scope for clients without automatic scoping.
func (c *nativeHelmChart) cacheRepoKey() string {
	return c.cacheRepoKeyScoped(c.indexCacheScope)
}

// cacheRepoKeyScoped returns the repo cache key under an explicit credential
// scope. An empty scope preserves the legacy bare-URL key.
func (c *nativeHelmChart) cacheRepoKeyScoped(scope string) string {
	if scope == "" {
		return c.repoURL
	}
	return scopedIndexIdentity(c.repoURL, scope, c.proxy, c.noProxy)
}

// cacheTagsKey returns the cache key for an OCI tags URL under this client's
// credential scope (tags share the index cache's key space).
func (c *nativeHelmChart) cacheTagsKey(tagsURL string) string {
	if c.indexCacheScope == "" {
		return tagsURL
	}
	return scopedIndexIdentity(tagsURL, c.indexCacheScope, c.proxy, c.noProxy)
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
			snap, snapErr := snapshotCreds(c.creds)
			if snapErr != nil {
				return "", nil, snapErr
			}
			if snap.password != "" && snap.GetUsername() != "" {
				_, err = helmCmd.RegistryLogin(ctx, c.repoURL, snap, c.plainHTTP)
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
	operation, err := c.withResolvedCredentials()
	if err != nil {
		return nil, err
	}
	c = operation

	unlock, err := indexClientLocks.acquire(ctx, "index\x00"+c.cacheRepoKey())
	if err != nil {
		return nil, err
	}
	defer unlock()

	var data []byte
	if !noCache && c.indexCache != nil {
		if err := c.indexCache.GetHelmIndex(c.cacheRepoKey(), &data); err != nil && !errors.Is(err, cache.ErrCacheMiss) {
			log.Warnf("Failed to load index cache for repo: %s: %v", c.repoURL, err)
		}
	}

	if c.scopedByCreds && len(data) > 0 && (int64(len(data)) > maxIndexSize || validateIndexBody(data) != nil) {
		data = nil
	}

	if len(data) == 0 {
		start := time.Now()
		var err error
		data, err = c.loadRepoIndex(ctx, maxIndexSize)
		if err != nil {
			return nil, fmt.Errorf("error loading repo index: %w", err)
		}
		log.WithFields(log.Fields{"seconds": time.Since(start).Seconds()}).Info("took to get index")

		if c.indexCache != nil {
			// When the cache can store body and validators as ONE versioned
			// record, publish them atomically: a partial publication must
			// never pair a new body with stale validators (or vice versa).
			if recorder, ok := c.indexCache.(indexCacheRecorder); ok {
				etag, lastModified := responseValidatorHeaders(c.lastValidators)
				if err := recorder.SetHelmIndexRecord(c.cacheRepoKey(), data, etag, lastModified); err != nil {
					log.Warnf("Failed to store index cache for repo: %s: %v", c.repoURL, err)
				}
			} else if err := c.indexCache.SetHelmIndex(c.cacheRepoKey(), data); err != nil {
				log.Warnf("Failed to store index cache for repo: %s: %v", c.repoURL, err)
			} else {
				// Body publication succeeded: now (and only now) publish the
				// validators. On failure the stale-body+new-validator poison
				// pairing is impossible.
				if vc, ok := c.indexCache.(indexCacheWithValidators); ok && c.lastValidators != nil {
					etag, lastModified := responseValidatorHeaders(c.lastValidators)
					if err := vc.SetHelmIndexValidators(c.cacheRepoKey(), etag, lastModified); err != nil {
						log.WithError(err).Debugf("failed to store helm index validators for repo %s", c.repoURL)
					}
				}
			}
		}
	}

	index := &Index{}
	err = yaml.NewDecoder(bytes.NewBuffer(data)).Decode(index)
	if err != nil {
		return nil, fmt.Errorf("error decoding index: %w", err)
	}

	return index, nil
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
	snap, err := snapshotCreds(c.creds)
	if err != nil {
		return false, err
	}
	if snap.GetUsername() != "" && snap.password != "" {
		_, err = helmCmd.RegistryLogin(ctx, c.repoURL, snap, c.plainHTTP)
		if err != nil {
			return false, fmt.Errorf("error logging into OCI registry: %w", err)
		}
		defer func() {
			_, _ = helmCmd.RegistryLogout(c.repoURL, snap)
		}()

		log.WithFields(log.Fields{"seconds": time.Since(start).Seconds()}).Info("took to test helm oci repository")
	}
	return true, nil
}

func (c *nativeHelmChart) loadRepoIndex(ctx context.Context, maxIndexSize int64) ([]byte, error) {
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

	// Resolve credentials ONCE for this operation: dynamic providers (e.g.
	// Azure workload identity) rotate tokens between GetPassword calls, and
	// the request's basic-auth values and the cache-scope fingerprint must
	// describe the SAME credential view. Without the snapshot, a rotation
	// between construction and this call pairs token B's index with token
	// A's cache identity.
	snap, err := snapshotCreds(c.creds)
	if err != nil {
		return nil, err
	}
	if snap.GetUsername() != "" || snap.password != "" {
		// only basic supported
		req.SetBasicAuth(snap.GetUsername(), snap.password)
	}

	// Conditional GET: when validators (ETag / Last-Modified) from the
	// previous successful fetch are known, revalidate instead of re-downloading.
	// A 304 response lets us keep the cached body; validation failures remain
	// errors. Cache keys use the
	// scope of THIS operation's snapshot: the request authenticates with
	// snap's token, so it must read and publish under snap's identity.
	repoKey := c.cacheRepoKeyScoped(c.indexCacheScope)
	if c.indexCache != nil && c.scopedByCreds {
		repoKey = c.cacheRepoKeyScoped(credsScope(snap))
	}
	var validators indexCacheWithValidators
	var cachedBody []byte
	var cachedRecordETag, cachedRecordLastModified string
	var retainedRecord bool
	if c.indexCache != nil {
		validators = validatorsFor(c.indexCache)
		// Read the COMPLETE record in one call when the cache supports it:
		// reading body and validators separately lets a concurrent replica
		// publication slip a new validator in between, so a 304 would pair
		// the new validator with the previous body.
		if recorder, ok := c.indexCache.(indexCacheRecordReader); ok {
			if rec, err := recorder.GetHelmIndexRetainedRecord(repoKey); err == nil && len(rec.Body) > 0 {
				cachedBody = rec.Body
				cachedRecordETag, cachedRecordLastModified = rec.ETag, rec.LastModified
				retainedRecord = true
			}
		}
		if !retainedRecord {
			if err := c.indexCache.GetHelmIndex(repoKey, &cachedBody); err != nil {
				cachedBody = nil
			}
		}
		if (c.requireCompleteIndex || c.scopedByCreds) && len(cachedBody) > 0 && (int64(len(cachedBody)) > maxIndexSize || validateIndexBody(cachedBody) != nil) {
			cachedBody = nil
			retainedRecord = false
			cachedRecordETag, cachedRecordLastModified = "", ""
		}

		if len(cachedBody) > 0 {
			etag, lastModified := cachedRecordETag, cachedRecordLastModified
			if !retainedRecord {
				etag, lastModified = getIndexValidators(validators, repoKey)
			}
			applyConditionalHeaders(req, etag, lastModified)
		}
	}

	tlsConf, err := newTLSConfig(snap)
	if err != nil {
		return nil, fmt.Errorf("error creating TLS config: %w", err)
	}

	tr := &http.Transport{
		Proxy:             proxy.GetCallback(c.proxy, c.noProxy),
		TLSClientConfig:   tlsConf,
		DisableKeepAlives: true,
		ForceAttemptHTTP2: true,
	}
	client := http.Client{Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making HTTP request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotModified && len(cachedBody) > 0 {
		log.WithFields(log.Fields{"repo": c.repoURL}).Info("helm index not modified (HTTP 304), using cached body")
		// The caller publishes body+validators after loadRepoIndex returns;
		// a 304 confirms the cached validators are STILL VALID, so expose
		// them via lastValidators too. Without this, a fresh client instance
		// (lastValidators nil) republishes the record with EMPTY validators
		// and the next revalidation degrades to a full GET (200→304→200
		// instead of 200→304→304).
		if c.lastValidators == nil {
			c.lastValidators = http.Header{}
		}
		if cachedRecordETag != "" {
			c.lastValidators.Set("ETag", cachedRecordETag)
		}
		if cachedRecordLastModified != "" {
			c.lastValidators.Set("Last-Modified", cachedRecordLastModified)
		}
		if cachedRecordETag == "" && cachedRecordLastModified == "" && !retainedRecord {
			// Legacy split path: recover the validators from the validators cache.
			etag, lastModified := getIndexValidators(validators, repoKey)
			if etag != "" {
				c.lastValidators.Set("ETag", etag)
			}
			if lastModified != "" {
				c.lastValidators.Set("Last-Modified", lastModified)
			}
		}
		for _, name := range []string{"ETag", "Last-Modified"} {
			if value := resp.Header.Get(name); value != "" {
				c.lastValidators.Set(name, value)
			}
		}

		if retainedRecord {
			// Re-publish the SAME body+validators as one record: this re-stamps
			// freshness so ordinary readers are served again, atomically.
			if recorder, ok := c.indexCache.(indexCacheRecorder); ok {
				if err := recorder.SetHelmIndexRecord(repoKey, cachedBody, c.lastValidators.Get("ETag"), c.lastValidators.Get("Last-Modified")); err != nil {
					log.WithError(err).Debugf("failed to refresh helm index record for repo %s", c.repoURL)
				}
			}
		} else {
			// Touch the store so validators' TTL (if any) is refreshed alongside
			// the body they validate.
			storeResponseValidators(validators, repoKey, resp)
		}
		return cachedBody, nil
	}
	if resp.StatusCode == http.StatusNotModified {
		// 304 without a usable cached body (e.g. the body was evicted but the
		// validators survived): retry once unconditionally, without validators.
		log.WithFields(log.Fields{"repo": c.repoURL}).Debug("helm index 304 without cached body, retrying without validators")
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
			_ = validators
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
	// a new validator with the previous body. The adapter-based revalidation
	// path relies on the same ordering (Revalidate publishes body+validators
	// in one record after this returns).
	_ = validators
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

	if creds.GetCAPath() != "" {
		caData, err := os.ReadFile(creds.GetCAPath())
		if err != nil {
			return nil, fmt.Errorf("error reading CA file %s: %w", creds.GetCAPath(), err)
		}
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
	strings.Join(strings.Split(chart, "/"), "_")
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
	operation, err := c.withResolvedCredentials()
	if err != nil {
		return nil, err
	}
	c = operation

	tagsKey := c.cacheTagsKey(tagsURL)

	unlock, err := indexClientLocks.acquire(ctx, "tags\x00"+tagsKey)
	if err != nil {
		return nil, err
	}
	defer unlock()

	var data []byte
	if !noCache && c.indexCache != nil {
		if err := c.indexCache.GetHelmIndex(tagsKey, &data); err != nil && !errors.Is(err, cache.ErrCacheMiss) {
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

		snap, err := snapshotCreds(c.creds)
		if err != nil {
			return nil, err
		}
		if c.scopedByCreds {
			tagsKey = scopedIndexIdentity(tagsURL, credsScope(snap), c.proxy, c.noProxy)
		}
		credential := auth.StaticCredential(repoHost, auth.Credential{
			Username: snap.GetUsername(),
			Password: snap.password,
		})

		// Try to fallback to the environment config, but we shouldn't error if the file is not set
		if snap.GetUsername() == "" && snap.password == "" {
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
			if err := c.indexCache.SetHelmIndex(tagsKey, cacheData); err != nil {
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
