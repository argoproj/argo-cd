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
				processFlagKey(flags, key)
			}
			key = strings.TrimPrefix(opt, "--")
		case key != "":
			flags[key] = append(flags[key], opt)
			key = ""
		default:
			return errors.New("ARGOCD_OPTS invalid at '" + opt + "'")
		}
	}
	if key != "" {
		processFlagKey(flags, key)
	}
	return nil
}

func processFlagKey(flags map[string][]string, key string) {
	// pkg shellquota doesn't recognize `=` so that the opts in format `foo=bar` could not work.
	// issue ref: https://github.com/argoproj/argo-cd/issues/6822
	if strings.Contains(key, "=") {
		kv := strings.SplitN(key, "=", 2)
		actualKey, actualValue := kv[0], kv[1]
		flags[actualKey] = append(flags[actualKey], actualValue)
	} else {
		if _, ok := flags[key]; !ok {
			// empty slice means bool flag.
			flags[key] = []string{}
		}
	}
}

func getFlag(key string) (string, bool) {
	val, ok := flags[key]
	if !ok {
		return "", false
	}
	if len(val) == 0 {
		// return "true" string for bool flag.
		return "true", true
	}
	// last flag wins for backward compatibility.
	return val[len(val)-1], true
}

func GetFlag(key, fallback string) string {
	val, ok := getFlag(key)
	if !ok {
		return fallback
	}
	return val
}

func GetBoolFlag(key string) bool {
	_, ok := flags[key]
	return ok
}

func GetIntFlag(key string, fallback int) int {
	val, ok := getFlag(key)
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
	val, ok := flags[key]
	if !ok {
		return fallback
	}

	s := []string{}
	for _, v := range val {
		// use csv reader to parse quoted string with comma
		csvReader := csv.NewReader(strings.NewReader(v))
		v, err := csvReader.Read()
		if err != nil {
			log.Fatal(err)
		}
		s = append(s, v...)
	}
	return s
}
