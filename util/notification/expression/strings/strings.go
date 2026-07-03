package strings

import (
	"strings"
)

func NewExprs() map[string]any {
	return map[string]any{
		"ReplaceAll": replaceAll,
		"ToUpper":    toUpper,
		"ToLower":    toLower,
	}
}

func replaceAll(s, old, newV string) string {
	return strings.ReplaceAll(s, old, newV)
}

func toUpper(s string) string {
	return strings.ToUpper(s)
}

func toLower(s string) string {
	return strings.ToLower(s)
}
