package rbac

import (
	"errors"
	"regexp"
	"sync"
)

// regexCacheSize is the maximum number of compiled regular expressions to cache. Policy patterns can come from
// AppProject roles, so the cache is bounded to keep its memory use in check, as the glob cache in util/glob is. A
// compiled pattern takes roughly 1KB to 20KB depending on its complexity, so a full cache of short patterns holds
// about 10MB.
const regexCacheSize = 10000

// regexCache stores compiled policy patterns for the regex match mode. Enforcing one request looks up several patterns
// per policy line, on every API server goroutine at once, so lookups must not contend with each other: a sync.Map
// serves hits without taking a lock, where an LRU would need an exclusive lock to record each hit.
var regexCache = newCompiledRegexCache(regexCacheSize)

type compiledRegexCache struct {
	// entries maps a pattern (string) to its compiled form (*regexp.Regexp).
	entries sync.Map
	// addLock guards additions to entries, and with them size.
	addLock sync.Mutex
	// size is the number of entries.
	size int
	// maxEntries is the size at which the cache is emptied before adding another entry.
	maxEntries int
}

func newCompiledRegexCache(maxEntries int) *compiledRegexCache {
	return &compiledRegexCache{maxEntries: maxEntries}
}

// getOrCompile returns the compiled form of pattern, compiling and caching it on a miss. Patterns that fail to compile
// are not cached. Adding to a full cache empties it first: the patterns still in use are compiled again as they are
// looked up, which costs no more per pattern than not caching at all.
func (c *compiledRegexCache) getOrCompile(pattern string) (*regexp.Regexp, error) {
	if cached, ok := c.entries.Load(pattern); ok {
		return cached.(*regexp.Regexp), nil
	}

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	c.addLock.Lock()
	defer c.addLock.Unlock()
	// Another goroutine may have added the pattern while this one was compiling it.
	if cached, ok := c.entries.Load(pattern); ok {
		return cached.(*regexp.Regexp), nil
	}
	if c.size >= c.maxEntries {
		c.entries.Clear()
		c.size = 0
	}
	c.entries.Store(pattern, compiled)
	c.size++
	return compiled, nil
}

// regexMatchFunc reports whether the request value (the first argument) matches the policy pattern (the second
// argument). It behaves like casbin's util.RegexMatchFunc, which compiles the pattern on every call, but caches the
// compiled pattern. Invalid arguments and invalid patterns return an error, which fails the enforcement.
func regexMatchFunc(args ...any) (any, error) {
	if len(args) != 2 {
		return false, errors.New("regexMatch: expected 2 arguments")
	}
	val, ok := args[0].(string)
	if !ok {
		return false, errors.New("regexMatch: request value must be a string")
	}
	pattern, ok := args[1].(string)
	if !ok {
		return false, errors.New("regexMatch: policy pattern must be a string")
	}

	compiled, err := regexCache.getOrCompile(pattern)
	if err != nil {
		return false, err
	}
	return compiled.MatchString(val), nil
}
