package services

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"

	gh_hash_token "github.com/bored-engineer/github-conditional-http-transport"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"k8s.io/utils/lru"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

var ExcludedCacheHeaders = []string{
	"Date",
	"Set-Cookie",
	"X-GitHub-Request-ID",
	"X-RateLimit-Limit",
	"X-RateLimit-Remaining",
	"X-RateLimit-Reset",
	"X-RateLimit-Resource",
	"X-RateLimit-Used",
}

var globalGitHubCache = &gitHubCacheRegistry{
	storages: make(map[string]Storage),
	lock:     &sync.RWMutex{},
}

// Metric names as constants
const (
	githubCacheStorageItemsTotal   = "argocd_github_cache_storage_items_total"
	githubCacheStorageItemsEvicted = "argocd_github_cache_storage_items_evicted_total"
	githubCacheCacheHits           = "argocd_github_cache_hits_total"
	githubCacheCacheTotal          = "argocd_github_cache_request_total"
)

type StorageMetrics struct {
	StorageItemsTotal   *prometheus.GaugeVec
	StorageItemsEvicted *prometheus.CounterVec
}

func NewGitHubStorageMetrics() *StorageMetrics {
	return &StorageMetrics{
		StorageItemsTotal:   NewGitHubStorageItemsTotal(),
		StorageItemsEvicted: NewGitHubStorageItemsEvicted(),
	}
}

func NewGitHubStorageItemsTotal() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: githubCacheStorageItemsTotal,
			Help: "Total number of items in GitHub cache storage",
		},
		[]string{"key"},
	)
}

func NewGitHubStorageItemsEvicted() *prometheus.CounterVec {
	return prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: githubCacheStorageItemsEvicted,
			Help: "Total number of items evicted from GitHub cache storage",
		},
		[]string{"key"},
	)
}

var globalGitHubStorageMetrics = NewGitHubStorageMetrics()

type CacheMetrics struct {
	CacheRequestHits  *prometheus.CounterVec
	CacheRequestTotal *prometheus.CounterVec
}

func NewGitHubCacheHits() *prometheus.CounterVec {
	return prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: githubCacheCacheHits,
			Help: "Total number of cache request hits in GitHub cache",
		},
		[]string{"key"},
	)
}

func NewGitHubCacheTotal() *prometheus.CounterVec {
	return prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: githubCacheCacheTotal,
			Help: "Total number of cache requests in GitHub cache",
		},
		[]string{"key"},
	)
}

func NewGitHubCacheMetrics() *CacheMetrics {
	return &CacheMetrics{
		CacheRequestHits:  NewGitHubCacheHits(),
		CacheRequestTotal: NewGitHubCacheTotal(),
	}
}

var globalGitHubCacheMetrics = NewGitHubCacheMetrics()

func init() {
	log.Debug("Registering GitHub Cache metrics")
	metrics.Registry.MustRegister(globalGitHubStorageMetrics.StorageItemsTotal)
	metrics.Registry.MustRegister(globalGitHubStorageMetrics.StorageItemsEvicted)
	metrics.Registry.MustRegister(globalGitHubCacheMetrics.CacheRequestHits)
	metrics.Registry.MustRegister(globalGitHubCacheMetrics.CacheRequestTotal)
}

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
	if !isSameVaryHeader(req, cachedResp.varyHeaders, cachedResp.varyHeadersValue) {
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
	value, err := httputil.DumpResponse(resp, true)
	if err != nil {
		return fmt.Errorf("httputil.DumpResponse failed: %w", err)
	}
	varyHeaders, err := parseVaryHeaders(resp.Header)
	if err != nil {
		// we cannot cache due to wildcard Vary header
		return nil
	}
	varyHeadersValue := map[string]string{}
	for _, header := range varyHeaders {
		val := req.Header.Get(header)
		if val != "" {
			if header == "Authorization" {
				val = gh_hash_token.HashToken(val) // Don't leak/cache the raw authentication token
			}
			varyHeadersValue[header] = val
		}
	}
	s.lruMap.Add(resp.Request.URL.String(), cachedResponse{responseBytes: value, varyHeaders: varyHeaders, varyHeadersValue: varyHeadersValue})
	globalGitHubStorageMetrics.StorageItemsTotal.WithLabelValues(s.key).Set(float64(s.lruMap.Len()))
	return nil
}

