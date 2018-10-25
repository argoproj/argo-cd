package http

import (
	"fmt"
	"strings"
)

// MakeCookieMetadata generates a string representing a Web cookie.  Yum!
func MakeCookieMetadata(key, value string, flags ...string) string {
	components := []string{
		fmt.Sprintf("%s=%s", key, value),
	}
	components = append(components, flags...)
	return strings.Join(components, "; ")
}
