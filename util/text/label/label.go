package label

import (
	"errors"
	"fmt"
	"maps"
	"strings"
)

const labelFieldDelimiter = "="

// LabelUpdate represents a single label update with value and deletion flag.
type LabelUpdate struct {
	Value  string
	Delete bool
}

// LabelMap holds label updates that can include deletions.
type LabelMap struct {
	Updates map[string]LabelUpdate
}

func NewLabelMap() *LabelMap {
	return &LabelMap{Updates: make(map[string]LabelUpdate)}
}

// Plain returns a simple map[string]string with only non-deleted labels.
func (lm *LabelMap) Plain() map[string]string {
	result := make(map[string]string)
	for k, v := range lm.Updates {
		if !v.Delete {
			result[k] = v.Value
		}
	}
	return result
}

// Parse populates the LabelMap from a slice of label strings.
// Supports deletion syntax (e.g., "key-").
func (lm *LabelMap) Parse(labels []string) error {
	if labels != nil {
		lm.Updates = make(map[string]LabelUpdate)
		for _, r := range labels {
			if strings.HasSuffix(r, "-") {
				key := strings.TrimSuffix(r, "-")
				if key == "" {
					return fmt.Errorf("invalid label deletion syntax: %s", r)
				}
				lm.Updates[key] = LabelUpdate{Value: "", Delete: true}
			} else {
				fields := strings.Split(r, labelFieldDelimiter)
				if len(fields) != 2 {
					return fmt.Errorf("labels should have key%svalue, but instead got: %s", labelFieldDelimiter, r)
				}
				lm.Updates[fields[0]] = LabelUpdate{Value: fields[1], Delete: false}
			}
		}
	}
	return nil
}

// Merge merges the updates in this LabelMap with existing labels, handling deletions.
func (lm *LabelMap) Merge(existing map[string]string) map[string]string {
	result := make(map[string]string)
	maps.Copy(result, existing)
	for k, v := range lm.Updates {
		if v.Delete {
			delete(result, k)
		} else {
			result[k] = v.Value
		}
	}
	return result
}

// Parse parses a slice of label strings into a simple map.
// For deletion support, use ParseToMap instead.
func Parse(labels []string) (map[string]string, error) {
	var selectedLabels map[string]string
	if labels != nil {
		selectedLabels = map[string]string{}
		for _, r := range labels {
			if strings.HasSuffix(r, "-") {
				return nil, errors.New("deletion syntax not supported in Parse(), use ParseToMap() instead")
			}
			fields := strings.Split(r, labelFieldDelimiter)
			if len(fields) != 2 {
				return nil, fmt.Errorf("labels should have key%svalue, but instead got: %s", labelFieldDelimiter, r)
			}
			selectedLabels[fields[0]] = fields[1]
		}
	}
	return selectedLabels, nil
}

// ParseLabelMap parses a slice of label strings into a LabelMap.
// Supports deletion syntax (e.g., "key-").
func ParseLabelMap(labels []string) (*LabelMap, error) {
	lm := NewLabelMap()
	err := lm.Parse(labels)
	if err != nil {
		return nil, err
	}
	return lm, nil
}
