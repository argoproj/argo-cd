package strings

import (
	"strings"
)

func NewExprs() map[string]interface{} {
	return map[string]interface{}{
		"ReplaceAll": replaceAll,
		"ToUpper":    toUpper,
		"ToLower":    toLower,
	}
}

func replaceAll(s, old, new string) string {
	return strings.ReplaceAll(s, old, new)
}

func toUpper(s string) string {
	return strings.ToUpper(s)
}

func toLower(s string) string {
	return strings.ToLower(s)
}
