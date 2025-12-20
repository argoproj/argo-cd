package glob

import (
	"sync"

	"github.com/gobwas/glob"
	log "github.com/sirupsen/logrus"
)

// globMemo stores memoized compiled glob patterns using sync.Map.
// sync.Map is optimized for the "write once, read many" pattern which matches
// our use case: glob patterns are compiled once and matched millions of times
// during RBAC policy evaluation.
var globMemo sync.Map

// getOrCompile returns a memoized compiled glob pattern, compiling it if necessary.
func getOrCompile(pattern string, separators ...rune) (glob.Glob, error) {
	// Fast path: already memoized (lock-free read)
	if cached, ok := globMemo.Load(pattern); ok {
		return cached.(glob.Glob), nil
	}

	// Slow path: compile and memoize
	compiled, err := glob.Compile(pattern, separators...)
	if err != nil {
		return nil, err
	}

	// Store result. If another goroutine stored it first, that's fine -
	// both compiled results are equivalent for the same pattern.
	globMemo.LoadOrStore(pattern, compiled)
	return compiled, nil
}

// Match tries to match a text with a given glob pattern.
// Compiled glob patterns are memoized for performance.
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
// Compiled glob patterns are memoized for performance.
func MatchWithError(pattern, text string, separators ...rune) (bool, error) {
	compiled, err := getOrCompile(pattern, separators...)
	if err != nil {
		return false, err
	}
	return compiled.Match(text), nil
}

