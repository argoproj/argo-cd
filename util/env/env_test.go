package env

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
)

func TestParseNumFromEnv(t *testing.T) {
	const envKey = "SOMEKEY"
	const min = math.MinInt + 1
	const max = math.MaxInt - 1
	const def = 10
	testCases := []struct {
		name     string
		env      string
		expected int
	}{
		{"Valid positive number", "200", 200},
		{"Valid negative number", "-200", -200},
		{"Invalid number", "abc", def},
		{"Equals minimum", fmt.Sprintf("%d", math.MinInt+1), min},
		{"Equals maximum", fmt.Sprintf("%d", math.MaxInt-1), max},
		{"Less than minimum", fmt.Sprintf("%d", math.MinInt), def},
		{"Greater than maximum", fmt.Sprintf("%d", math.MaxInt), def},
		{"Variable not set", "", def},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envKey, tt.env)
			n := ParseNumFromEnv(envKey, def, min, max)
			assert.Equal(t, tt.expected, n)
		})
	}
}

func TestParseFloatFromEnv(t *testing.T) {
	const envKey = "SOMEKEY"
	var min float32 = -1000.5
	var max float32 = 1000.5
	const def float32 = 10.5
	testCases := []struct {
		name     string
		env      string
		expected float32
	}{
		{"Valid positive float", "2.0", 2.0},
		{"Valid negative float", "-2.0", -2.0},
		{"Valid integer as float", "2", 2.0},
		{"Text as invalid float", "abc", def},
		{"Equals maximum", fmt.Sprintf("%v", max), max},
		{"Equals minimum", fmt.Sprintf("%v", min), min},
		{"Greater than maximum", fmt.Sprintf("%f", max+1), def},
		{"Lesser than minimum", fmt.Sprintf("%f", min-1), def},
		{"Environment not set at", "", def},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envKey, tt.env)
			f := ParseFloatFromEnv(envKey, def, min, max)
			assert.Equal(t, tt.expected, f)
		})
	}
}

func TestParseInt64FromEnv(t *testing.T) {
	const envKey = "SOMEKEY"
	const min int64 = 1
	const max int64 = math.MaxInt64 - 1
	const def int64 = 10
	testCases := []struct {
		name     string
		env      string
		expected int64
	}{
		{"Valid int64", "200", 200},
		{"Text as invalid int64", "abc", def},
		{"Equals maximum", fmt.Sprintf("%d", max), max},
		{"Equals minimum", fmt.Sprintf("%d", min), min},
		{"Greater than maximum", fmt.Sprintf("%d", max+1), def},
		{"Less than minimum", fmt.Sprintf("%d", min-1), def},
		{"Environment not set", "", def},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envKey, tt.env)
			n := ParseInt64FromEnv(envKey, def, min, max)
			assert.Equal(t, tt.expected, n)
		})
	}
}

func TestParseDurationFromEnv(t *testing.T) {
	envKey := "SOMEKEY"
	def := 3 * time.Second
	min := 2 * time.Second
	max := 5 * time.Second

	testCases := []struct {
		name     string
		env      string
		expected time.Duration
	}{{
		name:     "EnvNotSet",
		expected: def,
	}, {
		name:     "ValidValueSet",
		env:      "2s",
		expected: time.Second * 2,
	}, {
		name:     "MoreThanMaxSet",
		env:      "6s",
		expected: def,
	}, {
		name:     "LessThanMinSet",
		env:      "1s",
		expected: def,
	}, {
		name:     "InvalidSet",
		env:      "hello",
		expected: def,
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envKey, tc.env)
			val := ParseDurationFromEnv(envKey, def, min, max)
			assert.Equal(t, tc.expected, val)
		})
	}
}

func Test_ParseBoolFromEnv(t *testing.T) {
	envKey := "SOMEKEY"

	testCases := []struct {
		name     string
		env      string
		expected bool
		def      bool
	}{
		{"True value", "true", true, false},
		{"False value", "false", false, true},
		{"Invalid value with true default", "somevalue", true, true},
		{"Invalid value with false default", "somevalue", false, false},
		{"Env not set", "", false, false},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envKey, tt.env)
			b := ParseBoolFromEnv(envKey, tt.def)
			assert.Equal(t, tt.expected, b)
		})
	}
}

func TestStringFromEnv(t *testing.T) {
	envKey := "SOMEKEY"
	def := "somestring"

	testCases := []struct {
		name     string
		env      *string
		expected string
		def      string
		opts     []StringFromEnvOpts
	}{
		{"Some string", pointer.String("true"), "true", def, nil},
		{"Empty string with default", pointer.String(""), def, def, nil},
		{"Empty string without default", pointer.String(""), "", "", nil},
		{"No env variable with default allow empty", nil, "default", "default", []StringFromEnvOpts{{AllowEmpty: true}}},
		{"Some variable with default allow empty", pointer.String("true"), "true", "default", []StringFromEnvOpts{{AllowEmpty: true}}},
		{"Empty variable with default allow empty", pointer.String(""), "", "default", []StringFromEnvOpts{{AllowEmpty: true}}},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != nil {
				t.Setenv(envKey, *tt.env)
			}
			b := StringFromEnv(envKey, tt.def, tt.opts...)
			assert.Equal(t, tt.expected, b)
		})
	}
}

func TestStringsFromEnv(t *testing.T) {
	envKey := "SOMEKEY"
	def := []string{"one", "two"}

	testCases := []struct {
		name     string
		env      string
		expected []string
		def      []string
		sep      string
	}{
		{"List of strings", "one,two,three", []string{"one", "two", "three"}, def, ","},
		{"Comma separated with other delimeter", "one,two,three", []string{"one,two,three"}, def, ";"},
		{"With trimmed white space", "one, two   ,    three", []string{"one", "two", "three"}, def, ","},
		{"Env not set", "", def, def, ","},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envKey, tt.env)
			ss := StringsFromEnv(envKey, tt.def, tt.sep)
			assert.Equal(t, tt.expected, ss)
		})
	}
}
