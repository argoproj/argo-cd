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

// multiFlags holds values for flags that may be repeated (e.g. --header can appear multiple times).
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
	// pkg shellquote doesn't recognize `=` so that the opts in format `foo=bar` could not work.
	// issue ref: https://github.com/argoproj/argo-cd/issues/6822
	for k, v := range flags {
		if strings.Contains(k, "=") && v == "true" {
			kv := strings.SplitN(k, "=", 2)
			actualKey, actualValue := kv[0], kv[1]
			if _, ok := flags[actualKey]; !ok {
				flags[actualKey] = actualValue
				multiFlags[actualKey] = append(multiFlags[actualKey], actualValue)
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
		log.Fatal(err)
	}
	return v
}

func GetStringSliceFlag(key string, fallback []string) []string {
	vals, ok := multiFlags[key]
	if !ok {
		return fallback
	}

	// Each value may itself be a comma-separated list (legacy behaviour).
	// Expand all values and concatenate them.
	var result []string
	for _, val := range vals {
		if val == "" {
			continue
		}
		stringReader := strings.NewReader(val)
		csvReader := csv.NewReader(stringReader)
		parts, err := csvReader.Read()
		if err != nil {
			log.Fatal(err)
		}
		result = append(result, parts...)
	}
	if result == nil {
		return []string{}
	}
	return result
}
