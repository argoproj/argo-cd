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
			flag := strings.TrimPrefix(opt, "--")
			if strings.Contains(flag, "=") {
				kv := strings.SplitN(flag, "=", 2)
				flags[kv[0]] = append(flags[kv[0]], kv[1])
				key = ""
				continue
			}
			key = flag
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
		return val[0]
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

	v, err := strconv.Atoi(val[0])
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

	if len(val) == 0 {
		return []string{}
	}

	// If there's a single value with commas, split it as CSV
	// (backwards compatibility with comma-separated headers)
	if len(val) == 1 {
		stringReader := strings.NewReader(val[0])
		csvReader := csv.NewReader(stringReader)
		v, err := csvReader.Read()
		if err != nil {
			log.Fatal(err)
		}
		return v
	}

	return val
}
