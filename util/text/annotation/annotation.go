package annotation

import (
	"strings"
)

const annotationFieldDelimiter = "="

func Parse(annotations []string) map[string]string {
	var selectedAnnotations map[string]string
	if annotations != nil {
		selectedAnnotations = map[string]string{}
		for _, r := range annotations {
			key, value, _ := strings.Cut(r, annotationFieldDelimiter)
			selectedAnnotations[key] = value
		}
	}
	return selectedAnnotations
}
