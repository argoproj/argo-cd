package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testStruct struct {
	Metadata *testStruct `json:"metadata,omitempty"`
	Spec     *testStruct `json:"spec,omitempty"`
	Source   *testStruct `json:"source,omitempty"`
	Status   *testStruct `json:"status,omitempty"`

	Name    string `json:"name,omitempty"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message,omitempty"`
}

var (
	testVal = testStruct{
		Metadata: &testStruct{Name: "test"},
		Spec: &testStruct{
			Source: &testStruct{
				Path: "test_path",
			},
		},
		Status: &testStruct{
			Message: "Failed",
		},
	}
)

func TestMarshalerIncludeFields(t *testing.T) {
	m := messageMarshaler{fields: map[string]interface{}{
		"metadata.name": true,
		"spec.source":   true,
	}, exclude: false}

	out, err := m.Marshal(testVal)

	assert.Nil(t, err)
	assert.Equal(t, `{"metadata":{"name":"test"},"spec":{"source":{"path":"test_path"}}}`, string(out))
}

func TestMarshalerExcludeFields(t *testing.T) {
	m := messageMarshaler{fields: map[string]interface{}{
		"metadata.name": true,
	}, exclude: true}

	out, err := m.Marshal(testVal)

	assert.Nil(t, err)
	assert.Equal(t, `{"metadata":{},"spec":{"source":{"path":"test_path"}},"status":{"message":"Failed"}}`, string(out))
}

func TestMarshalerSSE(t *testing.T) {
	m := messageMarshaler{isSSE: true}

	out, err := m.Marshal(testVal)

	assert.Nil(t, err)
	assert.Equal(t, `data: {"metadata":{"name":"test"},"spec":{"source":{"path":"test_path"}},"status":{"message":"Failed"}} 

`, string(out))
}
