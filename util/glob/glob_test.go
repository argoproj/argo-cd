package glob

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	extglob "github.com/gobwas/glob"
	"github.com/stretchr/testify/require"
)

// Test helpers - these access internal variables for testing purposes

// resetGlobCacheForTest clears the cached glob patterns for testing.
func resetGlobCacheForTest() {
	globCacheLock.Lock()
	defer globCacheLock.Unlock()
	globCache.Clear()
}

// isPatternCached returns true if the pattern is cached.
func isPatternCached(pattern string) bool {
	globCacheLock.Lock()
	defer globCacheLock.Unlock()
	_, ok := globCache.Get(pattern)
	return ok
}

// globCacheLen returns the number of cached patterns.
func globCacheLen() int {
	globCacheLock.Lock()
	defer globCacheLock.Unlock()
	return globCache.Len()
}

func matchWithCompiler(pattern, text string, compiler compileFn, separators ...rune) bool {
	compiled, err := getOrCompile(pattern, compiler, separators...)
	if err != nil {
		return false
	}
	return compiled.Match(text)
}

func countingCompiler() (compileFn, *int32) {
	var compileCount int32
	compiler := func(pattern string, separators ...rune) (extglob.Glob, error) {
		atomic.AddInt32(&compileCount, 1)
		return extglob.Compile(pattern, separators...)
	}
	return compiler, &compileCount
}

func Test_Match(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		pattern string
		result  bool
	}{
		{"Exact match", "hello", "hello", true},
		{"Non-match exact", "hello", "hell", false},
		{"Long glob match", "hello", "hell*", true},
		{"Short glob match", "hello", "h*", true},
		{"Glob non-match", "hello", "e*", false},
		{"Invalid pattern", "e[[a*", "e[[a*", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := Match(tt.pattern, tt.input)
			require.Equal(t, tt.result, res)
		})
	}
}

func Test_MatchList(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		list         []string
		patternMatch string
		result       bool
	}{
		{"Exact name in list", "test", []string{"test"}, EXACT, true},
		{"Exact name not in list", "test", []string{"other"}, EXACT, false},
		{"Exact name not in list, multiple elements", "test", []string{"some", "other"}, EXACT, false},
		{"Exact name not in list, list empty", "test", []string{}, EXACT, false},
		{"Exact name not in list, empty element", "test", []string{""}, EXACT, false},
		{"Glob name in list, but exact wanted", "test", []string{"*"}, EXACT, false},
		{"Glob name in list with simple wildcard", "test", []string{"*"}, GLOB, true},
		{"Glob name in list without wildcard", "test", []string{"test"}, GLOB, true},
		{"Glob name in list, multiple elements", "test", []string{"other*", "te*"}, GLOB, true},
		{"match everything but specified word: fail", "disallowed", []string{"/^((?!disallowed).)*$/"}, REGEXP, false},
		{"match everything but specified word: pass", "allowed", []string{"/^((?!disallowed).)*$/"}, REGEXP, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := MatchStringInList(tt.list, tt.input, tt.patternMatch)
			require.Equal(t, tt.result, res)
		})
	}
}

func Test_MatchWithError(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		pattern     string
		result      bool
		expectedErr string
	}{
		{"Exact match", "hello", "hello", true, ""},
		{"Non-match exact", "hello", "hell", false, ""},
		{"Long glob match", "hello", "hell*", true, ""},
		{"Short glob match", "hello", "h*", true, ""},
		{"Glob non-match", "hello", "e*", false, ""},
		{"Invalid pattern", "e[[a*", "e[[a*", false, "unexpected end of input"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := MatchWithError(tt.pattern, tt.input)
			require.Equal(t, tt.result, res)
			if tt.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.expectedErr)
			}
		})
	}
}

func Test_GlobCaching(t *testing.T) {
	// Clear cache before test
	resetGlobCacheForTest()

	compiler, compileCount := countingCompiler()

	pattern := "test*pattern"
	text := "testABCpattern"

	// First call should compile and cache
	result1 := matchWithCompiler(pattern, text, compiler)
	require.True(t, result1)

	// Verify pattern is cached
	require.True(t, isPatternCached(pattern), "pattern should be cached after first Match call")

	// Second call should use cached value
	result2 := matchWithCompiler(pattern, text, compiler)
	require.True(t, result2)

	// Results should be consistent
	require.Equal(t, result1, result2)
	require.Equal(t, int32(1), atomic.LoadInt32(compileCount), "glob should compile once for the cached pattern")
}

