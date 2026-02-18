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

func TestStringSliceMultiple(t *testing.T) {
	loadOpts(t, "--header 'Content-Type: application/json; charset=utf-8' --header 'Strict-Transport-Security: max-age=31536000'")
	strings := GetStringSliceFlag("header", []string{})

	assert.Len(t, strings, 2)
	assert.Equal(t, "Content-Type: application/json; charset=utf-8", strings[0])
	assert.Equal(t, "Strict-Transport-Security: max-age=31536000", strings[1])
}

func TestStringSliceMultipleWithEqualSign(t *testing.T) {
	loadOpts(t, "--header='Content-Type: application/json; charset=utf-8' --header='Strict-Transport-Security: max-age=31536000'")
	strings := GetStringSliceFlag("header", []string{})

	assert.Len(t, strings, 2)
	assert.Equal(t, "Content-Type: application/json; charset=utf-8", strings[0])
	assert.Equal(t, "Strict-Transport-Security: max-age=31536000", strings[1])
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

func TestFlagMultiple(t *testing.T) {
	loadOpts(t, "--foo bar --foo baz")

	assert.Equal(t, "baz", GetFlag("foo", "qux"))
}

func TestProcessFlagKey(t *testing.T) {
	tests := []struct {
		name          string
		initialFlags  map[string][]string
		key           string
		expectedFlags map[string][]string
	}{
		{
			name:         "boolean flag",
			initialFlags: map[string][]string{},
			key:          "foo",
			expectedFlags: map[string][]string{
				"foo": {},
			},
		},
		{
			name: "boolean flag with existing key",
			initialFlags: map[string][]string{
				"foo": {},
			},
			key: "foo",
			expectedFlags: map[string][]string{
				"foo": {},
			},
		},
		{
			name:         "key with equal sign",
			initialFlags: map[string][]string{},
			key:          "foo=bar",
			expectedFlags: map[string][]string{
				"foo": {"bar"},
			},
		},
		{
			name:         "key with equal sign and empty value",
			initialFlags: map[string][]string{},
			key:          "foo=",
			expectedFlags: map[string][]string{
				"foo": {""},
			},
		},
		{
			name:         "key with multiple equal signs",
			initialFlags: map[string][]string{},
			key:          "foo=bar=baz",
			expectedFlags: map[string][]string{
				"foo": {"bar=baz"},
			},
		},
		{
			name: "key with equal sign appends to existing key",
			initialFlags: map[string][]string{
				"foo": {"bar"},
			},
			key: "foo=baz",
			expectedFlags: map[string][]string{
				"foo": {"bar", "baz"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := tt.initialFlags
			processFlagKey(flags, tt.key)
			assert.Equal(t, tt.expectedFlags, flags)
		})
	}
}
