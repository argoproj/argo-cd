package label

import (
	"fmt"
	"strings"
)

const (
	labelFieldDelimiter = "="
	deletionMarker      = "\x00DELETE\x00"
)

func Parse(labels []string) (map[string]string, error) {
	var selectedLabels map[string]string
	if labels != nil {
		selectedLabels = map[string]string{}
		for _, r := range labels {
			if strings.HasSuffix(r, "-") {
				key := strings.TrimSuffix(r, "-")
				if key == "" {
					return nil, fmt.Errorf("invalid label deletion syntax: %s", r)
				}
				selectedLabels[key] = deletionMarker
			} else {
				fields := strings.Split(r, labelFieldDelimiter)
				if len(fields) != 2 {
					return nil, fmt.Errorf("labels should have key%svalue, but instead got: %s", labelFieldDelimiter, r)
				}
				selectedLabels[fields[0]] = fields[1]
			}
		}
	}
	return selectedLabels, nil
}

func Merge(existing, updates map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range existing {
		result[k] = v
	}
	for k, v := range updates {
		if v == deletionMarker {
			delete(result, k)
		} else {
			result[k] = v
		}
	}
	return result
}
