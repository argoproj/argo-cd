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

// Map holds label updates that can include deletions.
type Map map[string]LabelUpdate

func NewMap() Map {
	return make(Map)
}

// Plain returns a simple map[string]string with only non-deleted labels.
func (lm Map) Plain() map[string]string {
	result := make(map[string]string)
	for k, v := range lm {
		if !v.Delete {
			result[k] = v.Value
		}
	}
	return result
}

// Parse populates the Map from a slice of label strings.
// Supports deletion syntax (e.g., "key-").
func (lm Map) Parse(labels []string) error {
	for _, r := range labels {
		if strings.HasSuffix(r, "-") {
			key := strings.TrimSuffix(r, "-")
			if key == "" {
				return fmt.Errorf("invalid label deletion syntax: %s", r)
			}
			lm[key] = LabelUpdate{Value: "", Delete: true}
		} else {
			fields := strings.Split(r, labelFieldDelimiter)
			if len(fields) != 2 {
				return fmt.Errorf("labels should have key%svalue, but instead got: %s", labelFieldDelimiter, r)
			}
			lm[fields[0]] = LabelUpdate{Value: fields[1], Delete: false}
		}
	}
	return nil
}

// Merge merges the updates in this Map with existing labels, handling deletions.
// Values for entries which already exist will be updated or deleted.
func (lm Map) Merge(existing map[string]string) map[string]string {
	result := make(map[string]string)
	maps.Copy(result, existing)
	for k, v := range lm {
		if v.Delete {
			delete(result, k)
		} else {
			result[k] = v.Value
		}
	}
	return result
}

// Parse parses a slice of label strings into a simple map.
// For deletion support, use ParseMap instead.
func Parse(labels []string) (map[string]string, error) {
	selectedLabels := map[string]string{}
	for _, r := range labels {
		if strings.HasSuffix(r, "-") {
			return nil, errors.New("deletion syntax not supported in Parse(), use ParseMap() instead")
		}
		fields := strings.Split(r, labelFieldDelimiter)
		if len(fields) != 2 {
			return nil, fmt.Errorf("labels should have key%svalue, but instead got: %s", labelFieldDelimiter, r)
		}
		selectedLabels[fields[0]] = fields[1]
	}
	return selectedLabels, nil
}

// ParseMap parses a slice of label strings into a label.Map
// Supports deletion syntax (e.g., "key-").
func ParseMap(labels []string) (Map, error) {
	lm := NewMap()
	err := lm.Parse(labels)
	if err != nil {
		return nil, err
	}
	return lm, nil
}
