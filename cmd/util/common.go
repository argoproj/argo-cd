package util

import (
	"fmt"
	"strings"
)

var (
	LogFormat string
	LogLevel  string
)

func GetNamespaceAndAppName(key string) (string, string, error) {
	var ns, appName string
	parts := strings.Split(key, "/")

	if len(parts) == 2 {
		ns = parts[0]
		appName = parts[1]
	} else if len(parts) == 1 {
		appName = parts[0]
	} else {
		return "", "", fmt.Errorf("APPNAME must be <namespace>/<appname> or <appname>, got: '%s' ", key)
	}
	return ns, appName, nil
}
