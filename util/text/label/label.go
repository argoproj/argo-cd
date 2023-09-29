package label

import (
	"fmt"
	"strings"
)

const labelFieldDelimiter = "="

func Parse(labels []string) (map[string]string, error) {
	var selectedLabels map[string]string
	if labels != nil {
		selectedLabels = map[string]string{}
		for _, r := range labels {
			fields := strings.Split(r, labelFieldDelimiter)
			if len(fields) != 2 {
				return nil, fmt.Errorf("labels should have key%svalue, but instead got: %s", labelFieldDelimiter, r)
			}
			selectedLabels[fields[0]] = fields[1]
		}
	}
	return selectedLabels, nil
}
