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

// flags maps a flag name to all of its values collected from ARGOCD_OPTS, in
// the order they appeared. Using a slice preserves the "can be repeated"
// semantics the CLI already supports for string-slice flags such as
// --header, keeping ARGOCD_OPTS consistent with command-line invocation.
// See https://github.com/argoproj/argo-cd/issues/24065.
var flags map[string][]string

func init() {
	err := LoadFlags()
	if err != nil {
		log.Fatal(err)
	}
}

func LoadFlags() error {
	flags = make(map[string][]string)

	opts, err := shellquote.Split(os.Getenv("ARGOCD_OPTS"))
	if err != nil {
		return err
	}

	var key string
	for _, opt := range opts {
		switch {
		case strings.HasPrefix(opt, "--"):
			if key != "" {
				flags[key] = append(flags[key], "true")
			}
			// shellquote leaves `--foo=bar` as a single token, so
			// split it inline. Doing the split here (rather than in a
			// post-processing pass over the map) keeps values in the
			// same order the user wrote them and means the `=` form
			// and the space-separated form can be mixed in a single
			// ARGOCD_OPTS without clobbering one another. See #6822,
			// #24065.
			trimmed := strings.TrimPrefix(opt, "--")
			if k, v, ok := strings.Cut(trimmed, "="); ok {
				flags[k] = append(flags[k], v)
				key = ""
			} else {
				key = trimmed
			}
		case key != "":
			flags[key] = append(flags[key], opt)
			key = ""
		default:
			return errors.New("ARGOCD_OPTS invalid at '" + opt + "'")
		}
	}
	if key != "" {
		flags[key] = append(flags[key], "true")
	}
	return nil
}

// lastValue returns the last value seen for key, or "" if the flag was not
// supplied. "Last wins" matches the semantics most CLI frameworks use when a
// scalar flag is repeated.
func lastValue(key string) (string, bool) {
	vs, ok := flags[key]
	if !ok || len(vs) == 0 {
		return "", false
	}
	return vs[len(vs)-1], true
}

func GetFlag(key, fallback string) string {
	if v, ok := lastValue(key); ok {
		return v
	}
	return fallback
}

func GetBoolFlag(key string) bool {
	return GetFlag(key, "false") == "true"
}

func GetIntFlag(key string, fallback int) int {
	v, ok := lastValue(key)
	if !ok {
		return fallback
	}

	i, err := strconv.Atoi(v)
	if err != nil {
		log.Fatal(err)
	}
	return i
}

// GetStringSliceFlag returns every value supplied for key in ARGOCD_OPTS,
// concatenated in order. Each individual value is still CSV-split, so both
// forms work and can be mixed:
//
//	ARGOCD_OPTS='--header "A" --header "B"'      -> []string{"A", "B"}
//	ARGOCD_OPTS='--header "A,B"'                 -> []string{"A", "B"}
//	ARGOCD_OPTS='--header "A,B" --header "C"'    -> []string{"A", "B", "C"}
//
// Prior to https://github.com/argoproj/argo-cd/issues/24065, repeated flags
// in ARGOCD_OPTS silently dropped all but the last value; only CSV-joined
// values worked. This matches the command-line behaviour of a cobra
// StringSliceVarP flag, where --header is documented as repeatable.
func GetStringSliceFlag(key string, fallback []string) []string {
	vals, ok := flags[key]
	if !ok {
		return fallback
	}

	result := make([]string, 0, len(vals))
	for _, val := range vals {
		if val == "" {
			continue
		}
		parts, err := csv.NewReader(strings.NewReader(val)).Read()
		if err != nil {
			log.Fatal(err)
		}
		result = append(result, parts...)
	}
	// If every supplied value was empty we would otherwise return an
	// empty slice and hide the fallback the caller configured, so fall
	// back explicitly when nothing survived CSV-splitting.
	if len(result) == 0 {
		return fallback
	}
	return result
}
