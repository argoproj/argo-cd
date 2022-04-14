package v1alpha1

import (
	"encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
	"sigs.k8s.io/yaml"
)

var StringOrObjectInvalidTypeError = fmt.Errorf("type StringOrObject must be either a string or an object")

// +patchStrategy=replace
// +protobuf.options.(gogoproto.goproto_stringer)=false
// +kubebuilder:validation:Type=""
type StringOrObject struct {
	stringValue string                `json:"-" protobuf:"bytes,1,opt,name=values"`
	rawValue    *runtime.RawExtension `json:"-" protobuf:"bytes,2,opt,name=raw"`
}

func NewStringOrObjectFromString(value string) StringOrObject {
	return StringOrObject{stringValue: value}
}

func NewStringOrObjectFromYAML(yamlBytes []byte) (*StringOrObject, error) {
	stringOrObject := &StringOrObject{}
	err := stringOrObject.SetYAMLValue(yamlBytes)
	if err != nil {
		return nil, err
	}
	return stringOrObject, nil
}

func (o *StringOrObject) SetStringValue(value string) {
	o.rawValue = nil
	o.stringValue = value
}

func (o *StringOrObject) SetYAMLValue(yamlBytes []byte) error {
	data, err := yaml.YAMLToJSON(yamlBytes)
	if err != nil {
		return fmt.Errorf("failed to set YAML value on StringOrObject: %w", err)
	}
	o.rawValue = &runtime.RawExtension{Raw: data}
	return nil
}

// IsEmpty returns true if the Object is empty
func (o StringOrObject) IsEmpty() bool {
	return len(o.YAML()) == 0
}

// YAML returns the value marshalled to YAML
func (o StringOrObject) YAML() []byte {
	if o.rawValue != nil {
		b, err := yaml.JSONToYAML(o.rawValue.Raw)
		if err != nil {
			// This should be impossible, because rawValue isn't set directly.
			return []byte{}
		}
		return b
	}
	return []byte(o.stringValue)
}

// MarshalJSON implements the json.Marshaller interface.
func (o StringOrObject) MarshalJSON() ([]byte, error) {
	if o.rawValue == nil {
		return json.Marshal(o.stringValue)
	}
	return o.rawValue.MarshalJSON()
}

// UnmarshalJSON implements the json.Unmarshaller interface.
func (o *StringOrObject) UnmarshalJSON(value []byte) error {
	var v interface{}
	if err := json.Unmarshal(value, &v); err != nil {
		return err
	}
	switch v.(type) {
	case string:
		o.stringValue = v.(string)
	case map[string]interface{}:
		// it's an object
		o.rawValue = &runtime.RawExtension{}
		return o.rawValue.UnmarshalJSON(value)
	default:
		return fmt.Errorf("invalid type %q: %w", reflect.TypeOf(v), StringOrObjectInvalidTypeError)
	}

	return nil
}

// String formats the Object as a string
func (o *StringOrObject) String() string {
	if o == nil {
		return "<nil>"
	}
	return string(o.YAML())
}

// ToUnstructured implements the value.UnstructuredConverter interface.
func (o StringOrObject) ToUnstructured() interface{} {
	return o.String()
}

// OpenAPISchemaType is used by the kube-openapi generator when constructing
// the OpenAPI spec of this type.
//
// See: https://github.com/kubernetes/kube-openapi/tree/master/pkg/generators
func (_ StringOrObject) OpenAPISchemaType() []string { return []string{"string"} }

// OpenAPISchemaFormat is used by the kube-openapi generator when constructing
// the OpenAPI spec of this type.
func (_ StringOrObject) OpenAPISchemaFormat() string { return "" }