type gitHubCacheRegistry struct {
	storages map[string]Storage
	lock     *sync.RWMutex
}

type GitHubCacheContext struct {
	TokenRef      *argoprojiov1alpha1.SecretRef
	AppSecretName string
}

func newLRUSStorage(cacheCtx *GitHubCacheContext, size int) Storage {
	globalGitHubCache.lock.Lock()
	defer globalGitHubCache.lock.Unlock()
	// Generate a unique key for this cache context
	cacheContextKey := "anonymous"
	if cacheCtx.AppSecretName != "" {
		cacheContextKey = "app/" + cacheCtx.AppSecretName
	} else if cacheCtx.TokenRef != nil {
		cacheContextKey = fmt.Sprintf("token/%s/%s", cacheCtx.TokenRef.SecretName, cacheCtx.TokenRef.Key)
	}
	if storage, exists := globalGitHubCache.storages[cacheContextKey]; exists {
		return storage
	}
	log.WithFields(log.Fields{
		"key": cacheContextKey,
	}).Debugf("Creating new GitHub Cache in memory %d size", size)
	globalGitHubStorageMetrics.StorageItemsEvicted.WithLabelValues(cacheContextKey).Add(0) // Initialize metric with zero value
	storage := Storage{
		key:  cacheContextKey,
		lock: &sync.RWMutex{},
		lruMap: lru.NewWithEvictionFunc(size, func(_ lru.Key, _ any) {
			globalGitHubStorageMetrics.StorageItemsEvicted.WithLabelValues(cacheContextKey).Inc()
		}),
	}
	globalGitHubCache.storages[cacheContextKey] = storage
	return storage
}

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

func parseVaryHeaders(headers http.Header) ([]string, error) {
	// Vary = #( "*" / field-name ) from RFC 9110 Section 12.5.5
	// RFC 9111 Section 4.1 Calculating Cache Keys with the Vary Header Field
	// A stored response with a Vary header field value containing a member "*" always fails to match
	result := []string{}
	for _, val := range headers.Values("Vary") {
		if val == "*" {
			return []string{}, errors.New("cannot cache due to wildcard Vary header")
		}
		for _, field := range strings.Split(val, ",") {
			field = strings.TrimSpace(field)
			if field != "" {
				result = append(result, http.CanonicalHeaderKey(field))
			}
		}
	}
	return result, nil
}

func isSameVaryHeader(req *http.Request, varyHeaders []string, varyHeadersValue map[string]string) bool {
	// Check if the hashed_token and Accept headers are the same
	for _, header := range varyHeaders {
		if header == "Authorization" {
			if gh_hash_token.HashToken(req.Header.Get(header)) != varyHeadersValue[header] {
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
		// Make a shallow copy of the *http.Response as we're going to modify the headers for storage
		cacheResp := *resp
		cacheResp.Header = maps.Clone(resp.Header)

		// Remove excluded headers from the cached response
		for _, header := range ExcludedCacheHeaders {
			cacheResp.Header.Del(header)
		}

		// Store the cached response body as bytes
		// Per the storage contract, they will restore the Body/ContentLength after consumption
		if err := t.storage.Put(req.Context(), req, &cacheResp); err != nil {
			return resp, fmt.Errorf("(Storage).Put failed: %w", err)
		}

		// Restore the copied response body with the cached body
		resp.Body = cacheResp.Body
		resp.ContentLength = cacheResp.ContentLength
	}

	return resp, nil
}

func NewGitHubCacheTransport(cacheCtx *GitHubCacheContext, size int, parent http.RoundTripper) *GitHubCacheTransport {
	storage := newLRUSStorage(cacheCtx, size)
	if parent == nil {
		parent = http.DefaultTransport
	}
	return &GitHubCacheTransport{
		parent:  parent,
		storage: storage,
	}
}

func NewGitHubCache(cacheCtx *GitHubCacheContext, size int, parent http.RoundTripper) *http.Client {
	return &http.Client{
		Transport: NewGitHubCacheTransport(cacheCtx, size, parent),
	}
}
