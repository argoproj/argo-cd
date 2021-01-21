package text

import (
	"strings"
	"unicode/utf8"
)

// truncates messages to n characters
func Trunc(message string, n int) string {
	if utf8.RuneCountInString(message) > n {
		return string([]rune(message)[0:n-3]) + "..."
	}
	return message
}

func SemVer(s string) string {
	return strings.ReplaceAll(s, "+", "")
}
