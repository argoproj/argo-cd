package glob

import (
	"sync"

	"github.com/gobwas/glob"
	"github.com/golang/groupcache/lru"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"
)

const (
	// DefaultGlobCacheSize is the default maximum number of compiled glob patterns to cache.
	// This limit prevents memory exhaustion from untrusted RBAC patterns.
	// 10,000 patterns should be sufficient for most deployments while limiting
	// memory usage to roughly ~10MB (assuming ~1KB per compiled pattern).
	DefaultGlobCacheSize = 10000
)

type compileFn func(pattern string, separators ...rune) (glob.Glob, error)

var (
	// globCache stores compiled glob patterns using an LRU cache with bounded size.
	// This prevents memory exhaustion from potentially untrusted RBAC patterns
	// while still providing significant performance benefits.
	globCache     *lru.Cache
	globCacheLock sync.Mutex
	compileGroup  singleflight.Group
	compileGlob   compileFn = glob.Compile
)

func init() {
	globCache = lru.New(DefaultGlobCacheSize)
}

// SetCacheSize reinitializes the glob cache with the given maximum number of entries.
// This should be called early during process startup, before concurrent access begins.
func SetCacheSize(maxEntries int) {
	globCacheLock.Lock()
	defer globCacheLock.Unlock()
	globCache = lru.New(maxEntries)
}

// globCacheKey uniquely identifies a compiled glob pattern.
// The same pattern compiled with different separators produces different globs,
// so both fields are needed.
type globCacheKey struct {
	Pattern    string
	Separators string
}

func cacheKey(pattern string, separators ...rune) globCacheKey {
	return globCacheKey{Pattern: pattern, Separators: string(separators)}
}

// getOrCompile returns a cached compiled glob pattern, compiling and caching it if necessary.
// Cache hits are a brief lock + map lookup. On cache miss, singleflight ensures each
// unique pattern is compiled exactly once even under concurrent access, while unrelated
// patterns compile in parallel.
// lru.Cache.Get() promotes entries (mutating), so a Mutex is used rather than RWMutex.
func getOrCompile(pattern string, compiler compileFn, separators ...rune) (glob.Glob, error) {
	key := cacheKey(pattern, separators...)

	globCacheLock.Lock()
	if cached, ok := globCache.Get(key); ok {
		globCacheLock.Unlock()
		return cached.(glob.Glob), nil
	}
	globCacheLock.Unlock()

	sfKey := key.Pattern + "\x00" + key.Separators
	v, err, _ := compileGroup.Do(sfKey, func() (any, error) {
		compiled, err := compiler(pattern, separators...)
		if err != nil {
			return nil, err
		}
		globCacheLock.Lock()
		globCache.Add(key, compiled)
		globCacheLock.Unlock()
		return compiled, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(glob.Glob), nil
}

// Match tries to match a text with a given glob pattern.
// Compiled glob patterns are cached for performance.
func Match(pattern, text string, separators ...rune) bool {
	compiled, err := getOrCompile(pattern, compileGlob, separators...)
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
	compiled, err := getOrCompile(pattern, compileGlob, separators...)
	if err != nil {
		return false, err
	}
	return compiled.Match(text), nil
}
