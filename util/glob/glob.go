package glob

import (
	"sync"

	"github.com/gobwas/glob"
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

type compileFn func(pattern string, separators ...rune) (*glob.Pattern, error)

var (
	// globCache stores compiled glob patterns in a cache with bounded size.
	// This prevents memory exhaustion from potentially untrusted RBAC patterns
	// while still providing significant performance benefits.
	globCache    = &patternCache{maxEntries: DefaultGlobCacheSize}
	compileGroup singleflight.Group
	compileGlob  compileFn = glob.Compile
)

// patternCache is a bounded cache of compiled glob patterns.
// Evaluating one RBAC request looks up several patterns per policy line, on every
// API server goroutine at once, so hits must not contend with each other: a sync.Map
// serves them without taking a lock, where an LRU needs an exclusive lock to record each hit.
// Without an access order there is nothing to evict by, so adding to a full cache empties it
// first. The patterns still in use are compiled again as they are looked up.
type patternCache struct {
	// entries maps a globCacheKey to its compiled pattern (*glob.Pattern).
	entries sync.Map
	// lock guards additions to entries, and with them size and maxEntries.
	lock sync.Mutex
	// size is the number of entries.
	size int
	// maxEntries is the size at which the cache is emptied before adding another entry.
	// Zero means no limit and a negative value disables caching, as they did when the cache was an LRU.
	maxEntries int
}

func (c *patternCache) get(key globCacheKey) (*glob.Pattern, bool) {
	cached, ok := c.entries.Load(key)
	if !ok {
		return nil, false
	}
	return cached.(*glob.Pattern), true
}

func (c *patternCache) add(key globCacheKey, compiled *glob.Pattern) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.maxEntries < 0 {
		return
	}
	if _, ok := c.entries.Load(key); ok {
		return
	}
	if c.maxEntries > 0 && c.size >= c.maxEntries {
		c.entries.Clear()
		c.size = 0
	}
	c.entries.Store(key, compiled)
	c.size++
}

// reset empties the cache and sets its maximum number of entries.
func (c *patternCache) reset(maxEntries int) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.entries.Clear()
	c.size = 0
	c.maxEntries = maxEntries
}

// SetCacheSize reinitializes the glob cache with the given maximum number of entries.
// This should be called early during process startup, before concurrent access begins.
func SetCacheSize(maxEntries int) {
	globCache.reset(maxEntries)
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
// Cache hits take no lock. On cache miss, singleflight ensures each
// unique pattern is compiled exactly once even under concurrent access, while unrelated
// patterns compile in parallel.
func getOrCompile(pattern string, compiler compileFn, separators ...rune) (*glob.Pattern, error) {
	key := cacheKey(pattern, separators...)

	if cached, ok := globCache.get(key); ok {
		return cached, nil
	}

	sfKey := key.Pattern + "\x00" + key.Separators
	v, err, _ := compileGroup.Do(sfKey, func() (any, error) {
		// Double-check cache: another singleflight call may have populated it
		// between our cache check and this singleflight registration.
		if cached, ok := globCache.get(key); ok {
			return cached, nil
		}

		compiled, err := compiler(pattern, separators...)
		if err != nil {
			return nil, err
		}
		globCache.add(key, compiled)
		return compiled, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*glob.Pattern), nil
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
