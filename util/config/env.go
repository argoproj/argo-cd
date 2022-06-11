package config

import (
	"errors"
	"os"
	"strings"

	"github.com/kballard/go-shellquote"
	log "github.com/sirupsen/logrus"
)

var flags map[string]string

func init() {
	err := loadFlags()
	if err != nil {
		log.Fatal(err)
	}
}

func loadFlags() error {
	flags = make(map[string]string)

	opts, err := shellquote.Split(os.Getenv("ARGOCD_OPTS"))
	if err != nil {
		return err
	}

	var key string
	for _, opt := range opts {
		if strings.HasPrefix(opt, "--") {
			if key != "" {
				flags[key] = "true"
			}
			key = strings.TrimPrefix(opt, "--")
		} else if key != "" {
			flags[key] = opt
			key = ""
		} else {
			return errors.New("ARGOCD_OPTS invalid at '" + opt + "'")
		}
	}
	if key != "" {
		flags[key] = "true"
	}
	// pkg shellquota doesn't recognize `=` so that the opts in format `foo=bar` could not work.
	// issue ref: https://github.com/argoproj/argo-cd/issues/6822
	for k, v := range flags {
		if strings.Contains(k, "=") && strings.Count(k, "=") == 1 && v == "true" {
			kv := strings.Split(k, "=")
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
		return val
	}
	return fallback
}

func GetBoolFlag(key string) bool {
	return GetFlag(key, "false") == "true"
}
