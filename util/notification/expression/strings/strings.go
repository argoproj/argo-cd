package strings

import "strings"

func NewExprs() map[string]interface{} {
	return map[string]interface{}{
		"ReplaceAll": replaceAll,
	}
}

func replaceAll(s, old, new string) string {
	return strings.ReplaceAll(s, old, new)
}
