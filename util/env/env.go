package env

import (
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
	num, err := strconv.Atoi(str)
	if err != nil {
		log.Warnf("Could not parse '%s' as a number from environment %s", str, env)
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
		log.Warnf("Could not parse '%s' as a number from environment %s", str, env)
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

func StringFromEnv(env string, defaultValue string) string {
	if str := os.Getenv(env); str != "" {
		return str
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
