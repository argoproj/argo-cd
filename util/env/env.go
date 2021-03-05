package env

import (
	"os"
	"strconv"
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

// Helper function to parse a time duration from an environment variable. Returns a
// default if env is not set, is not parseable to a duration, exceeds max (if
// max is greater than 0) or is less than min.
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
