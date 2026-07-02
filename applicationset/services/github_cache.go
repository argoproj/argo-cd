package services

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/argon2"
	"k8s.io/utils/lru"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// cacheArgon2Salt is a random per-process salt for argon2id hashing of
// Authorization header credentials. Generated once at startup so that:
//   - The same credential always maps to the same cache key within a process lifetime.
//   - The raw credential cannot be recovered from the in-memory cache even if its
//     contents are dumped, because the salt is never persisted or exposed.
var cacheArgon2Salt []byte

func init() {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		// rand.Read only fails on catastrophic OS entropy failures. Panic here is
		// preferable to silently falling back to a zero salt, which would weaken
		// the security property argon2 provides.
		panic("github cache: failed to generate argon2 salt: " + err.Error())
	}
	cacheArgon2Salt = salt
}

// Metrics for GitHub cache
// - Storage metrics:
//   - Total number of items currently in a GitHub cache storage bucket
//   - Total number of items evicted from GitHub cache storage (cumulative counter)
//   - Total number of bytes currently held by a GitHub cache storage bucket
const (
	githubCacheStorageItemsTotal        = "argocd_github_cache_storage_items_total"
	githubCacheStorageItemsEvictedTotal = "argocd_github_cache_storage_items_evicted_total"
	githubCacheStorageBytesTotal        = "argocd_github_cache_storage_bytes_total"
)

type StorageMetrics struct {
	StorageItemsTotal        *prometheus.GaugeVec
	StorageItemsEvictedTotal *prometheus.CounterVec
	StorageBytesTotal        *prometheus.GaugeVec
}

func NewGitHubStorageMetrics() *StorageMetrics {
	return &StorageMetrics{
		StorageItemsTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: githubCacheStorageItemsTotal,
				Help: "Current number of items in GitHub cache storage",
			},
			[]string{"key"},
		),
		StorageItemsEvictedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: githubCacheStorageItemsEvictedTotal,
				Help: "Total number of items evicted from GitHub cache storage",
			},
			[]string{"key"},
		),
		StorageBytesTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: githubCacheStorageBytesTotal,
				Help: "Current number of bytes held in GitHub cache storage (serialised HTTP responses)",
			},
			[]string{"key"},
		),
	}
}

var globalGitHubStorageMetrics = NewGitHubStorageMetrics()

// - Cache metrics:
//   - Total number of cache request hits in GitHub cache
//   - Total number of cache requests in GitHub cache
const (
	githubCacheHits       = "argocd_github_cache_hits_total"
	githubCacheCacheTotal = "argocd_github_cache_request_total"
)

type CacheMetrics struct {
	CacheRequestHits  *prometheus.CounterVec
	CacheRequestTotal *prometheus.CounterVec
}

func NewGitHubCacheMetrics() *CacheMetrics {
	return &CacheMetrics{
		CacheRequestHits: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: githubCacheHits,
				Help: "Total number of cache request hits in GitHub cache",
			},
			[]string{"key"},
		),
		CacheRequestTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: githubCacheCacheTotal,
				Help: "Total number of cache requests in GitHub cache",
			},
			[]string{"key"},
		),
	}
}

var globalGitHubCacheMetrics = NewGitHubCacheMetrics()

var registerMetricsOnce sync.Once

func RegisterGitHubCacheMetrics() {
	registerMetricsOnce.Do(func() {
		log.Debug("Registering GitHub Cache metrics")
		metrics.Registry.MustRegister(globalGitHubStorageMetrics.StorageItemsTotal)
		metrics.Registry.MustRegister(globalGitHubStorageMetrics.StorageItemsEvictedTotal)
		metrics.Registry.MustRegister(globalGitHubStorageMetrics.StorageBytesTotal)
		metrics.Registry.MustRegister(globalGitHubCacheMetrics.CacheRequestHits)
		metrics.Registry.MustRegister(globalGitHubCacheMetrics.CacheRequestTotal)
	})
}

// Cache Storage is a thread-safe LRU cache for storing HTTP responses from GitHub API requests depending of the authentification used (AppSecretName, TokenRef, or anonymous).
// It uses a map of LRU caches, one for each unique cache context.
// Each LRU cache is protected by a read-write mutex to ensure thread safety.
// The cache stores the response body as bytes and the Vary headers to determine if a cached response is valid for a given request.
// The cache also excludes certain volatile headers from being stored in the cache, such as Date, Set-Cookie, and X-RateLimit-* headers.
type Storage struct {
	key    string
	lock   *sync.RWMutex
	lruMap *lru.Cache
}

