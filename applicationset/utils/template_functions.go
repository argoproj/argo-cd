package utils

import (
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"sigs.k8s.io/yaml"
)

// SanitizeName sanitizes the name in accordance with the below rules
// 1. contain no more than 253 characters
// 2. contain only lowercase alphanumeric characters, '-' or '.'
// 3. start and end with an alphanumeric character
func SanitizeName(name string) string {
	invalidDNSNameChars := regexp.MustCompile("[^-a-z0-9.]")
	maxDNSNameLength := 253

	name = strings.ToLower(name)
	name = invalidDNSNameChars.ReplaceAllString(name, "-")
	if len(name) > maxDNSNameLength {
		name = name[:maxDNSNameLength]
	}

	return strings.Trim(name, "-.")
}

// This has been copied from helm and may be removed as soon as it is retrofited in sprig
// toYAML takes an interface, marshals it to yaml, and returns a string. It will
// always return a string, even on marshal error (empty string).
//
// This is designed to be called from a template.
func toYAML(v any) (string, error) {
	data, err := yaml.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template.
		return "", err
	}
	return strings.TrimSuffix(string(data), "\n"), nil
}

// This has been copied from helm and may be removed as soon as it is retrofited in sprig
// fromYAML converts a YAML document into a map[string]any.
//
// This is not a general-purpose YAML parser, and will not parse all valid
// YAML documents. Additionally, because its intended use is within templates
// it tolerates errors. It will insert the returned error message string into
// m["Error"] in the returned map.
func fromYAML(str string) (map[string]any, error) {
	m := map[string]any{}

	if err := yaml.Unmarshal([]byte(str), &m); err != nil {
		return nil, err
	}
	return m, nil
}

// This has been copied from helm and may be removed as soon as it is retrofited in sprig
// fromYAMLArray converts a YAML array into a []any.
//
// This is not a general-purpose YAML parser, and will not parse all valid
// YAML documents. Additionally, because its intended use is within templates
// it tolerates errors. It will insert the returned error message string as
// the first and only item in the returned array.
func fromYAMLArray(str string) ([]any, error) {
	a := []any{}

	if err := yaml.Unmarshal([]byte(str), &a); err != nil {
		return nil, err
	}
	return a, nil
}

// This has been copied from helm but adapted to our needs as we have a more limited use case for
// https://github.com/helm/helm/blob/ee018608f6fbf381fac1bae9759164a65c6a0b1f/pkg/engine/engine.go#L153-L195
func tplFun(parent *template.Template) func(string, any) (string, error) {
	return func(tpl string, vals any) (string, error) {
		t, err := parent.Clone()
		if err != nil {
			return "", fmt.Errorf("cannot clone template: %w", err)
		}

		// We need a .New template, as template text which is just blanks
		// or comments after parsing out defines just adds new named
		// template definitions without changing the main template.
		// https://pkg.go.dev/text/template#Template.Parse
		// Use the parent's name for lack of a better way to identify the tpl
		// text string. (Maybe we could use a hash appended to the name?)
		t, err = t.New(parent.Name()).Parse(tpl)
		if err != nil {
			return "", fmt.Errorf("cannot parse template %q: %w", tpl, err)
		}

		var buf strings.Builder
		if err := t.Execute(&buf, vals); err != nil {
			return "", fmt.Errorf("error during tpl function execution for %q: %w", tpl, err)
		}

		return strings.ReplaceAll(buf.String(), "<no value>", ""), nil
	}
}
