package glob

import (
	"fmt"
	"sync"
	"testing"

	extglob "github.com/gobwas/glob"
	"github.com/stretchr/testify/require"
)

// Test helpers - these access internal variables for testing purposes

// clearGlobMemo clears the memoized glob patterns for testing.
func clearGlobMemo() {
	globMemo.Range(func(key, _ any) bool {
		globMemo.Delete(key)
		return true
	})
}

// isPatternMemoized returns true if the pattern is memoized.
func isPatternMemoized(pattern string) bool {
	_, ok := globMemo.Load(pattern)
	return ok
}

// getMemoSize returns the number of memoized patterns.
func getMemoSize() int {
	count := 0
	globMemo.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
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

func Test_GlobMemoization(t *testing.T) {
	// Clear memo before test
	clearGlobMemo()

	pattern := "test*pattern"
	text := "testABCpattern"

	// First call should compile and memoize
	result1 := Match(pattern, text)
	require.True(t, result1)

	// Verify pattern is memoized
	require.True(t, isPatternMemoized(pattern), "pattern should be memoized after first Match call")

	// Second call should use memoized value
	result2 := Match(pattern, text)
	require.True(t, result2)

	// Results should be consistent
	require.Equal(t, result1, result2)
}

func Test_GlobMemoizationConcurrent(t *testing.T) {
	// Clear memo before test
	clearGlobMemo()

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

	// Verify pattern is memoized
	require.True(t, isPatternMemoized(pattern))
	require.Equal(t, 1, getMemoSize(), "should only have one memoized entry for the pattern")
}

// BenchmarkMatch_WithMemoization benchmarks Match with memoization (cache hit)
func BenchmarkMatch_WithMemoization(b *testing.B) {
	pattern := "proj:*/app-*"
	text := "proj:myproject/app-frontend"

	// Warm up the cache
	Match(pattern, text)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Match(pattern, text)
	}
}

// BenchmarkMatch_WithoutMemoization simulates the OLD behavior (compile every time)
func BenchmarkMatch_WithoutMemoization(b *testing.B) {
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
