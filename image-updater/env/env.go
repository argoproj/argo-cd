package env

import (
	"os"
	"strings"
)

// Package env provides some utility functions to interact with the environment
// of the process.

// GetBoolVal retrieves a boolean value from given environment envVar.
// Returns default value if envVar is not set.
func GetBoolVal(envVar string, defaultValue bool) bool {
	if val := os.Getenv(envVar); val != "" {
		if strings.ToLower(val) == "true" {
			return true
		} else if strings.ToLower(val) == "false" {
			return false
		}
	}
	return defaultValue
}

// GetStringVal retrieves a string value from given environment envVar
// Returns default value if envVar is not set.
func GetStringVal(envVar string, defaultValue string) string {
	if val := os.Getenv(envVar); val != "" {
		return val
	} else {
		return defaultValue
	}
}
