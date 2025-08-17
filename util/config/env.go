package config

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/kballard/go-shellquote"
	log "github.com/sirupsen/logrus"
)

var flags map[string]string
var multiFlags map[string][]string

func init() {
	err := LoadFlags()
	if err != nil {
		log.Fatal(err)
	}
}

func LoadFlags() error {
	flags = make(map[string]string)
	multiFlags = make(map[string][]string)

	opts, err := shellquote.Split(os.Getenv("ARGOCD_OPTS"))
	if err != nil {
		return err
	}

	// First pass: collect all flag occurrences to detect multi-value flags
	flagOccurrences := make(map[string][]string)
	var key string
	for _, opt := range opts {
		switch {
		case strings.HasPrefix(opt, "--"):
			if key != "" {
				// Previous flag without value - count it
				flagOccurrences[key] = append(flagOccurrences[key], "true")
			}
			
			if strings.Contains(opt, "=") {
				kv := strings.SplitN(opt, "=", 2)
				flagKey := strings.TrimPrefix(kv[0], "--")
				flagOccurrences[flagKey] = append(flagOccurrences[flagKey], kv[1])
				key = ""
			} else {
				key = strings.TrimPrefix(opt, "--")
			}
		case key != "":
			// Flag with value
			flagOccurrences[key] = append(flagOccurrences[key], opt)
			key = ""
		}
	}
	if key != "" {
		flagOccurrences[key] = append(flagOccurrences[key], "true")
	}

	// Second pass: process flags based on whether they're multi-value
	key = ""
	for _, opt := range opts {
		switch {
		case strings.HasPrefix(opt, "--"):
			if key != "" {
				// Previous flag without value
				if !isMultiValueFlag(key) || len(flagOccurrences[key]) == 1 {
					flags[key] = "true"
				}
			}
			
			if strings.Contains(opt, "=") {
				kv := strings.SplitN(opt, "=", 2)
				flagKey := strings.TrimPrefix(kv[0], "--")
				flagValue := kv[1]
				
				if isMultiValueFlag(flagKey) && len(flagOccurrences[flagKey]) > 1 {
					// Multi-value flag with multiple occurrences
					if _, exists := multiFlags[flagKey]; !exists {
						multiFlags[flagKey] = []string{}
					}
					multiFlags[flagKey] = append(multiFlags[flagKey], flagValue)
				} else {
					// Single-value flag or single occurrence of multi-value flag
					flags[flagKey] = flagValue
				}
				key = ""
			} else {
				key = strings.TrimPrefix(opt, "--")
			}
		case key != "":
			// Flag with value
			if isMultiValueFlag(key) && len(flagOccurrences[key]) > 1 {
				// Multi-value flag with multiple occurrences
				if _, exists := multiFlags[key]; !exists {
					multiFlags[key] = []string{}
				}
				multiFlags[key] = append(multiFlags[key], opt)
			} else {
				// Single-value flag or single occurrence of multi-value flag
				flags[key] = opt
			}
			key = ""
		default:
			return fmt.Errorf("ARGOCD_OPTS invalid at '%s'", opt)
		}
	}
	
	if key != "" {
		if !isMultiValueFlag(key) || len(flagOccurrences[key]) == 1 {
			flags[key] = "true"
		}
	}

	// Legacy handling for flags that weren't processed by the = logic above
	for k, v := range flags {
		if strings.Contains(k, "=") && v == "true" {
			kv := strings.SplitN(k, "=", 2)
			actualKey, actualValue := kv[0], kv[1]
			if _, ok := flags[actualKey]; !ok {
				flags[actualKey] = actualValue
				delete(flags, k)
			}
		}
	}
	
	return nil
}

// isMultiValueFlag returns true if the flag can have multiple values
func isMultiValueFlag(key string) bool {
	multiValueFlags := []string{
		"header",
		"plugin-env",     // For multiple plugin environment variables
		"label-selector", // For multiple label selectors (if supported)
		"annotation",     // For multiple annotations (if supported)
	}
	for _, flag := range multiValueFlags {
		if key == flag {
			return true
		}
	}
	return false
}

// validateMultiValueFlag validates the format of multi-value flags
func validateMultiValueFlag(key string, values []string) error {
	switch key {
	case "header":
		for _, header := range values {
			if strings.TrimSpace(header) == "" {
				return fmt.Errorf("header value cannot be empty")
			}
			if !strings.Contains(header, ":") {
				return fmt.Errorf("invalid header format: %s (expected 'Key: Value')", header)
			}
		}
	case "plugin-env":
		for _, env := range values {
			if strings.TrimSpace(env) == "" {
				return fmt.Errorf("plugin-env value cannot be empty")
			}
			if !strings.Contains(env, "=") {
				return fmt.Errorf("invalid plugin-env format: %s (expected 'KEY=VALUE')", env)
			}
		}
	}
	return nil
}

func GetFlag(key, fallback string) string {
	val, ok := flags[key]
	if ok {
		return val
	}
	return fallback
}

func GetBoolFlag(key string) bool {
	return GetFlag(key, "false") == "true"
}

func GetIntFlag(key string, fallback int) int {
	val, ok := flags[key]
	if !ok {
		return fallback
	}

	v, err := strconv.Atoi(val)
	if err != nil {
		log.Fatalf("Invalid integer value for flag '%s': %s", key, val)
	}
	return v
}

func GetStringSliceFlag(key string, fallback []string) []string {
	// First check if we have multiple values for this flag
	if multiValues, ok := multiFlags[key]; ok {
		// Validate the multi-values (but don't fail, just warn)
		if err := validateMultiValueFlag(key, multiValues); err != nil {
			log.Warnf("Invalid multi-value flag %s: %v", key, err)
		}
		return multiValues
	}
	
	// Fall back to the original single-value behavior
	val, ok := flags[key]
	if !ok {
		return fallback
	}

	if val == "" {
		return []string{}
	}
	
	// For backward compatibility, use CSV parsing for single header values
	// This handles comma-separated headers in a single --header flag
	stringReader := strings.NewReader(val)
	csvReader := csv.NewReader(stringReader)
	csvReader.TrimLeadingSpace = true
	v, err := csvReader.Read()
	if err != nil {
		log.Fatalf("Invalid CSV format for flag '%s': %s", key, val)
	}
	return v
}

// GetMultiValueFlag is a dedicated method for getting multi-value flags
// Returns empty slice if the flag is not a multi-value flag
func GetMultiValueFlag(key string) []string {
	if !isMultiValueFlag(key) {
		log.Warnf("Flag '%s' is not configured as a multi-value flag", key)
		return []string{}
	}
	return GetStringSliceFlag(key, []string{})
}

// HasFlag returns true if the flag was explicitly set (either single or multi-value)
func HasFlag(key string) bool {
	if _, ok := flags[key]; ok {
		return true
	}
	if _, ok := multiFlags[key]; ok {
		return true
	}
	return false
}

// GetAllFlags returns all single-value flags (for debugging)
func GetAllFlags() map[string]string {
	result := make(map[string]string)
	for k, v := range flags {
		result[k] = v
	}
	return result
}

// GetAllMultiFlags returns all multi-value flags (for debugging)
func GetAllMultiFlags() map[string][]string {
	result := make(map[string][]string)
	for k, v := range multiFlags {
		result[k] = make([]string, len(v))
		copy(result[k], v)
	}
	return result
}
