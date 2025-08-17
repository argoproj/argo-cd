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
	strings := GetStringSliceFlag("header", []string{})

	assert.Len(t, strings, 2)
	assert.Equal(t, "Content-Type: application/json; charset=utf-8", strings[0])
	assert.Equal(t, "Strict-Transport-Security: max-age=31536000", strings[1])
}

func TestStringSliceFlagAtStart(t *testing.T) {
	loadOpts(t, "--header='Strict-Transport-Security: max-age=31536000' --bar baz")
	strings := GetStringSliceFlag("header", []string{})

	assert.Len(t, strings, 1)
	assert.Equal(t, "Strict-Transport-Security: max-age=31536000", strings[0])
}

func TestStringSliceFlagInMiddle(t *testing.T) {
	loadOpts(t, "--bar baz --header='Strict-Transport-Security: max-age=31536000' --qux")
	strings := GetStringSliceFlag("header", []string{})

	assert.Len(t, strings, 1)
	assert.Equal(t, "Strict-Transport-Security: max-age=31536000", strings[0])
}

func TestStringSliceFlagAtEnd(t *testing.T) {
	loadOpts(t, "--bar baz --header='Strict-Transport-Security: max-age=31536000'")
	strings := GetStringSliceFlag("header", []string{})

	assert.Len(t, strings, 1)
	assert.Equal(t, "Strict-Transport-Security: max-age=31536000", strings[0])
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

func TestMultipleHeaderFlags(t *testing.T) {
	loadOpts(t, "--header 'CF-Access-Client-Id: foo' --header 'CF-Access-Client-Secret: bar'")
	strings := GetStringSliceFlag("header", []string{})

	assert.Len(t, strings, 2)
	assert.Equal(t, "CF-Access-Client-Id: foo", strings[0])
	assert.Equal(t, "CF-Access-Client-Secret: bar", strings[1])
}

func TestMultipleHeaderFlagsWithOtherFlags(t *testing.T) {
	loadOpts(t, "--server localhost:8080 --header 'CF-Access-Client-Id: foo' --header 'CF-Access-Client-Secret: bar' --insecure")
	strings := GetStringSliceFlag("header", []string{})

	assert.Len(t, strings, 2)
	assert.Equal(t, "CF-Access-Client-Id: foo", strings[0])
	assert.Equal(t, "CF-Access-Client-Secret: bar", strings[1])
	
	// Verify other flags still work
	assert.Equal(t, "localhost:8080", GetFlag("server", ""))
	assert.True(t, GetBoolFlag("insecure"))
}

func TestMultipleHeaderFlagsMixedWithCommaSeparated(t *testing.T) {
	loadOpts(t, "--header 'CF-Access-Client-Id: foo,CF-Access-Client-Secret: bar' --header 'X-Custom-Header: value'")
	strings := GetStringSliceFlag("header", []string{})

	assert.Len(t, strings, 2)
	assert.Equal(t, "CF-Access-Client-Id: foo,CF-Access-Client-Secret: bar", strings[0])
	assert.Equal(t, "X-Custom-Header: value", strings[1])

}

}

