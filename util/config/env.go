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
	// last value wins (backwards compatible)
	flags map[string]string
	// all occurrences preserved (new)
	flagMulti map[string][]string
)

func init() {
	if err := LoadFlags(); err != nil {
		log.Fatal(err)
	}
}

func LoadFlags() error {
	flags = make(map[string]string)
	flagMulti = make(map[string][]string)

	opts, err := shellquote.Split(os.Getenv("ARGOCD_OPTS"))
	if err != nil {
		return err
	}

	var key string
	for _, opt := range opts {
		switch {
		case strings.HasPrefix(opt, "--"):
			// finish previous boolean flag if it had no explicit value
			if key != "" {
				flags[key] = "true"
				flagMulti[key] = append(flagMulti[key], "true")
			}
			key = strings.TrimPrefix(opt, "--")

		case key != "":
			// value for the previous key
			flags[key] = opt
			flagMulti[key] = append(flagMulti[key], opt)
			key = ""

		default:
			return errors.New("ARGOCD_OPTS invalid at '" + opt + "'")
		}
	}
	// trailing boolean flag
	if key != "" {
		flags[key] = "true"
		flagMulti[key] = append(flagMulti[key], "true")
	}

	// Handle --foo=bar form (shellquote keeps it as one token "foo=bar").
	// Old behavior only fixed 'flags'; we also need to fill 'flagMulti'.
	for k, v := range flags {
		if strings.Contains(k, "=") && v == "true" {
			kv := strings.SplitN(k, "=", 2)
			actualKey, actualValue := kv[0], kv[1]

			// single-value map: only set if not already present (keeps "last wins" semantics)
			if _, ok := flags[actualKey]; !ok {
				flags[actualKey] = actualValue
			}
			// multi-value map: always append (we want all occurrences)
			flagMulti[actualKey] = append(flagMulti[actualKey], actualValue)
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
		log.Fatal(err)
	}
	return v
}

// GetStringSliceFlag returns:
// 1) If the flag was provided multiple times, the list of all values (no CSV splitting).
// 2) Otherwise, for a single value, preserve existing behavior: parse as CSV so
//    comma-separated lists still work.
func GetStringSliceFlag(key string, fallback []string) []string {
	// Prefer accumulated values when present
	if mv, ok := flagMulti[key]; ok && len(mv) > 0 {
		// If there was exactly one occurrence, keep compatibility with existing CSV style:
		// parse that single value as CSV. If there were multiple occurrences, return as-is.
		if len(mv) == 1 {
			val := mv[0]
			if val == "" {
				return []string{}
			}
			r := csv.NewReader(strings.NewReader(val))
			out, err := r.Read()
			if err != nil {
				log.Fatal(err)
			}
			return out
		}
		// multiple occurrences: no CSV splitting; each occurrence is one element
		return append([]string(nil), mv...)
	}

	// Fallback to old behavior if nothing in maps
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
		log.Fatal(err)
	}
	return out
}
