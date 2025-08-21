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

var (
	flags      map[string]string
	multiFlags map[string][]string
)

func init() {
	// Skip LoadFlags() during subprocess tests
	if os.Getenv("BE_TEST_IGNORE_LOADFLAGS") != "1" {
		if err := LoadFlags(); err != nil {
			log.Fatal(err)
		}
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
				multiFlags[key] = append(multiFlags[key], "true")
			}
			key = strings.TrimPrefix(opt, "--")
		case key != "":
			flags[key] = opt
			multiFlags[key] = append(multiFlags[key], opt)
			key = ""
		default:
			return errors.New("ARGOCD_OPTS invalid at '" + opt + "'")
		}
	}

	if key != "" {
		flags[key] = "true"
		multiFlags[key] = append(multiFlags[key], "true")
	}

	// Handle --foo=bar form
	for k, v := range flags {
		if strings.Contains(k, "=") && v == "true" {
			kv := strings.SplitN(k, "=", 2)
			actualKey, actualValue := kv[0], kv[1]
			if _, ok := flags[actualKey]; !ok {
				flags[actualKey] = actualValue
			}
			multiFlags[actualKey] = append(multiFlags[actualKey], actualValue)
		}
	}
	return nil
}

func GetFlag(key, fallback string) string {
	if val, ok := flags[key]; ok {
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
		log.Warnf("invalid int value for %s: %v, using fallback %d", key, err, fallback)
		return fallback
	}
	return v
}

// GetStringSliceFlag safely parses CSV or returns multiple occurrences, preserving empty strings.
func GetStringSliceFlag(key string, fallback []string) []string {
	mvs, ok := multiFlags[key]
	if ok && len(mvs) > 0 {
		// Multiple occurrences: return as-is, including empty strings
		if len(mvs) > 1 {
			return append([]string(nil), mvs...)
		}
		// Single occurrence: parse CSV
		val := mvs[0]
		if val == "" {
			return []string{}
		}
		r := csv.NewReader(strings.NewReader(val))
		out, err := r.Read()
		if err != nil {
			log.Warnf("invalid CSV for key %s: %v", key, err)
			return fallback
		}
		return out
	}

	// Single value from flags
	val, ok := flags[key]
	if !ok {
		return fallback
	}
	if val == "" {
		return []string{}
	}
	r := csv.NewReader(strings.NewReader(val))
	out, err := r.Read()
	if err != nil {
		log.Warnf("invalid CSV for key %s: %v", key, err)
		return fallback
	}
	return out
}
