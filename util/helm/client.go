package helm

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
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
	stdsync "sync"
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
	cryptoutil "github.com/argoproj/argo-cd/v3/util/crypto"
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
	if _, publisher := operation.indexCache.(indexCachePublisher); publisher {
		operation.requireCompleteIndex = true
	}
	if _, snapshot := operation.indexCache.(indexCacheSnapshotReader); snapshot {
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

	owned := true
	defer func() {
		if owned {
			_ = os.RemoveAll(tempDir)
		}
	}()

	cachedChartPath, err := c.getCachedChartPath(chart, version)
	if err != nil {
		return "", nil, fmt.Errorf("error getting cached chart path: %w", err)
	}

	c.repoLock.Lock(cachedChartPath)
	defer c.repoLock.Unlock(cachedChartPath)

	// check if chart tar is already downloaded
	exists, err := fileExist(cachedChartPath)
	if err != nil {
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
					return "", nil, fmt.Errorf("error logging into OCI registry: %w", err)
				}

				defer func() {
					_, _ = helmCmd.RegistryLogout(c.repoURL, c.creds)
				}()
			}

			// 'helm pull' ensures that chart is downloaded into temp directory
			_, err = helmCmd.PullOCI(c.repoURL, chart, version, tempDest, c.creds, c.plainHTTP)
			if err != nil {
				return "", nil, fmt.Errorf("error pulling OCI chart: %w", err)
			}
		} else {
			_, err = helmCmd.Fetch(c.repoURL, chart, version, tempDest, c.creds, passCredentials)
			if err != nil {
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
		return "", nil, fmt.Errorf("error untarring chart: %w", err)
	}
	owned = false
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
		if reader, ok := c.indexCache.(indexCacheSnapshotReader); ok {
			operation, cancel := indexCacheContext(ctx, 5*time.Second)
			record, fresh, err := reader.GetHelmIndexSnapshotContext(operation, repoKey)
			cancel()
			c.cacheReadAttempted = true
			c.cacheObservation = indexRecordObservation{record: record}
			if err == nil && fresh && record != nil && c.matchesIndexRequest(ctx, record.URLIdentity) {
				data = record.Body
			}
		} else if err := c.indexCache.GetHelmIndex(repoKey, &data); err != nil {
			data = nil
		} else {
			var identity string
			if reader, ok := c.indexCache.(indexCacheRecordReader); ok {
				c.cacheObservation = observeIndexRecord(ctx, reader, repoKey)
				c.cacheReadAttempted = true
				if record := c.cacheObservation.record; record != nil && bytes.Equal(data, record.Body) {
					identity = record.URLIdentity
				}
			} else {
				_, _, identity = sharedInMemoryValidators.getForResponse(repoKey, data)
			}
			if !c.matchesIndexRequest(ctx, identity) {
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
		var err error
		owner := beginIndexFetch(ctx, c.indexCache, repoKey)
		stopRenewing := keepIndexFetchAlive(ctx, c.indexCache, repoKey, owner)
		defer stopRenewing()
		if owner != "" {
			// A maintenance publication between the initial observation and
			// this claim cannot be invalidated retroactively. Read its record
			// before allowing a 304 to renew the claimed generation.
			c.cacheReadAttempted = false
			c.cacheObservation = indexRecordObservation{}
		}
		published := false
		defer func() {
			if !published {
				releaseIndexFetch(ctx, c.indexCache, repoKey, owner)
			}
		}()
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
			if _, ok := c.indexCache.(indexCachePublisher); ok {
				published, _ = publishIndexFetch(ctx, c.indexCache, repoKey, owner, c.fetchedIndexRecord(data))
			} else if !c.indexNotModified {
				_, reader := c.indexCache.(indexCacheRecordReader)
				_, writer := c.indexCache.(indexCacheRecorder)
				_, validators := c.indexCache.(indexCacheWithValidators)
				// Metadata adapters cannot fall back to a non-atomic write.
				// Only legacy raw-body caches retain their original setter contract.
				if !reader && !writer && !validators {
					if err := c.indexCache.SetHelmIndex(repoKey, data); err != nil {
						log.WithError(err).Debug("failed to publish legacy helm index body")
					}
				}
			}
			if _, paired := c.indexCache.(indexCacheRecordReader); !paired {
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
	// a new validator with the previous body. The adapter-based revalidation
	// path relies on the same ordering (Revalidate publishes body+validators
	// in one record after this returns).
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
		var caData []byte
		if snapshot, ok := creds.(credsSnapshot); ok {
			caData = snapshot.caData
		} else {
			var err error
			caData, err = os.ReadFile(creds.GetCAPath())
			if err != nil {
				return nil, fmt.Errorf("error reading CA file %s: %w", creds.GetCAPath(), err)
			}
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
	repoURL.RawPath = strings.TrimSuffix(repoURL.RawPath, "/") + "/"
	repoURL.Path = strings.TrimSuffix(repoURL.Path, "/") + "/"
	resolved := repoURL.ResolveReference(&url.URL{Path: indexFile})
	resolved.RawQuery = repoURL.RawQuery
	return resolved.String(), nil
}

// A prelookup uses one quarter of the remaining deadline. A challenge retry
// can use three quarters (up to thirty seconds), preserving time for anonymous
// bearer authentication if the optional helper still stalls. A successful
// lookup remains authoritative even if its timer expires just afterward.
func lookupRegistryCredential(ctx context.Context, resolver auth.CredentialFunc, host string, maximum time.Duration, quarters int) (auth.Credential, bool, error) {
	budget := maximum
	if deadline, ok := ctx.Deadline(); ok {
		budget = min(budget, time.Until(deadline)/4*time.Duration(quarters))
	}
	lookup, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	credential, err := resolver(lookup, host)
	timedOut := err != nil && lookup.Err() != nil && ctx.Err() == nil
	return credential, timedOut, err
}

// ORAS's native helper executor has no pipe-drain deadline. Preserve helper
// precedence and inline credential formats from one configuration snapshot,
// bounding cancellation and inherited stdout pipes when executing a helper.
func boundedDockerCredential(store *credentials.DynamicStore) (auth.CredentialFunc, error) {
	var config struct {
		Store   string                     `json:"credsStore"`
		Helpers map[string]string          `json:"credHelpers"`
		Auths   map[string]json.RawMessage `json:"auths"`
	}
	var fields map[string]json.RawMessage
	file, err := os.Open(store.ConfigPath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		err = json.NewDecoder(file).Decode(&fields)
		_ = file.Close()
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
	}
	for key, target := range map[string]any{"credsStore": &config.Store, "credHelpers": &config.Helpers, "auths": &config.Auths} {
		if raw, present := fields[key]; present {
			if err := json.Unmarshal(raw, target); err != nil {
				return nil, err
			}
		}
	}
	return func(ctx context.Context, host string) (auth.Credential, error) {
		address := credentials.ServerAddressFromHostname(host)
		if address == "" {
			return auth.EmptyCredential, nil
		}
		helper := config.Helpers[address]
		if helper == "" {
			helper = config.Store
		}
		if helper == "" {
			return dockerInlineCredential(config.Auths, address)
		}
		if err := acquireDockerConfigLoader(ctx, dockerHelperProcesses); err != nil {
			return auth.EmptyCredential, err
		}
		defer func() { <-dockerHelperProcesses }()
		cmd := exec.CommandContext(ctx, "docker-credential-"+helper, "get")
		cmd.Stdin = strings.NewReader(address)
		cmd.Stderr = os.Stderr
		cmd.WaitDelay = 25 * time.Millisecond
		output, err := cmd.Output()
		if err != nil && (!errors.Is(err, exec.ErrWaitDelay) || ctx.Err() != nil) {
			var exit *exec.ExitError
			if errors.As(err, &exit) && strings.TrimSpace(string(output)) == "credentials not found in native keychain" {
				return auth.EmptyCredential, nil
			}
			return auth.EmptyCredential, err
		}
		var result struct{ Username, Secret string }
		if err := json.Unmarshal(output, &result); err != nil {
			return auth.EmptyCredential, err
		}
		if result.Username == "<token>" {
			return auth.Credential{RefreshToken: result.Secret}, nil
		}
		return auth.Credential{Username: result.Username, Password: result.Secret}, nil
	}, nil
}

// Match ORAS's inline credential formats using the same immutable config
// snapshot that selected the helper. Never include decoded secrets in errors.
func dockerInlineCredential(auths map[string]json.RawMessage, address string) (auth.Credential, error) {
	raw, found := auths[address]
	if !found {
		for key, value := range auths {
			host, _, _ := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(key, "http://"), "https://"), "/")
			if host == address {
				raw, found = value, true
				break
			}
		}
	}
	if !found {
		return auth.EmptyCredential, nil
	}
	var entry struct {
		Username, Password, Auth, IdentityToken, RegistryToken string
	}
	if err := json.Unmarshal(raw, &entry); err != nil {
		return auth.EmptyCredential, err
	}
	credential := auth.Credential{Username: entry.Username, Password: entry.Password, RefreshToken: entry.IdentityToken, AccessToken: entry.RegistryToken}
	if entry.Auth != "" {
		decoded, err := base64.StdEncoding.DecodeString(entry.Auth)
		if err != nil {
			return auth.EmptyCredential, fmt.Errorf("decode Docker authentication: %w", err)
		}
		var ok bool
		credential.Username, credential.Password, ok = strings.Cut(string(decoded), ":")
		if !ok {
			return auth.EmptyCredential, errors.New("invalid Docker authentication format")
		}
	}
	return credential, nil
}

// Filesystem reads cannot always be interrupted. Bound their caller's wait
// and the number of outstanding loaders, including a blocked configuration open.
var dockerConfigLoaders = make(chan struct{}, 4)

// Optional lookups and challenge retries share this process limit. Resolving
// credentials precedes the tag-cache lock and can otherwise fan out on a hit.
var dockerHelperProcesses = make(chan struct{}, 4)

func acquireDockerConfigLoader(ctx context.Context, slots chan struct{}) error {
	select {
	case slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func loadDockerCredentialResolver(ctx context.Context) (auth.CredentialFunc, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	operation, cancel := indexCacheContext(ctx, time.Second)
	defer cancel()
	if err := acquireDockerConfigLoader(operation, dockerConfigLoaders); err != nil {
		return nil, err
	}
	type result struct {
		resolver auth.CredentialFunc
		err      error
	}
	finished := make(chan result, 1)
	go func() {
		defer func() { <-dockerConfigLoaders }()
		store, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
		var resolver auth.CredentialFunc
		if err == nil {
			resolver, err = boundedDockerCredential(store)
		}
		finished <- result{resolver, err}
	}()
	select {
	case <-operation.Done():
		return nil, operation.Err()
	case loaded := <-finished:
		return loaded.resolver, loaded.err
	}
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
	snap, err := snapshotCreds(c.creds)
	if err != nil {
		return nil, err
	}
	repo, err := remote.NewRepository(tagsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize repository: %w", err)
	}
	repoHost := repo.Reference.Host()
	credential := auth.Credential{Username: snap.GetUsername(), Password: snap.password}
	var credentialErr error
	var retryCredential auth.CredentialFunc
	storeUnavailable := false
	if credential.Username == "" && credential.Password == "" {
		// Resolve the Docker store before any shared-cache read. The same
		// immutable credential must authenticate and identify this operation.
		resolver, storeErr := loadDockerCredentialResolver(ctx)
		credentialErr = storeErr
		storeUnavailable = storeErr != nil
		if storeErr == nil {
			var optionalTimeout bool
			credential, optionalTimeout, credentialErr = lookupRegistryCredential(ctx, resolver, repoHost, time.Second, 1)
			if optionalTimeout {
				// Keep the remaining request budget for anonymous registry
				// access, without reading or publishing shared tag metadata.
				credential = auth.EmptyCredential
				c.indexCache = nil
				storeUnavailable = true
				var retryOnce stdsync.Once
				var retried auth.Credential
				var retryErr error
				retryCredential = func(ctx context.Context, host string) (auth.Credential, error) {
					if host != repoHost {
						return auth.EmptyCredential, nil
					}
					retryOnce.Do(func() {
						var timedOut bool
						retried, timedOut, retryErr = lookupRegistryCredential(ctx, resolver, host, 30*time.Second, 3)
						if timedOut {
							retried, retryErr = auth.EmptyCredential, nil
						}
					})
					return retried, retryErr
				}
			}
			if credential != auth.EmptyCredential {
				scope, err := credsScopeContext(ctx, snap)
				if err != nil {
					return nil, err
				}
				c.indexCacheScope, err = cryptoutil.CredentialScopeContext(ctx, "helm-docker-credentials",
					c.indexCacheScope, scope, credential.Username, credential.Password, credential.AccessToken, credential.RefreshToken)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	credentialFunc := auth.StaticCredential(repoHost, credential)
	if credentialErr != nil {
		// Public registries may never challenge. Unknown credentials cannot
		// identify shared data; retain the lookup error for an auth challenge.
		c.indexCache = nil
		// Failure to construct the optional store historically retained
		// anonymous bearer authentication. A loaded store's helper error still
		// fails an authentication challenge instead of hiding the lookup error.
		if retryCredential != nil {
			credentialFunc = retryCredential
		} else if !storeUnavailable {
			credentialFunc = func(context.Context, string) (auth.Credential, error) {
				return auth.EmptyCredential, fmt.Errorf("resolve registry credentials: %w", credentialErr)
			}
		}
	}

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
		var err error
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

		repo.Client = &auth.Client{
			Client:     client,
			Cache:      nil,
			Credential: credentialFunc,
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
				err = tagsCache.SetOCITags(tagsKey, cacheData)
			} else {
				err = c.indexCache.SetHelmIndex(tagsKey, cacheData)
			}
			if err != nil {
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
