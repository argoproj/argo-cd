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
				key = ""
			}
			trimmed := strings.TrimPrefix(opt, "--")
			// pkg shellquote doesn't recognize `=` so that the opts in format `foo=bar` could not work.
			// issue ref: https://github.com/argoproj/argo-cd/issues/6822
			if k, v, ok := strings.Cut(trimmed, "="); ok {
				flags[k] = append(flags[k], v)
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

func GetFlag(key, fallback string) string {
	vals, ok := flags[key]
	if ok && len(vals) > 0 {
		return vals[len(vals)-1]
	}
	return fallback
}

func GetBoolFlag(key string) bool {
	return GetFlag(key, "false") == "true"
}

func GetIntFlag(key string, fallback int) int {
	vals, ok := flags[key]
	if !ok || len(vals) == 0 {
		return fallback
	}
	val := vals[len(vals)-1]

	v, err := strconv.Atoi(val)
	if err != nil {
		log.Fatal(err)
	}
	return v
}

func GetStringSliceFlag(key string, fallback []string) []string {
	vals, ok := flags[key]
	if !ok || len(vals) == 0 {
		return fallback
	}

	var result []string
	for _, val := range vals {
		if val == "" {
			continue
		}
		stringReader := strings.NewReader(val)
		csvReader := csv.NewReader(stringReader)
		v, err := csvReader.Read()
		if err != nil {
			log.Fatal(err)
		}
		result = append(result, v...)
	}
	if len(result) == 0 && len(vals) > 0 {
		return []string{}
	}
	return result
}