func Test_GlobCachingConcurrent(t *testing.T) {
	// Clear cache before test
	resetGlobCacheForTest()

	compiler, compileCount := countingCompiler()

	pattern := "concurrent*test"
	text := "concurrentABCtest"

	var wg sync.WaitGroup
	numGoroutines := 100
	errChan := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := matchWithCompiler(pattern, text, compiler)
			if !result {
				errChan <- errors.New("expected match to return true")
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// Check for any errors from goroutines
	for err := range errChan {
		t.Error(err)
	}

	// Verify pattern is cached
	require.True(t, isPatternCached(pattern))
	require.Equal(t, 1, globCacheLen(), "should only have one cached entry for the pattern")
	require.Equal(t, int32(1), atomic.LoadInt32(compileCount), "glob should compile once for the cached pattern")
}

func Test_GlobCacheLRUEviction(t *testing.T) {
	// Clear cache before test
	resetGlobCacheForTest()

	// Fill cache beyond DefaultGlobCacheSize
	for i := 0; i < DefaultGlobCacheSize+100; i++ {
		pattern := fmt.Sprintf("pattern-%d-*", i)
		Match(pattern, "pattern-0-test")
	}

	// Cache size should be limited to DefaultGlobCacheSize
	require.Equal(t, DefaultGlobCacheSize, globCacheLen(), "cache size should be limited to DefaultGlobCacheSize")

	// The oldest patterns should be evicted
	oldest := fmt.Sprintf("pattern-%d-*", 0)
	require.False(t, isPatternCached(oldest), "oldest pattern should be evicted")

	// The most recently used patterns should still be cached
	require.True(t, isPatternCached(fmt.Sprintf("pattern-%d-*", DefaultGlobCacheSize+99)), "most recent pattern should be cached")
}

func Test_InvalidGlobNotCached(t *testing.T) {
	// Clear cache before test
	resetGlobCacheForTest()

	invalidPattern := "e[[a*"
	text := "test"

	// Match should return false for invalid pattern
	result := Match(invalidPattern, text)
	require.False(t, result)

	// Invalid patterns should NOT be cached
	require.False(t, isPatternCached(invalidPattern), "invalid pattern should not be cached")

	// Also test with MatchWithError
	_, err := MatchWithError(invalidPattern, text)
	require.Error(t, err)

	// Still should not be cached after MatchWithError
	require.False(t, isPatternCached(invalidPattern), "invalid pattern should not be cached after MatchWithError")
}

// BenchmarkMatch_WithCache benchmarks Match with caching (cache hit)
func BenchmarkMatch_WithCache(b *testing.B) {
	pattern := "proj:*/app-*"
	text := "proj:myproject/app-frontend"

	// Warm up the cache
	Match(pattern, text)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Match(pattern, text)
	}
}

// BenchmarkMatch_WithoutCache simulates the OLD behavior (compile every time)
func BenchmarkMatch_WithoutCache(b *testing.B) {
	text := "proj:myproject/app-frontend"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use unique pattern each iteration to force recompilation (simulates no cache)
		pattern := fmt.Sprintf("proj:*/app-%d", i)
		Match(pattern, text)
	}
}

// BenchmarkGlobCompile measures raw glob.Compile cost
func BenchmarkGlobCompile(b *testing.B) {
	pattern := "proj:*/app-*"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = extglob.Compile(pattern)
	}
}

// BenchmarkMatch_RBACSimulation simulates real RBAC evaluation scenario
// 50 policies × 1 app = what happens per application in List
func BenchmarkMatch_RBACSimulation(b *testing.B) {
	patterns := make([]string, 50)
	for i := 0; i < 50; i++ {
		patterns[i] = fmt.Sprintf("proj:team-%d/*", i)
	}
	text := "proj:team-25/my-app"

	// With caching: patterns are compiled once
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, pattern := range patterns {
			Match(pattern, text)
		}
	}
}
