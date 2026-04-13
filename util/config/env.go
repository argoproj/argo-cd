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
			}
			key = strings.TrimPrefix(opt, "--")
			// pkg shellquota doesn't recognize `=` so that the opts in format `foo=bar` could not work.
			// issue ref: https://github.com/argoproj/argo-cd/issues/6822
			if strings.Contains(key, "=") {
				kv := strings.SplitN(key, "=", 2)
				actualKey, actualValue := kv[0], kv[1]
				flags[actualKey] = append(flags[actualKey], actualValue)
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
	vals, ok := flags[key]
	if !ok {
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
	if len(result) == 0 && len(vals) > 0 && vals[len(vals)-1] == "" {
		return []string{}
	}
	return result
}
