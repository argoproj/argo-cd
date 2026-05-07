package v1alpha1

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
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
			return fmt.Errorf("failed converting yaml to json: %w", err)
		}
		var v any
		if err := json.Unmarshal(data, &v); err != nil {
			return fmt.Errorf("failed to unmarshal json: %w", err)
		}
		switch v.(type) {
		case string:
		case map[string]any:
		default:
			return fmt.Errorf("invalid type %q", reflect.TypeOf(v))
		}
		h.ValuesObject = &runtime.RawExtension{Raw: data}
		h.Values = ""
	}
	return nil
}

func (h *ApplicationSourceHelm) ValuesYAML() []byte {
	if h.ValuesObject == nil || h.ValuesObject.Raw == nil {
		return []byte(h.Values)
	}
	b, err := yaml.JSONToYAML(h.ValuesObject.Raw)
	if err != nil {
		// This should be impossible, because rawValue isn't set directly.
		return []byte{}
	}
	return b
}

func (h *ApplicationSourceHelm) ValuesIsEmpty() bool {
	return len(h.ValuesYAML()) == 0
}

// LogString returns a human-readable string representation of ApplicationSourceHelm
// suitable for logging. It renders ValuesObject as its YAML equivalent instead of
// raw bytes, preventing binary data from flooding log output.
func (h *ApplicationSourceHelm) LogString() string {
	if h == nil {
		return "nil"
	}
	if h.ValuesObject == nil {
		return h.String()
	}
	helmCopy := *h
	helmCopy.Values = h.ValuesString()
	helmCopy.ValuesObject = nil
	return helmCopy.String()
}

// LogString returns a human-readable string representation of ApplicationSource
// suitable for logging. When the source has Helm ValuesObject set, it renders
// the raw bytes as YAML instead of printing them as integers.
func (s *ApplicationSource) LogString() string {
	if s == nil {
		return "nil"
	}
	if s.Helm == nil || s.Helm.ValuesObject == nil {
		return s.String()
	}
	sourceCopy := *s
	helmCopy := *s.Helm
	sourceCopy.Helm = &helmCopy
	sourceCopy.Helm.Values = s.Helm.ValuesString()
	sourceCopy.Helm.ValuesObject = nil
	return sourceCopy.String()
}

func (h *ApplicationSourceHelm) ValuesString() string {
	if h.ValuesObject == nil || h.ValuesObject.Raw == nil {
		return h.Values
	}
	return strings.TrimSuffix(string(h.ValuesYAML()), "\n")
}
