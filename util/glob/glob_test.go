package glob

import (
	"fmt"
	"sync"
	"testing"

	extglob "github.com/gobwas/glob"
	"github.com/stretchr/testify/require"
)

// Test helpers - these access internal variables for testing purposes

// clearGlobCache clears the cached glob patterns for testing.
func clearGlobCache() {
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

// getCacheLen returns the number of cached patterns.
func getCacheLen() int {
	globCacheLock.Lock()
	defer globCacheLock.Unlock()
	return globCache.Len()
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
	clearGlobCache()

	pattern := "test*pattern"
	text := "testABCpattern"

	// First call should compile and cache
	result1 := Match(pattern, text)
	require.True(t, result1)

	// Verify pattern is cached
	require.True(t, isPatternCached(pattern), "pattern should be cached after first Match call")

	// Second call should use cached value
	result2 := Match(pattern, text)
	require.True(t, result2)

	// Results should be consistent
	require.Equal(t, result1, result2)
}

func Test_GlobCachingConcurrent(t *testing.T) {
	// Clear cache before test
	clearGlobCache()

	pattern := "concurrent*test"
	text := "concurrentABCtest"

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := Match(pattern, text)
			require.True(t, result)
		}()
	}

	wg.Wait()

	// Verify pattern is cached
	require.True(t, isPatternCached(pattern))
	require.Equal(t, 1, getCacheLen(), "should only have one cached entry for the pattern")
}

func Test_GlobCacheLRUEviction(t *testing.T) {
	// Clear cache before test
	clearGlobCache()

	// Fill cache beyond DefaultGlobCacheSize
	for i := 0; i < DefaultGlobCacheSize+100; i++ {
		pattern := fmt.Sprintf("pattern-%d-*", i)
		Match(pattern, "pattern-0-test")
	}

	// Cache size should be limited to DefaultGlobCacheSize
	require.Equal(t, DefaultGlobCacheSize, getCacheLen(), "cache size should be limited to DefaultGlobCacheSize")

	// The most recently used patterns should still be cached
	require.True(t, isPatternCached(fmt.Sprintf("pattern-%d-*", DefaultGlobCacheSize+99)), "most recent pattern should be cached")
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

	// With memoization: patterns are compiled once
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, pattern := range patterns {
			Match(pattern, text)
		}
	}
}
