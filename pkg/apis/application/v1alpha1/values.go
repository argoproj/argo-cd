package v1alpha1

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
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

// String returns a human-readable representation of ApplicationSourceHelm.
// It replaces the auto-generated stringer (suppressed via the
// +protobuf.options.(gogoproto.goproto_stringer)=false marker on the type)
// so that ValuesObject is rendered as YAML rather than as a raw byte-array
// dump, which previously flooded logs (see #18342).
//
// The format mirrors what gogo-proto produces for every other field, so the
// only observable difference is the ValuesObject rendering. If new fields are
// added to ApplicationSourceHelm, they must be added here as well — there is
// no codegen for this method.
func (h *ApplicationSourceHelm) String() string {
	if h == nil {
		return "nil"
	}
	repeatedStringForParameters := "[]HelmParameter{"
	var repeatedStringForParametersSb71 strings.Builder
	for _, f := range h.Parameters {
		repeatedStringForParametersSb71.WriteString(strings.Replace(strings.Replace(f.String(), "HelmParameter", "HelmParameter", 1), `&`, ``, 1) + ",")
	}
	repeatedStringForParameters += repeatedStringForParametersSb71.String()
	repeatedStringForParameters += "}"
	repeatedStringForFileParameters := "[]HelmFileParameter{"
	var repeatedStringForFileParametersSb76 strings.Builder
	for _, f := range h.FileParameters {
		repeatedStringForFileParametersSb76.WriteString(strings.Replace(strings.Replace(f.String(), "HelmFileParameter", "HelmFileParameter", 1), `&`, ``, 1) + ",")
	}
	repeatedStringForFileParameters += repeatedStringForFileParametersSb76.String()
	repeatedStringForFileParameters += "}"

	valuesObjectStr := "nil"
	if h.ValuesObject != nil {
		// Render the JSON-encoded RawExtension as YAML instead of the
		// default %v formatting of []byte (which produces "[123 34 ...]").
		valuesObjectStr = "&runtime.RawExtension{" + h.ValuesString() + "}"
	}

	return strings.Join([]string{
		`&ApplicationSourceHelm{`,
		`ValueFiles:` + fmt.Sprintf("%v", h.ValueFiles) + `,`,
		`Parameters:` + repeatedStringForParameters + `,`,
		`ReleaseName:` + h.ReleaseName + `,`,
		`Values:` + h.Values + `,`,
		`FileParameters:` + repeatedStringForFileParameters + `,`,
		`Version:` + h.Version + `,`,
		`PassCredentials:` + strconv.FormatBool(h.PassCredentials) + `,`,
		`IgnoreMissingValueFiles:` + strconv.FormatBool(h.IgnoreMissingValueFiles) + `,`,
		`SkipCrds:` + strconv.FormatBool(h.SkipCrds) + `,`,
		`ValuesObject:` + valuesObjectStr + `,`,
		`Namespace:` + h.Namespace + `,`,
		`KubeVersion:` + h.KubeVersion + `,`,
		`APIVersions:` + fmt.Sprintf("%v", h.APIVersions) + `,`,
		`SkipTests:` + strconv.FormatBool(h.SkipTests) + `,`,
		`SkipSchemaValidation:` + strconv.FormatBool(h.SkipSchemaValidation) + `,`,
		`}`,
	}, "")
}

func (h *ApplicationSourceHelm) ValuesString() string {
	if h.ValuesObject == nil || h.ValuesObject.Raw == nil {
		return h.Values
	}
	return strings.TrimSuffix(string(h.ValuesYAML()), "\n")
}
