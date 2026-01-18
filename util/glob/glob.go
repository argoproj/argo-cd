package glob

import (
	"math"
	"sync"

	"github.com/gobwas/glob"
	"github.com/golang/groupcache/lru"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/util/env"
)

const (
	// EnvGlobCacheSize is the environment variable name for configuring glob cache size.
	EnvGlobCacheSize = "ARGOCD_GLOB_CACHE_SIZE"

	// DefaultGlobCacheSize is the default maximum number of compiled glob patterns to cache.
	// This limit prevents memory exhaustion from untrusted RBAC patterns.
	// 10,000 patterns should be sufficient for most deployments while limiting
	// memory usage to roughly ~10MB (assuming ~1KB per compiled pattern).
	DefaultGlobCacheSize = 10000
)

var (
	// globCache stores compiled glob patterns using an LRU cache with bounded size.
	// This prevents memory exhaustion from potentially untrusted RBAC patterns
	// while still providing significant performance benefits.
	globCache     *lru.Cache
	globCacheLock sync.Mutex
	compileGlob   = glob.Compile
)

func init() {
	globCache = lru.New(env.ParseNumFromEnv(EnvGlobCacheSize, DefaultGlobCacheSize, 1, math.MaxInt))
}

// getOrCompile returns a cached compiled glob pattern, compiling and caching it if necessary.
func getOrCompile(pattern string, separators ...rune) (glob.Glob, error) {
	globCacheLock.Lock()
	defer globCacheLock.Unlock()

	// Check cache first
	if cached, ok := globCache.Get(pattern); ok {
		return cached.(glob.Glob), nil
	}

	// Compile and cache
	compiled, err := compileGlob(pattern, separators...)
	if err != nil {
		return nil, err
	}

	globCache.Add(pattern, compiled)
	return compiled, nil
}

// Match tries to match a text with a given glob pattern.
// Compiled glob patterns are cached for performance.
func Match(pattern, text string, separators ...rune) bool {
	compiled, err := getOrCompile(pattern, separators...)
	if err != nil {
		log.Warnf("failed to compile pattern %s due to error %v", pattern, err)
		return false
	}
	return compiled.Match(text)
}

// MatchWithError tries to match a text with a given glob pattern.
// Returns error if the glob pattern fails to compile.
// Compiled glob patterns are cached for performance.
func MatchWithError(pattern, text string, separators ...rune) (bool, error) {
	compiled, err := getOrCompile(pattern, separators...)
	if err != nil {
		return false, err
	}
	return compiled.Match(text), nil
}
