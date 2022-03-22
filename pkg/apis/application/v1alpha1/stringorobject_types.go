package v1alpha1

import (
	"encoding/json"
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

// +patchStrategy=replace
// +protobuf.options.(gogoproto.goproto_stringer)=false
// +kubebuilder:validation:Type=""
type StringOrObject struct {
	// Values as string if Raw == nil
	Values string `json:"-" protobuf:"bytes,1,opt,name=values"`
	// Raw is Values in raw format if Raw != nil
	Raw *runtime.RawExtension `json:"-" protobuf:"bytes,2,opt,name=raw"`
}

// IsEmpty returns true if the Object is empty
func (o StringOrObject) IsEmpty() bool {
	return o.Raw == nil && o.Values == ""
}

// Value returns either the value in Raw or Values
func (o StringOrObject) Value() []byte {
	if o.Raw != nil {
		b, err := yaml.JSONToYAML(o.Raw.Raw)
		if err != nil {
			return []byte{}
		}
		return b
	}
	return []byte(o.Values)
}

// MarshalJSON implements the json.Marshaller interface.
func (o StringOrObject) MarshalJSON() ([]byte, error) {
	if o.Raw == nil {
		if o.Values != "" {
			return json.Marshal(o.Values)
		}
		return []byte("null"), nil
	}
	return o.Raw.MarshalJSON()
}

// UnmarshalJSON implements the json.Unmarshaller interface.
func (o *StringOrObject) UnmarshalJSON(value []byte) error {
	var v interface{}
	if err := json.Unmarshal(value, &v); err != nil {
		// handle error
		return err
	}
	switch v.(type) {
	case map[string]interface{}:
		// it's an object
		o.Raw = &runtime.RawExtension{}
		return o.Raw.UnmarshalJSON(value)
	default:
		// default to string
		s, err := strconv.Unquote(string(value))
		if err != nil {
			return err
		}
		o.Values = s
	}

	return nil
}

// String formats the Object as a string
func (o *StringOrObject) String() string {
	if o == nil {
		return "<nil>"
	}
	if o.Raw == nil {
		return o.Values
	}

	b, err := yaml.JSONToYAML(o.Raw.Raw)
	if err != nil {
		return "<nil>"
	}
	return string(b)
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
