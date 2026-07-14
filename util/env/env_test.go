package env

import (
	"fmt"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseNumFromEnv(t *testing.T) {
	const envKey = "SOMEKEY"
	const minimum = math.MinInt + 1
	const maximum = math.MaxInt - 1
	const def = 10
	testCases := []struct {
		name     string
		env      string
		expected int
	}{
		{"Valid positive number", "200", 200},
		{"Valid negative number", "-200", -200},
		{"Invalid number", "abc", def},
		{"Equals minimum", strconv.Itoa(math.MinInt + 1), minimum},
		{"Equals maximum", strconv.Itoa(math.MaxInt - 1), maximum},
		{"Less than minimum", strconv.Itoa(math.MinInt), def},
		{"Greater than maximum", strconv.Itoa(math.MaxInt), def},
		{"Variable not set", "", def},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envKey, tt.env)
			n := ParseNumFromEnv(envKey, def, minimum, maximum)
			assert.Equal(t, tt.expected, n)
		})
	}
}

func TestParseFloatFromEnv(t *testing.T) {
	const envKey = "SOMEKEY"
	var minimum float32 = -1000.5
	var maximum float32 = 1000.5
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
		{"Equals maximum", fmt.Sprintf("%v", maximum), maximum},
		{"Equals minimum", fmt.Sprintf("%v", minimum), minimum},
		{"Greater than maximum", fmt.Sprintf("%f", maximum+1), def},
		{"Lesser than minimum", fmt.Sprintf("%f", minimum-1), def},
		{"Environment not set at", "", def},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envKey, tt.env)
			f := ParseFloatFromEnv(envKey, def, minimum, maximum)
			assert.InEpsilon(t, tt.expected, f, 0.0001)
		})
	}
}

func TestParseInt64FromEnv(t *testing.T) {
	const envKey = "SOMEKEY"
	const minimum int64 = 1
	const maximum int64 = math.MaxInt64 - 1
	const def int64 = 10
	testCases := []struct {
		name     string
		env      string
		expected int64
	}{
		{"Valid int64", "200", 200},
		{"Text as invalid int64", "abc", def},
		{"Equals maximum", strconv.FormatInt(maximum, 10), maximum},
		{"Equals minimum", strconv.FormatInt(minimum, 10), minimum},
		{"Greater than maximum", strconv.FormatInt(maximum+1, 10), def},
		{"Less than minimum", strconv.FormatInt(minimum-1, 10), def},
		{"Environment not set", "", def},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envKey, tt.env)
			n := ParseInt64FromEnv(envKey, def, minimum, maximum)
			assert.Equal(t, tt.expected, n)
		})
	}
}

func TestParseDurationFromEnv(t *testing.T) {
	envKey := "SOMEKEY"
	def := 3 * time.Second
	minimum := 2 * time.Second
	maximum := 5 * time.Second

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
		name:     "ValidValueSetMs",
		env:      "2500ms",
		expected: time.Millisecond * 2500,
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
			val := ParseDurationFromEnv(envKey, def, minimum, maximum)
			assert.Equal(t, tc.expected, val)
		})
	}
}

