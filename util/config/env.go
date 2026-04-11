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

var flags map[string]interface{}

func init() {
	err := LoadFlags()
	if err != nil {
		log.Fatal(err)
	}
}

func LoadFlags() error {
	flags = make(map[string]interface{})

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
			existing, exists := flags[key]
			if !exists {
				flags[key] = opt
			} else {
				switch v := existing.(type) {
				case string:
					flags[key] = []string{v, opt}
				case []string:
					flags[key] = append(v, opt)
				}
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

func GetFlag(key, fallback string) string {
	val, ok := flags[key]
	if ok {
		switch v := val.(type) {
		case string:
			return v
		case []string:
			// For backwards compatibility, if someone asks for a single value on multi flag return first
			return v[0]
		}
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

	switch v := val.(type) {
	case string:
		ival, err := strconv.Atoi(v)
		if err != nil {
			log.Fatal(err)
		}
		return ival
	case []string:
		ival, err := strconv.Atoi(v[0])
		if err != nil {
			log.Fatal(err)
		}
		return ival
	}
	return fallback
}

func GetStringSliceFlag(key string, fallback []string) []string {
	val, ok := flags[key]
	if !ok {
		return fallback
	}

	switch v := val.(type) {
	case string:
		if v == "" {
			return []string{}
		}
		stringReader := strings.NewReader(v)
		csvReader := csv.NewReader(stringReader)
		res, err := csvReader.Read()
		if err != nil {
			log.Fatal(err)
		}
		return res
	case []string:
		return v
	}
	return fallback
}
