package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func loadOpts(t *testing.T, opts string) {
	assert.Nil(t, os.Setenv("ARGOCD_OPTS", opts))
	assert.Nil(t, loadFlags())
}
func loadInvalidOpts(t *testing.T, opts string) {
	assert.Nil(t, os.Setenv("ARGOCD_OPTS", opts))
	assert.Error(t, loadFlags())
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
