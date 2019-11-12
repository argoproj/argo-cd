package json

import (
	"encoding/json"
	"io"

	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
)

// JSONMarshaler is a type which satisfies the grpc-gateway Marshaler interface
type JSONMarshaler struct{}

// ContentType implements gwruntime.Marshaler.
func (j *JSONMarshaler) ContentType() string {
	return "application/json"
}

// Marshal implements gwruntime.Marshaler.
func (j *JSONMarshaler) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// NewDecoder implements gwruntime.Marshaler.
func (j *JSONMarshaler) NewDecoder(r io.Reader) gwruntime.Decoder {
	return json.NewDecoder(r)
}

// NewEncoder implements gwruntime.Marshaler.
func (j *JSONMarshaler) NewEncoder(w io.Writer) gwruntime.Encoder {
	return json.NewEncoder(w)
}

// Unmarshal implements gwruntime.Marshaler.
func (j *JSONMarshaler) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// https://github.com/ksonnet/ksonnet/blob/master/pkg/kubecfg/diff.go
func removeFields(config, live interface{}) interface{} {
	switch c := config.(type) {
	case map[string]interface{}:
		return RemoveMapFields(c, live.(map[string]interface{}))
	case []interface{}:
		return removeListFields(c, live.([]interface{}))
	default:
		return live
	}
}

// RemoveMapFields remove all non-existent fields in the live that don't exist in the config
func RemoveMapFields(config, live map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	for k, v1 := range config {
		v2, ok := live[k]
		if !ok {
			continue
		}
		if v2 != nil {
			v2 = removeFields(v1, v2)
		}
		result[k] = v2
	}
	return result
}

func removeListFields(config, live []interface{}) []interface{} {
	// If live is longer than config, then the extra elements at the end of the
	// list will be returned as-is so they appear in the diff.
	result := make([]interface{}, 0, len(live))
	for i, v2 := range live {
		if len(config) > i {
			if v2 != nil {
				v2 = removeFields(config[i], v2)
			}
			result = append(result, v2)
		} else {
			result = append(result, v2)
		}
	}
	return result
}

// MustMarshal is a convenience function to marshal an object successfully or panic
func MustMarshal(v interface{}) []byte {
	bytes, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return bytes
}
