package v1alpha1

import (
	"reflect"
	"strings"
	"testing"
	"unicode"
)

// commonInitialisms follows Go CodeReviewComments (and Kubernetes API conventions
// where they agree). When adding a field whose name contains a new acronym, add it
// here so the casing check catches regressions.
//
// See: https://go.dev/wiki/CodeReviewComments#initialisms
var commonInitialisms = map[string]bool{
	"API":   true,
	"DB":    true,
	"GRPC":  true,
	"HTTP":  true,
	"HTTPS": true,
	"ID":    true,
	"IP":    true,
	"JSON":  true,
	"JQ":    true,
	"OCI":   true,
	"OIDC":  true,
	"OTLP":  true,
	"PKCE":  true,
	"QPS":   true,
	"RBAC":  true,
	"SCM":   true,
	"SSA":   true,
	"SSL":   true,
	"TCP":   true,
	"TLS":   true,
	"TTL":   true,
	"TXT":   true,
	"UID":   true,
	"URI":   true,
	"URL":   true,
	"UUID":  true,
}

func TestExportedFieldInitialisms(t *testing.T) {
	seen := map[reflect.Type]bool{}
	var bad []string
	checkType(t, reflect.TypeOf(ArgoCDConfiguration{}), seen, &bad)
	if len(bad) > 0 {
		t.Fatalf("initialism casing violations (Go/K8s style — see CONTRIBUTING.md):\n  %s",
			strings.Join(bad, "\n  "))
	}
}

func checkType(t *testing.T, typ reflect.Type, seen map[reflect.Type]bool, bad *[]string) {
	t.Helper()
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return
	}
	if seen[typ] {
		return
	}
	seen[typ] = true

	for i := 0; i < typ.NumField(); i++ {
		sf := typ.Field(i)
		if sf.PkgPath != "" {
			continue // unexported
		}
		if sugg := suggestInitialismFix(sf.Name); sugg != "" && sugg != sf.Name {
			*bad = append(*bad, typ.String()+"."+sf.Name+" → want "+sugg)
		}
		ft := sf.Type
		for ft.Kind() == reflect.Pointer || ft.Kind() == reflect.Slice || ft.Kind() == reflect.Array || ft.Kind() == reflect.Map {
			if ft.Kind() == reflect.Map {
				checkType(t, ft.Key(), seen, bad)
			}
			ft = ft.Elem()
		}
		if ft.PkgPath() == typ.PkgPath() || strings.HasPrefix(ft.PkgPath(), "github.com/crenshaw-dev/argocd-config/api/") {
			checkType(t, ft, seen, bad)
		}
	}
}

// suggestInitialismFix returns the correctly-cased name when an initialism
// appears with wrong casing, or "" if the name is fine / has no known initialisms.
func suggestInitialismFix(name string) string {
	// Split into CamelCase words (runs of capitals count as one word except the last capital
	// starts the next word when followed by lowercase — standard Go splitting).
	words := splitCamel(name)
	changed := false
	for i, w := range words {
		upper := strings.ToUpper(w)
		if !commonInitialisms[upper] {
			continue
		}
		// Initialism must be all-caps (URL, not Url / url when mid-identifier).
		if w != upper {
			words[i] = upper
			changed = true
		}
	}
	if !changed {
		return ""
	}
	return strings.Join(words, "")
}

func splitCamel(name string) []string {
	if name == "" {
		return nil
	}
	var words []string
	runes := []rune(name)
	start := 0
	for i := 1; i < len(runes); i++ {
		prev, cur := runes[i-1], runes[i]
		// Boundary: lower→Upper, or Upper→Upper+lower (HTTPServer → HTTP, Server)
		if unicode.IsUpper(cur) {
			if !unicode.IsUpper(prev) {
				words = append(words, string(runes[start:i]))
				start = i
			} else if i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				words = append(words, string(runes[start:i]))
				start = i
			}
		}
	}
	words = append(words, string(runes[start:]))
	return words
}
