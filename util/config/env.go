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

// slice per key: some flags (e.g. --header) are meant to be repeatable
// issue ref: https://github.com/argoproj/argo-cd/issues/24065
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
			key = strings.TrimPrefix(opt, "--")
			// pkg shellquota doesn't recognize `=` so that the opts in format `foo=bar` could not work.
			// issue ref: https://github.com/argoproj/argo-cd/issues/6822
			if idx := strings.Index(key, "="); idx >= 0 {
				flags[key[:idx]] = append(flags[key[:idx]], key[idx+1:])
				key = ""
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
	val, ok := flags[key]
	if ok && len(val) > 0 {
		return val[len(val)-1]
	}
	return fallback
}

func GetBoolFlag(key string) bool {
	return GetFlag(key, "false") == "true"
}

func GetIntFlag(key string, fallback int) int {
	val, ok := flags[key]
	if !ok || len(val) == 0 {
		return fallback
	}

	v, err := strconv.Atoi(val[len(val)-1])
	if err != nil {
		log.Fatal(err)
	}
	return v
}

func GetStringSliceFlag(key string, fallback []string) []string {
	val, ok := flags[key]
	if !ok {
		return fallback
	}

	result := []string{}
	for _, v := range val {
		if v == "" {
			continue
		}
		stringReader := strings.NewReader(v)
		csvReader := csv.NewReader(stringReader)
		parsed, err := csvReader.Read()
		if err != nil {
			log.Fatal(err)
		}
		result = append(result, parsed...)
	}
	return result
}
