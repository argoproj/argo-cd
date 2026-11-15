package regex

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		text    string
		want    bool
	}{
		{name: "simple match", pattern: "^team-[a-z]+$", text: "team-foo", want: true},
		{name: "simple mismatch", pattern: "^team-[a-z]+$", text: "team-1", want: false},
		{name: "alternation", pattern: "^(dev|staging)$", text: "staging", want: true},
		{name: "negative lookahead", pattern: "^(?!kube-).*$", text: "kube-system", want: false},
		{name: "negative lookahead permits", pattern: "^(?!kube-).*$", text: "default", want: true},
		{name: "lookbehind", pattern: "(?<!test)-ns$", text: "prod-ns", want: true},
		{name: "backreference", pattern: "^(a)\\1$", text: "aa", want: true},
		{name: "inline ignore case", pattern: "(?i)^PROD$", text: "prod", want: true},
		{name: "invalid pattern does not match", pattern: "([a-", text: "a", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Match(tt.pattern, tt.text))
		})
	}
}

// Regression test for CVE-2026-77299 / GHSA-wq9v-j77v-qr26: a short pattern
// applying a large fixed-count quantifier to a zero-width subexpression must not
// cause unbounded memory allocation. Patterns that exceed the backtracking limit
// must fail closed (no match).
func TestMatch_ZeroWidthQuantifierMemoryAmplification(t *testing.T) {
	const maxAllocBytes = 64 << 20 // vulnerable versions allocate ~1 GiB here

	tests := []struct {
		pattern string
		want    bool
	}{
		{pattern: "(?:){5000000}", want: true},
		{pattern: "(?:|){5000000}", want: true},
		{pattern: "(){5000000}", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			got := Match(tt.pattern, "")
			runtime.ReadMemStats(&after)

			assert.Equal(t, tt.want, got)
			assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(maxAllocBytes),
				"matching %q allocated too much memory", tt.pattern)
		})
	}
}