type cachedResponse struct {
	varyHeaders      []string
	varyHeadersValue map[string]string
	responseBytes    []byte
}

type gitHubCacheRegistry struct {
	storages map[string]Storage
	lock     *sync.RWMutex
}

var globalGitHubCacheRegistry = &gitHubCacheRegistry{
	storages: make(map[string]Storage),
	lock:     &sync.RWMutex{},
}

// List of headers that should not be stored in the cache, as they are volatile and can change between requests
var excludedCacheHeaders = []string{
	"Date",
	"Set-Cookie",
	"X-GitHub-Request-ID",
	"X-RateLimit-Limit",
	"X-RateLimit-Remaining",
	"X-RateLimit-Reset",
	"X-RateLimit-Resource",
	"X-RateLimit-Used",
}

func (s Storage) Get(_ context.Context, req *http.Request) (*http.Response, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	value, ok := s.lruMap.Get(req.URL.String())
	if !ok {
		return nil, nil
	}
	cachedResp, ok := value.(cachedResponse)
	if !ok {
		return nil, errors.New("value is not a cachedResponse")
	}
	// Check if the request headers match the cached response's Vary headers
	if !isSameVaryHeaders(req, cachedResp.varyHeaders, cachedResp.varyHeadersValue) {
		return nil, nil
	}
	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(cachedResp.responseBytes)), nil)
	if err != nil {
		return nil, fmt.Errorf("http.ReadResponse failed: %w", err)
	}
	return resp, nil
}

func (s Storage) Put(_ context.Context, req *http.Request, resp *http.Response) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	varyHeaders, err := parseVaryHeaders(resp.Header)
	if err != nil {
		// Cannot cache due to wildcard Vary header.
		return nil
	}
	// Clean the response headers to remove any volatile headers that should not be stored in the cache
	cleanedHeader := maps.Clone(resp.Header)
	for _, header := range excludedCacheHeaders {
		cleanedHeader.Del(header)
	}
	varyHeadersValue := map[string]string{}
	for _, header := range varyHeaders {
		val := req.Header.Get(header)
		if val != "" {
			if header == "Authorization" {
				val = githubHashToken(val) // Don't leak/cache the raw authentication token
			}
			varyHeadersValue[header] = val
		}
	}

	// Temporarily substitute the cloned, stripped header so DumpResponse
	// serialises the clean version without touching the original.
	origHeader := resp.Header
	resp.Header = cleanedHeader
	value, err := httputil.DumpResponse(resp, true)
	resp.Header = origHeader
	if err != nil {
		return fmt.Errorf("httputil.DumpResponse failed: %w", err)
	}

	s.lruMap.Add(req.URL.String(), cachedResponse{
		responseBytes:    value,
		varyHeaders:      varyHeaders,
		varyHeadersValue: varyHeadersValue,
	})
	// The eviction callback (fired on both capacity eviction and key replacement)
	// subtracts the old entry's bytes. Add the new entry's bytes unconditionally.
	globalGitHubStorageMetrics.StorageItemsTotal.WithLabelValues(s.key).Set(float64(s.lruMap.Len()))
	globalGitHubStorageMetrics.StorageBytesTotal.WithLabelValues(s.key).Add(float64(len(value)))
	return nil
}

// githubHashToken extracts the credential from an Authorization header and
// returns its argon2id hash, encoded as base64. This prevents raw secrets from
// being stored in the in-memory cache and satisfies CodeQL's requirement for a
// computationally expensive hash function on sensitive data.
//
// argon2 parameters are deliberately lightweight (time=1, memory=64 KiB,
// threads=1, keyLen=32) because this function is a cache-key discriminator,
// not a password-storage function. The output still cannot be brute-forced
// feasibly thanks to the per-process random salt (cacheArgon2Salt).
//
// Supported schemes (https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/about-authentication-to-github):
//   - Bearer <token>
//   - token <token>
//   - Basic <base64(username:password)>  — only the password is hashed
//
// Returns an empty string for an empty or unrecognised header so that callers
// can distinguish "no credential" from any valid hash.
func githubHashToken(authorization string) string {
	var token string
	if bearer, ok := strings.CutPrefix(authorization, "Bearer "); ok && bearer != "" {
		token = bearer
	} else if t, ok := strings.CutPrefix(authorization, "token "); ok && t != "" {
		token = t
	} else if basic, ok := strings.CutPrefix(authorization, "Basic "); ok && basic != "" {
		// Only hash the password portion; the username is not sensitive.
		if decoded, err := base64.StdEncoding.DecodeString(basic); err == nil {
			if _, password, ok := bytes.Cut(decoded, []byte{':'}); ok && len(password) > 0 {
				token = string(password)
			}
		}
	}
	if token == "" {
		// Unrecognised or empty header — return empty so callers can treat this
		// as a cache miss rather than colliding with other empty-credential requests.
		return ""
	}
	// argon2id: time=1, memory=64KiB, threads=1, keyLen=32 bytes.
	hashed := argon2.IDKey([]byte(token), cacheArgon2Salt, 1, 64*1024, 1, 32)
	return base64.StdEncoding.EncodeToString(hashed)
}

