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

// NormalizeValuesObject rewrites ValuesObject.Raw into a canonical JSON form so that two semantically
// equal values also compare equal at the byte level. This is required because the JSON produced by the
// Kubernetes API server does not HTML-escape characters such as '&', '<' and '>', whereas Go's
// encoding/json (used when a request source is parsed) does. Without normalization, a byte-wise comparison
// such as reflect.DeepEqual would report a spurious difference between an otherwise identical stored source
// and the source supplied in a request.
func (h *ApplicationSourceHelm) NormalizeValuesObject() {
	if h == nil || h.ValuesObject == nil || h.ValuesObject.Raw == nil {
		return
	}
	var v any
	if err := json.Unmarshal(h.ValuesObject.Raw, &v); err != nil {
		// Leave the raw value untouched if it is not valid JSON; it should never happen because
		// ValuesObject is only ever set from JSON, but we must not panic or corrupt the value.
		return
	}
	normalized, err := json.Marshal(v)
	if err != nil {
		return
	}
	h.ValuesObject.Raw = normalized
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
// It replaces the suppressed auto-generated stringer so that
// ValuesObject is rendered as YAML rather than as a raw byte-array
// See https://github.com/argoproj/argo-cd/issues/18342
//
// The format mirrors what gogo-proto produces for every other field, so the
// only observable difference is the ValuesObject rendering. If new fields are
// added to ApplicationSourceHelm, they must be added here as well.
func (h *ApplicationSourceHelm) String() string {
	if h == nil {
		return "nil"
	}

	var parametersBuilder strings.Builder
	parametersBuilder.WriteString("[]HelmParameter{")
	for _, f := range h.Parameters {
		parametersBuilder.WriteString(strings.Replace(f.String(), `&`, ``, 1))
		parametersBuilder.WriteString(",")
	}
	parametersBuilder.WriteString("}")

	var fileParametersBuilder strings.Builder
	fileParametersBuilder.WriteString("[]HelmFileParameter{")
	for _, f := range h.FileParameters {
		fileParametersBuilder.WriteString(strings.Replace(f.String(), `&`, ``, 1))
		fileParametersBuilder.WriteString(",")
	}
	fileParametersBuilder.WriteString("}")

	valuesObjectStr := "nil"
	if h.ValuesObject != nil {
		// Render the JSON-encoded RawExtension as YAML instead of the
		// default %v formatting of []byte (which produces "[123 34 ...]").
		valuesObjectStr = "&runtime.RawExtension{" + h.ValuesString() + "}"
	}

	return strings.Join([]string{
		`&ApplicationSourceHelm{`,
		`ValueFiles:` + fmt.Sprintf("%v", h.ValueFiles) + `,`,
		`Parameters:` + parametersBuilder.String() + `,`,
		`ReleaseName:` + h.ReleaseName + `,`,
		`Values:` + h.Values + `,`,
		`FileParameters:` + fileParametersBuilder.String() + `,`,
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
