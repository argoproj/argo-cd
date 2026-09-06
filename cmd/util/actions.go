package util

import (
	"fmt"
	"strings"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
)

// ParseActionParameters parses a slice of "name=value" strings into ResourceActionParameters.
func ParseActionParameters(params []string) ([]*applicationpkg.ResourceActionParameters, error) {
	parsedParams := make([]*applicationpkg.ResourceActionParameters, 0, len(params))
	for _, param := range params {
		parts := strings.SplitN(param, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid parameter format %q: expected name=value", param)
		}
		name := parts[0]
		value := parts[1]
		if name == "" {
			return nil, fmt.Errorf("invalid parameter format %q: parameter name cannot be empty", param)
		}
		parsedParams = append(parsedParams, &applicationpkg.ResourceActionParameters{
			Name:  &name,
			Value: &value,
		})
	}
	return parsedParams, nil
}

// DuplicateActionParameterNames returns the names of any parameters specified more than once.
func DuplicateActionParameterNames(params []*applicationpkg.ResourceActionParameters) []string {
	seen := make(map[string]int, len(params))
	var duplicates []string
	for _, p := range params {
		if p == nil || p.Name == nil {
			continue
		}
		seen[*p.Name]++
		if seen[*p.Name] == 2 {
			duplicates = append(duplicates, *p.Name)
		}
	}
	return duplicates
}