// Parse the Vary header from the response and return a list of headers that should be used to determine if a cached response is valid for a given request
func parseVaryHeaders(headers http.Header) ([]string, error) {
	// Vary = #( "*" / field-name ) from RFC 9110 Section 12.5.5
	// RFC 9111 Section 4.1 Calculating Cache Keys with the Vary Header Field
	// A stored response with a Vary header field value containing a member "*" always fails to match
	result := []string{}
	for _, val := range headers.Values("Vary") {
		if val == "*" {
			return []string{}, errors.New("cannot cache due to wildcard Vary header")
		}
		for field := range strings.SplitSeq(val, ",") {
			field = strings.TrimSpace(field)
			if field != "" {
				result = append(result, http.CanonicalHeaderKey(field))
			}
		}
	}
	return result, nil
}

// Check if the request headers match the cached response's Vary headers
func isSameVaryHeaders(req *http.Request, varyHeaders []string, varyHeadersValue map[string]string) bool {
	for _, header := range varyHeaders {
		// For the Authorization header, we need to hash the token or user/password from the request header to avoid leaking sensitive information from the cache.
		if header == "Authorization" {
			if githubHashToken(req.Header.Get(header)) != varyHeadersValue[header] {
				return false
			}
		} else {
			if req.Header.Get(header) != varyHeadersValue[header] {
				return false
			}
		}
	}
	return true
}

// Create a new LRU cache based on the cache context (AppSecretName, TokenRef, or anonymous) and size.
// If a cache already exists for the given context, it will be returned instead of creating a new one.
func newLRUStorage(cacheCtx *GitHubCacheContext, size int) Storage {
	globalGitHubCacheRegistry.lock.Lock()
	defer globalGitHubCacheRegistry.lock.Unlock()
	// Generate a unique key for this cache context
	cacheContextKey := "anonymous"
	if cacheCtx.AppSecretName != "" {
		cacheContextKey = "app/" + cacheCtx.AppSecretName
	} else if cacheCtx.TokenRef != nil {
		cacheContextKey = fmt.Sprintf("token/%s/%s", cacheCtx.TokenRef.SecretName, cacheCtx.TokenRef.Key)
	}
	if storage, exists := globalGitHubCacheRegistry.storages[cacheContextKey]; exists {
		return storage
	}
	log.WithFields(log.Fields{
		"key": cacheContextKey,
	}).Debugf("Creating new GitHub Cache in memory %d size", size)
	// Initialise counters/gauges at zero so they appear in metrics output
	// before any entry is stored or evicted.
	globalGitHubStorageMetrics.StorageItemsEvictedTotal.WithLabelValues(cacheContextKey).Add(0)
	globalGitHubStorageMetrics.StorageBytesTotal.WithLabelValues(cacheContextKey).Set(0)
	storage := Storage{
		key:  cacheContextKey,
		lock: &sync.RWMutex{},
		lruMap: lru.NewWithEvictionFunc(size, func(_ lru.Key, value any) {
			evicted, ok := value.(cachedResponse)
			if ok {
				globalGitHubStorageMetrics.StorageBytesTotal.WithLabelValues(cacheContextKey).Sub(float64(len(evicted.responseBytes)))
			}
			globalGitHubStorageMetrics.StorageItemsEvictedTotal.WithLabelValues(cacheContextKey).Inc()
		}),
	}
	globalGitHubCacheRegistry.storages[cacheContextKey] = storage
	return storage
}

// GitHubCacheTransport is an http.RoundTripper that wraps another http.RoundTripper and adds caching for GitHub API requests.
type GitHubCacheTransport struct {
	parent  http.RoundTripper
	storage Storage
}

