package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func loadOpts(t *testing.T, opts string) {
	t.Helper()
	t.Setenv("ARGOCD_OPTS", opts)
	assert.NoError(t, LoadFlags())
}

func loadInvalidOpts(t *testing.T, opts string) {
	t.Helper()
	t.Setenv("ARGOCD_OPTS", opts)
	assert.Error(t, LoadFlags())
}

func TestNilOpts(t *testing.T) {
	assert.Equal(t, "foo", GetFlag("foo", "foo"))
}

func TestEmptyOpts(t *testing.T) {
	loadOpts(t, "")

	assert.Equal(t, "foo", GetFlag("foo", "foo"))
}

func TestRubbishOpts(t *testing.T) {
	loadInvalidOpts(t, "rubbish")
}

func TestBoolFlag(t *testing.T) {
	loadOpts(t, "--foo")

	assert.True(t, GetBoolFlag("foo"))
}

func TestBoolFlagAtStart(t *testing.T) {
	loadOpts(t, "--foo --bar baz")

	assert.True(t, GetBoolFlag("foo"))
}

func TestBoolFlagInMiddle(t *testing.T) {
	loadOpts(t, "--bar baz --foo --qux")

	assert.True(t, GetBoolFlag("foo"))
}

func TestBooleanFlagAtEnd(t *testing.T) {
	loadOpts(t, "--bar baz --foo")

	assert.True(t, GetBoolFlag("foo"))
}

func TestIntFlag(t *testing.T) {
	loadOpts(t, "--foo 2")

	assert.Equal(t, 2, GetIntFlag("foo", 0))
}

func TestIntFlagAtStart(t *testing.T) {
	loadOpts(t, "--foo 2 --bar baz")

	assert.Equal(t, 2, GetIntFlag("foo", 0))
}

func TestIntFlagInMiddle(t *testing.T) {
	loadOpts(t, "--bar baz --foo 2 --qux")

	assert.Equal(t, 2, GetIntFlag("foo", 0))
}

func TestIntFlagAtEnd(t *testing.T) {
	loadOpts(t, "--bar baz --foo 2")

	assert.Equal(t, 2, GetIntFlag("foo", 0))
}

func TestStringSliceFlag(t *testing.T) {
	loadOpts(t, "--header='Content-Type: application/json; charset=utf-8,Strict-Transport-Security: max-age=31536000'")
	headers := GetStringSliceFlag("header", []string{})

	assert.Len(t, headers, 2)
	assert.Equal(t, "Content-Type: application/json; charset=utf-8", headers[0])
	assert.Equal(t, "Strict-Transport-Security: max-age=31536000", headers[1])
}

func TestStringSliceFlagAtStart(t *testing.T) {
	loadOpts(t, "--header='Strict-Transport-Security: max-age=31536000' --bar baz")
	headers := GetStringSliceFlag("header", []string{})

	assert.Len(t, headers, 1)
	assert.Equal(t, "Strict-Transport-Security: max-age=31536000", headers[0])
}

func TestStringSliceFlagInMiddle(t *testing.T) {
	loadOpts(t, "--bar baz --header='Strict-Transport-Security: max-age=31536000' --qux")
	headers := GetStringSliceFlag("header", []string{})

	assert.Len(t, headers, 1)
	assert.Equal(t, "Strict-Transport-Security: max-age=31536000", headers[0])
}

func TestStringSliceFlagAtEnd(t *testing.T) {
	loadOpts(t, "--bar baz --header='Strict-Transport-Security: max-age=31536000'")
	headers := GetStringSliceFlag("header", []string{})

	assert.Len(t, headers, 1)
	assert.Equal(t, "Strict-Transport-Security: max-age=31536000", headers[0])
}

func TestFlagAtStart(t *testing.T) {
	loadOpts(t, "--foo bar")

	assert.Equal(t, "bar", GetFlag("foo", ""))
}

func TestFlagInTheMiddle(t *testing.T) {
	loadOpts(t, "--baz --foo bar --qux")

	assert.Equal(t, "bar", GetFlag("foo", ""))
}

func TestFlagAtTheEnd(t *testing.T) {
	loadOpts(t, "--baz --foo bar")

	assert.Equal(t, "bar", GetFlag("foo", ""))
}

func TestFlagWithSingleQuotes(t *testing.T) {
	loadOpts(t, "--foo 'bar baz'")

	assert.Equal(t, "bar baz", GetFlag("foo", ""))
}

