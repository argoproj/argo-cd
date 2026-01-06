package services

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"sync"

	gh_hash_token "github.com/bored-engineer/github-conditional-http-transport"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"k8s.io/utils/lru"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

var VaryHeaders = []string{
	"Accept-Encoding",
	"Accept",
	"Authorization",
}

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

type cachedResponse struct {
	Response *http.Response
	Body     []byte
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

func (s Storage) Get(_ context.Context, u *url.URL) (*http.Response, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	body, ok := s.lruMap.Get(u.String())
	if !ok {
		return nil, nil
	}
	bodyCached, valid := body.(cachedResponse)
	if !valid {
		return nil, nil
	}
	resp := *bodyCached.Response
	resp.Body = io.NopCloser(bytes.NewReader(bodyCached.Body))
	return &resp, nil
}

func (s Storage) Put(_ context.Context, u *url.URL, resp *http.Response) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("(*http.Response).Body.Read failed: %w", err)
	}
	if err := resp.Body.Close(); err != nil {
		return fmt.Errorf("(*http.Response).Body.Close failed: %w", err)
	}
	resp.Body = nil
	s.lruMap.Add(u.String(), cachedResponse{
		Response: resp,
		Body:     body,
	})
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

func NewLRUSStorage(cacheCtx *GitHubCacheContext, size int) Storage {
	globalGitHubCache.lock.Lock()
	defer globalGitHubCache.lock.Unlock()
	// Generate a unique key for this cache context
	cacheContextKey := "anonymous"
	if cacheCtx.AppSecretName != "" {
		cacheContextKey = "app" + cacheCtx.AppSecretName
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
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		return false
	}
	if req.Header.Get("Range") != "" {
		return false
	}
	if req.URL.Path == "/rate_limit" || req.URL.Path == "/api/v3/rate_limit" {
		return false
	}
	return true
}

func isSameCachedHeader(req *http.Request, resp *http.Response) bool {
	// Check if the hashed_token and Accept headers are the same
	for _, header := range VaryHeaders {
		if header == "Authorization" {
			if gh_hash_token.HashToken(req.Header.Get(header)) != resp.Header.Get("X-Varied-"+header) {
				return false
			}
		} else {
			if req.Header.Get(header) != resp.Header.Get("X-Varied-"+header) {
				return false
			}
		}
	}
	return true
}

func (t *GitHubCacheTransport) cacheResponse(req *http.Request, resp *http.Response) (*http.Response, error) {
	// We can only cache successful responses
	if resp.StatusCode != http.StatusOK {
		return resp, nil
	}

	// If there was no ETag, we can't cache it
	if resp.Header.Get("Etag") == "" {
		return resp, nil
	}

	// Read the response body into memory
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, fmt.Errorf("(*http.Response).Body.Read failed: %w", err)
	}
	if err := resp.Body.Close(); err != nil {
		return resp, fmt.Errorf("(*http.Response).Body.Close failed: %w", err)
	}

	// Make a shallow copy of the *http.Response as we're going to modify the body/headers
	cacheResp := *resp
	cacheResp.Body = io.NopCloser(bytes.NewReader(body))
	cacheResp.ContentLength = int64(len(body))
	cacheResp.Header = maps.Clone(resp.Header)

	// Remove excluded headers from the cached response
	for _, header := range ExcludedCacheHeaders {
		cacheResp.Header.Del(header)
	}

	// Similar to httpcache, inject fake X-Varied-<header> "response" headers
	for _, header := range VaryHeaders {
		if vals := req.Header.Values(header); len(vals) > 0 {
			if header == "Authorization" {
				vals = []string{gh_hash_token.HashToken(vals[0])} // Don't leak/cache the raw authentication token
			}
			cacheResp.Header["X-Varied-"+header] = vals
		}
	}

	if err := t.storage.Put(req.Context(), req.URL, &cacheResp); err != nil {
		return resp, fmt.Errorf("(Storage).Put failed: %w", err)
	}

	// Replace the response body with the cached body
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	return resp, nil
}

