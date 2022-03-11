package v1alpha1

import (
	"encoding/json"
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"
)

// +kubebuilder:validation:Type=""
type Object struct {
	// Values as string if Raw == nil
	Values string `json:"-" protobuf:"bytes,1,opt,name=values"`
	// Raw is Values in raw format if Raw != nil
	Raw *runtime.RawExtension `json:"-" protobuf:"bytes,2,opt,name=raw"`
}

// IsEmpty returns true if the Object is empty
func (o Object) IsEmpty() bool {
	return o.Raw == nil && o.Values == ""
}

// Value returns the value
func (o Object) Value() []byte {
	if o.Raw != nil {
		return o.Raw.Raw
	}
	return []byte(o.Values)
}

// MarshalJSON implements the json.Marshaller interface.
func (o Object) MarshalJSON() ([]byte, error) {
	if o.Raw == nil {
		if o.Values != "" {
			return json.Marshal(o.Values)
		}
		return []byte("null"), nil
	}
	return o.Raw.MarshalJSON()
}

// UnmarshalJSON implements the json.Unmarshaller interface.
func (o *Object) UnmarshalJSON(value []byte) error {
	var v interface{}
	if err := json.Unmarshal([]byte(value), &v); err != nil {
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

// OpenAPISchemaType is used by the kube-openapi generator when constructing
// the OpenAPI spec of this type.
//
// See: https://github.com/kubernetes/kube-openapi/tree/master/pkg/generators
func (_ Object) OpenAPISchemaType() []string { return []string{"string"} }

// OpenAPISchemaFormat is used by the kube-openapi generator when constructing
// the OpenAPI spec of this type.
func (_ Object) OpenAPISchemaFormat() string { return "" }
