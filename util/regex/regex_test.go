package regex

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompileFilterSupportsLookaround(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		pattern string
		text    string
		want    bool
	}{
		{"negative lookahead excludes", `^(?!release/).*`, "release/1.0", false},
		{"negative lookahead includes", `^(?!release/).*`, "feature/one", true},
		{"lookbehind matches", `(?<=WIP: ).*`, "WIP: add feature", true},
		{"lookbehind does not match", `(?<=WIP: ).*`, "add feature", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			re, err := CompileFilter(tt.pattern)
			require.NoError(t, err)
			got, err := re.MatchString(tt.text)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Filter patterns were originally evaluated by Go's regexp package, so patterns written for it must keep their meaning.
func TestCompileFilterKeepsGoRegexpSemantics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		pattern string
		text    string
		want    bool
	}{
		{"\\d matches ASCII digits", `^feature-1\d{2}$`, "feature-123", true},
		{"\\d does not match other Unicode digits", `^feature-1\d{2}$`, "feature-1١٢", false},
		{"\\w does not match non-ASCII letters", `^\w+$`, "café", false},
		{"POSIX character classes", `^[[:alpha:]]+$`, "abc", true},
		{"$ does not match before a trailing newline", `^a$`, "a\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			re, err := CompileFilter(tt.pattern)
			require.NoError(t, err)
			got, err := re.MatchString(tt.text)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCompileFilterAcceptsNamedGroups(t *testing.T) {
	t.Parallel()
	re, err := CompileFilter(`^(?P<type>feat|fix): .*`)
	require.NoError(t, err)
	got, err := re.MatchString("feat: add thing")
	require.NoError(t, err)
	assert.True(t, got)
}

// regexp2 compiles \Q...\E in RE2 mode but never matches it, so it must fail loudly instead of silently matching nothing.
func TestCompileFilterRejectsLiteralQuoting(t *testing.T) {
	t.Parallel()
	_, err := CompileFilter(`^\Qa.b\E$`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `\Q`)

	// An escaped backslash followed by Q is a literal backslash and a literal Q, not a quote sequence.
	re, err := CompileFilter(`^a\\Qb$`)
	require.NoError(t, err)
	got, err := re.MatchString(`a\Qb`)
	require.NoError(t, err)
	assert.True(t, got)
}

func TestHasLiteralQuoting(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		pattern string
		want    bool
	}{
		{"empty pattern", ``, false},
		{"plain pattern", `^feature/.*$`, false},
		{"quote pair", `\Qa.b\E`, true},
		{"quote at the start", `\Qa`, true},
		{"quote in the middle", `a\Qb`, true},
		{"Q without a backslash", `aQb`, false},
		{"lowercase q", `a\qb`, false},
		{"escaped backslash then Q", `a\\Qb`, false},
		{"escaped backslash then quote", `a\\\Qb`, true},
		{"two escaped backslashes then Q", `a\\\\Qb`, false},
		{"trailing backslash", `abc\`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, hasLiteralQuoting(tt.pattern))
		})
	}
}

func TestCompileFilterRejectsMalformedPattern(t *testing.T) {
	t.Parallel()
	_, err := CompileFilter("(")
	require.Error(t, err)
}

// A backtracking engine can take exponential time on some pattern/input pairs; matching must be bounded and report
// the overrun as an error instead of running unbounded.
func TestCompileFilterBoundsMatchTime(t *testing.T) {
	t.Parallel()
	re, err := CompileFilter(`^(a+)+$`)
	require.NoError(t, err)
	_, err = re.MatchString(strings.Repeat("a", 28) + "!")
	require.Error(t, err)
}