func (t *GitHubCacheTransport) injectEtagHeader(req *http.Request) (resp *http.Response, err error) {
	// Check if we have a cached response available in the storage for this URL, else bail
	resp, err = t.storage.Get(req.Context(), req.URL)
	if err != nil {
		return nil, fmt.Errorf("(Storage).Get failed: %w", err)
	} else if resp == nil {
		return nil, nil
	}
	defer func() {
		// If we're not using the cached response, ensure we close the body
		// But first, read it to completion to ensure the connection can be re-used
		if resp == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
	}()

	// If we're using the same header, we can directly use the cached etag
	if isSameCachedHeader(req, resp) {
		req.Header.Set("If-None-Match", resp.Header.Get("Etag"))
		return resp, nil
	}

	// We'll have to read the cached response body into memory to calculate the ETag
	var buf bytes.Buffer

	// Calculate the _expected_ ETag from the _input_ headers but the cached body
	h := gh_hash_token.Hash(req.Header, VaryHeaders)
	if _, err := io.Copy(io.MultiWriter(&buf, h), resp.Body); err != nil {
		return nil, fmt.Errorf("(*http.Response).Body.Read failed: %w", err)
	}
	if err := resp.Body.Close(); err != nil {
		return nil, fmt.Errorf("(*http.Response).Body.Close failed: %w", err)
	}

	// Add the If-None-Match header to the request with that calculated ETag
	req.Header.Set("If-None-Match", `"`+hex.EncodeToString(h.Sum(nil))+`"`)

	// Make the next "read" from the cached body use the bytes we just read
	resp.Body = io.NopCloser(&buf)
	resp.ContentLength = int64(buf.Len())

	return resp, nil
}

func (t *GitHubCacheTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// If the request is not cacheable, just pass it through to the parent RoundTripper
	if !cacheable(req) {
		return t.parent.RoundTrip(req)
	}

	// Attempt to fetch from storage and inject the cache headers to the request
	cachedResp, err := t.injectEtagHeader(req)
	if err != nil {
		return nil, err
	}
	if cachedResp != nil {
		// We attempted to use a cached response
		globalGitHubCacheMetrics.CacheRequestTotal.WithLabelValues(t.storage.key).Inc()
	}

	// Perform the upstream request
	resp, err := t.parent.RoundTrip(req)
	if err != nil {
		if cachedResp != nil {
			cachedResp.Body.Close()
		}
		return nil, err
	}

	// If the upstream response is 304 Not Modified, we can use the cached response
	if cachedResp != nil {
		if resp.StatusCode == http.StatusNotModified {
			// We hit the cache properly
			globalGitHubCacheMetrics.CacheRequestHits.WithLabelValues(t.storage.key).Inc()
			// Consume the rest of the response body to ensure the connection can be re-used
			if _, err := io.Copy(io.Discard, resp.Body); err != nil {
				cachedResp.Body.Close()
				return nil, fmt.Errorf("(*http.Response).Body.Read failed: %w", err)
			}
			if err := resp.Body.Close(); err != nil {
				cachedResp.Body.Close()
				return nil, fmt.Errorf("(*http.Response).Body.Close failed: %w", err)
			}

			// Copy in any cached headers that are not already set
			for key, vals := range cachedResp.Header {
				if strings.HasPrefix(key, "X-Varied-") {
					continue // Skip the X-Varied-* headers, they are "internal" to the cache
				}
				if _, ok := resp.Header[key]; !ok {
					resp.Header[key] = vals
				}
			}

			// Copy the body and status from the cache
			resp.StatusCode = cachedResp.StatusCode
			resp.Status = cachedResp.Status
			resp.Body = cachedResp.Body
			resp.ContentLength = cachedResp.ContentLength

			return resp, nil
		}
		// Discard the cached response body, it wasn't valid/used
		_, _ = io.Copy(io.Discard, cachedResp.Body)
		_ = cachedResp.Body.Close()
	}

	// We got a valid response, try to cache it
	resp, err = t.cacheResponse(req, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func NewGitHubCacheTransport(storage Storage, parent http.RoundTripper) *GitHubCacheTransport {
	if parent == nil {
		parent = http.DefaultTransport
	}
	return &GitHubCacheTransport{
		parent:  parent,
		storage: storage,
	}
}

func NewGitHubCache(cacheCtx *GitHubCacheContext, size int, parent http.RoundTripper) *http.Client {
	storage := NewLRUSStorage(cacheCtx, size)
	return &http.Client{
		Transport: NewGitHubCacheTransport(storage, parent),
	}
}
