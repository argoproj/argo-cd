package env

import (
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	timeutil "github.com/argoproj/pkg/time"

	log "github.com/sirupsen/logrus"
)

// Helper function to parse a number from an environment variable. Returns a
// default if env is not set, is not parseable to a number, exceeds max (if
// max is greater than 0) or is less than min.
//
// nolint:unparam
func ParseNumFromEnv(env string, defaultValue, min, max int) int {
	str := os.Getenv(env)
	if str == "" {
		return defaultValue
	}
	num, err := strconv.ParseInt(str, 10, 0)
	if err != nil {
		log.Warnf("Could not parse '%s' as a number from environment %s", str, env)
		return defaultValue
	}
	if num > math.MaxInt || num < math.MinInt {
		log.Warnf("Value in %s is %d is outside of the min and max %d allowed values. Using default %d", env, num, min, defaultValue)
		return defaultValue
	}
	if int(num) < min {
		log.Warnf("Value in %s is %d, which is less than minimum %d allowed", env, num, min)
		return defaultValue
	}
	if int(num) > max {
		log.Warnf("Value in %s is %d, which is greater than maximum %d allowed", env, num, max)
		return defaultValue
	}
	return int(num)
}

// Helper function to parse a int64 from an environment variable. Returns a
// default if env is not set, is not parseable to a number, exceeds max (if
// max is greater than 0) or is less than min.
//
// nolint:unparam
func ParseInt64FromEnv(env string, defaultValue, min, max int64) int64 {
	str := os.Getenv(env)
	if str == "" {
		return defaultValue
	}

	num, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		log.Warnf("Could not parse '%s' as a int64 from environment %s", str, env)
		return defaultValue
	}
	if num < min {
		log.Warnf("Value in %s is %d, which is less than minimum %d allowed", env, num, min)
		return defaultValue
	}
	if num > max {
		log.Warnf("Value in %s is %d, which is greater than maximum %d allowed", env, num, max)
		return defaultValue
	}
	return num
}

// Helper function to parse a float32 from an environment variable. Returns a
// default if env is not set, is not parseable to a number, exceeds max (if
// max is greater than 0) or is less than min (and min is greater than 0).
//
// nolint:unparam
func ParseFloatFromEnv(env string, defaultValue, min, max float32) float32 {
	str := os.Getenv(env)
	if str == "" {
		return defaultValue
	}

	num, err := strconv.ParseFloat(str, 32)
	if err != nil {
		log.Warnf("Could not parse '%s' as a float32 from environment %s", str, env)
		return defaultValue
	}
	if float32(num) < min {
		log.Warnf("Value in %s is %f, which is less than minimum %f allowed", env, num, min)
		return defaultValue
	}
	if float32(num) > max {
		log.Warnf("Value in %s is %f, which is greater than maximum %f allowed", env, num, max)
		return defaultValue
	}
	return float32(num)
}

// Helper function to parse a float64 from an environment variable. Returns a
// default if env is not set, is not parseable to a number, exceeds max (if
// max is greater than 0) or is less than min (and min is greater than 0).
//
// nolint:unparam
func ParseFloat64FromEnv(env string, defaultValue, min, max float64) float64 {
	str := os.Getenv(env)
	if str == "" {
		return defaultValue
	}

	num, err := strconv.ParseFloat(str, 64)
	if err != nil {
		log.Warnf("Could not parse '%s' as a float32 from environment %s", str, env)
		return defaultValue
	}
	if num < min {
		log.Warnf("Value in %s is %f, which is less than minimum %f allowed", env, num, min)
		return defaultValue
	}
	if num > max {
		log.Warnf("Value in %s is %f, which is greater than maximum %f allowed", env, num, max)
		return defaultValue
	}
	return num
}

// Helper function to parse a time duration from an environment variable. Returns a
// default if env is not set, is not parseable to a duration, exceeds max (if
// max is greater than 0) or is less than min.
//
// nolinit:unparam
func ParseDurationFromEnv(env string, defaultValue, min, max time.Duration) time.Duration {
	str := os.Getenv(env)
	if str == "" {
		return defaultValue
	}
	durPtr, err := timeutil.ParseDuration(str)
	if err != nil {
		log.Warnf("Could not parse '%s' as a duration string from environment %s", str, env)
		return defaultValue
	}

	dur := *durPtr
	if dur < min {
		log.Warnf("Value in %s is %s, which is less than minimum %s allowed", env, dur, min)
		return defaultValue
	}
	if dur > max {
		log.Warnf("Value in %s is %s, which is greater than maximum %s allowed", env, dur, max)
		return defaultValue
	}
	return dur
}

type StringFromEnvOpts struct {
	// AllowEmpty allows the value to be empty as long as the environment variable is set.
	AllowEmpty bool
}

func StringFromEnv(env string, defaultValue string, opts ...StringFromEnvOpts) string {
	opt := StringFromEnvOpts{}
	for _, o := range opts {
		opt.AllowEmpty = opt.AllowEmpty || o.AllowEmpty
	}
	if str, ok := os.LookupEnv(env); opt.AllowEmpty && ok || str != "" {
		return str
	}
	return defaultValue
}

// StringsFromEnv parses given value from the environment as a list of strings,
// using separator as the delimeter, and returns them as a slice. The strings
// in the returned slice will have leading and trailing white space removed.
func StringsFromEnv(env string, defaultValue []string, separator string) []string {
	if str := os.Getenv(env); str != "" {
		ss := strings.Split(str, separator)
		for i, s := range ss {
			ss[i] = strings.TrimSpace(s)
		}
		return ss
	}
	return defaultValue
}

// ParseBoolFromEnv retrieves a boolean value from given environment envVar.
// Returns default value if envVar is not set.
//
// nolinit:unparam
func ParseBoolFromEnv(envVar string, defaultValue bool) bool {
	if val := os.Getenv(envVar); val != "" {
		if strings.ToLower(val) == "true" {
			return true
		} else if strings.ToLower(val) == "false" {
			return false
		}
	}
	return defaultValue
}

// ParseStringToStringVar parses given value from the environment as a map of string.
// Returns default value if envVar is not set.
func ParseStringToStringFromEnv(envVar string, defaultValue map[string]string, separator string) map[string]string {
	str := os.Getenv(envVar)
	str = strings.TrimSpace(str)
	if str == "" {
		return defaultValue
	}

	parsed := make(map[string]string)
	for _, pair := range strings.Split(str, separator) {
		keyvalue := strings.Split(pair, "=")
		if len(keyvalue) != 2 {
			log.Warnf("Invalid key-value pair when parsing environment '%s' as a string map", str)
			return defaultValue
		}
		key := strings.TrimSpace(keyvalue[0])
		value := strings.TrimSpace(keyvalue[1])
		if _, ok := parsed[key]; ok {
			log.Warnf("Duplicate key '%s' when parsing environment '%s' as a string map", key, str)
			return defaultValue
		}
		parsed[key] = value
	}
	return parsed
}
