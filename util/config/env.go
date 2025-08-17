package config

import (
	"encoding/csv"
	"errors"
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

	var key string
	for _, opt := range opts {
		switch {
		case strings.HasPrefix(opt, "--"):
			if key != "" {
				flags[key] = "true"
			}
			key = strings.TrimPrefix(opt, "--")
		case key != "":
			// Check if this is a multi-value flag (like --header)
			if isMultiValueFlag(key) {
				if _, exists := multiFlags[key]; !exists {
					multiFlags[key] = []string{}
				}
				multiFlags[key] = append(multiFlags[key], opt)
			} else {
				flags[key] = opt
			}
			key = ""
		default:
			return errors.New("ARGOCD_OPTS invalid at '" + opt + "'")
		}
	}
	if key != "" {
		flags[key] = "true"
	}
	// pkg shellquota doesn't recognize `=` so that the opts in format `foo=bar` could not work.
	// issue ref: https://github.com/argoproj/argo-cd/issues/6822
	for k, v := range flags {
		if strings.Contains(k, "=") && v == "true" {
			kv := strings.SplitN(k, "=", 2)
			actualKey, actualValue := kv[0], kv[1]
			if _, ok := flags[actualKey]; !ok {
				flags[actualKey] = actualValue
			}
		}
	}
	return nil
}

// isMultiValueFlag returns true if the flag can have multiple values
func isMultiValueFlag(key string) bool {
	multiValueFlags := []string{"header"}
	for _, flag := range multiValueFlags {
		if key == flag {
			return true
		}
	}
	return false
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
		log.Fatal(err)
	}
	return v
}

func GetStringSliceFlag(key string, fallback []string) []string {
	// First check if we have multiple values for this flag
	if multiValues, ok := multiFlags[key]; ok {
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
	stringReader := strings.NewReader(val)
	csvReader := csv.NewReader(stringReader)
	v, err := csvReader.Read()
	if err != nil {
		log.Fatal(err)
	}
	return v
}
