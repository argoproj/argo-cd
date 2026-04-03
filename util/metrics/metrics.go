package metrics

import (
	"fmt"
	"regexp"
)

// Prometheus invalid labels, more info: https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels.
var invalidPromLabelChars = regexp.MustCompile(`[^a-zA-Z0-9_]`)

func NormalizeLabels(prefix string, labels []string) []string {
	results := []string{}
	for _, label := range labels {
		// prometheus labels don't accept dash in their name
		curr := invalidPromLabelChars.ReplaceAllString(label, "_")
		result := fmt.Sprintf("%s_%s", prefix, curr)
		results = append(results, result)
	}
	return results
}
