package v1alpha1

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"sigs.k8s.io/yaml"
)

// Set the ValuesObject property to the json representation of the yaml contained in value
// Remove Values property if present
func (h *ApplicationSourceHelm) SetValuesString(value string) error {
	if value == "" {
		h.ValuesObject = nil
		h.Values = ""
	} else {
		data, err := yaml.YAMLToJSON([]byte(value))
		if err != nil {
			return fmt.Errorf("failed converting yaml to json: %v", err)
		}
		var v interface{}
		if err := json.Unmarshal(data, &v); err != nil {
			return fmt.Errorf("failed to unmarshal json: %v", err)
		}
		switch v.(type) {
		case string:
		case map[string]interface{}:
		default:
			return fmt.Errorf("invalid type %q", reflect.TypeOf(v))
		}
		h.ValuesObject = data
		h.Values = ""
	}
	return nil
}

func (h *ApplicationSourceHelm) ValuesYAML() []byte {
	if h.ValuesObject == nil {
		return []byte(h.Values)
	}
	b, err := yaml.JSONToYAML(h.ValuesObject)
	if err != nil {
		// This should be impossible, because rawValue isn't set directly.
		return []byte{}
	}
	return b
}

func (h *ApplicationSourceHelm) ValuesIsEmpty() bool {
	return len(h.ValuesYAML()) == 0
}

func (h *ApplicationSourceHelm) ValuesString() string {
	if h.ValuesObject == nil {
		return h.Values
	}
	return strings.TrimSuffix(string(h.ValuesYAML()), "\n")
}
