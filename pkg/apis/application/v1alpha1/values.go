package v1alpha1

import (
	"encoding/json"
	"fmt"
	reflect "reflect"
	"strings"

	"github.com/ghodss/yaml"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

func (h *ApplicationSourceHelm) SetValuesString(value string) error {
	if value == "" {
		h.Values = nil
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
		h.Values = &runtime.RawExtension{Raw: data}
	}
	return nil
}

func (h *ApplicationSourceHelm) ValuesYAML() []byte {
	if h.Values == nil || h.Values.Raw == nil {
		return []byte{}
	}
	b, err := yaml.JSONToYAML(h.Values.Raw)
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
	if h.Values == nil || h.Values.Raw == nil {
		return ""
	}
	return strings.TrimSuffix(string(h.ValuesYAML()), "\n")
}
