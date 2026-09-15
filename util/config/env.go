package config

import (
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
	// pkg shellquota doesn't recognize `=` so that the opts in format `foo=bar` could not work.
	// issue ref: https://github.com/argoproj/argo-cd/issues/6822
	for k, v := range flags {
		if strings.Contains(k, "=") && len(v) > 0 && v[0] == "true" {
			kv := strings.SplitN(k, "=", 2)
			actualKey, actualValue := kv[0], kv[1]
			if _, ok := flags[actualKey]; !ok {
				flags[actualKey] = []string{actualValue}
			} else {
				flags[actualKey] = append(flags[actualKey], actualValue)
			}
			delete(flags, k)
		}
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
	if !ok || len(val) == 0 {
		return fallback
	}
	return val
}