func TestFlagWithDoubleQuotes(t *testing.T) {
	loadOpts(t, "--foo \"bar baz\"")

	assert.Equal(t, "bar baz", GetFlag("foo", ""))
}

func TestFlagWithEqualSign(t *testing.T) {
	loadOpts(t, "--foo=bar")

	assert.Equal(t, "bar", GetFlag("foo", ""))
}

// https://github.com/argoproj/argo-cd/issues/24065 — repeated string-slice
// flags in ARGOCD_OPTS should behave the same as on the command line, where
// they accumulate instead of overwriting one another.
func TestStringSliceFlagRepeated(t *testing.T) {
	loadOpts(t, `--header "CF-Access-Client-Id: foo" --header "CF-Access-Client-Secret: bar"`)
	headers := GetStringSliceFlag("header", []string{})

	assert.Equal(t, []string{
		"CF-Access-Client-Id: foo",
		"CF-Access-Client-Secret: bar",
	}, headers)
}

func TestStringSliceFlagRepeatedMixedWithCSV(t *testing.T) {
	loadOpts(t, `--header "A: 1,B: 2" --header "C: 3"`)
	headers := GetStringSliceFlag("header", []string{})

	assert.Equal(t, []string{"A: 1", "B: 2", "C: 3"}, headers)
}

func TestStringSliceFlagRepeatedWithEquals(t *testing.T) {
	loadOpts(t, `--header=A --header=B`)
	headers := GetStringSliceFlag("header", []string{})

	assert.Equal(t, []string{"A", "B"}, headers)
}

// Scalar flags repeated in ARGOCD_OPTS use last-wins, matching cobra's
// default behaviour for non-repeatable flags.
func TestFlagRepeatedScalarLastWins(t *testing.T) {
	loadOpts(t, "--foo bar --foo baz")

	assert.Equal(t, "baz", GetFlag("foo", ""))
}

// An empty-string slice flag (e.g. `--header ""`) should be treated as no
// contribution to the slice, not a spurious empty entry.
func TestStringSliceFlagEmptyValueSkipped(t *testing.T) {
	loadOpts(t, `--header "" --header "X-Real-Key: v"`)
	headers := GetStringSliceFlag("header", []string{})

	assert.Equal(t, []string{"X-Real-Key: v"}, headers)
}

// When the key is unset, the fallback must be returned verbatim.
func TestStringSliceFlagFallbackWhenUnset(t *testing.T) {
	loadOpts(t, "")
	assert.Equal(t, []string{"fallback"}, GetStringSliceFlag("absent", []string{"fallback"}))
}

// GetIntFlag should return the fallback when the key is unset, and the
// parsed int (last value, matching last-wins scalar semantics) when set.
func TestIntFlagRepeatedScalarLastWins(t *testing.T) {
	loadOpts(t, "--n 1 --n 2 --n 3")
	assert.Equal(t, 3, GetIntFlag("n", 0))
}

func TestIntFlagFallbackWhenUnset(t *testing.T) {
	loadOpts(t, "")
	assert.Equal(t, 42, GetIntFlag("absent", 42))
}

// Mixing --flag=value and --flag value for the same key in a single
// ARGOCD_OPTS should preserve the order in which values were written and
// produce one slice containing all of them.
func TestStringSliceFlagMixedSyntax(t *testing.T) {
	loadOpts(t, `--header=A --header "B"`)
	headers := GetStringSliceFlag("header", []string{})

	assert.Equal(t, []string{"A", "B"}, headers)
}

// Boolean flags follow the scalar last-wins rule, matching cobra's
// default behaviour when a non-repeatable flag appears more than once.
func TestBoolFlagRepeatedScalarLastWins(t *testing.T) {
	loadOpts(t, "--verbose=true --verbose=false")
	assert.False(t, GetBoolFlag("verbose"))

	loadOpts(t, "--verbose=false --verbose=true")
	assert.True(t, GetBoolFlag("verbose"))
}

// If every supplied value for a slice flag was empty (or missing), we should
// return the caller's fallback rather than an empty slice, so that callers
// using StringSliceVarP do not end up wiping their own configured defaults.
func TestStringSliceFlagAllEmptyReturnsFallback(t *testing.T) {
	loadOpts(t, `--header "" --header ""`)
	headers := GetStringSliceFlag("header", []string{"default-header"})

	assert.Equal(t, []string{"default-header"}, headers)
}