func TestParseDurationFromEnvEdgeCases(t *testing.T) {
	envKey := "SOME_ENV_KEY"
	def := 3 * time.Minute
	minimum := 1 * time.Second
	maximum := 2160 * time.Hour // 3 months

	testCases := []struct {
		name     string
		env      string
		expected time.Duration
	}{{
		name:     "EnvNotSet",
		expected: def,
	}, {
		name:     "Durations defined as days are valid",
		env:      "12d",
		expected: time.Hour * 24 * 12,
	}, {
		name:     "Negative durations should fail parsing and use the default value",
		env:      "-1h",
		expected: def,
	}, {
		name:     "Negative day durations should fail parsing and use the default value",
		env:      "-12d",
		expected: def,
	}, {
		name:     "Scientific notation should fail parsing and use the default value",
		env:      "1e3s",
		expected: def,
	}, {
		name:     "Durations with a leading zero are considered valid and parsed as decimal notation",
		env:      "0755s",
		expected: time.Second * 755,
	}, {
		name:     "Durations with many leading zeroes are considered valid and parsed as decimal notation",
		env:      "000083m",
		expected: time.Minute * 83,
	}, {
		name:     "Decimal Durations should not fail parsing",
		env:      "30.5m",
		expected: time.Minute*30 + time.Second*30,
	}, {
		name:     "Decimal Day Durations should fail parsing and use the default value",
		env:      "30.5d",
		expected: def,
	}, {
		name:     "Fraction Durations should fail parsing and use the default value",
		env:      "1/2h",
		expected: def,
	}, {
		name:     "Durations without a time unit should fail parsing and use the default value",
		env:      "15",
		expected: def,
	}, {
		name:     "Durations with a trailing symbol should fail parsing and use the default value",
		env:      "+12d",
		expected: def,
	}, {
		name:     "Leading space Duration should fail parsing use the default value",
		env:      " 2h",
		expected: def,
	}, {
		name:     "Trailing space Duration should fail parsing use the default value",
		env:      "6m ",
		expected: def,
	}, {
		name:     "Empty Duration should fail parsing use the default value",
		env:      "",
		expected: def,
	}, {
		name:     "Whitespace Duration should fail parsing and use the default value",
		env:      "    ",
		expected: def,
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envKey, tc.env)
			val := ParseDurationFromEnv(envKey, def, minimum, maximum)
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
		{"Some string", new("true"), "true", def, nil},
		{"Empty string with default", new(""), def, def, nil},
		{"Empty string without default", new(""), "", "", nil},
		{"No env variable with default allow empty", nil, "default", "default", []StringFromEnvOpts{{AllowEmpty: true}}},
		{"Some variable with default allow empty", new("true"), "true", "default", []StringFromEnvOpts{{AllowEmpty: true}}},
		{"Empty variable with default allow empty", new(""), "", "default", []StringFromEnvOpts{{AllowEmpty: true}}},
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

func TestParseStringToStringFromEnv(t *testing.T) {
	envKey := "SOMEKEY"
	def := map[string]string{}

	testCases := []struct {
		name     string
		env      string
		expected map[string]string
		def      map[string]string
		sep      string
	}{
		{"success, no key-value", "", map[string]string{}, def, ","},
		{"success, one key, no value", "key1=", map[string]string{"key1": ""}, def, ","},
		{"success, one key, no value, with spaces", "key1 = ", map[string]string{"key1": ""}, def, ","},
		{"success, one pair", "key1=value1", map[string]string{"key1": "value1"}, def, ","},
		{"success, one pair with spaces", "key1 = value1", map[string]string{"key1": "value1"}, def, ","},
		{"success, one pair with spaces and no value", "key1 = ", map[string]string{"key1": ""}, def, ","},
		{"success, two keys, no value", "key1=,key2=", map[string]string{"key1": "", "key2": ""}, def, ","},
		{"success, two keys, no value, with spaces", "key1 = , key2 = ", map[string]string{"key1": "", "key2": ""}, def, ","},
		{"success, two pairs", "key1=value1,key2=value2", map[string]string{"key1": "value1", "key2": "value2"}, def, ","},
		{"success, two pairs with semicolon as separator", "key1=value1;key2=value2", map[string]string{"key1": "value1", "key2": "value2"}, def, ";"},
		{"success, two pairs with spaces", "key1 = value1, key2 = value2", map[string]string{"key1": "value1", "key2": "value2"}, def, ","},
		{"failure, one key", "key1", map[string]string{}, def, ","},
		{"failure, duplicate keys", "key1=value1,key1=value2", map[string]string{}, def, ","},
		{"failure, one key ending with two successive equals to", "key1==", map[string]string{}, def, ","},
		{"failure, one valid pair and invalid one key", "key1=value1,key2", map[string]string{}, def, ","},
		{"failure, two valid pairs and invalid two keys", "key1=value1,key2=value2,key3,key4", map[string]string{}, def, ","},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envKey, tt.env)
			got := ParseStringToStringFromEnv(envKey, tt.def, tt.sep)
			assert.Equal(t, tt.expected, got)
		})
	}
}