func cacheable(req *http.Request) bool {
	// RFC 9111 Section 4.4 Invalidating Stored Responses
	// Because unsafe request methods (Section 9.2.1 of [HTTP]) such as PUT, POST, or DELETE
	// have the potential for changing state on the origin server, intervening caches are
	// required to invalidate stored responses to keep their contents up to date.
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		return false
	}
	// RFC 9111 Section 3.3 Storing Incomplete Responses
	// A cache MUST NOT store incomplete or partial-content responses if it does not
	// support the Range and Content-Range header fields or if it does not understand
	// the range units used in those fields.
	if req.Header.Get("Range") != "" {
		return false
	}
	// REST API endpoints for rate limits is a GET method
	// see https://docs.github.com/en/rest/rate-limit/rate-limit?apiVersion=2022-11-28
	// However shouldn't be cached
	// - Github `/rate_limit`
	// - Github Enterprise `/api/v3/rate_limit`
	if req.URL.Path == "/rate_limit" || req.URL.Path == "/api/v3/rate_limit" {
		return false
	}
	return true
}

func (t *GitHubCacheTransport) RoundTrip(req *http.Request) (resp *http.Response, _ error) {
	// If the request is not cacheable, just pass it through to the parent RoundTripper
	if !cacheable(req) {
		return t.parent.RoundTrip(req)
	}
	globalGitHubCacheMetrics.CacheRequestTotal.WithLabelValues(t.storage.key).Inc()

	// Attempt to fetch from storage
	cached, err := t.storage.Get(req.Context(), req)
	if err != nil {
		return nil, fmt.Errorf("(Storage).Get failed: %w", err)
	}
	defer func() {
		// If we did not utilize the cached response, ensure it is consumed and closed
		if cached != nil && cached.Body != nil && (resp == nil || resp.Body != cached.Body) {
			_, _ = io.Copy(io.Discard, cached.Body)
			_ = cached.Body.Close()
		}
	}()

	// Per the http.RoundTripper contract, we cannot modify the request in-place, we need to shallow clone it
	req = req.Clone(req.Context())

	if cached != nil {
		// Inject the conditional headers to the request
		req.Header.Set("If-None-Match", cached.Header.Get("Etag"))
	}

	// Perform the upstream request
	resp, err = t.parent.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("(http.RoundTripper).RoundTrip failed: %w", err)
	}

	if resp.StatusCode == http.StatusNotModified && cached != nil {
		// If the upstream response is 304 Not Modified, we can use the cached response
		globalGitHubCacheMetrics.CacheRequestHits.WithLabelValues(t.storage.key).Inc()

		// Consume the rest of the response body to ensure the connection can be re-used
		if _, err := io.Copy(io.Discard, resp.Body); err != nil {
			return nil, fmt.Errorf("(*http.Response).Body.Read failed: %w", err)
		}
		if err := resp.Body.Close(); err != nil {
			return nil, fmt.Errorf("(*http.Response).Body.Close failed: %w", err)
		}

		// Copy in any cached headers that are not already set
		for key, vals := range cached.Header {
			if _, ok := resp.Header[key]; !ok {
				resp.Header[key] = vals
			}
		}

		// Copy the body and status from the cache
		resp.StatusCode = cached.StatusCode
		resp.Status = cached.Status

		// As a special case, if the request is a HEAD, we return an empty body
		if req.Method == http.MethodHead {
			resp.Body = io.NopCloser(strings.NewReader(""))
			resp.ContentLength = 0
		} else {
			resp.Body = cached.Body
			resp.ContentLength = cached.ContentLength
		}
	} else if resp.StatusCode == http.StatusOK && req.Method == http.MethodGet && resp.Header.Get("Etag") != "" {
		// Store the cached response body as bytes
		// Per the storage contract, they will restore the Body/ContentLength after consumption
		if err := t.storage.Put(req.Context(), req, resp); err != nil {
			return resp, fmt.Errorf("(Storage).Put failed: %w", err)
		}
	}

	return resp, nil
}

type GitHubCacheContext struct {
	TokenRef      *argoprojiov1alpha1.SecretRef
	AppSecretName string
}

// Default constructor
func NewGitHubCacheTransport(parent http.RoundTripper, cacheCtx *GitHubCacheContext, size int) *GitHubCacheTransport {
	if parent == nil {
		parent = http.DefaultTransport
	}
	storage := newLRUStorage(cacheCtx, size)
	return &GitHubCacheTransport{
		parent:  parent,
		storage: storage,
	}
}

// NewGitHubCacheFrom returns a new http.Client wrapping the provided one with cache middleware
func NewGitHubCacheFrom(httpClient *http.Client, cacheCtx *GitHubCacheContext, size int) *http.Client {
	log.Debug("Creating new GitHub cache")
	httpClientCopy := *httpClient
	transport := httpClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	httpClientCopy.Transport = NewGitHubCacheTransport(transport, cacheCtx, size)
	return &httpClientCopy
}
