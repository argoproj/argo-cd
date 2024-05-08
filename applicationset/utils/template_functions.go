package utils

import (
	"regexp"
	"strings"

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
func toYAML(v interface{}) (string, error) {
	data, err := yaml.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template.
		return "", err
	}
	return strings.TrimSuffix(string(data), "\n"), nil
}

// This has been copied from helm and may be removed as soon as it is retrofited in sprig
// fromYAML converts a YAML document into a map[string]interface{}.
//
// This is not a general-purpose YAML parser, and will not parse all valid
// YAML documents. Additionally, because its intended use is within templates
// it tolerates errors. It will insert the returned error message string into
// m["Error"] in the returned map.
func fromYAML(str string) (map[string]interface{}, error) {
	m := map[string]interface{}{}

	if err := yaml.Unmarshal([]byte(str), &m); err != nil {
		return nil, err
	}
	return m, nil
}

// This has been copied from helm and may be removed as soon as it is retrofited in sprig
// fromYAMLArray converts a YAML array into a []interface{}.
//
// This is not a general-purpose YAML parser, and will not parse all valid
// YAML documents. Additionally, because its intended use is within templates
// it tolerates errors. It will insert the returned error message string as
// the first and only item in the returned array.
func fromYAMLArray(str string) ([]interface{}, error) {
	a := []interface{}{}

	if err := yaml.Unmarshal([]byte(str), &a); err != nil {
		return nil, err
	}
	return a, nil
}
